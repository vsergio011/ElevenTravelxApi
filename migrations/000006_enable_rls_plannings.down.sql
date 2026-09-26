DROP POLICY IF EXISTS planning_activity_insert_member ON public.planning_activity;
DROP POLICY IF EXISTS planning_activity_select_member ON public.planning_activity;
DROP POLICY IF EXISTS planning_members_delete_manager ON public.planning_members;
DROP POLICY IF EXISTS planning_members_update_manager ON public.planning_members;
DROP POLICY IF EXISTS planning_members_insert_manager_or_owner ON public.planning_members;
DROP POLICY IF EXISTS planning_members_select_member ON public.planning_members;
DROP POLICY IF EXISTS plannings_delete_owner ON public.plannings;
DROP POLICY IF EXISTS plannings_update_manager ON public.plannings;
DROP POLICY IF EXISTS plannings_insert_owner ON public.plannings;
DROP POLICY IF EXISTS plannings_select_member ON public.plannings;

ALTER TABLE public.planning_activity DISABLE ROW LEVEL SECURITY;
ALTER TABLE public.planning_members DISABLE ROW LEVEL SECURITY;
ALTER TABLE public.plannings DISABLE ROW LEVEL SECURITY;