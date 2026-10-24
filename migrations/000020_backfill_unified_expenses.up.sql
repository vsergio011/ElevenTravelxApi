-- Unified expense model, phase 2: backfill. Idempotent; legacy columns/tables are kept untouched.

-- 1. Common items for existing flights and bookings.
CREATE TEMP TABLE _item_map (kind TEXT, subtype_id UUID, item_id UUID, planning_id UUID) ON COMMIT DROP;

INSERT INTO _item_map (kind, subtype_id, item_id, planning_id)
SELECT 'flight', id, gen_random_uuid(), planning_id FROM public.flights WHERE planning_item_id IS NULL;
INSERT INTO _item_map (kind, subtype_id, item_id, planning_id)
SELECT 'booking', id, gen_random_uuid(), planning_id FROM public.bookings WHERE planning_item_id IS NULL;

INSERT INTO public.planning_items (id, planning_id, item_type) SELECT item_id, planning_id, kind FROM _item_map;
UPDATE public.flights f SET planning_item_id = m.item_id FROM _item_map m WHERE m.kind = 'flight' AND m.subtype_id = f.id;
UPDATE public.bookings b SET planning_item_id = m.item_id FROM _item_map m WHERE m.kind = 'booking' AND m.subtype_id = b.id;

-- 2. One expense per flight/booking with a cost. The creator is the payer when no payment is recorded.
CREATE TEMP TABLE _new_expense (expense_id UUID, kind TEXT, subtype_id UUID) ON COMMIT DROP;

WITH flight_src AS (
  SELECT f.id AS subtype_id, f.planning_id, f.planning_item_id, f.created_by_user_id, f.cost_cents, f.cost_distribution, f.currency,
    COALESCE(
      (SELECT s.origin_label || ' → ' || l.destination_label
       FROM (SELECT origin_label FROM public.flight_segments WHERE flight_id = f.id ORDER BY direction DESC, position LIMIT 1) s,
            (SELECT destination_label FROM public.flight_segments WHERE flight_id = f.id ORDER BY direction DESC, position DESC LIMIT 1) l),
      CASE WHEN NULLIF(btrim(f.flight_number), '') IS NOT NULL THEN 'Vuelo ' || f.flight_number END,
      'Vuelo') AS title,
    COALESCE((SELECT MIN(departure_at) FROM public.flight_segments WHERE flight_id = f.id), f.created_at) AS spent_at
  FROM public.flights f
  WHERE f.cost_cents > 0
    AND NOT EXISTS (SELECT 1 FROM public.expenses e WHERE e.planning_item_id = f.planning_item_id)
), ins AS (
  INSERT INTO public.expenses (planning_id, planning_item_id, created_by_user_id, paid_by_user_id, title, amount_cents, distribution, category, spent_at, currency)
  SELECT planning_id, planning_item_id, created_by_user_id, NULL, title, cost_cents, cost_distribution, 'flight', spent_at, currency FROM flight_src
  RETURNING id, planning_item_id
)
INSERT INTO _new_expense SELECT ins.id, 'flight', f.id FROM ins JOIN public.flights f ON f.planning_item_id = ins.planning_item_id;

WITH booking_src AS (
  SELECT b.* FROM public.bookings b
  WHERE b.cost_cents > 0
    AND NOT EXISTS (SELECT 1 FROM public.expenses e WHERE e.planning_item_id = b.planning_item_id)
), ins AS (
  INSERT INTO public.expenses (planning_id, planning_item_id, created_by_user_id, paid_by_user_id, title, amount_cents, distribution, category, spent_at, currency)
  SELECT planning_id, planning_item_id, created_by_user_id, NULL, title, cost_cents, cost_distribution, type, COALESCE(occurs_at, created_at), 'EUR' FROM booking_src
  RETURNING id, planning_item_id
)
INSERT INTO _new_expense SELECT ins.id, 'booking', b.id FROM ins JOIN public.bookings b ON b.planning_item_id = ins.planning_item_id;

-- 3. Payments: booking_payments become expense_payments; everything else gets the creator as payer.
INSERT INTO public.expense_payments (expense_id, user_id, amount_cents, paid_at)
SELECT n.expense_id, p.user_id, p.amount_cents, p.paid_at
FROM _new_expense n JOIN public.booking_payments p ON n.kind = 'booking' AND p.booking_id = n.subtype_id;

INSERT INTO public.expense_payments (expense_id, user_id, amount_cents, paid_at)
SELECT e.id, e.created_by_user_id, e.amount_cents, e.spent_at
FROM public.expenses e
WHERE e.id IN (SELECT expense_id FROM _new_expense)
  AND NOT EXISTS (SELECT 1 FROM public.expense_payments p WHERE p.expense_id = e.id);

-- Manual expenses keep their single payer as one full payment.
INSERT INTO public.expense_payments (expense_id, user_id, amount_cents, paid_at)
SELECT e.id, e.paid_by_user_id, e.amount_cents, e.spent_at
FROM public.expenses e
WHERE e.paid_by_user_id IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM public.expense_payments p WHERE p.expense_id = e.id);

-- 4. Shares. Same rule as the legacy code: equal split, remainder cents to the first users in UUID order;
-- 'individual' = every eligible user owes the full amount; 'selected_participants' uses item/expense participants.
CREATE TEMP TABLE _targets (expense_id UUID, user_id UUID, amount_cents BIGINT, distribution TEXT) ON COMMIT DROP;

INSERT INTO _targets
SELECT e.id, pm.user_id, e.amount_cents, e.distribution
FROM public.expenses e
JOIN public.planning_members pm ON pm.planning_id = e.planning_id
WHERE e.distribution IN ('group', 'individual')
  AND (e.id IN (SELECT expense_id FROM _new_expense)
       OR NOT EXISTS (SELECT 1 FROM public.expense_participants ep WHERE ep.expense_id = e.id AND ep.share_cents > 0));

INSERT INTO _targets
SELECT n.expense_id, fp.user_id, e.amount_cents, e.distribution
FROM _new_expense n JOIN public.expenses e ON e.id = n.expense_id
JOIN public.flight_participants fp ON n.kind = 'flight' AND fp.flight_id = n.subtype_id
WHERE e.distribution = 'selected_participants';

INSERT INTO _targets
SELECT n.expense_id, bp.user_id, e.amount_cents, e.distribution
FROM _new_expense n JOIN public.expenses e ON e.id = n.expense_id
JOIN public.booking_participants bp ON n.kind = 'booking' AND bp.booking_id = n.subtype_id
WHERE e.distribution = 'selected_participants';

INSERT INTO _targets
SELECT e.id, ep.user_id, e.amount_cents, e.distribution
FROM public.expenses e JOIN public.expense_participants ep ON ep.expense_id = e.id
WHERE e.distribution = 'selected_participants'
  AND e.id NOT IN (SELECT expense_id FROM _new_expense)
  AND NOT EXISTS (SELECT 1 FROM public.expense_participants x WHERE x.expense_id = e.id AND x.share_cents > 0);

INSERT INTO public.expense_participants (expense_id, user_id, share_cents)
SELECT expense_id, user_id,
  CASE WHEN distribution = 'individual' THEN amount_cents
       ELSE amount_cents / n + CASE WHEN rn <= amount_cents % n THEN 1 ELSE 0 END END
FROM (
  SELECT t.*, row_number() OVER (PARTITION BY expense_id ORDER BY user_id) AS rn,
         count(*) OVER (PARTITION BY expense_id) AS n
  FROM (SELECT DISTINCT expense_id, user_id, amount_cents, distribution FROM _targets) t
) s
ON CONFLICT (expense_id, user_id) DO UPDATE SET share_cents = EXCLUDED.share_cents;

-- Fire the deferred share/amount checks now; otherwise the pending trigger events block the ALTER TABLE statements below.
SET CONSTRAINTS ALL IMMEDIATE;

-- 5. Reconciliation view: compare legacy totals with the new model; anomalies need manual review.
CREATE OR REPLACE VIEW public.expense_migration_report AS
SELECT 'flight'::text AS kind, f.id AS source_id, f.planning_id, f.cost_cents AS legacy_cents,
       COALESCE(e.total, 0) AS expense_cents, COALESCE(e.payments, 0) AS payment_cents, COALESCE(e.shares, 0) AS share_cents,
       CASE
         WHEN f.cost_cents > 0 AND e.total IS NULL THEN 'missing_expense'
         WHEN e.total IS NOT NULL AND e.total <> f.cost_cents THEN 'amount_mismatch'
         WHEN e.total IS NOT NULL AND e.payments <> e.total THEN 'payments_differ_from_amount'
         WHEN e.total IS NOT NULL AND e.share_rows = 0 THEN 'no_participants'
       END AS anomaly
FROM public.flights f
LEFT JOIN LATERAL (
  SELECT SUM(x.amount_cents) AS total,
         (SELECT COALESCE(SUM(amount_cents), 0) FROM public.expense_payments p WHERE p.expense_id IN (SELECT id FROM public.expenses WHERE planning_item_id = f.planning_item_id)) AS payments,
         (SELECT COALESCE(SUM(share_cents), 0) FROM public.expense_participants s WHERE s.expense_id IN (SELECT id FROM public.expenses WHERE planning_item_id = f.planning_item_id)) AS shares,
         (SELECT COUNT(*) FROM public.expense_participants s WHERE s.expense_id IN (SELECT id FROM public.expenses WHERE planning_item_id = f.planning_item_id)) AS share_rows
  FROM public.expenses x WHERE x.planning_item_id = f.planning_item_id
) e ON true
UNION ALL
SELECT 'booking', b.id, b.planning_id, b.cost_cents,
       COALESCE(e.total, 0), COALESCE(e.payments, 0), COALESCE(e.shares, 0),
       CASE
         WHEN b.cost_cents > 0 AND e.total IS NULL THEN 'missing_expense'
         WHEN e.total IS NOT NULL AND e.total <> b.cost_cents THEN 'amount_mismatch'
         WHEN e.total IS NOT NULL AND e.payments <> e.total THEN 'payments_differ_from_amount'
         WHEN e.total IS NOT NULL AND e.share_rows = 0 THEN 'no_participants'
       END
FROM public.bookings b
LEFT JOIN LATERAL (
  SELECT SUM(x.amount_cents) AS total,
         (SELECT COALESCE(SUM(amount_cents), 0) FROM public.expense_payments p WHERE p.expense_id IN (SELECT id FROM public.expenses WHERE planning_item_id = b.planning_item_id)) AS payments,
         (SELECT COALESCE(SUM(share_cents), 0) FROM public.expense_participants s WHERE s.expense_id IN (SELECT id FROM public.expenses WHERE planning_item_id = b.planning_item_id)) AS shares,
         (SELECT COUNT(*) FROM public.expense_participants s WHERE s.expense_id IN (SELECT id FROM public.expenses WHERE planning_item_id = b.planning_item_id)) AS share_rows
  FROM public.expenses x WHERE x.planning_item_id = b.planning_item_id
) e ON true;

-- New flights/bookings get their common item automatically, so writers never leave a subtype without one.
CREATE OR REPLACE FUNCTION public.ensure_planning_item_for_subtype()
RETURNS TRIGGER AS $$
BEGIN
  IF NEW.planning_item_id IS NULL THEN
    INSERT INTO public.planning_items (planning_id, item_type) VALUES (NEW.planning_id, TG_ARGV[0])
    RETURNING id INTO NEW.planning_item_id;
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_flights_ensure_planning_item
BEFORE INSERT ON public.flights
FOR EACH ROW EXECUTE FUNCTION public.ensure_planning_item_for_subtype('flight');

CREATE TRIGGER trg_bookings_ensure_planning_item
BEFORE INSERT ON public.bookings
FOR EACH ROW EXECUTE FUNCTION public.ensure_planning_item_for_subtype('booking');

-- planning_item_id is now mandatory for subtypes.
ALTER TABLE public.flights ALTER COLUMN planning_item_id SET NOT NULL;
ALTER TABLE public.bookings ALTER COLUMN planning_item_id SET NOT NULL;
