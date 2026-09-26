CREATE INDEX IF NOT EXISTS idx_plannings_group_created_at
  ON public.plannings (group_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_plannings_owner_created_at
  ON public.plannings (owner_user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_plannings_status_starts_at
  ON public.plannings (status, starts_at);

CREATE INDEX IF NOT EXISTS idx_plannings_group_active_starts_at
  ON public.plannings (group_id, starts_at)
  WHERE is_archived = false;

CREATE INDEX IF NOT EXISTS idx_planning_members_user_created_at
  ON public.planning_members (user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_planning_members_planning_role
  ON public.planning_members (planning_id, role);

CREATE UNIQUE INDEX IF NOT EXISTS idx_planning_members_owner_unique
  ON public.planning_members (planning_id)
  WHERE role = 'owner';

CREATE INDEX IF NOT EXISTS idx_planning_activity_planning_created_at
  ON public.planning_activity (planning_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_planning_activity_actor_created_at
  ON public.planning_activity (actor_user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_planning_activity_entity_created_at
  ON public.planning_activity (entity_type, entity_id, created_at DESC);