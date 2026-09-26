ALTER TABLE public.flights
  ADD COLUMN IF NOT EXISTS offer_url TEXT NULL,
  ADD COLUMN IF NOT EXISTS cabin_class TEXT NULL,
  ADD COLUMN IF NOT EXISTS baggage TEXT NULL;

ALTER TABLE public.flights
  ADD CONSTRAINT flights_offer_url_check CHECK (offer_url IS NULL OR offer_url ~ '^https?://');
