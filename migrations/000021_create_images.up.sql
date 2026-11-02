CREATE TABLE IF NOT EXISTS public.images (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id UUID NOT NULL,
    purpose TEXT NOT NULL CHECK (purpose IN ('avatar', 'profile_media')),
    visibility TEXT NOT NULL CHECK (visibility IN ('public', 'private')),
    bucket TEXT NOT NULL,
    object_key TEXT NOT NULL UNIQUE,
    mime_type TEXT NOT NULL,
    declared_size_bytes BIGINT NOT NULL CHECK (declared_size_bytes > 0),
    size_bytes BIGINT,
    width INTEGER,
    height INTEGER,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'completed')),
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_images_owner ON public.images (owner_user_id, purpose);
CREATE INDEX IF NOT EXISTS idx_images_pending_expiry ON public.images (expires_at) WHERE status = 'pending';

ALTER TABLE public.images ENABLE ROW LEVEL SECURITY;

ALTER TABLE public.user_media_assets
    ADD COLUMN IF NOT EXISTS image_id UUID REFERENCES public.images(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_user_media_assets_image ON public.user_media_assets (image_id);
