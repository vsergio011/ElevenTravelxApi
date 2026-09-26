CREATE TABLE IF NOT EXISTS public.user_badges (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    achievement_id UUID NOT NULL REFERENCES public.user_achievements(id) ON DELETE CASCADE,
    location_id UUID NOT NULL REFERENCES public.user_locations(id) ON DELETE CASCADE,
    code TEXT NOT NULL,
    label TEXT NOT NULL,
    icon_url TEXT,
    unlocked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT user_badges_user_location_code_key UNIQUE (user_id, location_id, code)
);

ALTER TABLE public.user_achievements
ADD COLUMN IF NOT EXISTS code TEXT;

ALTER TABLE public.user_achievements
ADD COLUMN IF NOT EXISTS title TEXT;

ALTER TABLE public.user_achievements
ADD COLUMN IF NOT EXISTS description TEXT;

ALTER TABLE public.user_achievements
ADD COLUMN IF NOT EXISTS is_verified BOOLEAN;

ALTER TABLE public.user_achievements
ADD COLUMN IF NOT EXISTS verified_at TIMESTAMPTZ;

UPDATE public.user_achievements AS ua
SET code = b.code,
    title = b.title,
    description = b.description,
    is_verified = TRUE,
    verified_at = ua.obtained_at
FROM public.badges AS b
WHERE b.id = ua.badge_id;

ALTER TABLE public.user_achievements
ALTER COLUMN code SET NOT NULL;

ALTER TABLE public.user_achievements
ALTER COLUMN title SET NOT NULL;

ALTER TABLE public.user_achievements
ALTER COLUMN is_verified SET NOT NULL;

ALTER TABLE public.user_achievements
ALTER COLUMN is_verified SET DEFAULT FALSE;

INSERT INTO public.user_badges (
    user_id,
    achievement_id,
    location_id,
    code,
    label,
    icon_url,
    unlocked_at
)
SELECT
    ua.user_id,
    ua.id,
    ua.location_id,
    b.code,
    b.title,
    b.icon_url,
    ua.obtained_at
FROM public.user_achievements AS ua
INNER JOIN public.badges AS b ON b.id = ua.badge_id
ON CONFLICT (user_id, location_id, code) DO NOTHING;

ALTER TABLE public.user_achievements
DROP CONSTRAINT IF EXISTS user_achievements_user_badge_key;

ALTER TABLE public.user_achievements
DROP CONSTRAINT IF EXISTS user_achievements_badge_fk;

ALTER TABLE public.user_achievements
ADD CONSTRAINT user_achievements_user_id_location_id_code_key
UNIQUE (user_id, location_id, code);

DROP INDEX IF EXISTS idx_user_achievements_location;
DROP INDEX IF EXISTS idx_badges_available;
DROP INDEX IF EXISTS idx_user_achievements_user;
CREATE INDEX IF NOT EXISTS idx_user_achievements_user ON public.user_achievements(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_user_badges_user ON public.user_badges(user_id, unlocked_at DESC);
CREATE INDEX IF NOT EXISTS idx_user_badges_location ON public.user_badges(location_id, unlocked_at DESC);

ALTER TABLE public.user_achievements
DROP COLUMN IF EXISTS badge_id,
DROP COLUMN IF EXISTS obtained_at;

DROP TABLE IF EXISTS public.badges;