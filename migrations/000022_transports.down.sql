DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM public.transports) THEN
    RAISE EXCEPTION 'Cannot remove transports while transport records exist';
  END IF;
END;
$$;
DROP TABLE public.transport_participants;
DROP TABLE public.transports;
ALTER TABLE public.planning_items DROP CONSTRAINT planning_items_type_check;
ALTER TABLE public.planning_items ADD CONSTRAINT planning_items_type_check
  CHECK (item_type IN ('flight', 'booking'));
