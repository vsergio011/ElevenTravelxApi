CREATE TABLE IF NOT EXISTS public.plannings (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  group_id UUID NOT NULL,
  owner_user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE RESTRICT,
  name TEXT NOT NULL,
  description TEXT NULL,
  destination_name TEXT NULL,
  starts_at TIMESTAMPTZ NULL,
  ends_at TIMESTAMPTZ NULL,
  status TEXT NOT NULL DEFAULT 'draft',
  cover_image_url TEXT NULL,
  is_archived BOOLEAN NOT NULL DEFAULT false,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT plannings_status_check CHECK (status IN ('draft', 'active', 'completed')),
  CONSTRAINT plannings_date_range_check CHECK (starts_at IS NULL OR ends_at IS NULL OR starts_at <= ends_at)
);

CREATE TRIGGER trg_plannings_set_updated_at
BEFORE UPDATE ON public.plannings
FOR EACH ROW
EXECUTE FUNCTION public.set_updated_at();