CREATE TABLE IF NOT EXISTS public.flights (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  planning_id UUID NOT NULL REFERENCES public.plannings(id) ON DELETE CASCADE,
  created_by_user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE RESTRICT,
  status TEXT NOT NULL DEFAULT 'draft',
  airline TEXT NULL,
  flight_number TEXT NULL,
  reservation_code TEXT NULL,
  notes TEXT NULL,
  cost_cents BIGINT NOT NULL DEFAULT 0,
  currency CHAR(3) NOT NULL DEFAULT 'EUR',
  cost_distribution TEXT NOT NULL DEFAULT 'group',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT flights_status_check CHECK (status IN ('draft', 'confirmed')),
  CONSTRAINT flights_cost_cents_check CHECK (cost_cents >= 0),
  CONSTRAINT flights_currency_check CHECK (currency IN ('EUR', 'USD')),
  CONSTRAINT flights_cost_distribution_check CHECK (cost_distribution IN ('individual', 'group', 'selected_participants'))
);

CREATE TABLE IF NOT EXISTS public.flight_segments (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  flight_id UUID NOT NULL REFERENCES public.flights(id) ON DELETE CASCADE,
  direction TEXT NOT NULL,
  position INTEGER NOT NULL,
  origin_label TEXT NOT NULL,
  origin_city TEXT NULL,
  origin_airport_name TEXT NULL,
  origin_airport_code TEXT NULL,
  origin_mapbox_id TEXT NULL,
  destination_label TEXT NOT NULL,
  destination_city TEXT NULL,
  destination_airport_name TEXT NULL,
  destination_airport_code TEXT NULL,
  destination_mapbox_id TEXT NULL,
  departure_at TIMESTAMPTZ NOT NULL,
  arrival_at TIMESTAMPTZ NOT NULL,
  departure_terminal TEXT NULL,
  arrival_terminal TEXT NULL,
  airline TEXT NULL,
  flight_number TEXT NULL,
  CONSTRAINT flight_segments_direction_check CHECK (direction IN ('outbound', 'return')),
  CONSTRAINT flight_segments_position_check CHECK (position >= 0),
  CONSTRAINT flight_segments_dates_check CHECK (arrival_at > departure_at),
  UNIQUE (flight_id, direction, position)
);

CREATE TABLE IF NOT EXISTS public.flight_participants (
  flight_id UUID NOT NULL REFERENCES public.flights(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
  PRIMARY KEY (flight_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_flights_planning_status ON public.flights(planning_id, status);
CREATE INDEX IF NOT EXISTS idx_flight_segments_flight_direction_position ON public.flight_segments(flight_id, direction, position);
CREATE INDEX IF NOT EXISTS idx_flight_participants_user_id ON public.flight_participants(user_id);

CREATE TRIGGER trg_flights_set_updated_at
BEFORE UPDATE ON public.flights
FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();
