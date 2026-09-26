ALTER TABLE public.plannings ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.planning_members ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.planning_activity ENABLE ROW LEVEL SECURITY;

CREATE POLICY plannings_select_member
ON public.plannings
FOR SELECT
USING (
  EXISTS (
    SELECT 1
    FROM public.planning_members pm
    WHERE pm.planning_id = plannings.id
      AND pm.user_id = auth.uid()
  )
);

CREATE POLICY plannings_insert_owner
ON public.plannings
FOR INSERT
WITH CHECK (owner_user_id = auth.uid());

CREATE POLICY plannings_update_manager
ON public.plannings
FOR UPDATE
USING (
  EXISTS (
    SELECT 1
    FROM public.planning_members pm
    WHERE pm.planning_id = plannings.id
      AND pm.user_id = auth.uid()
      AND pm.role IN ('owner', 'admin')
  )
)
WITH CHECK (
  EXISTS (
    SELECT 1
    FROM public.planning_members pm
    WHERE pm.planning_id = plannings.id
      AND pm.user_id = auth.uid()
      AND pm.role IN ('owner', 'admin')
  )
);

CREATE POLICY plannings_delete_owner
ON public.plannings
FOR DELETE
USING (
  EXISTS (
    SELECT 1
    FROM public.planning_members pm
    WHERE pm.planning_id = plannings.id
      AND pm.user_id = auth.uid()
      AND pm.role = 'owner'
  )
);

CREATE POLICY planning_members_select_member
ON public.planning_members
FOR SELECT
USING (
  EXISTS (
    SELECT 1
    FROM public.planning_members actor
    WHERE actor.planning_id = planning_members.planning_id
      AND actor.user_id = auth.uid()
  )
);

CREATE POLICY planning_members_insert_manager_or_owner
ON public.planning_members
FOR INSERT
WITH CHECK (
  (role = 'owner' AND user_id = auth.uid())
  OR EXISTS (
    SELECT 1
    FROM public.planning_members actor
    WHERE actor.planning_id = planning_members.planning_id
      AND actor.user_id = auth.uid()
      AND actor.role IN ('owner', 'admin')
  )
);

CREATE POLICY planning_members_update_manager
ON public.planning_members
FOR UPDATE
USING (
  EXISTS (
    SELECT 1
    FROM public.planning_members actor
    WHERE actor.planning_id = planning_members.planning_id
      AND actor.user_id = auth.uid()
      AND actor.role IN ('owner', 'admin')
  )
)
WITH CHECK (
  EXISTS (
    SELECT 1
    FROM public.planning_members actor
    WHERE actor.planning_id = planning_members.planning_id
      AND actor.user_id = auth.uid()
      AND actor.role IN ('owner', 'admin')
  )
);

CREATE POLICY planning_members_delete_manager
ON public.planning_members
FOR DELETE
USING (
  EXISTS (
    SELECT 1
    FROM public.planning_members actor
    WHERE actor.planning_id = planning_members.planning_id
      AND actor.user_id = auth.uid()
      AND actor.role IN ('owner', 'admin')
  )
);

CREATE POLICY planning_activity_select_member
ON public.planning_activity
FOR SELECT
USING (
  EXISTS (
    SELECT 1
    FROM public.planning_members pm
    WHERE pm.planning_id = planning_activity.planning_id
      AND pm.user_id = auth.uid()
  )
);

CREATE POLICY planning_activity_insert_member
ON public.planning_activity
FOR INSERT
WITH CHECK (
  EXISTS (
    SELECT 1
    FROM public.planning_members pm
    WHERE pm.planning_id = planning_activity.planning_id
      AND pm.user_id = auth.uid()
  )
  AND (actor_user_id IS NULL OR actor_user_id = auth.uid())
);