ALTER TABLE public.bookings
  ADD COLUMN IF NOT EXISTS cost_cents BIGINT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS cost_distribution TEXT NOT NULL DEFAULT 'group';

ALTER TABLE public.bookings
  ADD CONSTRAINT bookings_cost_cents_check CHECK (cost_cents >= 0),
  ADD CONSTRAINT bookings_cost_distribution_check CHECK (cost_distribution IN ('individual', 'group', 'selected_participants'));

CREATE TABLE IF NOT EXISTS public.booking_participants (
  booking_id UUID NOT NULL REFERENCES public.bookings(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
  PRIMARY KEY (booking_id, user_id)
);
