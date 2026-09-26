package bookings

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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

func (r *PostgresRepository) CreateBooking(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, input CreateBookingInput) (Booking, error) {
	booking, err := scanBooking(r.pool.QueryRow(ctx, `
		INSERT INTO public.bookings (
			id, planning_id, created_by_user_id, title, type, status, external_url, occurs_at, location, notes, cost_cents, cost_distribution, route_stop_id
		)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, (SELECT id FROM public.planning_route_stops WHERE id = $12 AND planning_id = $1))
		RETURNING id, planning_id, created_by_user_id, title, type, status, external_url, occurs_at, location, notes, cost_cents, cost_distribution, route_stop_id, created_at, updated_at
	`, planningID, actorUserID, input.Title, input.Type, input.Status, input.ExternalURL, input.OccursAt, input.Location, input.Notes, input.CostCents, input.CostDistribution, input.RouteStopID))
	if err != nil {
		return Booking{}, err
	}

	if err := r.replaceParticipants(ctx, booking.ID, input.ParticipantUserIDs); err != nil {
		return Booking{}, err
	}
	booking.ParticipantUserIDs = input.ParticipantUserIDs
	if err := r.replacePayments(ctx, booking.ID, input.Payments); err != nil {
		return Booking{}, err
	}
	booking.Payments = input.Payments
	return booking, nil
}

func (r *PostgresRepository) GetBookingByID(ctx context.Context, planningID uuid.UUID, bookingID uuid.UUID) (Booking, error) {
	booking, err := scanBooking(r.pool.QueryRow(ctx, `
		SELECT id, planning_id, created_by_user_id, title, type, status, external_url, occurs_at, location, notes, cost_cents, cost_distribution, route_stop_id, created_at, updated_at
		FROM public.bookings
		WHERE id = $1 AND planning_id = $2
	`, bookingID, planningID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Booking{}, ErrBookingNotFound
		}

		return Booking{}, err
	}

	booking.ParticipantUserIDs, err = r.listParticipants(ctx, booking.ID)
	if err != nil {
		return Booking{}, err
	}
	booking.Payments, err = r.listPayments(ctx, booking.ID)
	if err != nil {
		return Booking{}, err
	}
	return booking, nil
}

func (r *PostgresRepository) ListBookings(ctx context.Context, planningID uuid.UUID, filters BookingFilters) (PageResult[Booking], error) {
	countQuery := `SELECT count(*) FROM public.bookings WHERE planning_id = $1`
	listQuery := `
		SELECT id, planning_id, created_by_user_id, title, type, status, external_url, occurs_at, location, notes, cost_cents, cost_distribution, route_stop_id, created_at, updated_at
		FROM public.bookings
		WHERE planning_id = $1
	`

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
	for rows.Next() {
		booking, scanErr := scanBooking(rows)
		if scanErr != nil {
			return PageResult[Booking]{}, scanErr
		}
		booking.ParticipantUserIDs, scanErr = r.listParticipants(ctx, booking.ID)
		if scanErr != nil {
			return PageResult[Booking]{}, scanErr
		}
		booking.Payments, scanErr = r.listPayments(ctx, booking.ID)
		if scanErr != nil {
			return PageResult[Booking]{}, scanErr
		}
		bookings = append(bookings, booking)
	}

	if rows.Err() != nil {
		return PageResult[Booking]{}, rows.Err()
	}

	return newPageResult(bookings, page, total), nil
}

func (r *PostgresRepository) UpdateBooking(ctx context.Context, planningID uuid.UUID, bookingID uuid.UUID, input UpdateBookingInput) (Booking, error) {
	booking, err := scanBooking(r.pool.QueryRow(ctx, `
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
			cost_cents = COALESCE($10, cost_cents),
			cost_distribution = COALESCE($11, cost_distribution),
			route_stop_id = CASE WHEN $12::uuid IS NULL THEN NULL ELSE (SELECT id FROM public.planning_route_stops WHERE id = $12::uuid AND planning_id = $2) END
		WHERE id = $1 AND planning_id = $2
			RETURNING id, planning_id, created_by_user_id, title, type, status, external_url, occurs_at, location, notes, cost_cents, cost_distribution, route_stop_id, created_at, updated_at
	`, bookingID, planningID, input.Title, input.Type, input.Status, input.ExternalURL, input.OccursAt, input.Location, input.Notes, input.CostCents, input.CostDistribution, input.RouteStopID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Booking{}, ErrBookingNotFound
		}

		return Booking{}, err
	}

	if input.ParticipantUserIDs != nil {
		if err := r.replaceParticipants(ctx, booking.ID, *input.ParticipantUserIDs); err != nil {
			return Booking{}, err
		}
		booking.ParticipantUserIDs = *input.ParticipantUserIDs
	} else {
		booking.ParticipantUserIDs, err = r.listParticipants(ctx, booking.ID)
		if err != nil {
			return Booking{}, err
		}
	}
	if input.Payments != nil {
		if err := r.replacePayments(ctx, booking.ID, *input.Payments); err != nil {
			return Booking{}, err
		}
		booking.Payments = *input.Payments
	} else {
		booking.Payments, err = r.listPayments(ctx, booking.ID)
		if err != nil {
			return Booking{}, err
		}
	}
	return booking, nil
}

func (r *PostgresRepository) ConfirmBooking(ctx context.Context, planningID uuid.UUID, bookingID uuid.UUID) (Booking, error) {
	booking, err := scanBooking(r.pool.QueryRow(ctx, `
		UPDATE public.bookings
		SET status = $3, updated_at = now()
		WHERE id = $1 AND planning_id = $2
		RETURNING id, planning_id, created_by_user_id, title, type, status, external_url, occurs_at, location, notes, cost_cents, cost_distribution, route_stop_id, created_at, updated_at
	`, bookingID, planningID, StatusConfirmed))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Booking{}, ErrBookingNotFound
		}

		return Booking{}, err
	}
	booking.ParticipantUserIDs, err = r.listParticipants(ctx, booking.ID)
	if err != nil {
		return Booking{}, err
	}
	booking.Payments, err = r.listPayments(ctx, booking.ID)
	if err != nil {
		return Booking{}, err
	}
	return booking, nil
}

func (r *PostgresRepository) DeleteBooking(ctx context.Context, planningID uuid.UUID, bookingID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM public.bookings
		WHERE id = $1 AND planning_id = $2
	`, bookingID, planningID)
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
		&booking.CostCents,
		&booking.CostDistribution,
		&booking.RouteStopID,
		&booking.CreatedAt,
		&booking.UpdatedAt,
	)
	return booking, err
}

func (r *PostgresRepository) replaceParticipants(ctx context.Context, bookingID uuid.UUID, userIDs []uuid.UUID) error {
	if _, err := r.pool.Exec(ctx, `DELETE FROM public.booking_participants WHERE booking_id = $1`, bookingID); err != nil {
		return err
	}
	for _, userID := range userIDs {
		if _, err := r.pool.Exec(ctx, `INSERT INTO public.booking_participants (booking_id, user_id) VALUES ($1, $2)`, bookingID, userID); err != nil {
			return err
		}
	}
	return nil
}

func (r *PostgresRepository) listParticipants(ctx context.Context, bookingID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx, `SELECT user_id FROM public.booking_participants WHERE booking_id = $1 ORDER BY user_id`, bookingID)
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

func (r *PostgresRepository) replacePayments(ctx context.Context, bookingID uuid.UUID, payments []Payment) error {
	if _, err := r.pool.Exec(ctx, `DELETE FROM public.booking_payments WHERE booking_id = $1`, bookingID); err != nil {
		return err
	}
	for _, payment := range payments {
		if _, err := r.pool.Exec(ctx, `INSERT INTO public.booking_payments (booking_id, user_id, amount_cents, paid_at) VALUES ($1, $2, $3, $4)`, bookingID, payment.UserID, payment.AmountCents, payment.PaidAt); err != nil {
			return err
		}
	}
	return nil
}

func (r *PostgresRepository) listPayments(ctx context.Context, bookingID uuid.UUID) ([]Payment, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, user_id, amount_cents, paid_at FROM public.booking_payments WHERE booking_id = $1 ORDER BY paid_at, id`, bookingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]Payment, 0)
	for rows.Next() {
		var payment Payment
		if err := rows.Scan(&payment.ID, &payment.UserID, &payment.AmountCents, &payment.PaidAt); err != nil {
			return nil, err
		}
		result = append(result, payment)
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
