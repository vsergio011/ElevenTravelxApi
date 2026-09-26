CREATE TABLE IF NOT EXISTS public.planning_route_days (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    planning_id UUID NOT NULL REFERENCES public.plannings(id) ON DELETE CASCADE,
    label TEXT NOT NULL,
    date_label TEXT NOT NULL,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS public.planning_route_stops (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    planning_id UUID NOT NULL REFERENCES public.plannings(id) ON DELETE CASCADE,
    day_id UUID NOT NULL REFERENCES public.planning_route_days(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    address TEXT NOT NULL DEFAULT '',
    time_label TEXT NOT NULL DEFAULT '',
    duration_minutes INT NOT NULL DEFAULT 0,
    notes TEXT NOT NULL DEFAULT '',
    cost_estimate NUMERIC(10,2) NOT NULL DEFAULT 0,
    currency TEXT NOT NULL DEFAULT 'EUR',
    external_url TEXT NOT NULL DEFAULT '',
    image_url TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    category TEXT NOT NULL DEFAULT 'landmark',
    is_optional BOOLEAN NOT NULL DEFAULT false,
    latitude DOUBLE PRECISION NOT NULL DEFAULT 0,
    longitude DOUBLE PRECISION NOT NULL DEFAULT 0,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_planning_route_days_planning_id ON public.planning_route_days(planning_id, sort_order);
CREATE INDEX IF NOT EXISTS idx_planning_route_stops_planning_id_day_id ON public.planning_route_stops(planning_id, day_id, sort_order);
