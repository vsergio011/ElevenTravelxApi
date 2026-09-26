-- Seed inicial de UserProfile para desarrollo local.

INSERT INTO public.user_profiles (
    user_id,
    username,
    full_name,
    bio,
    avatar_url,
    multimedia_visibility,
    badges_visibility
)
VALUES (
    '11111111-1111-1111-1111-111111111111',
    'sergioviaja',
    'Sergio Lopez',
    'Siempre con una ruta lista para despegar.',
    'https://images.unsplash.com/photo-1500648767791-00dcc994a43e',
    'public',
    'public'
)
ON CONFLICT (user_id) DO NOTHING;

INSERT INTO public.user_locations (
    id,
    user_id,
    name,
    latitude,
    longitude,
    country_code,
    visited_at
)
VALUES
    (
        '21111111-1111-1111-1111-111111111111',
        '11111111-1111-1111-1111-111111111111',
        'Tokyo',
        35.6762,
        139.6503,
        'JP',
        NOW() - INTERVAL '180 days'
    ),
    (
        '31111111-1111-1111-1111-111111111111',
        '11111111-1111-1111-1111-111111111111',
        'Lisbon',
        38.7223,
        -9.1393,
        'PT',
        NOW() - INTERVAL '90 days'
    )
ON CONFLICT (id) DO NOTHING;

INSERT INTO public.user_media_assets (
    id,
    user_id,
    location_id,
    media_type,
    url,
    thumbnail_url,
    caption
)
VALUES
    (
        '41111111-1111-1111-1111-111111111111',
        '11111111-1111-1111-1111-111111111111',
        '21111111-1111-1111-1111-111111111111',
        'image',
        'https://images.unsplash.com/photo-1536098561742-ca998e48cbcc',
        'https://images.unsplash.com/photo-1536098561742-ca998e48cbcc?w=320&q=70',
        'Atardecer en Shibuya'
    ),
    (
        '51111111-1111-1111-1111-111111111111',
        '11111111-1111-1111-1111-111111111111',
        '21111111-1111-1111-1111-111111111111',
        'video',
        'https://example.com/media/tokyo-night.mp4',
        'https://images.unsplash.com/photo-1492571350019-22de08371fd3?w=320&q=70',
        'Cruce de noche'
    )
ON CONFLICT (id) DO NOTHING;

INSERT INTO public.badges (
    id,
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
VALUES (
    '81111111-1111-1111-1111-111111111111',
    'gold-pin',
    'Pin Dorado',
    'Primer destino verificado en mapa.',
    'https://example.com/badges/gold-pin.svg',
    'Tokyo',
    35.6762,
    139.6503,
    'JP',
    TRUE
)
ON CONFLICT (code) DO NOTHING;

INSERT INTO public.user_achievements (
    id,
    user_id,
    badge_id,
    location_id,
    obtained_at
)
VALUES (
    '61111111-1111-1111-1111-111111111111',
    '11111111-1111-1111-1111-111111111111',
    '81111111-1111-1111-1111-111111111111',
    '21111111-1111-1111-1111-111111111111',
    NOW() - INTERVAL '30 days'
)
ON CONFLICT (user_id, badge_id) DO NOTHING;
