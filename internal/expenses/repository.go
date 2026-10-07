package expenses

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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

const expenseColumns = `id, planning_id, created_by_user_id, paid_by_user_id, title, amount_cents, distribution, category, notes, spent_at, created_at, updated_at`

func scanExpense(row pgx.Row) (Expense, error) {
	var expense Expense
	err := row.Scan(&expense.ID, &expense.PlanningID, &expense.CreatedByUserID, &expense.PaidByUserID, &expense.Title,
		&expense.AmountCents, &expense.Distribution, &expense.Category, &expense.Notes, &expense.SpentAt,
		&expense.CreatedAt, &expense.UpdatedAt)
	expense.ParticipantUserIDs = []uuid.UUID{}
	return expense, err
}

func (r *PostgresRepository) Create(ctx context.Context, planningID uuid.UUID, actorUserID uuid.UUID, input CreateExpenseInput) (Expense, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Expense{}, err
	}
	defer tx.Rollback(ctx)

	expense, err := scanExpense(tx.QueryRow(ctx, `
		INSERT INTO public.expenses (planning_id, created_by_user_id, paid_by_user_id, title, amount_cents, distribution, category, notes, spent_at)
		VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7::text, ''), NULLIF($8::text, ''), COALESCE($9, now()))
		RETURNING `+expenseColumns,
		planningID, actorUserID, input.PaidByUserID, input.Title, input.AmountCents, input.Distribution,
		input.Category, input.Notes, input.SpentAt))
	if err != nil {
		return Expense{}, err
	}
	if err := replaceParticipants(ctx, tx, expense.ID, input.ParticipantUserIDs); err != nil {
		return Expense{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Expense{}, err
	}
	expense.ParticipantUserIDs = input.ParticipantUserIDs
	return expense, nil
}

func (r *PostgresRepository) Get(ctx context.Context, planningID uuid.UUID, expenseID uuid.UUID) (Expense, error) {
	expense, err := scanExpense(r.pool.QueryRow(ctx, `SELECT `+expenseColumns+` FROM public.expenses WHERE id = $1 AND planning_id = $2`, expenseID, planningID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Expense{}, ErrExpenseNotFound
		}
		return Expense{}, err
	}
	participants, err := r.listParticipants(ctx, []uuid.UUID{expense.ID})
	if err != nil {
		return Expense{}, err
	}
	if ids, ok := participants[expense.ID]; ok {
		expense.ParticipantUserIDs = ids
	}
	return expense, nil
}

func (r *PostgresRepository) List(ctx context.Context, planningID uuid.UUID) ([]Expense, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+expenseColumns+` FROM public.expenses WHERE planning_id = $1 ORDER BY spent_at DESC, created_at DESC`, planningID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	expenses := []Expense{}
	ids := []uuid.UUID{}
	for rows.Next() {
		expense, scanErr := scanExpense(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		expenses = append(expenses, expense)
		ids = append(ids, expense.ID)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	if len(ids) == 0 {
		return expenses, nil
	}
	participants, err := r.listParticipants(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range expenses {
		if list, ok := participants[expenses[i].ID]; ok {
			expenses[i].ParticipantUserIDs = list
		}
	}
	return expenses, nil
}

func (r *PostgresRepository) Update(ctx context.Context, planningID uuid.UUID, expenseID uuid.UUID, input UpdateExpenseInput) (Expense, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Expense{}, err
	}
	defer tx.Rollback(ctx)

	_, err = scanExpense(tx.QueryRow(ctx, `
		UPDATE public.expenses SET
			title = COALESCE($3, title),
			amount_cents = COALESCE($4, amount_cents),
			paid_by_user_id = COALESCE($5, paid_by_user_id),
			distribution = COALESCE($6, distribution),
			category = CASE WHEN $7::text IS NULL THEN category ELSE NULLIF($7::text, '') END,
			notes = CASE WHEN $8::text IS NULL THEN notes ELSE NULLIF($8::text, '') END,
			spent_at = COALESCE($9, spent_at)
		WHERE id = $1 AND planning_id = $2
		RETURNING `+expenseColumns,
		expenseID, planningID, input.Title, input.AmountCents, input.PaidByUserID, input.Distribution,
		input.Category, input.Notes, input.SpentAt))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Expense{}, ErrExpenseNotFound
		}
		return Expense{}, err
	}
	if input.ParticipantUserIDs != nil {
		if err := replaceParticipants(ctx, tx, expenseID, *input.ParticipantUserIDs); err != nil {
			return Expense{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Expense{}, err
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

func replaceParticipants(ctx context.Context, tx pgx.Tx, expenseID uuid.UUID, userIDs []uuid.UUID) error {
	if _, err := tx.Exec(ctx, `DELETE FROM public.expense_participants WHERE expense_id = $1`, expenseID); err != nil {
		return err
	}
	for _, userID := range userIDs {
		if _, err := tx.Exec(ctx, `INSERT INTO public.expense_participants (expense_id, user_id) VALUES ($1, $2)`, expenseID, userID); err != nil {
			return err
		}
	}
	return nil
}

func (r *PostgresRepository) listParticipants(ctx context.Context, expenseIDs []uuid.UUID) (map[uuid.UUID][]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx, `SELECT expense_id, user_id FROM public.expense_participants WHERE expense_id = ANY($1) ORDER BY created_at`, expenseIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := map[uuid.UUID][]uuid.UUID{}
	for rows.Next() {
		var expenseID, userID uuid.UUID
		if err := rows.Scan(&expenseID, &userID); err != nil {
			return nil, err
		}
		result[expenseID] = append(result[expenseID], userID)
	}
	return result, rows.Err()
}
