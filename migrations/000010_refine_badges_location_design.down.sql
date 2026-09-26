DROP INDEX IF EXISTS idx_user_badges_location;

ALTER TABLE public.user_badges
DROP CONSTRAINT IF EXISTS user_badges_user_location_code_key;

ALTER TABLE public.user_badges
DROP CONSTRAINT IF EXISTS user_badges_location_fk;

ALTER TABLE public.user_badges
ADD CONSTRAINT user_badges_achievement_id_code_key
UNIQUE (achievement_id, code);

ALTER TABLE public.user_badges
DROP COLUMN IF EXISTS location_id;
