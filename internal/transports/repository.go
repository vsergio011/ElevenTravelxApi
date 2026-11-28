package transports

import (
	"context"
	"errors"
	"time"

	"github.com/eleventravel/eleventravel-api/internal/expenseledger"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct{ pool *pgxpool.Pool }

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

const columns = `id, planning_id, created_by_user_id, planning_item_id, status, transport_type, origin, destination,
	departure_date::text, to_char(departure_time, 'HH24:MI'), arrival_date::text, to_char(arrival_time, 'HH24:MI'),
	operator, reference, external_url, notes, currency, cost_distribution, created_at, updated_at`

func scan(row pgx.Row) (Transport, error) {
	var item Transport
	err := row.Scan(&item.ID, &item.PlanningID, &item.CreatedByUserID, &item.PlanningItemID,
		&item.Status, &item.Type, &item.Origin, &item.Destination, &item.DepartureDate, &item.DepartureTime,
		&item.ArrivalDate, &item.ArrivalTime, &item.Operator, &item.Reference, &item.ExternalURL, &item.Notes,
		&item.Currency, &item.CostDistribution, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return item, ErrNotFound
	}
	return item, err
}

func (r *PostgresRepository) get(ctx context.Context, q expenseledger.Querier, planning, id uuid.UUID) (Transport, error) {
	item, err := scan(q.QueryRow(ctx, `SELECT `+columns+` FROM public.transports WHERE planning_id=$1 AND id=$2`, planning, id))
	if err != nil {
		return item, err
	}
	rows, err := q.Query(ctx, `SELECT user_id FROM public.transport_participants WHERE transport_id=$1 ORDER BY user_id`, id)
	if err != nil {
		return item, err
	}
	item.ParticipantUserIDs = []uuid.UUID{}
	for rows.Next() {
		var user uuid.UUID
		if err = rows.Scan(&user); err != nil {
			rows.Close()
			return item, err
		}
		item.ParticipantUserIDs = append(item.ParticipantUserIDs, user)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return item, err
	}
	costs, err := expenseledger.LoadItemCosts(ctx, q, []uuid.UUID{item.PlanningItemID})
	if err != nil {
		return item, err
	}
	payments := []Payment{}
	if cost, ok := costs[item.PlanningItemID]; ok {
		item.CostCents = cost.AmountCents
		item.Currency = cost.Currency
		item.CostDistribution = cost.Distribution
		for _, payment := range cost.Payments {
			payments = append(payments, Payment{UserID: payment.UserID, AmountCents: payment.AmountCents, PaidAt: payment.PaidAt})
		}
	}
	item.Payments = &payments
	return item, nil
}

func (r *PostgresRepository) Get(ctx context.Context, planning, id uuid.UUID) (Transport, error) {
	return r.get(ctx, r.pool, planning, id)
}

func (r *PostgresRepository) List(ctx context.Context, planning uuid.UUID) ([]Transport, error) {
	rows, err := r.pool.Query(ctx, `SELECT id FROM public.transports WHERE planning_id=$1 ORDER BY departure_date NULLS LAST, departure_time NULLS LAST, created_at DESC`, planning)
	if err != nil {
		return nil, err
	}
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	items := make([]Transport, 0, len(ids))
	for _, id := range ids {
		item, getErr := r.get(ctx, r.pool, planning, id)
		if getErr != nil {
			return nil, getErr
		}
		items = append(items, item)
	}
	return items, nil
}

func (r *PostgresRepository) Create(ctx context.Context, actor, planning uuid.UUID, input Input) (Transport, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Transport{}, err
	}
	defer tx.Rollback(ctx)
	item, err := scan(tx.QueryRow(ctx, `INSERT INTO public.transports
		(planning_id, created_by_user_id, status, transport_type, origin, destination, departure_date, departure_time, arrival_date, arrival_time, operator, reference, external_url, notes, currency, cost_distribution)
		VALUES ($1,$2,$3,$4,$5,$6,$7::date,$8::time,$9::date,$10::time,$11,$12,$13,$14,$15,$16) RETURNING `+columns,
		planning, actor, input.Status, input.Type, input.Origin, input.Destination, input.DepartureDate, input.DepartureTime, input.ArrivalDate, input.ArrivalTime, input.Operator, input.Reference, input.ExternalURL, input.Notes, input.Currency, input.CostDistribution))
	if err != nil {
		return Transport{}, err
	}
	if err = sync(ctx, tx, item, input); err != nil {
		return Transport{}, err
	}
	item, err = r.get(ctx, tx, planning, item.ID)
	if err != nil {
		return Transport{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Transport{}, err
	}
	return item, nil
}

func (r *PostgresRepository) Update(ctx context.Context, planning, id uuid.UUID, input Input) (Transport, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Transport{}, err
	}
	defer tx.Rollback(ctx)
	item, err := scan(tx.QueryRow(ctx, `UPDATE public.transports SET
		status=$3, transport_type=$4, origin=$5, destination=$6, departure_date=$7::date, departure_time=$8::time,
		arrival_date=$9::date, arrival_time=$10::time, operator=$11, reference=$12, external_url=$13, notes=$14, currency=$15, cost_distribution=$16
		WHERE planning_id=$1 AND id=$2 RETURNING `+columns,
		planning, id, input.Status, input.Type, input.Origin, input.Destination, input.DepartureDate, input.DepartureTime, input.ArrivalDate, input.ArrivalTime, input.Operator, input.Reference, input.ExternalURL, input.Notes, input.Currency, input.CostDistribution))
	if err != nil {
		return Transport{}, err
	}
	if err = sync(ctx, tx, item, input); err != nil {
		return Transport{}, err
	}
	item, err = r.get(ctx, tx, planning, id)
	if err != nil {
		return Transport{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Transport{}, err
	}
	return item, nil
}

func sync(ctx context.Context, tx pgx.Tx, item Transport, input Input) error {
	if _, err := tx.Exec(ctx, `DELETE FROM public.transport_participants WHERE transport_id=$1`, item.ID); err != nil {
		return err
	}
	for _, id := range input.ParticipantUserIDs {
		if _, err := tx.Exec(ctx, `INSERT INTO public.transport_participants (transport_id, user_id) VALUES ($1,$2)`, item.ID, id); err != nil {
			return err
		}
	}
	date := item.CreatedAt
	if input.DepartureDate != nil {
		parsed, err := time.Parse("2006-01-02", *input.DepartureDate)
		if err != nil {
			return err
		}
		date = parsed
	}
	var payments *[]expenseledger.Payment
	if input.Payments != nil {
		values := make([]expenseledger.Payment, 0, len(*input.Payments))
		for _, p := range *input.Payments {
			values = append(values, expenseledger.Payment{UserID: p.UserID, AmountCents: p.AmountCents, PaidAt: p.PaidAt})
		}
		payments = &values
	}
	return expenseledger.SyncItemExpense(ctx, tx, expenseledger.ItemSync{
		PlanningID: item.PlanningID, PlanningItemID: item.PlanningItemID, ActorUserID: item.CreatedByUserID,
		Title: input.Origin + " → " + input.Destination, Category: "transport", SpentAt: date,
		AmountCents: &input.CostCents, Currency: &input.Currency, Distribution: &input.CostDistribution,
		ParticipantUserIDs: input.ParticipantUserIDs, Payments: payments,
	})
}

func (r *PostgresRepository) Delete(ctx context.Context, planning, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM public.transports WHERE planning_id=$1 AND id=$2`, planning, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
