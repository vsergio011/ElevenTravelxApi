-- Unified expense model, phase 1: additive schema only. Legacy columns stay until the app reads the new model.

CREATE TABLE IF NOT EXISTS public.planning_items (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  planning_id UUID NOT NULL REFERENCES public.plannings(id) ON DELETE CASCADE,
  item_type TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT planning_items_type_check CHECK (item_type IN ('flight', 'booking')),
  CONSTRAINT planning_items_id_planning_unique UNIQUE (id, planning_id),
  CONSTRAINT planning_items_id_type_unique UNIQUE (id, item_type)
);

CREATE INDEX IF NOT EXISTS idx_planning_items_planning_id ON public.planning_items(planning_id);

DROP TRIGGER IF EXISTS trg_planning_items_set_updated_at ON public.planning_items;
CREATE TRIGGER trg_planning_items_set_updated_at
BEFORE UPDATE ON public.planning_items
FOR EACH ROW
EXECUTE FUNCTION public.set_updated_at();

-- Subtype link: same planning and matching type are enforced by composite FKs.
-- Columns are nullable until the backfill (000020) completes.
ALTER TABLE public.flights
  DROP CONSTRAINT IF EXISTS flights_planning_item_unique,
  DROP CONSTRAINT IF EXISTS flights_planning_item_planning_fk,
  DROP CONSTRAINT IF EXISTS flights_planning_item_type_fk;

ALTER TABLE public.bookings
  DROP CONSTRAINT IF EXISTS bookings_planning_item_unique,
  DROP CONSTRAINT IF EXISTS bookings_planning_item_planning_fk,
  DROP CONSTRAINT IF EXISTS bookings_planning_item_type_fk;

ALTER TABLE public.flights
  ADD COLUMN IF NOT EXISTS planning_item_id UUID NULL,
  ADD COLUMN IF NOT EXISTS item_type TEXT GENERATED ALWAYS AS ('flight') STORED,
  ADD CONSTRAINT flights_planning_item_unique UNIQUE (planning_item_id),
  ADD CONSTRAINT flights_planning_item_planning_fk FOREIGN KEY (planning_item_id, planning_id) REFERENCES public.planning_items(id, planning_id),
  ADD CONSTRAINT flights_planning_item_type_fk FOREIGN KEY (planning_item_id, item_type) REFERENCES public.planning_items(id, item_type);

ALTER TABLE public.bookings
  ADD COLUMN IF NOT EXISTS planning_item_id UUID NULL,
  ADD COLUMN IF NOT EXISTS item_type TEXT GENERATED ALWAYS AS ('booking') STORED,
  ADD CONSTRAINT bookings_planning_item_unique UNIQUE (planning_item_id),
  ADD CONSTRAINT bookings_planning_item_planning_fk FOREIGN KEY (planning_item_id, planning_id) REFERENCES public.planning_items(id, planning_id),
  ADD CONSTRAINT bookings_planning_item_type_fk FOREIGN KEY (planning_item_id, item_type) REFERENCES public.planning_items(id, item_type);

-- Deleting a flight/booking removes its common item (and, by cascade, its linked expenses).
CREATE OR REPLACE FUNCTION public.delete_planning_item_of_subtype()
RETURNS TRIGGER AS $$
BEGIN
  IF OLD.planning_item_id IS NOT NULL THEN
    DELETE FROM public.planning_items WHERE id = OLD.planning_item_id;
  END IF;
  RETURN OLD;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_flights_delete_planning_item ON public.flights;
CREATE TRIGGER trg_flights_delete_planning_item
AFTER DELETE ON public.flights
FOR EACH ROW EXECUTE FUNCTION public.delete_planning_item_of_subtype();

DROP TRIGGER IF EXISTS trg_bookings_delete_planning_item ON public.bookings;
CREATE TRIGGER trg_bookings_delete_planning_item
AFTER DELETE ON public.bookings
FOR EACH ROW EXECUTE FUNCTION public.delete_planning_item_of_subtype();

-- Expenses: optional link to a planning item, currency and optional legacy payer.
ALTER TABLE public.expenses
  DROP CONSTRAINT IF EXISTS expenses_currency_check,
  DROP CONSTRAINT IF EXISTS expenses_planning_item_fk;

ALTER TABLE public.expenses
  ADD COLUMN IF NOT EXISTS planning_item_id UUID NULL,
  ADD COLUMN IF NOT EXISTS currency CHAR(3) NOT NULL DEFAULT 'EUR',
  ADD CONSTRAINT expenses_currency_check CHECK (currency IN ('EUR', 'USD')),
  ADD CONSTRAINT expenses_planning_item_fk FOREIGN KEY (planning_item_id, planning_id) REFERENCES public.planning_items(id, planning_id) ON DELETE CASCADE;

-- Tolerates databases where the legacy payer column was already removed.
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = 'public' AND table_name = 'expenses' AND column_name = 'paid_by_user_id') THEN
    ALTER TABLE public.expenses ALTER COLUMN paid_by_user_id DROP NOT NULL;
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_expenses_planning_item_id ON public.expenses(planning_item_id);

-- 'individual' keeps its existing meaning: every participant owes the full amount.
ALTER TABLE public.expenses DROP CONSTRAINT IF EXISTS expenses_distribution_check;
ALTER TABLE public.expenses
  ADD CONSTRAINT expenses_distribution_check   CHECK (distribution IN ('individual', 'group', 'selected_participants', 'custom'));

ALTER TABLE public.expense_participants DROP CONSTRAINT IF EXISTS expense_participants_share_cents_check;
ALTER TABLE public.expense_participants
  ADD COLUMN IF NOT EXISTS share_cents BIGINT NOT NULL DEFAULT 0,
  ADD CONSTRAINT expense_participants_share_cents_check CHECK (share_cents >= 0);

-- Provider payments: who actually paid how much for an expense (several payers allowed).
CREATE TABLE IF NOT EXISTS public.expense_payments (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  expense_id UUID NOT NULL REFERENCES public.expenses(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE RESTRICT,
  amount_cents BIGINT NOT NULL CHECK (amount_cents > 0),
  paid_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_expense_payments_expense_id ON public.expense_payments(expense_id);

-- Sum of shares must equal the amount (except 'individual'). An expense with no share rows is
-- treated as not yet distributed. Total payments may differ from the amount: that is a partial/pending payment.
CREATE OR REPLACE FUNCTION public.check_expense_shares_total()
RETURNS TRIGGER AS $$
DECLARE
  target_id UUID;
  exp RECORD;
  share_total BIGINT;
  share_rows BIGINT;
BEGIN
  IF TG_TABLE_NAME = 'expenses' THEN
    target_id := NEW.id;
  ELSIF TG_OP = 'DELETE' THEN
    target_id := OLD.expense_id;
  ELSE
    target_id := NEW.expense_id;
  END IF;

  SELECT id, amount_cents, distribution INTO exp FROM public.expenses WHERE id = target_id;
  IF NOT FOUND OR exp.distribution = 'individual' THEN
    RETURN NULL;
  END IF;

  SELECT COALESCE(SUM(share_cents), 0), COUNT(*) INTO share_total, share_rows
  FROM public.expense_participants WHERE expense_id = target_id;

  IF share_rows > 0 AND share_total <> exp.amount_cents THEN
    RAISE EXCEPTION 'expense % shares (%) do not match amount (%)', target_id, share_total, exp.amount_cents
      USING ERRCODE = '23514';
  END IF;
  RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_expense_participants_total ON public.expense_participants;
CREATE CONSTRAINT TRIGGER trg_expense_participants_total
AFTER INSERT OR UPDATE ON public.expense_participants
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION public.check_expense_shares_total();

DROP TRIGGER IF EXISTS trg_expenses_shares_total ON public.expenses;
CREATE CONSTRAINT TRIGGER trg_expenses_shares_total
AFTER INSERT OR UPDATE OF amount_cents, distribution ON public.expenses
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION public.check_expense_shares_total();

-- RLS: members of the planning only. The API still enforces roles.
ALTER TABLE public.planning_items ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.expenses ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.expense_participants ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.expense_payments ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS planning_items_member_all ON public.planning_items;
DROP POLICY IF EXISTS expenses_member_all ON public.expenses;
DROP POLICY IF EXISTS expense_participants_member_all ON public.expense_participants;
DROP POLICY IF EXISTS expense_payments_member_all ON public.expense_payments;

CREATE POLICY planning_items_member_all ON public.planning_items
FOR ALL
USING (EXISTS (SELECT 1 FROM public.planning_members pm WHERE pm.planning_id = planning_items.planning_id AND pm.user_id = auth.uid()))
WITH CHECK (EXISTS (SELECT 1 FROM public.planning_members pm WHERE pm.planning_id = planning_items.planning_id AND pm.user_id = auth.uid()));

CREATE POLICY expenses_member_all ON public.expenses
FOR ALL
USING (EXISTS (SELECT 1 FROM public.planning_members pm WHERE pm.planning_id = expenses.planning_id AND pm.user_id = auth.uid()))
WITH CHECK (EXISTS (SELECT 1 FROM public.planning_members pm WHERE pm.planning_id = expenses.planning_id AND pm.user_id = auth.uid()));

CREATE POLICY expense_participants_member_all ON public.expense_participants
FOR ALL
USING (EXISTS (SELECT 1 FROM public.expenses e JOIN public.planning_members pm ON pm.planning_id = e.planning_id WHERE e.id = expense_participants.expense_id AND pm.user_id = auth.uid()))
WITH CHECK (EXISTS (SELECT 1 FROM public.expenses e JOIN public.planning_members pm ON pm.planning_id = e.planning_id WHERE e.id = expense_participants.expense_id AND pm.user_id = auth.uid()));

CREATE POLICY expense_payments_member_all ON public.expense_payments
FOR ALL
USING (EXISTS (SELECT 1 FROM public.expenses e JOIN public.planning_members pm ON pm.planning_id = e.planning_id WHERE e.id = expense_payments.expense_id AND pm.user_id = auth.uid()))
WITH CHECK (EXISTS (SELECT 1 FROM public.expenses e JOIN public.planning_members pm ON pm.planning_id = e.planning_id WHERE e.id = expense_payments.expense_id AND pm.user_id = auth.uid()));
