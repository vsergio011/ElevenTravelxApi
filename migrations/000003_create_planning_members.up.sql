CREATE TABLE IF NOT EXISTS public.planning_members (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  planning_id UUID NOT NULL REFERENCES public.plannings(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
  role TEXT NOT NULL,
  invited_by_user_id UUID NULL REFERENCES auth.users(id) ON DELETE SET NULL,
  joined_at TIMESTAMPTZ NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT planning_members_role_check CHECK (role IN ('owner', 'admin', 'member')),
  CONSTRAINT planning_members_unique_planning_user UNIQUE (planning_id, user_id)
);

CREATE TRIGGER trg_planning_members_set_updated_at
BEFORE UPDATE ON public.planning_members
FOR EACH ROW
EXECUTE FUNCTION public.set_updated_at();