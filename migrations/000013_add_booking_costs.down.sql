DROP TABLE IF EXISTS public.booking_participants;
ALTER TABLE public.bookings DROP CONSTRAINT IF EXISTS bookings_cost_cents_check;
ALTER TABLE public.bookings DROP CONSTRAINT IF EXISTS bookings_cost_distribution_check;
ALTER TABLE public.bookings DROP COLUMN IF EXISTS cost_cents;
ALTER TABLE public.bookings DROP COLUMN IF EXISTS cost_distribution;
