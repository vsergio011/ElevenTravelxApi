-- WARNING: This schema is for context only and is not meant to be run.
-- Table order and constraints may not be valid for execution.

CREATE TABLE public.plannings (
  id uuid NOT NULL DEFAULT gen_random_uuid(),
  owner_user_id uuid NOT NULL,
  name text NOT NULL,
  description text,
  destination_name text,
  starts_at timestamp with time zone,
  ends_at timestamp with time zone,
  status text NOT NULL DEFAULT 'draft'::text CHECK (status = ANY (ARRAY['draft'::text, 'confirmed'::text, 'in_progress'::text, 'finished'::text])),
  cover_image_url text,
  is_archived boolean NOT NULL DEFAULT false,
  created_at timestamp with time zone NOT NULL DEFAULT now(),
  updated_at timestamp with time zone NOT NULL DEFAULT now(),
  group_id text,
  CONSTRAINT plannings_pkey PRIMARY KEY (id),
  CONSTRAINT plannings_owner_user_id_fkey FOREIGN KEY (owner_user_id) REFERENCES auth.users(id)
);
CREATE TABLE public.planning_members (
  id uuid NOT NULL DEFAULT gen_random_uuid(),
  planning_id uuid NOT NULL,
  user_id uuid NOT NULL,
  role text NOT NULL CHECK (role = ANY (ARRAY['owner'::text, 'admin'::text, 'member'::text])),
  invited_by_user_id uuid,
  joined_at timestamp with time zone,
  created_at timestamp with time zone NOT NULL DEFAULT now(),
  updated_at timestamp with time zone NOT NULL DEFAULT now(),
  CONSTRAINT planning_members_pkey PRIMARY KEY (id),
  CONSTRAINT planning_members_planning_id_fkey FOREIGN KEY (planning_id) REFERENCES public.plannings(id),
  CONSTRAINT planning_members_user_id_fkey FOREIGN KEY (user_id) REFERENCES auth.users(id),
  CONSTRAINT planning_members_invited_by_user_id_fkey FOREIGN KEY (invited_by_user_id) REFERENCES auth.users(id)
);
CREATE TABLE public.planning_activity (
  id uuid NOT NULL DEFAULT gen_random_uuid(),
  planning_id uuid NOT NULL,
  actor_user_id uuid,
  event_type text NOT NULL,
  entity_type text NOT NULL CHECK (entity_type = ANY (ARRAY['planning'::text, 'member'::text])),
  entity_id uuid,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamp with time zone NOT NULL DEFAULT now(),
  CONSTRAINT planning_activity_pkey PRIMARY KEY (id),
  CONSTRAINT planning_activity_planning_id_fkey FOREIGN KEY (planning_id) REFERENCES public.plannings(id),
  CONSTRAINT planning_activity_actor_user_id_fkey FOREIGN KEY (actor_user_id) REFERENCES auth.users(id)
);
CREATE TABLE public.planning_route_days (
  id uuid NOT NULL DEFAULT gen_random_uuid(),
  planning_id uuid NOT NULL,
  label text NOT NULL,
  date_label text NOT NULL,
  sort_order integer NOT NULL DEFAULT 0,
  created_at timestamp with time zone NOT NULL DEFAULT now(),
  updated_at timestamp with time zone NOT NULL DEFAULT now(),
  CONSTRAINT planning_route_days_pkey PRIMARY KEY (id),
  CONSTRAINT planning_route_days_planning_id_fkey FOREIGN KEY (planning_id) REFERENCES public.plannings(id)
);
CREATE TABLE public.planning_route_stops (
  id uuid NOT NULL DEFAULT gen_random_uuid(),
  planning_id uuid NOT NULL,
  day_id uuid NOT NULL,
  title text NOT NULL,
  description text NOT NULL DEFAULT ''::text,
  address text NOT NULL DEFAULT ''::text,
  time_label text NOT NULL DEFAULT ''::text,
  duration_minutes integer NOT NULL DEFAULT 0,
  notes text NOT NULL DEFAULT ''::text,
  cost_estimate numeric NOT NULL DEFAULT 0,
  currency text NOT NULL DEFAULT 'EUR'::text,
  external_url text NOT NULL DEFAULT ''::text,
  image_url text,
  status text NOT NULL DEFAULT 'pending'::text,
  category text NOT NULL DEFAULT 'landmark'::text,
  is_optional boolean NOT NULL DEFAULT false,
  latitude double precision NOT NULL DEFAULT 0,
  longitude double precision NOT NULL DEFAULT 0,
  sort_order integer NOT NULL DEFAULT 0,
  created_at timestamp with time zone NOT NULL DEFAULT now(),
  updated_at timestamp with time zone NOT NULL DEFAULT now(),
  CONSTRAINT planning_route_stops_pkey PRIMARY KEY (id),
  CONSTRAINT planning_route_stops_planning_id_fkey FOREIGN KEY (planning_id) REFERENCES public.plannings(id),
  CONSTRAINT planning_route_stops_day_id_fkey FOREIGN KEY (day_id) REFERENCES public.planning_route_days(id)
);
CREATE TABLE public.user_profiles (
  user_id uuid NOT NULL,
  username text NOT NULL UNIQUE,
  full_name text NOT NULL,
  bio text,
  avatar_url text,
  multimedia_visibility text NOT NULL DEFAULT 'public'::text CHECK (multimedia_visibility = ANY (ARRAY['public'::text, 'private'::text, 'followers'::text])),
  badges_visibility text NOT NULL DEFAULT 'public'::text CHECK (badges_visibility = ANY (ARRAY['public'::text, 'private'::text, 'followers'::text])),
  created_at timestamp with time zone NOT NULL DEFAULT now(),
  updated_at timestamp with time zone NOT NULL DEFAULT now(),
  CONSTRAINT user_profiles_pkey PRIMARY KEY (user_id)
);
CREATE TABLE public.user_follows (
  follower_user_id uuid NOT NULL,
  following_user_id uuid NOT NULL,
  created_at timestamp with time zone NOT NULL DEFAULT now(),
  CONSTRAINT user_follows_pkey PRIMARY KEY (follower_user_id, following_user_id)
);
CREATE TABLE public.user_locations (
  id uuid NOT NULL DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL,
  name text NOT NULL,
  latitude double precision NOT NULL,
  longitude double precision NOT NULL,
  country_code text,
  visited_at timestamp with time zone,
  created_at timestamp with time zone NOT NULL DEFAULT now(),
  updated_at timestamp with time zone NOT NULL DEFAULT now(),
  CONSTRAINT user_locations_pkey PRIMARY KEY (id)
);
CREATE TABLE public.user_media_assets (
  id uuid NOT NULL DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL,
  location_id uuid NOT NULL,
  media_type text NOT NULL CHECK (media_type = ANY (ARRAY['image'::text, 'video'::text])),
  url text NOT NULL,
  thumbnail_url text,
  caption text,
  created_at timestamp with time zone NOT NULL DEFAULT now(),
  CONSTRAINT user_media_assets_pkey PRIMARY KEY (id),
  CONSTRAINT user_media_assets_location_id_fkey FOREIGN KEY (location_id) REFERENCES public.user_locations(id)
);
CREATE TABLE public.user_achievements (
  id uuid NOT NULL DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL,
  location_id uuid NOT NULL,
  created_at timestamp with time zone NOT NULL DEFAULT now(),
  badge_id uuid NOT NULL,
  obtained_at timestamp with time zone NOT NULL,
  CONSTRAINT user_achievements_pkey PRIMARY KEY (id),
  CONSTRAINT user_achievements_location_id_fkey FOREIGN KEY (location_id) REFERENCES public.user_locations(id),
  CONSTRAINT user_achievements_badge_fk FOREIGN KEY (badge_id) REFERENCES public.badges(id)
);
CREATE TABLE public.badges (
  id uuid NOT NULL DEFAULT gen_random_uuid(),
  code text NOT NULL UNIQUE,
  title text NOT NULL,
  description text,
  icon_url text,
  location_name text NOT NULL,
  latitude double precision NOT NULL,
  longitude double precision NOT NULL,
  country_code text,
  is_available boolean NOT NULL DEFAULT true,
  created_at timestamp with time zone NOT NULL DEFAULT now(),
  updated_at timestamp with time zone NOT NULL DEFAULT now(),
  CONSTRAINT badges_pkey PRIMARY KEY (id)
);
