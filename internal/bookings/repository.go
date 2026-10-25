package bookings

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/eleventravel/eleventravel-api/internal/expenseledger"
)

type Repository interface {
	CreateBooking(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, input CreateBookingInput) (Booking, error)
	GetBookingByID(ctx context.Context, planningID uuid.UUID, bookingID uuid.UUID) (Booking, error)
	ListBookings(ctx context.Context, planningID uuid.UUID, filters BookingFilters) (PageResult[Booking], error)
	UpdateBooking(ctx context.Context, planningID uuid.UUID, bookingID uuid.UUID, input UpdateBookingInput) (Booking, error)
	ConfirmBooking(ctx context.Context, planningID uuid.UUID, bookingID uuid.UUID) (Booking, error)
	DeleteBooking(ctx context.Context, planningID uuid.UUID, bookingID uuid.UUID) error
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// Cost, distribution and payments are not stored on bookings: they live in the expense linked
// through planning_item_id and are overlaid on read (see applyCosts).
const bookingColumns = `id, planning_id, created_by_user_id, title, type, status, external_url, occurs_at, location, notes, route_stop_id, planning_item_id, created_at, updated_at`

type queryer = expenseledger.Querier

func (r *PostgresRepository) CreateBooking(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, input CreateBookingInput) (Booking, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Booking{}, err
	}
	defer tx.Rollback(ctx)

	booking, err := scanBooking(tx.QueryRow(ctx, `
		INSERT INTO public.bookings (
			id, planning_id, created_by_user_id, title, type, status, external_url, occurs_at, location, notes, route_stop_id
		)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7, $8, $9, (SELECT id FROM public.planning_route_stops WHERE id = $10 AND planning_id = $1))
		RETURNING `+bookingColumns,
		planningID, actorUserID, input.Title, input.Type, input.Status, input.ExternalURL, input.OccursAt, input.Location, input.Notes, input.RouteStopID))
	if err != nil {
		return Booking{}, err
	}
	if err := replaceParticipants(ctx, tx, booking.ID, input.ParticipantUserIDs); err != nil {
		return Booking{}, err
	}
	booking.ParticipantUserIDs = input.ParticipantUserIDs
	payments := input.Payments
	if err := syncExpense(ctx, tx, booking, actorUserID, &input.CostCents, &input.CostDistribution, &payments); err != nil {
		return Booking{}, err
	}
	if err := applyCosts(ctx, tx, []*Booking{&booking}); err != nil {
		return Booking{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Booking{}, err
	}
	return booking, nil
}

// syncExpense pushes the booking cost into the shared expense model. Payments are explicit:
// an empty list means nobody has paid the provider yet.
func syncExpense(ctx context.Context, q queryer, booking Booking, actor uuid.UUID, cost *int64, distribution *string, payments *[]Payment) error {
	spentAt := booking.CreatedAt
	if booking.OccursAt != nil {
		spentAt = *booking.OccursAt
	}
	sync := expenseledger.ItemSync{
		PlanningID: booking.PlanningID, PlanningItemID: booking.PlanningItemID, ActorUserID: actor,
		Title: booking.Title, Category: booking.Type, SpentAt: spentAt,
		AmountCents: cost, Distribution: distribution, ParticipantUserIDs: booking.ParticipantUserIDs,
	}
	if payments != nil {
		list := make([]expenseledger.Payment, 0, len(*payments))
		for _, payment := range *payments {
			list = append(list, expenseledger.Payment{UserID: payment.UserID, AmountCents: payment.AmountCents, PaidAt: payment.PaidAt})
		}
		sync.Payments = &list
	}
	return expenseledger.SyncItemExpense(ctx, q, sync)
}

// applyCosts fills cost, distribution and payments of each booking from its linked expense.
func applyCosts(ctx context.Context, q queryer, bookings []*Booking) error {
	ids := make([]uuid.UUID, 0, len(bookings))
	for _, booking := range bookings {
		ids = append(ids, booking.PlanningItemID)
	}
	costs, err := expenseledger.LoadItemCosts(ctx, q, ids)
	if err != nil {
		return err
	}
	for _, booking := range bookings {
		booking.CostCents, booking.CostDistribution, booking.Payments = 0, DistributionGroup, []Payment{}
		cost, ok := costs[booking.PlanningItemID]
		if !ok {
			continue
		}
		booking.CostCents, booking.CostDistribution = cost.AmountCents, cost.Distribution
		for _, payment := range cost.Payments {
			booking.Payments = append(booking.Payments, Payment{ID: payment.ID, UserID: payment.UserID, AmountCents: payment.AmountCents, PaidAt: payment.PaidAt})
		}
	}
	return nil
}

func (r *PostgresRepository) GetBookingByID(ctx context.Context, planningID uuid.UUID, bookingID uuid.UUID) (Booking, error) {
	return r.getBooking(ctx, r.pool, planningID, bookingID)
}

func (r *PostgresRepository) getBooking(ctx context.Context, q queryer, planningID uuid.UUID, bookingID uuid.UUID) (Booking, error) {
	booking, err := scanBooking(q.QueryRow(ctx, `SELECT `+bookingColumns+` FROM public.bookings WHERE id = $1 AND planning_id = $2`, bookingID, planningID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Booking{}, ErrBookingNotFound
		}
		return Booking{}, err
	}
	booking.ParticipantUserIDs, err = listParticipants(ctx, q, booking.ID)
	if err != nil {
		return Booking{}, err
	}
	if err := applyCosts(ctx, q, []*Booking{&booking}); err != nil {
		return Booking{}, err
	}
	return booking, nil
}

func (r *PostgresRepository) ListBookings(ctx context.Context, planningID uuid.UUID, filters BookingFilters) (PageResult[Booking], error) {
	countQuery := `SELECT count(*) FROM public.bookings WHERE planning_id = $1`
	listQuery := `SELECT ` + bookingColumns + ` FROM public.bookings WHERE planning_id = $1`

	args := []any{planningID}
	argIndex := 2

	if filters.Status != nil {
		countQuery += fmt.Sprintf(" AND status = $%d", argIndex)
		listQuery += fmt.Sprintf(" AND status = $%d", argIndex)
		args = append(args, *filters.Status)
		argIndex++
	}

	if filters.Type != nil {
		countQuery += fmt.Sprintf(" AND type = $%d", argIndex)
		listQuery += fmt.Sprintf(" AND type = $%d", argIndex)
		args = append(args, *filters.Type)
		argIndex++
	}

	var total int
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return PageResult[Booking]{}, err
	}

	page := filters.Page
	offset := (page.Number - 1) * page.Size
	listQuery += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argIndex, argIndex+1)
	args = append(args, page.Size, offset)

	rows, err := r.pool.Query(ctx, listQuery, args...)
	if err != nil {
		return PageResult[Booking]{}, err
	}
	defer rows.Close()

	bookings := make([]Booking, 0)
	bookingIDs := make([]uuid.UUID, 0, page.Size)
	for rows.Next() {
		booking, scanErr := scanBooking(rows)
		if scanErr != nil {
			return PageResult[Booking]{}, scanErr
		}
		bookings = append(bookings, booking)
		bookingIDs = append(bookingIDs, booking.ID)
	}

	if rows.Err() != nil {
		return PageResult[Booking]{}, rows.Err()
	}
	rows.Close()

	participants, err := r.listParticipantsForBookings(ctx, bookingIDs)
	if err != nil {
		return PageResult[Booking]{}, err
	}
	pointers := make([]*Booking, 0, len(bookings))
	for index := range bookings {
		bookings[index].ParticipantUserIDs = participants[bookings[index].ID]
		pointers = append(pointers, &bookings[index])
	}
	if err := applyCosts(ctx, r.pool, pointers); err != nil {
		return PageResult[Booking]{}, err
	}

	return newPageResult(bookings, page, total), nil
}

func (r *PostgresRepository) UpdateBooking(ctx context.Context, planningID uuid.UUID, bookingID uuid.UUID, input UpdateBookingInput) (Booking, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Booking{}, err
	}
	defer tx.Rollback(ctx)

	booking, err := scanBooking(tx.QueryRow(ctx, `
		UPDATE public.bookings
		SET
			title = COALESCE($3, title),
			type = COALESCE($4, type),
			status = COALESCE($5, status),
			external_url = COALESCE($6, external_url),
			occurs_at = COALESCE($7, occurs_at),
			location = COALESCE($8, location),
			notes = COALESCE($9, notes),
			updated_at = now(),
			route_stop_id = CASE
				WHEN $10::boolean THEN CASE WHEN $11::uuid IS NULL THEN NULL ELSE (SELECT id FROM public.planning_route_stops WHERE id = $11::uuid AND planning_id = $2) END
				ELSE route_stop_id
			END
		WHERE id = $1 AND planning_id = $2
		RETURNING `+bookingColumns,
		bookingID, planningID, input.Title, input.Type, input.Status, input.ExternalURL, input.OccursAt, input.Location, input.Notes, input.RouteStopIDSet, input.RouteStopID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Booking{}, ErrBookingNotFound
		}
		return Booking{}, err
	}

	if input.ParticipantUserIDs != nil {
		if err := replaceParticipants(ctx, tx, booking.ID, *input.ParticipantUserIDs); err != nil {
			return Booking{}, err
		}
	}
	booking.ParticipantUserIDs, err = listParticipants(ctx, tx, booking.ID)
	if err != nil {
		return Booking{}, err
	}
	if err := syncExpense(ctx, tx, booking, booking.CreatedByUserID, input.CostCents, input.CostDistribution, input.Payments); err != nil {
		return Booking{}, err
	}
	if err := applyCosts(ctx, tx, []*Booking{&booking}); err != nil {
		return Booking{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Booking{}, err
	}
	return booking, nil
}

func (r *PostgresRepository) ConfirmBooking(ctx context.Context, planningID uuid.UUID, bookingID uuid.UUID) (Booking, error) {
	tag, err := r.pool.Exec(ctx, `UPDATE public.bookings SET status = $3, updated_at = now() WHERE id = $1 AND planning_id = $2`, bookingID, planningID, StatusConfirmed)
	if err != nil {
		return Booking{}, err
	}
	if tag.RowsAffected() == 0 {
		return Booking{}, ErrBookingNotFound
	}
	return r.GetBookingByID(ctx, planningID, bookingID)
}

// DeleteBooking relies on a trigger that removes the planning item and, by cascade, its expenses.
func (r *PostgresRepository) DeleteBooking(ctx context.Context, planningID uuid.UUID, bookingID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM public.bookings WHERE id = $1 AND planning_id = $2`, bookingID, planningID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrBookingNotFound
	}
	return nil
}

type bookingScanner interface {
	Scan(dest ...any) error
}

func scanBooking(scanner bookingScanner) (Booking, error) {
	booking := Booking{}
	err := scanner.Scan(
		&booking.ID,
		&booking.PlanningID,
		&booking.CreatedByUserID,
		&booking.Title,
		&booking.Type,
		&booking.Status,
		&booking.ExternalURL,
		&booking.OccursAt,
		&booking.Location,
		&booking.Notes,
		&booking.RouteStopID,
		&booking.PlanningItemID,
		&booking.CreatedAt,
		&booking.UpdatedAt,
	)
	return booking, err
}

func replaceParticipants(ctx context.Context, q queryer, bookingID uuid.UUID, userIDs []uuid.UUID) error {
	if _, err := q.Exec(ctx, `DELETE FROM public.booking_participants WHERE booking_id = $1`, bookingID); err != nil {
		return err
	}
	for _, userID := range userIDs {
		if _, err := q.Exec(ctx, `INSERT INTO public.booking_participants (booking_id, user_id) VALUES ($1, $2)`, bookingID, userID); err != nil {
			return err
		}
	}
	return nil
}

func listParticipants(ctx context.Context, q queryer, bookingID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := q.Query(ctx, `SELECT user_id FROM public.booking_participants WHERE booking_id = $1 ORDER BY user_id`, bookingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]uuid.UUID, 0)
	for rows.Next() {
		var userID uuid.UUID
		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}
		result = append(result, userID)
	}
	return result, rows.Err()
}

func (r *PostgresRepository) listParticipantsForBookings(ctx context.Context, bookingIDs []uuid.UUID) (map[uuid.UUID][]uuid.UUID, error) {
	result := make(map[uuid.UUID][]uuid.UUID, len(bookingIDs))
	if len(bookingIDs) == 0 {
		return result, nil
	}
	rows, err := r.pool.Query(ctx, `SELECT booking_id, user_id FROM public.booking_participants WHERE booking_id = ANY($1) ORDER BY booking_id, user_id`, bookingIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var bookingID, userID uuid.UUID
		if err := rows.Scan(&bookingID, &userID); err != nil {
			return nil, err
		}
		result[bookingID] = append(result[bookingID], userID)
	}
	return result, rows.Err()
}

func newPageResult[T any](items []T, page Page, total int) PageResult[T] {
	totalPages := 0
	if total > 0 {
		totalPages = (total + page.Size - 1) / page.Size
	}

	return PageResult[T]{
		Items:      items,
		Page:       page.Number,
		PageSize:   page.Size,
		Total:      total,
		TotalPages: totalPages,
	}
}
