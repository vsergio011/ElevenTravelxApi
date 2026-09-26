CREATE TABLE IF NOT EXISTS public.bookings (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  planning_id UUID NOT NULL REFERENCES public.plannings(id) ON DELETE CASCADE,
  created_by_user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE RESTRICT,
  title TEXT NOT NULL,
  type TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'proposal',
  external_url TEXT NULL,
  occurs_at TIMESTAMPTZ NULL,
  location TEXT NULL,
  notes TEXT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT bookings_type_check CHECK (type IN ('hotel', 'restaurant', 'attraction', 'car', 'activity')),
  CONSTRAINT bookings_status_check CHECK (status IN ('proposal', 'confirmed'))
);

CREATE INDEX IF NOT EXISTS idx_bookings_planning_id_status ON public.bookings(planning_id, status);
CREATE INDEX IF NOT EXISTS idx_bookings_planning_id_type ON public.bookings(planning_id, type);

CREATE TRIGGER trg_bookings_set_updated_at
BEFORE UPDATE ON public.bookings
FOR EACH ROW
EXECUTE FUNCTION public.set_updated_at();
