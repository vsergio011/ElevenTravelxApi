ALTER TABLE public.planning_items DROP CONSTRAINT planning_items_type_check;
ALTER TABLE public.planning_items ADD CONSTRAINT planning_items_type_check
  CHECK (item_type IN ('flight', 'booking', 'transport'));

CREATE TABLE public.transports (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  planning_id uuid NOT NULL REFERENCES public.plannings(id) ON DELETE CASCADE,
  created_by_user_id uuid NOT NULL REFERENCES auth.users(id) ON DELETE RESTRICT,
  planning_item_id uuid NOT NULL UNIQUE,
  item_type text NOT NULL DEFAULT 'transport' CHECK (item_type = 'transport'),
  status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'confirmed')),
  transport_type text NOT NULL CHECK (transport_type IN ('train', 'bus', 'urban', 'ferry', 'taxi', 'rental_car', 'other')),
  origin text NOT NULL,
  destination text NOT NULL,
  departure_date date,
  departure_time time,
  arrival_date date,
  arrival_time time,
  operator text,
  reference text,
  notes text,
  currency text NOT NULL DEFAULT 'EUR' CHECK (currency IN ('EUR', 'USD')),
  cost_distribution text NOT NULL DEFAULT 'group' CHECK (cost_distribution IN ('individual', 'group', 'selected_participants')),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  FOREIGN KEY (planning_item_id, planning_id) REFERENCES public.planning_items(id, planning_id),
  FOREIGN KEY (planning_item_id, item_type) REFERENCES public.planning_items(id, item_type)
);

CREATE INDEX transports_planning_id_idx ON public.transports(planning_id);

CREATE TABLE public.transport_participants (
  transport_id uuid NOT NULL REFERENCES public.transports(id) ON DELETE CASCADE,
  user_id uuid NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
  PRIMARY KEY (transport_id, user_id)
);

ALTER TABLE public.transports ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.transport_participants ENABLE ROW LEVEL SECURITY;

CREATE TRIGGER trg_transports_ensure_planning_item BEFORE INSERT ON public.transports
  FOR EACH ROW EXECUTE FUNCTION public.ensure_planning_item_for_subtype('transport');
CREATE TRIGGER trg_transports_delete_planning_item AFTER DELETE ON public.transports
  FOR EACH ROW EXECUTE FUNCTION public.delete_planning_item_of_subtype();
CREATE TRIGGER trg_transports_set_updated_at BEFORE UPDATE ON public.transports
  FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();
