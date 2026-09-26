-- Store the IANA timezone of each segment endpoint so the UI can show local time without ambiguity.
ALTER TABLE public.flight_segments
  ADD COLUMN IF NOT EXISTS origin_timezone TEXT NULL,
  ADD COLUMN IF NOT EXISTS destination_timezone TEXT NULL;
