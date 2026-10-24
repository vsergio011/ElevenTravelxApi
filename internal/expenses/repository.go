package expenses

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/eleventravel/eleventravel-api/internal/expenseledger"
	"github.com/eleventravel/eleventravel-api/internal/http/response"
)

type Repository interface {
	Create(ctx context.Context, planningID uuid.UUID, actorUserID uuid.UUID, input CreateExpenseInput) (Expense, error)
	Get(ctx context.Context, planningID uuid.UUID, expenseID uuid.UUID) (Expense, error)
	List(ctx context.Context, planningID uuid.UUID) ([]Expense, error)
	Update(ctx context.Context, planningID uuid.UUID, expenseID uuid.UUID, input UpdateExpenseInput) (Expense, error)
	Delete(ctx context.Context, planningID uuid.UUID, expenseID uuid.UUID) error
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// The linked flight/booking status is joined only to mark the expense pending until it is confirmed.
const expenseSelect = `
	SELECT e.id, e.planning_id, e.created_by_user_id, e.planning_item_id, pi.item_type, COALESCE(f.status, b.status), COALESCE(f.id, b.id),
		e.title, e.amount_cents, e.currency, e.distribution, e.category, e.notes, e.spent_at, e.created_at, e.updated_at
	FROM public.expenses e
	LEFT JOIN public.planning_items pi ON pi.id = e.planning_item_id
	LEFT JOIN public.flights f ON f.planning_item_id = pi.id
	LEFT JOIN public.bookings b ON b.planning_item_id = pi.id`

func scanExpense(row pgx.Row) (Expense, error) {
	var expense Expense
	err := row.Scan(&expense.ID, &expense.PlanningID, &expense.CreatedByUserID, &expense.PlanningItemID, &expense.ItemType, &expense.ItemStatus, &expense.ItemRefID,
		&expense.Title, &expense.AmountCents, &expense.Currency, &expense.Distribution, &expense.Category, &expense.Notes,
		&expense.SpentAt, &expense.CreatedAt, &expense.UpdatedAt)
	expense.ParticipantUserIDs = []uuid.UUID{}
	expense.Shares = []Share{}
	expense.Payments = []Payment{}
	return expense, err
}

func (r *PostgresRepository) Create(ctx context.Context, planningID uuid.UUID, actorUserID uuid.UUID, input CreateExpenseInput) (Expense, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Expense{}, err
	}
	defer tx.Rollback(ctx)

	id, err := expenseledger.Insert(ctx, tx, expenseledger.WriteInput{
		PlanningID: planningID, ActorUserID: actorUserID, PlanningItemID: input.PlanningItemID,
		Title: input.Title, Category: input.Category, Notes: input.Notes, SpentAt: input.SpentAt,
		AmountCents: input.AmountCents, Currency: input.Currency, Distribution: input.Distribution,
		ParticipantUserIDs: input.ParticipantUserIDs, CustomShares: input.CustomShares, Payments: input.Payments,
	})
	if err != nil {
		return Expense{}, mapWriteError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Expense{}, mapWriteError(err)
	}
	return r.Get(ctx, planningID, id)
}

func (r *PostgresRepository) Get(ctx context.Context, planningID uuid.UUID, expenseID uuid.UUID) (Expense, error) {
	return r.get(ctx, r.pool, planningID, expenseID)
}

func (r *PostgresRepository) get(ctx context.Context, q expenseledger.Querier, planningID uuid.UUID, expenseID uuid.UUID) (Expense, error) {
	expense, err := scanExpense(q.QueryRow(ctx, expenseSelect+` WHERE e.id = $1 AND e.planning_id = $2`, expenseID, planningID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Expense{}, ErrExpenseNotFound
		}
		return Expense{}, err
	}
	list := []Expense{expense}
	if err := r.attachDetails(ctx, q, list); err != nil {
		return Expense{}, err
	}
	return list[0], nil
}

func (r *PostgresRepository) List(ctx context.Context, planningID uuid.UUID) ([]Expense, error) {
	rows, err := r.pool.Query(ctx, expenseSelect+` WHERE e.planning_id = $1 ORDER BY e.spent_at DESC, e.created_at DESC`, planningID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	expenses := []Expense{}
	for rows.Next() {
		expense, scanErr := scanExpense(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		expenses = append(expenses, expense)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	rows.Close()
	if err := r.attachDetails(ctx, r.pool, expenses); err != nil {
		return nil, err
	}
	return expenses, nil
}

// attachDetails loads shares and payments for the given expenses in two queries.
func (r *PostgresRepository) attachDetails(ctx context.Context, q expenseledger.Querier, expenses []Expense) error {
	if len(expenses) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, 0, len(expenses))
	index := make(map[uuid.UUID]int, len(expenses))
	for i, expense := range expenses {
		ids = append(ids, expense.ID)
		index[expense.ID] = i
	}
	rows, err := q.Query(ctx, `SELECT expense_id, user_id, share_cents FROM public.expense_participants WHERE expense_id = ANY($1) ORDER BY expense_id, user_id`, ids)
	if err != nil {
		return err
	}
	for rows.Next() {
		var expenseID uuid.UUID
		var share Share
		if err := rows.Scan(&expenseID, &share.UserID, &share.Cents); err != nil {
			rows.Close()
			return err
		}
		i := index[expenseID]
		expenses[i].Shares = append(expenses[i].Shares, share)
		if expenses[i].Distribution != DistributionGroup {
			expenses[i].ParticipantUserIDs = append(expenses[i].ParticipantUserIDs, share.UserID)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	payments, err := expenseledger.LoadPayments(ctx, q, ids)
	if err != nil {
		return err
	}
	for expenseID, list := range payments {
		expenses[index[expenseID]].Payments = list
	}
	return nil
}

func (r *PostgresRepository) Update(ctx context.Context, planningID uuid.UUID, expenseID uuid.UUID, input UpdateExpenseInput) (Expense, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Expense{}, err
	}
	defer tx.Rollback(ctx)

	current, err := r.get(ctx, tx, planningID, expenseID)
	if err != nil {
		return Expense{}, err
	}
	write := expenseledger.WriteInput{
		PlanningID: planningID, Title: current.Title, AmountCents: current.AmountCents, Currency: current.Currency,
		Distribution: current.Distribution, Category: current.Category, Notes: current.Notes,
		ParticipantUserIDs: current.ParticipantUserIDs, CustomShares: current.Shares, SpentAt: input.SpentAt,
	}
	if input.Title != nil {
		write.Title = *input.Title
	}
	if input.AmountCents != nil {
		write.AmountCents = *input.AmountCents
	}
	if input.Currency != nil {
		write.Currency = *input.Currency
	}
	if input.Distribution != nil {
		write.Distribution = *input.Distribution
	}
	if input.Category != nil {
		write.Category = input.Category
	}
	if input.Notes != nil {
		write.Notes = input.Notes
	}
	if input.ParticipantUserIDs != nil {
		write.ParticipantUserIDs = *input.ParticipantUserIDs
	}
	if input.CustomShares != nil {
		write.CustomShares = *input.CustomShares
	}
	if err := expenseledger.Update(ctx, tx, expenseID, write); err != nil {
		return Expense{}, mapWriteError(err)
	}
	if input.Payments != nil {
		if err := expenseledger.ReplacePayments(ctx, tx, expenseID, *input.Payments); err != nil {
			return Expense{}, mapWriteError(err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Expense{}, mapWriteError(err)
	}
	return r.Get(ctx, planningID, expenseID)
}

func (r *PostgresRepository) Delete(ctx context.Context, planningID uuid.UUID, expenseID uuid.UUID) error {
	result, err := r.pool.Exec(ctx, `DELETE FROM public.expenses WHERE id = $1 AND planning_id = $2`, expenseID, planningID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrExpenseNotFound
	}
	return nil
}

// mapWriteError turns integrity violations (foreign item of another planning, shares not matching the amount)
// into validation errors so the API answers 400 instead of 500.
func mapWriteError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrExpenseNotFound
	}
	if errors.Is(err, expenseledger.ErrInvalidShares) {
		return ValidationError{Details: []response.ErrorDetail{{Field: "participants", Message: "shares must add up to amount_cents"}}}
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23503":
			return ValidationError{Details: []response.ErrorDetail{{Field: "planning_item_id", Message: "planning_item_id does not belong to this planning"}}}
		case "23514":
			return ValidationError{Details: []response.ErrorDetail{{Field: "participants", Message: "shares must add up to amount_cents"}}}
		}
	}
	return err
}
