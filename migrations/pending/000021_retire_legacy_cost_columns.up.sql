-- Retires the legacy economic columns once expenses is the single source of truth.
-- Apply only after reviewing public.expense_migration_report. Dropping is irreversible for data not copied to expenses.
SET CONSTRAINTS ALL IMMEDIATE;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM public.expense_migration_report WHERE anomaly IN ('missing_expense', 'amount_mismatch')) THEN
    RAISE EXCEPTION 'expense_migration_report still has blocking anomalies';
  END IF;
END $$;

-- The report reads the legacy columns, so it goes away with them.
DROP VIEW IF EXISTS public.expense_migration_report;

DROP TABLE IF EXISTS public.booking_payments;

ALTER TABLE public.bookings
  DROP COLUMN IF EXISTS cost_cents,
  DROP COLUMN IF EXISTS cost_distribution;

ALTER TABLE public.flights
  DROP COLUMN IF EXISTS cost_cents,
  DROP COLUMN IF EXISTS currency,
  DROP COLUMN IF EXISTS cost_distribution;

ALTER TABLE public.expenses
  DROP COLUMN IF EXISTS paid_by_user_id;
