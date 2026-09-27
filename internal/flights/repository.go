package flights

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	Create(ctx context.Context, actor, planning uuid.UUID, input CreateInput) (Flight, error)
	Get(ctx context.Context, planning, id uuid.UUID) (Flight, error)
	List(ctx context.Context, planning uuid.UUID, filters Filters) ([]Flight, error)
	Update(ctx context.Context, planning, id uuid.UUID, input UpdateInput) (Flight, error)
	Confirm(ctx context.Context, planning, id uuid.UUID) (Flight, error)
	Delete(ctx context.Context, planning, id uuid.UUID) error
}

type PostgresRepository struct{ pool *pgxpool.Pool }

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

const flightColumns = `id, planning_id, created_by_user_id, status, airline, flight_number, reservation_code, notes, offer_url, cabin_class, baggage, cost_cents, currency, cost_distribution, created_at, updated_at`

func (r *PostgresRepository) Create(ctx context.Context, actor, planning uuid.UUID, input CreateInput) (Flight, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Flight{}, err
	}
	defer tx.Rollback(ctx)
	flight, err := scanFlight(tx.QueryRow(ctx, `INSERT INTO public.flights (planning_id, created_by_user_id, status, airline, flight_number, reservation_code, notes, offer_url, cabin_class, baggage, cost_cents, currency, cost_distribution) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13) RETURNING `+flightColumns, planning, actor, input.Status, input.Airline, input.FlightNumber, input.ReservationCode, input.Notes, input.OfferURL, input.CabinClass, input.Baggage, input.CostCents, input.Currency, input.CostDistribution))
	if err != nil {
		return Flight{}, err
	}
	if err = replaceSegments(ctx, tx, flight.ID, input.Segments); err != nil {
		return Flight{}, err
	}
	if err = replaceParticipants(ctx, tx, flight.ID, input.ParticipantUserIDs); err != nil {
		return Flight{}, err
	}
	flight.Segments, err = listSegments(ctx, tx, flight.ID)
	if err != nil {
		return Flight{}, err
	}
	flight.ParticipantUserIDs, err = listParticipants(ctx, tx, flight.ID)
	if err != nil {
		return Flight{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Flight{}, err
	}
	return flight, nil
}

func (r *PostgresRepository) Get(ctx context.Context, planning, id uuid.UUID) (Flight, error) {
	return r.get(ctx, r.pool, planning, id)
}
func (r *PostgresRepository) get(ctx context.Context, q queryer, planning, id uuid.UUID) (Flight, error) {
	flight, err := scanFlight(q.QueryRow(ctx, `SELECT `+flightColumns+` FROM public.flights WHERE planning_id=$1 AND id=$2`, planning, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Flight{}, ErrFlightNotFound
	}
	if err != nil {
		return Flight{}, err
	}
	flight.Segments, err = listSegments(ctx, q, flight.ID)
	if err != nil {
		return Flight{}, err
	}
	flight.ParticipantUserIDs, err = listParticipants(ctx, q, flight.ID)
	return flight, err
}

func (r *PostgresRepository) List(ctx context.Context, planning uuid.UUID, filters Filters) ([]Flight, error) {
	args := []any{planning}
	query := `SELECT ` + flightColumns + ` FROM public.flights WHERE planning_id=$1`
	if filters.Status != nil {
		query += ` AND status=$2`
		args = append(args, *filters.Status)
	}
	query += ` ORDER BY created_at DESC`
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Flight{}
	flightIDs := make([]uuid.UUID, 0)
	for rows.Next() {
		flight, scanErr := scanFlight(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, flight)
		flightIDs = append(flightIDs, flight.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	segments, err := listSegmentsForFlights(ctx, r.pool, flightIDs)
	if err != nil {
		return nil, err
	}
	participants, err := listParticipantsForFlights(ctx, r.pool, flightIDs)
	if err != nil {
		return nil, err
	}
	for index := range result {
		result[index].Segments = segments[result[index].ID]
		result[index].ParticipantUserIDs = participants[result[index].ID]
	}
	return result, nil
}

func (r *PostgresRepository) Update(ctx context.Context, planning, id uuid.UUID, input UpdateInput) (Flight, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Flight{}, err
	}
	defer tx.Rollback(ctx)
	flight, err := scanFlight(tx.QueryRow(ctx, `UPDATE public.flights SET status=COALESCE($3,status),airline=COALESCE($4,airline),flight_number=COALESCE($5,flight_number),reservation_code=COALESCE($6,reservation_code),notes=COALESCE($7,notes),offer_url=COALESCE($8,offer_url),cabin_class=COALESCE($9,cabin_class),baggage=COALESCE($10,baggage),cost_cents=COALESCE($11,cost_cents),currency=COALESCE($12,currency),cost_distribution=COALESCE($13,cost_distribution),updated_at=now() WHERE planning_id=$1 AND id=$2 RETURNING `+flightColumns, planning, id, input.Status, input.Airline, input.FlightNumber, input.ReservationCode, input.Notes, input.OfferURL, input.CabinClass, input.Baggage, input.CostCents, input.Currency, input.CostDistribution))
	if errors.Is(err, pgx.ErrNoRows) {
		return Flight{}, ErrFlightNotFound
	}
	if err != nil {
		return Flight{}, err
	}
	if input.Segments != nil {
		if err = replaceSegments(ctx, tx, id, *input.Segments); err != nil {
			return Flight{}, err
		}
	}
	if input.ParticipantUserIDs != nil {
		if err = replaceParticipants(ctx, tx, id, *input.ParticipantUserIDs); err != nil {
			return Flight{}, err
		}
	}
	flight.Segments, err = listSegments(ctx, tx, id)
	if err != nil {
		return Flight{}, err
	}
	flight.ParticipantUserIDs, err = listParticipants(ctx, tx, id)
	if err != nil {
		return Flight{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Flight{}, err
	}
	return flight, nil
}
func (r *PostgresRepository) Confirm(ctx context.Context, planning, id uuid.UUID) (Flight, error) {
	status := StatusConfirmed
	return r.Update(ctx, planning, id, UpdateInput{Status: &status})
}
func (r *PostgresRepository) Delete(ctx context.Context, planning, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM public.flights WHERE planning_id=$1 AND id=$2`, planning, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrFlightNotFound
	}
	return nil
}

type queryer interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

func scanFlight(row interface{ Scan(...any) error }) (Flight, error) {
	var f Flight
	err := row.Scan(&f.ID, &f.PlanningID, &f.CreatedByUserID, &f.Status, &f.Airline, &f.FlightNumber, &f.ReservationCode, &f.Notes, &f.OfferURL, &f.CabinClass, &f.Baggage, &f.CostCents, &f.Currency, &f.CostDistribution, &f.CreatedAt, &f.UpdatedAt)
	return f, err
}
func replaceParticipants(ctx context.Context, q queryer, id uuid.UUID, ids []uuid.UUID) error {
	if _, err := q.Exec(ctx, `DELETE FROM public.flight_participants WHERE flight_id=$1`, id); err != nil {
		return err
	}
	for _, userID := range ids {
		if _, err := q.Exec(ctx, `INSERT INTO public.flight_participants(flight_id,user_id) VALUES($1,$2)`, id, userID); err != nil {
			return err
		}
	}
	return nil
}
func listParticipants(ctx context.Context, q queryer, id uuid.UUID) ([]uuid.UUID, error) {
	rows, err := q.Query(ctx, `SELECT user_id FROM public.flight_participants WHERE flight_id=$1 ORDER BY user_id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []uuid.UUID{}
	for rows.Next() {
		var uid uuid.UUID
		if err := rows.Scan(&uid); err != nil {
			return nil, err
		}
		result = append(result, uid)
	}
	return result, rows.Err()
}

func listParticipantsForFlights(ctx context.Context, q queryer, ids []uuid.UUID) (map[uuid.UUID][]uuid.UUID, error) {
	result := make(map[uuid.UUID][]uuid.UUID, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	rows, err := q.Query(ctx, `SELECT flight_id, user_id FROM public.flight_participants WHERE flight_id = ANY($1) ORDER BY flight_id, user_id`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var flightID, userID uuid.UUID
		if err := rows.Scan(&flightID, &userID); err != nil {
			return nil, err
		}
		result[flightID] = append(result[flightID], userID)
	}
	return result, rows.Err()
}
func replaceSegments(ctx context.Context, q queryer, id uuid.UUID, segments []Segment) error {
	if _, err := q.Exec(ctx, `DELETE FROM public.flight_segments WHERE flight_id=$1`, id); err != nil {
		return err
	}
	for _, s := range segments {
		if _, err := q.Exec(ctx, `INSERT INTO public.flight_segments(flight_id,direction,position,origin_label,origin_city,origin_airport_name,origin_airport_code,origin_mapbox_id,destination_label,destination_city,destination_airport_name,destination_airport_code,destination_mapbox_id,departure_at,arrival_at,departure_terminal,arrival_terminal,airline,flight_number,origin_timezone,destination_timezone) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21)`, id, s.Direction, s.Position, s.OriginLabel, s.OriginCity, s.OriginAirportName, s.OriginAirportCode, s.OriginMapboxID, s.DestinationLabel, s.DestinationCity, s.DestinationAirportName, s.DestinationAirportCode, s.DestinationMapboxID, s.DepartureAt, s.ArrivalAt, s.DepartureTerminal, s.ArrivalTerminal, s.Airline, s.FlightNumber, s.OriginTimezone, s.DestinationTimezone); err != nil {
			return err
		}
	}
	return nil
}
func listSegments(ctx context.Context, q queryer, id uuid.UUID) ([]Segment, error) {
	rows, err := q.Query(ctx, `SELECT id,direction,position,origin_label,origin_city,origin_airport_name,origin_airport_code,origin_mapbox_id,destination_label,destination_city,destination_airport_name,destination_airport_code,destination_mapbox_id,departure_at,arrival_at,departure_terminal,arrival_terminal,airline,flight_number,origin_timezone,destination_timezone FROM public.flight_segments WHERE flight_id=$1 ORDER BY direction,position`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Segment{}
	for rows.Next() {
		var s Segment
		if err := rows.Scan(&s.ID, &s.Direction, &s.Position, &s.OriginLabel, &s.OriginCity, &s.OriginAirportName, &s.OriginAirportCode, &s.OriginMapboxID, &s.DestinationLabel, &s.DestinationCity, &s.DestinationAirportName, &s.DestinationAirportCode, &s.DestinationMapboxID, &s.DepartureAt, &s.ArrivalAt, &s.DepartureTerminal, &s.ArrivalTerminal, &s.Airline, &s.FlightNumber, &s.OriginTimezone, &s.DestinationTimezone); err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

func listSegmentsForFlights(ctx context.Context, q queryer, ids []uuid.UUID) (map[uuid.UUID][]Segment, error) {
	result := make(map[uuid.UUID][]Segment, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	rows, err := q.Query(ctx, `SELECT flight_id,id,direction,position,origin_label,origin_city,origin_airport_name,origin_airport_code,origin_mapbox_id,destination_label,destination_city,destination_airport_name,destination_airport_code,destination_mapbox_id,departure_at,arrival_at,departure_terminal,arrival_terminal,airline,flight_number,origin_timezone,destination_timezone FROM public.flight_segments WHERE flight_id = ANY($1) ORDER BY flight_id, direction, position`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var flightID uuid.UUID
		var segment Segment
		if err := rows.Scan(&flightID, &segment.ID, &segment.Direction, &segment.Position, &segment.OriginLabel, &segment.OriginCity, &segment.OriginAirportName, &segment.OriginAirportCode, &segment.OriginMapboxID, &segment.DestinationLabel, &segment.DestinationCity, &segment.DestinationAirportName, &segment.DestinationAirportCode, &segment.DestinationMapboxID, &segment.DepartureAt, &segment.ArrivalAt, &segment.DepartureTerminal, &segment.ArrivalTerminal, &segment.Airline, &segment.FlightNumber, &segment.OriginTimezone, &segment.DestinationTimezone); err != nil {
			return nil, err
		}
		result[flightID] = append(result[flightID], segment)
	}
	return result, rows.Err()
}
