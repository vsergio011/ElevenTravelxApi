UPDATE public.plannings
SET group_id = gen_random_uuid()
WHERE group_id IS NULL;

ALTER TABLE public.plannings
  ALTER COLUMN group_id SET NOT NULL;
