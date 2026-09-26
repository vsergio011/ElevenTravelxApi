ALTER TABLE public.bookings
  ADD COLUMN IF NOT EXISTS route_stop_id UUID REFERENCES public.planning_route_stops(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_bookings_route_stop_id ON public.bookings(route_stop_id);

CREATE TABLE IF NOT EXISTS public.booking_payments (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  booking_id UUID NOT NULL REFERENCES public.bookings(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE RESTRICT,
  amount_cents BIGINT NOT NULL CHECK (amount_cents > 0),
  paid_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_booking_payments_booking_id ON public.booking_payments(booking_id);
