-- Restores the legacy columns and refills them from the linked expenses. booking_payments is not recreated.
ALTER TABLE public.expenses ADD COLUMN IF NOT EXISTS paid_by_user_id UUID REFERENCES auth.users(id);

ALTER TABLE public.bookings
  ADD COLUMN IF NOT EXISTS cost_cents BIGINT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS cost_distribution TEXT NOT NULL DEFAULT 'group';

ALTER TABLE public.flights
  ADD COLUMN IF NOT EXISTS cost_cents BIGINT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS currency CHAR(3) NOT NULL DEFAULT 'EUR',
  ADD COLUMN IF NOT EXISTS cost_distribution TEXT NOT NULL DEFAULT 'group';

UPDATE public.bookings b SET cost_cents = e.amount_cents, cost_distribution = e.distribution
FROM (SELECT DISTINCT ON (planning_item_id) planning_item_id, amount_cents, distribution
      FROM public.expenses WHERE planning_item_id IS NOT NULL ORDER BY planning_item_id, created_at) e
WHERE e.planning_item_id = b.planning_item_id AND e.distribution IN ('individual', 'group', 'selected_participants');

UPDATE public.flights f SET cost_cents = e.amount_cents, currency = e.currency, cost_distribution = e.distribution
FROM (SELECT DISTINCT ON (planning_item_id) planning_item_id, amount_cents, currency, distribution
      FROM public.expenses WHERE planning_item_id IS NOT NULL ORDER BY planning_item_id, created_at) e
WHERE e.planning_item_id = f.planning_item_id AND e.distribution IN ('individual', 'group', 'selected_participants');

UPDATE public.expenses e SET paid_by_user_id = p.user_id
FROM (SELECT DISTINCT ON (expense_id) expense_id, user_id FROM public.expense_payments ORDER BY expense_id, amount_cents DESC) p
WHERE p.expense_id = e.id;
