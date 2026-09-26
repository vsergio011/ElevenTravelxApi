CREATE TABLE IF NOT EXISTS public.user_profiles (
    user_id UUID PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    full_name TEXT NOT NULL,
    bio TEXT,
    avatar_url TEXT,
    multimedia_visibility TEXT NOT NULL DEFAULT 'public' CHECK (multimedia_visibility IN ('public', 'private', 'followers')),
    badges_visibility TEXT NOT NULL DEFAULT 'public' CHECK (badges_visibility IN ('public', 'private', 'followers')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS public.user_follows (
    follower_user_id UUID NOT NULL,
    following_user_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (follower_user_id, following_user_id),
    CONSTRAINT user_follows_not_self CHECK (follower_user_id <> following_user_id)
);

CREATE TABLE IF NOT EXISTS public.user_locations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    name TEXT NOT NULL,
    latitude DOUBLE PRECISION NOT NULL,
    longitude DOUBLE PRECISION NOT NULL,
    country_code TEXT,
    visited_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS public.user_media_assets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    location_id UUID NOT NULL REFERENCES public.user_locations(id) ON DELETE CASCADE,
    media_type TEXT NOT NULL CHECK (media_type IN ('image', 'video')),
    url TEXT NOT NULL,
    thumbnail_url TEXT,
    caption TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS public.user_achievements (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    location_id UUID NOT NULL REFERENCES public.user_locations(id) ON DELETE CASCADE,
    code TEXT NOT NULL,
    title TEXT NOT NULL,
    description TEXT,
    is_verified BOOLEAN NOT NULL DEFAULT FALSE,
    verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, location_id, code)
);

CREATE TABLE IF NOT EXISTS public.user_badges (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    achievement_id UUID NOT NULL REFERENCES public.user_achievements(id) ON DELETE CASCADE,
    code TEXT NOT NULL,
    label TEXT NOT NULL,
    icon_url TEXT,
    unlocked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (achievement_id, code)
);

CREATE INDEX IF NOT EXISTS idx_user_profiles_username ON public.user_profiles(username);
CREATE INDEX IF NOT EXISTS idx_user_follows_following ON public.user_follows(following_user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_user_follows_follower ON public.user_follows(follower_user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_user_locations_user ON public.user_locations(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_user_media_user ON public.user_media_assets(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_user_media_location ON public.user_media_assets(location_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_user_achievements_user ON public.user_achievements(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_user_badges_user ON public.user_badges(user_id, unlocked_at DESC);
