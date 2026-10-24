DROP TABLE IF EXISTS public.expense_payments;
DROP TRIGGER IF EXISTS trg_expense_participants_total ON public.expense_participants;
DROP TRIGGER IF EXISTS trg_expenses_shares_total ON public.expenses;
DROP FUNCTION IF EXISTS public.check_expense_shares_total();

ALTER TABLE public.expense_participants DROP COLUMN IF EXISTS share_cents;

ALTER TABLE public.expenses DROP CONSTRAINT IF EXISTS expenses_distribution_check;
ALTER TABLE public.expenses
  ADD CONSTRAINT expenses_distribution_check CHECK (distribution IN ('group', 'selected_participants'));
ALTER TABLE public.expenses
  DROP CONSTRAINT IF EXISTS expenses_planning_item_fk,
  DROP CONSTRAINT IF EXISTS expenses_currency_check,
  DROP COLUMN IF EXISTS planning_item_id,
  DROP COLUMN IF EXISTS currency;

DROP TRIGGER IF EXISTS trg_flights_delete_planning_item ON public.flights;
DROP TRIGGER IF EXISTS trg_bookings_delete_planning_item ON public.bookings;
DROP FUNCTION IF EXISTS public.delete_planning_item_of_subtype();

ALTER TABLE public.bookings
  DROP CONSTRAINT IF EXISTS bookings_planning_item_planning_fk,
  DROP CONSTRAINT IF EXISTS bookings_planning_item_type_fk,
  DROP CONSTRAINT IF EXISTS bookings_planning_item_unique,
  DROP COLUMN IF EXISTS item_type,
  DROP COLUMN IF EXISTS planning_item_id;

ALTER TABLE public.flights
  DROP CONSTRAINT IF EXISTS flights_planning_item_planning_fk,
  DROP CONSTRAINT IF EXISTS flights_planning_item_type_fk,
  DROP CONSTRAINT IF EXISTS flights_planning_item_unique,
  DROP COLUMN IF EXISTS item_type,
  DROP COLUMN IF EXISTS planning_item_id;

DROP TABLE IF EXISTS public.planning_items;
