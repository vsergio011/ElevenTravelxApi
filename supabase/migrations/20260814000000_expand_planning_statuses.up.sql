-- Keep the persistence model aligned with the four statuses exposed in the app.
ALTER TABLE public.plannings
  DROP CONSTRAINT IF EXISTS plannings_status_check;

-- Map the former values to their equivalent current statuses.
UPDATE public.plannings
SET status = CASE status
  WHEN 'active' THEN 'in_progress'
  WHEN 'completed' THEN 'finished'
  ELSE status
END
WHERE status IN ('active', 'completed');

ALTER TABLE public.plannings
  ADD CONSTRAINT plannings_status_check
  CHECK (status = ANY (ARRAY['draft'::text, 'confirmed'::text, 'in_progress'::text, 'finished'::text]));
