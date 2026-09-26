CREATE TABLE IF NOT EXISTS public.badges (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code TEXT NOT NULL UNIQUE,
    title TEXT NOT NULL,
    description TEXT,
    icon_url TEXT,
    location_name TEXT NOT NULL,
    latitude DOUBLE PRECISION NOT NULL,
    longitude DOUBLE PRECISION NOT NULL,
    country_code TEXT,
    is_available BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO public.badges (
    code,
    title,
    description,
    icon_url,
    location_name,
    latitude,
    longitude,
    country_code,
    is_available
)
SELECT DISTINCT ON (COALESCE(NULLIF(ub.code, ''), ua.code))
    COALESCE(NULLIF(ub.code, ''), ua.code) AS code,
    COALESCE(NULLIF(ub.label, ''), ua.title) AS title,
    ua.description,
    ub.icon_url,
    ul.name,
    ul.latitude,
    ul.longitude,
    ul.country_code,
    TRUE
FROM public.user_achievements AS ua
INNER JOIN public.user_locations AS ul ON ul.id = ua.location_id
LEFT JOIN public.user_badges AS ub ON ub.achievement_id = ua.id
ORDER BY COALESCE(NULLIF(ub.code, ''), ua.code), COALESCE(ua.verified_at, ua.created_at) DESC
ON CONFLICT (code) DO NOTHING;

ALTER TABLE public.user_achievements
ADD COLUMN IF NOT EXISTS badge_id UUID;

ALTER TABLE public.user_achievements
ADD COLUMN IF NOT EXISTS obtained_at TIMESTAMPTZ;

UPDATE public.user_achievements AS ua
SET badge_id = b.id,
    obtained_at = COALESCE(ua.verified_at, ua.created_at)
FROM public.badges AS b
WHERE b.code = ua.code
  AND (ua.badge_id IS NULL OR ua.obtained_at IS NULL);

ALTER TABLE public.user_achievements
ALTER COLUMN badge_id SET NOT NULL;

ALTER TABLE public.user_achievements
ALTER COLUMN obtained_at SET NOT NULL;

ALTER TABLE public.user_achievements
DROP CONSTRAINT IF EXISTS user_achievements_user_id_location_id_code_key;

ALTER TABLE public.user_achievements
DROP CONSTRAINT IF EXISTS user_achievements_badge_fk;

ALTER TABLE public.user_achievements
ADD CONSTRAINT user_achievements_badge_fk
FOREIGN KEY (badge_id) REFERENCES public.badges(id) ON DELETE CASCADE;

ALTER TABLE public.user_achievements
ADD CONSTRAINT user_achievements_user_badge_key
UNIQUE (user_id, badge_id);

ALTER TABLE public.user_achievements
DROP COLUMN IF EXISTS code,
DROP COLUMN IF EXISTS title,
DROP COLUMN IF EXISTS description,
DROP COLUMN IF EXISTS is_verified,
DROP COLUMN IF EXISTS verified_at;

DROP INDEX IF EXISTS idx_user_achievements_user;
CREATE INDEX IF NOT EXISTS idx_user_achievements_user ON public.user_achievements(user_id, obtained_at DESC);
CREATE INDEX IF NOT EXISTS idx_user_achievements_location ON public.user_achievements(location_id, obtained_at DESC);
CREATE INDEX IF NOT EXISTS idx_badges_available ON public.badges(is_available, code);

DROP TABLE IF EXISTS public.user_badges;