// Package expenseledger is the single writer/reader of the shared economic model
// (expenses, expense_participants, expense_payments). Flights, bookings and manual
// expenses all go through it so a cost is never stored twice.
package expenseledger

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	DistributionIndividual = "individual"
	DistributionGroup      = "group"
	DistributionSelected   = "selected_participants"
	DistributionCustom     = "custom"

	DefaultCurrency = "EUR"
)

// ErrInvalidShares means the explicit shares do not add up to the expense amount.
var ErrInvalidShares = errors.New("shares must add up to the expense amount")

type Querier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

type Share struct {
	UserID uuid.UUID
	Cents  int64
}

type Payment struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	AmountCents int64
	PaidAt      time.Time
}

// ComputeShares resolves the monetary share of every participant in integer cents.
// Equal splits give the remainder cents to the first users in UUID order. "individual" makes
// every eligible user owe the full amount. "custom" uses the explicit shares as given.
func ComputeShares(distribution string, amountCents int64, members []uuid.UUID, selected []uuid.UUID, custom []Share) []Share {
	if distribution == DistributionCustom {
		return append([]Share{}, custom...)
	}
	eligible := members
	if distribution == DistributionSelected {
		eligible = selected
	}
	sorted := append([]uuid.UUID(nil), eligible...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].String() < sorted[j].String() })
	shares := make([]Share, 0, len(sorted))
	if len(sorted) == 0 {
		return shares
	}
	if distribution == DistributionIndividual {
		for _, id := range sorted {
			shares = append(shares, Share{UserID: id, Cents: amountCents})
		}
		return shares
	}
	count := int64(len(sorted))
	base, remainder := amountCents/count, amountCents%count
	for i, id := range sorted {
		cents := base
		if int64(i) < remainder {
			cents++
		}
		shares = append(shares, Share{UserID: id, Cents: cents})
	}
	return shares
}

// ValidateShares checks the invariant enforced by the database: Σ shares = amount (except "individual").
func ValidateShares(distribution string, amountCents int64, shares []Share) error {
	if distribution == DistributionIndividual || len(shares) == 0 {
		return nil
	}
	var total int64
	for _, share := range shares {
		if share.Cents < 0 {
			return ErrInvalidShares
		}
		total += share.Cents
	}
	if total != amountCents {
		return ErrInvalidShares
	}
	return nil
}

// PaymentStatus compares recorded payments with the expense amount.
func PaymentStatus(amountCents int64, payments []Payment) string {
	var paid int64
	for _, payment := range payments {
		paid += payment.AmountCents
	}
	switch {
	case paid == 0:
		return "unpaid"
	case paid < amountCents:
		return "partial"
	case paid > amountCents:
		return "overpaid"
	default:
		return "paid"
	}
}

// WriteInput describes the full economic state of one expense.
type WriteInput struct {
	PlanningID         uuid.UUID
	ActorUserID        uuid.UUID
	PlanningItemID     *uuid.UUID
	Title              string
	Category           *string
	Notes              *string
	SpentAt            *time.Time
	AmountCents        int64
	Currency           string
	Distribution       string
	ParticipantUserIDs []uuid.UUID
	CustomShares       []Share
	Payments           []Payment
}

// Insert creates an expense with its shares and payments. Run it inside a transaction.
func Insert(ctx context.Context, q Querier, in WriteInput) (uuid.UUID, error) {
	var id uuid.UUID
	err := q.QueryRow(ctx, `
		INSERT INTO public.expenses (planning_id, planning_item_id, created_by_user_id, title, amount_cents, currency, distribution, category, notes, spent_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8::text, ''), NULLIF($9::text, ''), COALESCE($10, now()))
		RETURNING id`,
		in.PlanningID, in.PlanningItemID, in.ActorUserID, in.Title, in.AmountCents, currencyOrDefault(in.Currency), in.Distribution,
		in.Category, in.Notes, in.SpentAt).Scan(&id)
	if err != nil {
		return uuid.Nil, err
	}
	if err := writeShares(ctx, q, id, in); err != nil {
		return uuid.Nil, err
	}
	if err := ReplacePayments(ctx, q, id, in.Payments); err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

// Update rewrites the expense row and its shares. Payments are left alone; use ReplacePayments.
func Update(ctx context.Context, q Querier, id uuid.UUID, in WriteInput) error {
	tag, err := q.Exec(ctx, `
		UPDATE public.expenses SET title = $3, amount_cents = $4, currency = $5, distribution = $6,
			category = NULLIF($7::text, ''), notes = NULLIF($8::text, ''), spent_at = COALESCE($9, spent_at)
		WHERE id = $1 AND planning_id = $2`,
		id, in.PlanningID, in.Title, in.AmountCents, currencyOrDefault(in.Currency), in.Distribution, in.Category, in.Notes, in.SpentAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return writeShares(ctx, q, id, in)
}

func writeShares(ctx context.Context, q Querier, expenseID uuid.UUID, in WriteInput) error {
	members, err := planningMembers(ctx, q, in.PlanningID)
	if err != nil {
		return err
	}
	shares := ComputeShares(in.Distribution, in.AmountCents, members, in.ParticipantUserIDs, in.CustomShares)
	if err := ValidateShares(in.Distribution, in.AmountCents, shares); err != nil {
		return err
	}
	if _, err := q.Exec(ctx, `DELETE FROM public.expense_participants WHERE expense_id = $1`, expenseID); err != nil {
		return err
	}
	for _, share := range shares {
		if _, err := q.Exec(ctx, `INSERT INTO public.expense_participants (expense_id, user_id, share_cents) VALUES ($1, $2, $3)`, expenseID, share.UserID, share.Cents); err != nil {
			return err
		}
	}
	return nil
}

func ReplacePayments(ctx context.Context, q Querier, expenseID uuid.UUID, payments []Payment) error {
	if _, err := q.Exec(ctx, `DELETE FROM public.expense_payments WHERE expense_id = $1`, expenseID); err != nil {
		return err
	}
	for _, payment := range payments {
		paidAt := payment.PaidAt
		if paidAt.IsZero() {
			paidAt = time.Now().UTC()
		}
		if _, err := q.Exec(ctx, `INSERT INTO public.expense_payments (expense_id, user_id, amount_cents, paid_at) VALUES ($1, $2, $3, $4)`, expenseID, payment.UserID, payment.AmountCents, paidAt); err != nil {
			return err
		}
	}
	return nil
}

func planningMembers(ctx context.Context, q Querier, planningID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := q.Query(ctx, `SELECT user_id FROM public.planning_members WHERE planning_id = $1`, planningID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []uuid.UUID{}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func currencyOrDefault(value string) string {
	if value == "" {
		return DefaultCurrency
	}
	return value
}

// ItemCost is the cost of a flight/booking as stored in its primary expense
// (the oldest expense linked to the planning item).
type ItemCost struct {
	ExpenseID    uuid.UUID
	AmountCents  int64
	Currency     string
	Distribution string
	Payments     []Payment
}

// LoadItemCosts returns the primary expense cost for each planning item that has one.
func LoadItemCosts(ctx context.Context, q Querier, itemIDs []uuid.UUID) (map[uuid.UUID]ItemCost, error) {
	result := make(map[uuid.UUID]ItemCost, len(itemIDs))
	if len(itemIDs) == 0 {
		return result, nil
	}
	rows, err := q.Query(ctx, `
		SELECT DISTINCT ON (planning_item_id) planning_item_id, id, amount_cents, currency, distribution
		FROM public.expenses WHERE planning_item_id = ANY($1)
		ORDER BY planning_item_id, created_at, id`, itemIDs)
	if err != nil {
		return nil, err
	}
	byExpense := map[uuid.UUID]uuid.UUID{}
	expenseIDs := []uuid.UUID{}
	for rows.Next() {
		var itemID uuid.UUID
		var cost ItemCost
		if err := rows.Scan(&itemID, &cost.ExpenseID, &cost.AmountCents, &cost.Currency, &cost.Distribution); err != nil {
			rows.Close()
			return nil, err
		}
		cost.Payments = []Payment{}
		result[itemID] = cost
		byExpense[cost.ExpenseID] = itemID
		expenseIDs = append(expenseIDs, cost.ExpenseID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	payments, err := LoadPayments(ctx, q, expenseIDs)
	if err != nil {
		return nil, err
	}
	for expenseID, list := range payments {
		itemID := byExpense[expenseID]
		cost := result[itemID]
		cost.Payments = list
		result[itemID] = cost
	}
	return result, nil
}

func LoadPayments(ctx context.Context, q Querier, expenseIDs []uuid.UUID) (map[uuid.UUID][]Payment, error) {
	result := map[uuid.UUID][]Payment{}
	if len(expenseIDs) == 0 {
		return result, nil
	}
	rows, err := q.Query(ctx, `SELECT id, expense_id, user_id, amount_cents, paid_at FROM public.expense_payments WHERE expense_id = ANY($1) ORDER BY expense_id, paid_at, id`, expenseIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var expenseID uuid.UUID
		var payment Payment
		if err := rows.Scan(&payment.ID, &expenseID, &payment.UserID, &payment.AmountCents, &payment.PaidAt); err != nil {
			return nil, err
		}
		result[expenseID] = append(result[expenseID], payment)
	}
	return result, rows.Err()
}

// ItemSync is what a flight/booking repository pushes into the shared model after writing its own data.
// Nil pointers keep the current value of the primary expense.
type ItemSync struct {
	PlanningID         uuid.UUID
	PlanningItemID     uuid.UUID
	ActorUserID        uuid.UUID
	Title              string
	Category           string
	SpentAt            time.Time
	AmountCents        *int64
	Currency           *string
	Distribution       *string
	ParticipantUserIDs []uuid.UUID
	// Payments replaces recorded payments when set.
	Payments *[]Payment
	// DefaultPayerUserID pays the full amount when the expense has no payments at all.
	DefaultPayerUserID *uuid.UUID
	// FollowAmount keeps a single payment equal to the amount when the amount changes.
	FollowAmount bool
}

// SyncItemExpense creates, updates or removes the primary expense of a planning item (amount 0 removes it).
func SyncItemExpense(ctx context.Context, q Querier, in ItemSync) error {
	costs, err := LoadItemCosts(ctx, q, []uuid.UUID{in.PlanningItemID})
	if err != nil {
		return err
	}
	current, exists := costs[in.PlanningItemID]

	amount := current.AmountCents
	if in.AmountCents != nil {
		amount = *in.AmountCents
	}
	if amount <= 0 {
		if exists {
			_, err = q.Exec(ctx, `DELETE FROM public.expenses WHERE id = $1`, current.ExpenseID)
		}
		return err
	}
	distribution := current.Distribution
	if in.Distribution != nil {
		distribution = *in.Distribution
	}
	if distribution == "" {
		distribution = DistributionGroup
	}
	currency := current.Currency
	if in.Currency != nil {
		currency = *in.Currency
	}
	category := in.Category
	write := WriteInput{
		PlanningID: in.PlanningID, ActorUserID: in.ActorUserID, PlanningItemID: &in.PlanningItemID,
		Title: in.Title, Category: &category, SpentAt: &in.SpentAt, AmountCents: amount, Currency: currency,
		Distribution: distribution, ParticipantUserIDs: in.ParticipantUserIDs,
	}

	if !exists {
		payments := []Payment{}
		if in.Payments != nil {
			payments = *in.Payments
		} else if in.DefaultPayerUserID != nil {
			payments = []Payment{{UserID: *in.DefaultPayerUserID, AmountCents: amount, PaidAt: time.Now().UTC()}}
		}
		write.Payments = payments
		_, err = Insert(ctx, q, write)
		return err
	}

	if err := Update(ctx, q, current.ExpenseID, write); err != nil {
		return err
	}
	switch {
	case in.Payments != nil:
		return ReplacePayments(ctx, q, current.ExpenseID, *in.Payments)
	case len(current.Payments) == 0 && in.DefaultPayerUserID != nil:
		return ReplacePayments(ctx, q, current.ExpenseID, []Payment{{UserID: *in.DefaultPayerUserID, AmountCents: amount, PaidAt: time.Now().UTC()}})
	case in.FollowAmount && len(current.Payments) == 1 && current.Payments[0].AmountCents != amount:
		only := current.Payments[0]
		only.AmountCents = amount
		return ReplacePayments(ctx, q, current.ExpenseID, []Payment{only})
	}
	return nil
}
