ALTER TABLE public.user_badges
ADD COLUMN IF NOT EXISTS location_id UUID;

UPDATE public.user_badges AS b
SET location_id = a.location_id
FROM public.user_achievements AS a
WHERE b.location_id IS NULL
  AND b.achievement_id = a.id;

ALTER TABLE public.user_badges
ALTER COLUMN location_id SET NOT NULL;

ALTER TABLE public.user_badges
DROP CONSTRAINT IF EXISTS user_badges_achievement_id_code_key;

ALTER TABLE public.user_badges
ADD CONSTRAINT user_badges_location_fk
FOREIGN KEY (location_id) REFERENCES public.user_locations(id) ON DELETE CASCADE;

ALTER TABLE public.user_badges
ADD CONSTRAINT user_badges_user_location_code_key
UNIQUE (user_id, location_id, code);

CREATE INDEX IF NOT EXISTS idx_user_badges_location
ON public.user_badges(location_id, unlocked_at DESC);
