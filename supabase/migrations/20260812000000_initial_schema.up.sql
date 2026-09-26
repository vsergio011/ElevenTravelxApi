-- Initial Migration: Schema Setup
-- Created on 2026-08-12

BEGIN;

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- 1. Table: user_profiles
CREATE TABLE IF NOT EXISTS user_profiles (
    user_id UUID PRIMARY KEY REFERENCES auth.users(id) ON DELETE CASCADE,
    username TEXT UNIQUE NOT NULL,
    full_name TEXT,
    bio TEXT,
    avatar_url TEXT,
    multimedia_visibility TEXT DEFAULT 'private',
    badges_visibility TEXT DEFAULT 'private',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- 2. Table: badges
CREATE TABLE IF NOT EXISTS badges (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    code TEXT UNIQUE NOT NULL,
    title TEXT NOT NULL,
    description TEXT,
    icon_url TEXT,
    location_name TEXT,
    latitude FLOAT8,
    longitude FLOAT8,
    country_code TEXT,
    is_available BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- 3. Table: plannings
CREATE TABLE IF NOT EXISTS plannings (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    owner_user_id UUID REFERENCES auth.users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT,
    destination_name TEXT,
    starts_at TIMESTAMPTZ,
    ends_at TIMESTAMPTZ,
    status TEXT DEFAULT 'draft',
    cover_image_url TEXT,
    is_archived BOOLEAN DEFAULT FALSE,
    group_id TEXT, -- Can be linked to a groups table later
    created_at TIMESTAMPT_TZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- 4. Table: planning_members
CREATE TABLE IF NOT EXISTS planning_members (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    planning_id UUID REFERENCES plannings(id) ON DELETE CASCADE,
    user_id UUID REFERENCES auth.users(id) ON DELETE CASCADE,
    role TEXT NOT NULL, -- 'owner', 'admin', 'member'
    invited_by_user_id UUID REFERENCES auth.users(id),
    joined_at TIMESTAMPTZ DEFAULT NOW(),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- 5. Table: planning_activity
CREATE TABLE IF NOT EXISTS planning_activity (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    planning_id UUID REFERENCES plannings(id) ON DELETE CASCADE,
    actor_user_id UUID REFERENCES auth.un(id),
    event_type TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id UUID,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- 6. Table: planning_route_days
CREATE TABLE IF NOT EXISTS planning_route_days (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    planning_id UUID REFERENCES plannings(id) ON DELETE CASCADE,
    label TEXT,
    date_label TEXT,
    sort_order INT4,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- 7. Table: planning_route_stops
CREATE TABLE IF NOT EXISTS planning_route_stops (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    planning_id UUID REFERENCES plannings(id) ON DELETE CASCADE,
    day_id UUID REFERENCES planning_route_days(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    description TEXT,
    address TEXT,
    time_label TEXT,
    duration_minutes INT4,
    notes TEXT,
    cost_estimate NUMERIC,
    currency TEXT DEFAULT 'EUR',
    external_url TEXT,
    image_url TEXT,
    status TEXT DEFAULT 'planned',
    category TEXT,
    is_optional BOOLEAN DEFAULT FALSE,
    latitude FLOAT8,
    longitude FLOAT8,
    sort_order INT4,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- 8. Table: user_follows
CREATE TABLE IF NOT EXISTS user_follows (
    follower_user_id UUID REFERENCES auth.users(id) ON DELETE CASCADE,
    following_user_id UUID REFERENCES auth.users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (follower_user_id, following_user_id)
);

-- 9. Table: user_locations
CREATE TABLE IF NOT EXISTS user_locations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID REFERENCES auth.users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    latitude FLOAT8 NOT NULL,
    longitude FLOAT8 NOT NULL,
    country_code TEXT,
    visited_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
_ ... (truncated for brevity in thought)
