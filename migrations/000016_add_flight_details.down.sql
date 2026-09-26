ALTER TABLE public.flights DROP CONSTRAINT IF EXISTS flights_offer_url_check;
ALTER TABLE public.flights
  DROP COLUMN IF EXISTS baggage,
  DROP COLUMN IF EXISTS cabin_class,
  DROP COLUMN IF EXISTS offer_url;
