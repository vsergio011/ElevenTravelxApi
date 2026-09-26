WITH constants AS (
  SELECT
    '11111111-1111-1111-1111-111111111111'::uuid AS owner_user_id,
    '22222222-2222-2222-2222-222222222222'::uuid AS admin_user_id,
    '33333333-3333-3333-3333-333333333333'::uuid AS member_user_id,
    '44444444-4444-4444-4444-444444444444'::uuid AS guest_user_id,
    'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa'::uuid AS group_alpha_id,
    'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb'::uuid AS group_beta_id
),
planning_rows AS (
  SELECT
    '8f1d9ac7-a0f1-4ea7-89bf-2cf83066c70e'::uuid AS id,
    c.group_alpha_id AS group_id,
    c.owner_user_id AS owner_user_id,
    'Road trip por la Costa Amalfitana'::text AS name,
    'Ruta de 10 días por Nápoles, Positano, Amalfi y Ravello'::text AS description,
    'Costa Amalfitana'::text AS destination_name,
    '2026-08-12 08:00:00+00'::timestamptz AS starts_at,
    '2026-08-21 20:00:00+00'::timestamptz AS ends_at,
    'active'::text AS status,
    'https://images.unsplash.com/photo-1523905330026-b8bd1f5f320e?auto=format&fit=crop&w=1400&q=80'::text AS cover_image_url,
    false AS is_archived,
    now() - interval '9 days' AS created_at,
    now() - interval '1 day' AS updated_at
  FROM constants c

  UNION ALL

  SELECT
    '9d9d453d-7e88-4e45-97f4-1d14f4f9dd51'::uuid,
    c.group_alpha_id,
    c.owner_user_id,
    'Escapada urbana en Tokio',
    'Viaje corto con foco en barrios, gastronomía y transporte',
    'Tokio',
    '2026-09-03 06:00:00+00',
    '2026-09-10 20:00:00+00',
    'draft',
    'https://images.unsplash.com/photo-1540959733332-eab4deabeeaf?auto=format&fit=crop&w=1400&q=80',
    false,
    now() - interval '4 days',
    now() - interval '4 days'
  FROM constants c

  UNION ALL

  SELECT
    'c6e62a3f-6db3-4d31-9f0d-8b2c1b7d1a11'::uuid,
    c.group_beta_id,
    c.owner_user_id,
    'Expedición a Islandia',
    'Ruta de auroras, cascadas y coche de alquiler para el invierno',
    'Islandia',
    '2026-02-10 09:00:00+00',
    '2026-02-18 18:00:00+00',
    'completed',
    'https://images.unsplash.com/photo-1500375592092-40eb2168fd21?auto=format&fit=crop&w=1400&q=80',
    true,
    now() - interval '40 days',
    now() - interval '2 days'
  FROM constants c
)
INSERT INTO public.plannings (
  id,
  group_id,
  owner_user_id,
  name,
  description,
  destination_name,
  starts_at,
  ends_at,
  status,
  cover_image_url,
  is_archived,
  created_at,
  updated_at
)
SELECT
  id,
  group_id,
  owner_user_id,
  name,
  description,
  destination_name,
  starts_at,
  ends_at,
  status,
  cover_image_url,
  is_archived,
  created_at,
  updated_at
FROM planning_rows
ON CONFLICT (id) DO UPDATE SET
  group_id = EXCLUDED.group_id,
  owner_user_id = EXCLUDED.owner_user_id,
  name = EXCLUDED.name,
  description = EXCLUDED.description,
  destination_name = EXCLUDED.destination_name,
  starts_at = EXCLUDED.starts_at,
  ends_at = EXCLUDED.ends_at,
  status = EXCLUDED.status,
  cover_image_url = EXCLUDED.cover_image_url,
  is_archived = EXCLUDED.is_archived,
  created_at = EXCLUDED.created_at,
  updated_at = EXCLUDED.updated_at;

INSERT INTO public.planning_members (id, planning_id, user_id, role, invited_by_user_id, joined_at, created_at, updated_at)
VALUES
  ('a1111111-1111-1111-1111-111111111111'::uuid, '8f1d9ac7-a0f1-4ea7-89bf-2cf83066c70e'::uuid, '11111111-1111-1111-1111-111111111111'::uuid, 'owner', NULL, now() - interval '9 days', now() - interval '9 days', now() - interval '1 day'),
  ('a2222222-2222-2222-2222-222222222222'::uuid, '8f1d9ac7-a0f1-4ea7-89bf-2cf83066c70e'::uuid, '22222222-2222-2222-2222-222222222222'::uuid, 'admin', '11111111-1111-1111-1111-111111111111'::uuid, now() - interval '8 days', now() - interval '8 days', now() - interval '1 day'),
  ('a3333333-3333-3333-3333-333333333333'::uuid, '8f1d9ac7-a0f1-4ea7-89bf-2cf83066c70e'::uuid, '33333333-3333-3333-3333-333333333333'::uuid, 'member', '22222222-2222-2222-2222-222222222222'::uuid, now() - interval '7 days', now() - interval '7 days', now() - interval '1 day'),
  ('b1111111-1111-1111-1111-111111111111'::uuid, '9d9d453d-7e88-4e45-97f4-1d14f4f9dd51'::uuid, '11111111-1111-1111-1111-111111111111'::uuid, 'owner', NULL, now() - interval '4 days', now() - interval '4 days', now() - interval '4 days'),
  ('b2222222-2222-2222-2222-222222222222'::uuid, '9d9d453d-7e88-4e45-97f4-1d14f4f9dd51'::uuid, '33333333-3333-3333-3333-333333333333'::uuid, 'member', '11111111-1111-1111-1111-111111111111'::uuid, now() - interval '3 days', now() - interval '3 days', now() - interval '3 days'),
  ('c1111111-1111-1111-1111-111111111111'::uuid, 'c6e62a3f-6db3-4d31-9f0d-8b2c1b7d1a11'::uuid, '11111111-1111-1111-1111-111111111111'::uuid, 'owner', NULL, now() - interval '40 days', now() - interval '40 days', now() - interval '2 days'),
  ('c2222222-2222-2222-2222-222222222222'::uuid, 'c6e62a3f-6db3-4d31-9f0d-8b2c1b7d1a11'::uuid, '44444444-4444-4444-4444-444444444444'::uuid, 'member', '11111111-1111-1111-1111-111111111111'::uuid, now() - interval '39 days', now() - interval '39 days', now() - interval '2 days')
ON CONFLICT (planning_id, user_id) DO UPDATE SET
  role = EXCLUDED.role,
  invited_by_user_id = EXCLUDED.invited_by_user_id,
  joined_at = EXCLUDED.joined_at,
  created_at = EXCLUDED.created_at,
  updated_at = EXCLUDED.updated_at;

INSERT INTO public.planning_activity (id, planning_id, actor_user_id, event_type, entity_type, entity_id, metadata, created_at)
VALUES
  ('d0000000-0000-0000-0000-000000000000'::uuid, '8f1d9ac7-a0f1-4ea7-89bf-2cf83066c70e'::uuid, '11111111-1111-1111-1111-111111111111'::uuid, 'dashboard.budget_snapshot', 'planning', NULL, jsonb_build_object('target', 3600, 'confirmed', 2240, 'pending', 840), now() - interval '3 days'),
  ('d0000000-0000-0000-0000-000000000001'::uuid, '8f1d9ac7-a0f1-4ea7-89bf-2cf83066c70e'::uuid, '11111111-1111-1111-1111-111111111111'::uuid, 'dashboard.upcoming_activity', 'planning', NULL, jsonb_build_object('title', 'Paseo por Spaccanapoli', 'starts_at', '2026-08-12T10:30:00Z', 'kind_label', 'Ruta urbana', 'image_url', 'https://images.unsplash.com/photo-1516483638261-f4dbaf036963?auto=format&fit=crop&w=1000&q=80'), now() - interval '8 days'),
  ('d0000000-0000-0000-0000-000000000002'::uuid, '8f1d9ac7-a0f1-4ea7-89bf-2cf83066c70e'::uuid, '11111111-1111-1111-1111-111111111111'::uuid, 'dashboard.upcoming_activity', 'planning', NULL, jsonb_build_object('title', 'Clase de cocina italiana', 'starts_at', '2026-08-13T18:00:00Z', 'kind_label', 'Experiencia', 'image_url', 'https://images.unsplash.com/photo-1466978913421-dad2ebd01d17?auto=format&fit=crop&w=1000&q=80'), now() - interval '7 days'),
  ('d0000000-0000-0000-0000-000000000003'::uuid, '8f1d9ac7-a0f1-4ea7-89bf-2cf83066c70e'::uuid, '11111111-1111-1111-1111-111111111111'::uuid, 'dashboard.upcoming_activity', 'planning', NULL, jsonb_build_object('title', 'Atardecer en Positano', 'starts_at', '2026-08-14T20:00:00Z', 'kind_label', 'Mirador', 'image_url', 'https://images.unsplash.com/photo-1511988617509-a57c8a288659?auto=format&fit=crop&w=1000&q=80'), now() - interval '6 days'),
  ('d0000000-0000-0000-0000-000000000004'::uuid, '8f1d9ac7-a0f1-4ea7-89bf-2cf83066c70e'::uuid, '11111111-1111-1111-1111-111111111111'::uuid, 'dashboard.next_stop', 'planning', NULL, jsonb_build_object('title', 'Ravello y Villa Rufolo', 'time_label', '15:45', 'location', 'Ravello, Salerno', 'description', 'Traslado en coche y visita guiada por los jardines con vistas al Tirreno.', 'image_url', 'https://images.unsplash.com/photo-1505764706515-aa95265c5abc?auto=format&fit=crop&w=1400&q=80'), now() - interval '1 day'),
  ('d1111111-1111-1111-1111-111111111111'::uuid, '8f1d9ac7-a0f1-4ea7-89bf-2cf83066c70e'::uuid, '11111111-1111-1111-1111-111111111111'::uuid, 'planning.created', 'planning', '8f1d9ac7-a0f1-4ea7-89bf-2cf83066c70e'::uuid, jsonb_build_object('name', 'Road trip por la Costa Amalfitana'), now() - interval '9 days'),
  ('d2222222-2222-2222-2222-222222222222'::uuid, '8f1d9ac7-a0f1-4ea7-89bf-2cf83066c70e'::uuid, '11111111-1111-1111-1111-111111111111'::uuid, 'member.added', 'member', '22222222-2222-2222-2222-222222222222'::uuid, jsonb_build_object('role', 'admin'), now() - interval '8 days'),
  ('d3333333-3333-3333-3333-333333333333'::uuid, '8f1d9ac7-a0f1-4ea7-89bf-2cf83066c70e'::uuid, '22222222-2222-2222-2222-222222222222'::uuid, 'planning.updated', 'planning', '8f1d9ac7-a0f1-4ea7-89bf-2cf83066c70e'::uuid, jsonb_build_object('status', 'active', 'is_archived', false), now() - interval '1 day'),
  ('d4444444-4444-4444-4444-444444444444'::uuid, '8f1d9ac7-a0f1-4ea7-89bf-2cf83066c70e'::uuid, '11111111-1111-1111-1111-111111111111'::uuid, 'flight.confirmed', 'planning', '8f1d9ac7-a0f1-4ea7-89bf-2cf83066c70e'::uuid, jsonb_build_object('status', 'confirmed'), now() - interval '2 days'),
  ('d5555555-5555-5555-5555-555555555555'::uuid, '8f1d9ac7-a0f1-4ea7-89bf-2cf83066c70e'::uuid, '11111111-1111-1111-1111-111111111111'::uuid, 'booking.confirmed', 'planning', '8f1d9ac7-a0f1-4ea7-89bf-2cf83066c70e'::uuid, jsonb_build_object('status', 'confirmed'), now() - interval '2 days'),
  ('d6666666-6666-6666-6666-666666666666'::uuid, '8f1d9ac7-a0f1-4ea7-89bf-2cf83066c70e'::uuid, '11111111-1111-1111-1111-111111111111'::uuid, 'expense.added', 'planning', '8f1d9ac7-a0f1-4ea7-89bf-2cf83066c70e'::uuid, jsonb_build_object('amount', 120), now() - interval '1 day'),
  ('e1111111-1111-1111-1111-111111111111'::uuid, '9d9d453d-7e88-4e45-97f4-1d14f4f9dd51'::uuid, '11111111-1111-1111-1111-111111111111'::uuid, 'planning.created', 'planning', '9d9d453d-7e88-4e45-97f4-1d14f4f9dd51'::uuid, jsonb_build_object('name', 'Escapada urbana en Tokio'), now() - interval '4 days'),
  ('e2222222-2222-2222-2222-222222222222'::uuid, '9d9d453d-7e88-4e45-97f4-1d14f4f9dd51'::uuid, '33333333-3333-3333-3333-333333333333'::uuid, 'flight.proposed', 'planning', '9d9d453d-7e88-4e45-97f4-1d14f4f9dd51'::uuid, jsonb_build_object('status', 'proposal'), now() - interval '3 days'),
  ('f1111111-1111-1111-1111-111111111111'::uuid, 'c6e62a3f-6db3-4d31-9f0d-8b2c1b7d1a11'::uuid, '11111111-1111-1111-1111-111111111111'::uuid, 'planning.created', 'planning', 'c6e62a3f-6db3-4d31-9f0d-8b2c1b7d1a11'::uuid, jsonb_build_object('name', 'Expedición a Islandia'), now() - interval '40 days'),
  ('f2222222-2222-2222-2222-222222222222'::uuid, 'c6e62a3f-6db3-4d31-9f0d-8b2c1b7d1a11'::uuid, '11111111-1111-1111-1111-111111111111'::uuid, 'planning.archived', 'planning', 'c6e62a3f-6db3-4d31-9f0d-8b2c1b7d1a11'::uuid, jsonb_build_object('is_archived', true), now() - interval '2 days')
ON CONFLICT (id) DO UPDATE SET
  planning_id = EXCLUDED.planning_id,
  actor_user_id = EXCLUDED.actor_user_id,
  event_type = EXCLUDED.event_type,
  entity_type = EXCLUDED.entity_type,
  entity_id = EXCLUDED.entity_id,
  metadata = EXCLUDED.metadata,
  created_at = EXCLUDED.created_at;

INSERT INTO public.planning_route_days (
  id,
  planning_id,
  label,
  date_label,
  sort_order,
  created_at,
  updated_at
)
VALUES
  ('c10d8fe2-9a5b-4e3f-bd8f-1323b0d6b7a1'::uuid, '8f1d9ac7-a0f1-4ea7-89bf-2cf83066c70e'::uuid, 'Día 1', '12 ago', 1, now(), now()),
  ('d2f3217d-93d4-4572-95ae-b8d0c7700ab2'::uuid, '8f1d9ac7-a0f1-4ea7-89bf-2cf83066c70e'::uuid, 'Día 2', '13 ago', 2, now(), now())
ON CONFLICT (id) DO UPDATE SET
  planning_id = EXCLUDED.planning_id,
  label = EXCLUDED.label,
  date_label = EXCLUDED.date_label,
  sort_order = EXCLUDED.sort_order,
  updated_at = EXCLUDED.updated_at;

INSERT INTO public.planning_route_stops (
  id,
  planning_id,
  day_id,
  title,
  description,
  address,
  time_label,
  duration_minutes,
  notes,
  cost_estimate,
  currency,
  external_url,
  image_url,
  status,
  category,
  is_optional,
  latitude,
  longitude,
  sort_order,
  created_at,
  updated_at
)
VALUES
  (
    '1f8d1f70-2e2b-4ad5-a4eb-7ca8ad4f4cf8'::uuid,
    '8f1d9ac7-a0f1-4ea7-89bf-2cf83066c70e'::uuid,
    'c10d8fe2-9a5b-4e3f-bd8f-1323b0d6b7a1'::uuid,
    'Llegada a Nápoles',
    'Check-in y paseo inicial por el centro histórico.',
    'Naples, Italy',
    '09:00',
    180,
    'Dejar maleta y empezar con una caminata por Spaccanapoli.',
    120.00,
    'EUR',
    'https://www.naples.com',
    'https://images.unsplash.com/photo-1533907650686-70576141c4f5?auto=format&fit=crop&w=1200&q=80',
    'in_progress',
    'landmark',
    false,
    40.8522,
    14.2681,
    1,
    now(),
    now()
  ),
  (
    '903d0a82-5a38-4d61-9cf1-0a7a95d8b1b4'::uuid,
    '8f1d9ac7-a0f1-4ea7-89bf-2cf83066c70e'::uuid,
    'c10d8fe2-9a5b-4e3f-bd8f-1323b0d6b7a1'::uuid,
    'Cena en Posillipo',
    'Puesta de sol con comida local.',
    'Posillipo, Naples, Italy',
    '20:00',
    120,
    'Reservar mesa con vista al mar.',
    85.00,
    'EUR',
    'https://www.tripadvisor.com',
    'https://images.unsplash.com/photo-1504674900247-0877df9cc836?auto=format&fit=crop&w=1200&q=80',
    'pending',
    'restaurant',
    false,
    40.8264,
    14.1777,
    2,
    now(),
    now()
  ),
  (
    '7a7dc3d9-1f4f-4c6e-b9ea-ea6d7ef2e1ce'::uuid,
    '8f1d9ac7-a0f1-4ea7-89bf-2cf83066c70e'::uuid,
    'd2f3217d-93d4-4572-95ae-b8d0c7700ab2'::uuid,
    'Visita a Amalfi',
    'Ruta por el pueblo y paseo por la costa.',
    'Amalfi, Italy',
    '11:30',
    240,
    'Llevar calzado cómodo para subir cuestas.',
    65.00,
    'EUR',
    'https://www.amalfi.it',
    'https://images.unsplash.com/photo-1507525428034-b723cf961d3e?auto=format&fit=crop&w=1200&q=80',
    'pending',
    'landmark',
    false,
    40.6333,
    14.6022,
    1,
    now(),
    now()
  )
ON CONFLICT (id) DO UPDATE SET
  planning_id = EXCLUDED.planning_id,
  day_id = EXCLUDED.day_id,
  title = EXCLUDED.title,
  description = EXCLUDED.description,
  address = EXCLUDED.address,
  time_label = EXCLUDED.time_label,
  duration_minutes = EXCLUDED.duration_minutes,
  notes = EXCLUDED.notes,
  cost_estimate = EXCLUDED.cost_estimate,
  currency = EXCLUDED.currency,
  external_url = EXCLUDED.external_url,
  image_url = EXCLUDED.image_url,
  status = EXCLUDED.status,
  category = EXCLUDED.category,
  is_optional = EXCLUDED.is_optional,
  latitude = EXCLUDED.latitude,
  longitude = EXCLUDED.longitude,
  sort_order = EXCLUDED.sort_order,
  updated_at = EXCLUDED.updated_at;