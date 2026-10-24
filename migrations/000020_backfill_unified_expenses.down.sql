-- Data backfill is not reversed: it only added rows. Remove the structural pieces added by 000020.
DROP TRIGGER IF EXISTS trg_flights_ensure_planning_item ON public.flights;
DROP TRIGGER IF EXISTS trg_bookings_ensure_planning_item ON public.bookings;
DROP FUNCTION IF EXISTS public.ensure_planning_item_for_subtype();
DROP VIEW IF EXISTS public.expense_migration_report;
ALTER TABLE public.flights ALTER COLUMN planning_item_id DROP NOT NULL;
ALTER TABLE public.bookings ALTER COLUMN planning_item_id DROP NOT NULL;
