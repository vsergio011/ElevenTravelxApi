package plannings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/eleventravel/eleventravel-api/internal/http/response"
)

type Repository interface {
	CreatePlanning(ctx context.Context, actorUserID uuid.UUID, input CreatePlanningInput) (Planning, error)
	GetPlanningByID(ctx context.Context, planningID uuid.UUID) (Planning, error)
	GetSummary(ctx context.Context, planningID uuid.UUID) (PlanningSummary, error)
	GetDashboardActivity(ctx context.Context, planningID uuid.UUID) (PlanningDashboardActivity, error)
	ListPlannings(ctx context.Context, filters PlanningFilters) (PageResult[Planning], error)
	GetMember(ctx context.Context, planningID uuid.UUID, userID uuid.UUID) (PlanningMember, error)
	EnsureOwnerMembership(ctx context.Context, planningID uuid.UUID, userID uuid.UUID) (PlanningMember, error)
	GetUserIDByEmail(ctx context.Context, email string) (uuid.UUID, error)
	ListMembers(ctx context.Context, planningID uuid.UUID) ([]PlanningMember, error)
	UpdatePlanning(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, input UpdatePlanningInput) (Planning, error)
	SetPlanningArchived(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, archived bool) (Planning, error)
	AddMember(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, input AddPlanningMemberInput) (PlanningMember, error)
	UpdateMemberRole(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, targetUserID uuid.UUID, role string) (PlanningMember, error)
	RemoveMember(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, targetUserID uuid.UUID) error
	ListActivity(ctx context.Context, planningID uuid.UUID, page Page) (PageResult[PlanningActivity], error)
	GetPlanningItinerary(ctx context.Context, planningID uuid.UUID) (PlanningItinerary, error)
	CreatePlanningRouteDay(ctx context.Context, planningID uuid.UUID, input CreatePlanningRouteDayInput) (PlanningItineraryDay, error)
	DeletePlanningRouteDay(ctx context.Context, planningID uuid.UUID, dayID uuid.UUID) error
	GetPlanningRouteByID(ctx context.Context, planningID uuid.UUID, routeID uuid.UUID) (PlanningItineraryStop, error)
	CreatePlanningRouteStop(ctx context.Context, planningID uuid.UUID, input CreatePlanningRouteStopInput) (PlanningItineraryStop, error)
	UpdatePlanningRouteStop(ctx context.Context, planningID uuid.UUID, routeID uuid.UUID, input UpdatePlanningRouteStopInput) (PlanningItineraryStop, error)
	DeletePlanningRouteStop(ctx context.Context, planningID uuid.UUID, routeID uuid.UUID) error
	ReorderPlanningRouteStops(ctx context.Context, planningID uuid.UUID, dayID uuid.UUID, stopIDs []uuid.UUID) error
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) CreatePlanning(ctx context.Context, actorUserID uuid.UUID, input CreatePlanningInput) (Planning, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Planning{}, err
	}
	defer tx.Rollback(ctx)

	planning := Planning{}
	err = tx.QueryRow(ctx, `
		INSERT INTO public.plannings (
			id, group_id, owner_user_id, name, description, destination_name, starts_at, ends_at, status, cover_image_url, is_archived
		)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7, $8, NULL, false)
		RETURNING id, group_id, owner_user_id, name, description, destination_name, starts_at, ends_at, status, cover_image_url, is_archived, created_at, updated_at
	`, input.GroupID, actorUserID, input.Name, input.Description, input.DestinationName, input.StartsAt, input.EndsAt, StatusDraft).Scan(
		&planning.ID,
		&planning.GroupID,
		&planning.OwnerUserID,
		&planning.Name,
		&planning.Description,
		&planning.DestinationName,
		&planning.StartsAt,
		&planning.EndsAt,
		&planning.Status,
		&planning.CoverImageURL,
		&planning.IsArchived,
		&planning.CreatedAt,
		&planning.UpdatedAt,
	)
	if err != nil {
		return Planning{}, err
	}

	member := AddPlanningMemberInput{UserID: actorUserID, Role: RoleOwner}
	if _, err := r.addMemberTx(ctx, tx, actorUserID, planning.ID, member); err != nil {
		return Planning{}, err
	}

	if err := r.insertActivityTx(ctx, tx, planning.ID, &actorUserID, "planning.created", "planning", &planning.ID, map[string]any{"name": planning.Name}); err != nil {
		return Planning{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Planning{}, err
	}

	return planning, nil
}

func (r *PostgresRepository) GetPlanningByID(ctx context.Context, planningID uuid.UUID) (Planning, error) {
	planning, err := scanPlanning(r.pool.QueryRow(ctx, `
		SELECT id, group_id, owner_user_id, name, description, destination_name, starts_at, ends_at, status, cover_image_url, is_archived, created_at, updated_at
		FROM public.plannings
		WHERE id = $1
	`, planningID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Planning{}, ErrPlanningNotFound
		}

		return Planning{}, err
	}

	return planning, nil
}

func (r *PostgresRepository) GetSummary(ctx context.Context, planningID uuid.UUID) (PlanningSummary, error) {
	if _, err := r.GetPlanningByID(ctx, planningID); err != nil {
		return PlanningSummary{}, err
	}

	var summary PlanningSummary
	summary.PlanningID = planningID

	var budgetRow PlanningActivity
	budgetErr := r.pool.QueryRow(ctx, `
		SELECT id, planning_id, actor_user_id, event_type, entity_type, entity_id, metadata, created_at
		FROM public.planning_activity
		WHERE planning_id = $1 AND event_type = 'dashboard.budget_snapshot'
		ORDER BY created_at DESC
		LIMIT 1
	`, planningID).Scan(
		&budgetRow.ID,
		&budgetRow.PlanningID,
		&budgetRow.ActorUserID,
		&budgetRow.EventType,
		&budgetRow.EntityType,
		&budgetRow.EntityID,
		&budgetRow.Metadata,
		&budgetRow.CreatedAt,
	)
	if budgetErr == nil {
		summary.Budget = &PlanningSummaryBudget{
			Target:    metadataInt(budgetRow.Metadata, "target"),
			Confirmed: metadataInt(budgetRow.Metadata, "confirmed"),
			Pending:   metadataInt(budgetRow.Metadata, "pending"),
		}
	} else if !errors.Is(budgetErr, pgx.ErrNoRows) {
		return PlanningSummary{}, budgetErr
	}

	err := r.pool.QueryRow(ctx, `
		SELECT
			count(*) FILTER (WHERE event_type LIKE 'flight.%')::int,
			count(*) FILTER (WHERE event_type LIKE 'booking.%')::int,
			count(*) FILTER (WHERE event_type LIKE 'expense.%')::int
		FROM public.planning_activity
		WHERE planning_id = $1
	`, planningID).Scan(&summary.Shortcuts.FlightsCount, &summary.Shortcuts.BookingsCount, &summary.Shortcuts.ExpensesCount)
	if err != nil {
		return PlanningSummary{}, err
	}

	return summary, nil
}

func (r *PostgresRepository) GetDashboardActivity(ctx context.Context, planningID uuid.UUID) (PlanningDashboardActivity, error) {
	activity := PlanningDashboardActivity{PlanningID: planningID}

	rows, err := r.pool.Query(ctx, `
		SELECT s.id, s.title, s.description, s.address, s.time_label, s.image_url, s.category,
			s.sort_order, d.sort_order, s.created_at, p.starts_at
		FROM public.planning_route_stops s
		JOIN public.planning_route_days d ON d.id = s.day_id AND d.planning_id = s.planning_id
		JOIN public.plannings p ON p.id = s.planning_id
		WHERE s.planning_id = $1 AND s.status NOT IN ('discarded', 'visited')
		ORDER BY d.sort_order ASC, s.sort_order ASC, s.created_at ASC
	`, planningID)
	if err != nil {
		return PlanningDashboardActivity{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			id               uuid.UUID
			title            string
			description      string
			address          string
			timeLabel        string
			imageURL         *string
			category         string
			stopSortOrder    int
			daySortOrder     int
			createdAt        time.Time
			planningStartsAt *time.Time
		)
		if scanErr := rows.Scan(
			&id,
			&title,
			&description,
			&address,
			&timeLabel,
			&imageURL,
			&category,
			&stopSortOrder,
			&daySortOrder,
			&createdAt,
			&planningStartsAt,
		); scanErr != nil {
			return PlanningDashboardActivity{}, scanErr
		}
		_ = stopSortOrder

		startsAt := routeStopStartsAt(planningStartsAt, daySortOrder, timeLabel, createdAt)

		activity.UpcomingActivities = append(activity.UpcomingActivities, PlanningUpcomingActivity{
			ID:        id,
			Title:     title,
			StartsAt:  startsAt,
			KindLabel: planningStringPointer(category),
			ImageURL:  imageURL,
		})

		if activity.NextStop == nil {
			activity.NextStop = &PlanningNextStop{
				ID:          id,
				Title:       title,
				TimeLabel:   planningStringPointer(timeLabel),
				Location:    planningStringPointer(address),
				Description: planningStringPointer(description),
				ImageURL:    imageURL,
			}
		}
	}

	if rows.Err() != nil {
		return PlanningDashboardActivity{}, rows.Err()
	}

	return activity, nil
}

func routeStopStartsAt(planningStartsAt *time.Time, daySortOrder int, timeLabel string, fallback time.Time) time.Time {
	base := fallback
	if planningStartsAt != nil {
		base = planningStartsAt.AddDate(0, 0, daySortOrder)
	}

	parsedTime, err := time.Parse("15:04", strings.TrimSpace(timeLabel))
	if err != nil {
		return base
	}

	return time.Date(base.Year(), base.Month(), base.Day(), parsedTime.Hour(), parsedTime.Minute(), 0, 0, base.Location())
}

func planningStringPointer(value string) *string {
	return &value
}

func (r *PostgresRepository) ListPlannings(ctx context.Context, filters PlanningFilters) (PageResult[Planning], error) {
	countQuery := `
		SELECT count(*)
		FROM public.plannings p
		LEFT JOIN public.planning_members pm ON pm.planning_id = p.id AND pm.user_id = $1
		WHERE pm.user_id = $1 OR p.owner_user_id = $1
	`
	listQuery := `
		SELECT p.id, p.group_id::text, p.owner_user_id, p.name, p.description, p.destination_name, p.starts_at, p.ends_at, p.status, p.cover_image_url, p.is_archived, p.created_at, p.updated_at
		FROM public.plannings p
		LEFT JOIN public.planning_members pm ON pm.planning_id = p.id AND pm.user_id = $1
		WHERE pm.user_id = $1 OR p.owner_user_id = $1
	`

	args := []any{filters.OnlyMember}
	argIndex := 2

	if filters.GroupID != nil {
		countQuery += fmt.Sprintf(" AND p.group_id::text = $%d::text", argIndex)
		listQuery += fmt.Sprintf(" AND p.group_id::text = $%d::text", argIndex)
		args = append(args, *filters.GroupID)
		argIndex++
	}

	if filters.Status != nil {
		countQuery += fmt.Sprintf(" AND p.status = $%d", argIndex)
		listQuery += fmt.Sprintf(" AND p.status = $%d", argIndex)
		args = append(args, *filters.Status)
		argIndex++
	}

	if filters.Archived != nil {
		countQuery += fmt.Sprintf(" AND p.is_archived = $%d", argIndex)
		listQuery += fmt.Sprintf(" AND p.is_archived = $%d", argIndex)
		args = append(args, *filters.Archived)
		argIndex++
	}

	var total int
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return PageResult[Planning]{}, err
	}

	page := filters.Page
	offset := (page.Number - 1) * page.Size
	listQuery += fmt.Sprintf(" ORDER BY p.created_at DESC LIMIT $%d OFFSET $%d", argIndex, argIndex+1)
	args = append(args, page.Size, offset)

	rows, err := r.pool.Query(ctx, listQuery, args...)
	if err != nil {
		return PageResult[Planning]{}, err
	}
	defer rows.Close()

	plannings := make([]Planning, 0)
	for rows.Next() {
		planning, scanErr := scanPlanning(rows)
		if scanErr != nil {
			return PageResult[Planning]{}, scanErr
		}
		plannings = append(plannings, planning)
	}

	if rows.Err() != nil {
		return PageResult[Planning]{}, rows.Err()
	}

	return newPageResult(plannings, page, total), nil
}

func (r *PostgresRepository) GetMember(ctx context.Context, planningID uuid.UUID, userID uuid.UUID) (PlanningMember, error) {
	member, err := scanPlanningMember(r.pool.QueryRow(ctx, `
		SELECT id, planning_id, user_id, role, invited_by_user_id, joined_at, created_at, updated_at
		FROM public.planning_members
		WHERE planning_id = $1 AND user_id = $2
	`, planningID, userID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PlanningMember{}, ErrMemberNotFound
		}

		return PlanningMember{}, err
	}

	return member, nil
}

// EnsureOwnerMembership repairs the invariant needed by the planning API when
// an owner membership was removed manually. It is safe to call repeatedly.
func (r *PostgresRepository) EnsureOwnerMembership(ctx context.Context, planningID uuid.UUID, userID uuid.UUID) (PlanningMember, error) {
	return scanPlanningMember(r.pool.QueryRow(ctx, `
		INSERT INTO public.planning_members (id, planning_id, user_id, role, invited_by_user_id, joined_at)
		SELECT gen_random_uuid(), p.id, p.owner_user_id, 'owner', p.owner_user_id, now()
		FROM public.plannings p
		WHERE p.id = $1 AND p.owner_user_id = $2
		ON CONFLICT (planning_id, user_id) DO UPDATE SET role = 'owner'
		RETURNING id, planning_id, user_id, role, invited_by_user_id, joined_at, created_at, updated_at
	`, planningID, userID))
}

func (r *PostgresRepository) ListMembers(ctx context.Context, planningID uuid.UUID) ([]PlanningMember, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT pm.id, pm.planning_id, pm.user_id, up.username, up.full_name, up.avatar_url,
			pm.role, pm.invited_by_user_id, pm.joined_at, pm.created_at, pm.updated_at
		FROM public.planning_members pm
		LEFT JOIN public.user_profiles up ON up.user_id = pm.user_id
		WHERE pm.planning_id = $1
		ORDER BY pm.created_at ASC
	`, planningID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	members := make([]PlanningMember, 0)
	for rows.Next() {
		member := PlanningMember{}
		scanErr := rows.Scan(
			&member.ID,
			&member.PlanningID,
			&member.UserID,
			&member.Username,
			&member.FullName,
			&member.AvatarURL,
			&member.Role,
			&member.InvitedByUserID,
			&member.JoinedAt,
			&member.CreatedAt,
			&member.UpdatedAt,
		)
		if scanErr != nil {
			return nil, scanErr
		}
		members = append(members, member)
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return members, nil
}

func (r *PostgresRepository) UpdatePlanning(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, input UpdatePlanningInput) (Planning, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Planning{}, err
	}
	defer tx.Rollback(ctx)

	planning, err := scanPlanning(tx.QueryRow(ctx, `
		UPDATE public.plannings
		SET
			name = COALESCE($2, name),
			description = COALESCE($3, description),
			destination_name = COALESCE($4, destination_name),
			starts_at = COALESCE($5, starts_at),
			ends_at = COALESCE($6, ends_at),
			status = COALESCE($7, status),
			cover_image_url = COALESCE($8, cover_image_url),
			is_archived = COALESCE($9, is_archived),
			updated_at = now()
		WHERE id = $1
		RETURNING id, group_id, owner_user_id, name, description, destination_name, starts_at, ends_at, status, cover_image_url, is_archived, created_at, updated_at
	`, planningID, input.Name, input.Description, input.DestinationName, input.StartsAt, input.EndsAt, input.Status, input.CoverImageURL, input.IsArchived))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Planning{}, ErrPlanningNotFound
		}

		return Planning{}, err
	}

	if err := r.insertActivityTx(ctx, tx, planningID, &actorUserID, "planning.updated", "planning", &planningID, map[string]any{"status": planning.Status, "is_archived": planning.IsArchived}); err != nil {
		return Planning{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Planning{}, err
	}

	return planning, nil
}

func (r *PostgresRepository) SetPlanningArchived(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, archived bool) (Planning, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Planning{}, err
	}
	defer tx.Rollback(ctx)

	planning, err := scanPlanning(tx.QueryRow(ctx, `
		UPDATE public.plannings
		SET is_archived = $2, updated_at = now()
		WHERE id = $1
		RETURNING id, group_id, owner_user_id, name, description, destination_name, starts_at, ends_at, status, cover_image_url, is_archived, created_at, updated_at
	`, planningID, archived))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Planning{}, ErrPlanningNotFound
		}

		return Planning{}, err
	}

	eventType := "planning.archived"
	if !archived {
		eventType = "planning.unarchived"
	}

	if err := r.insertActivityTx(ctx, tx, planningID, &actorUserID, eventType, "planning", &planningID, map[string]any{"is_archived": archived}); err != nil {
		return Planning{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Planning{}, err
	}

	return planning, nil
}

func (r *PostgresRepository) AddMember(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, input AddPlanningMemberInput) (PlanningMember, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return PlanningMember{}, err
	}
	defer tx.Rollback(ctx)

	member, err := r.addMemberTx(ctx, tx, actorUserID, planningID, input)
	if err != nil {
		return PlanningMember{}, err
	}

	if err := r.insertActivityTx(ctx, tx, planningID, &actorUserID, "member.added", "member", &input.UserID, map[string]any{"role": input.Role}); err != nil {
		return PlanningMember{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return PlanningMember{}, err
	}

	return member, nil
}

func (r *PostgresRepository) UpdateMemberRole(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, targetUserID uuid.UUID, role string) (PlanningMember, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return PlanningMember{}, err
	}
	defer tx.Rollback(ctx)

	member, err := scanPlanningMember(tx.QueryRow(ctx, `
		UPDATE public.planning_members
		SET role = $3, updated_at = now()
		WHERE planning_id = $1 AND user_id = $2
		RETURNING id, planning_id, user_id, role, invited_by_user_id, joined_at, created_at, updated_at
	`, planningID, targetUserID, role))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PlanningMember{}, ErrMemberNotFound
		}

		return PlanningMember{}, err
	}

	if err := r.insertActivityTx(ctx, tx, planningID, &actorUserID, "member.role_changed", "member", &targetUserID, map[string]any{"role": role}); err != nil {
		return PlanningMember{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return PlanningMember{}, err
	}

	return member, nil
}

func (r *PostgresRepository) RemoveMember(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, targetUserID uuid.UUID) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	commandTag, err := tx.Exec(ctx, `
		DELETE FROM public.planning_members
		WHERE planning_id = $1 AND user_id = $2
	`, planningID, targetUserID)
	if err != nil {
		return err
	}

	if commandTag.RowsAffected() == 0 {
		return ErrMemberNotFound
	}

	if err := r.insertActivityTx(ctx, tx, planningID, &actorUserID, "member.removed", "member", &targetUserID, map[string]any{}); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *PostgresRepository) ListActivity(ctx context.Context, planningID uuid.UUID, page Page) (PageResult[PlanningActivity], error) {
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM public.planning_activity WHERE planning_id = $1`, planningID).Scan(&total); err != nil {
		return PageResult[PlanningActivity]{}, err
	}

	offset := (page.Number - 1) * page.Size
	rows, err := r.pool.Query(ctx, `
		SELECT id, planning_id, actor_user_id, event_type, entity_type, entity_id, metadata, created_at
		FROM public.planning_activity
		WHERE planning_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, planningID, page.Size, offset)
	if err != nil {
		return PageResult[PlanningActivity]{}, err
	}
	defer rows.Close()

	activities := make([]PlanningActivity, 0)
	for rows.Next() {
		activity, scanErr := scanPlanningActivity(rows)
		if scanErr != nil {
			return PageResult[PlanningActivity]{}, scanErr
		}
		activities = append(activities, activity)
	}

	if rows.Err() != nil {
		return PageResult[PlanningActivity]{}, rows.Err()
	}

	return newPageResult(activities, page, total), nil
}

func (r *PostgresRepository) GetPlanningItinerary(ctx context.Context, planningID uuid.UUID) (PlanningItinerary, error) {
	itinerary := PlanningItinerary{PlanningID: planningID}

	planningRow := r.pool.QueryRow(ctx, `SELECT name FROM public.plannings WHERE id = $1`, planningID)
	if err := planningRow.Scan(&itinerary.PlanningName); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PlanningItinerary{}, ErrPlanningNotFound
		}
		return PlanningItinerary{}, err
	}

	dayRows, err := r.pool.Query(ctx, `
		SELECT id, label, date_label
		FROM public.planning_route_days
		WHERE planning_id = $1
		ORDER BY sort_order ASC, created_at ASC
	`, planningID)
	if err != nil {
		return PlanningItinerary{}, err
	}
	defer dayRows.Close()

	for dayRows.Next() {
		day := PlanningItineraryDay{}
		if err := dayRows.Scan(&day.ID, &day.Label, &day.DateLabel); err != nil {
			return PlanningItinerary{}, err
		}

		stopRows, err := r.pool.Query(ctx, `
			SELECT id, title, description, address, time_label, duration_minutes, notes, cost_estimate, currency, external_url, image_url, status, category, is_optional, latitude, longitude
			FROM public.planning_route_stops
			WHERE planning_id = $1 AND day_id = $2
			ORDER BY sort_order ASC, created_at ASC
		`, planningID, day.ID)
		if err != nil {
			return PlanningItinerary{}, err
		}

		for stopRows.Next() {
			stop := PlanningItineraryStop{}
			var imageURL *string
			if err := stopRows.Scan(
				&stop.ID,
				&stop.Title,
				&stop.Description,
				&stop.Address,
				&stop.TimeLabel,
				&stop.DurationMinutes,
				&stop.Notes,
				&stop.CostEstimate,
				&stop.Currency,
				&stop.ExternalURL,
				&imageURL,
				&stop.Status,
				&stop.Category,
				&stop.IsOptional,
				&stop.Coordinates.Latitude,
				&stop.Coordinates.Longitude,
			); err != nil {
				stopRows.Close()
				return PlanningItinerary{}, err
			}
			stop.ImageURL = imageURL
			stop.Bookings, err = r.listRouteStopBookings(ctx, stop.ID)
			if err != nil {
				stopRows.Close()
				return PlanningItinerary{}, err
			}
			day.Stops = append(day.Stops, stop)
		}
		if err := stopRows.Err(); err != nil {
			stopRows.Close()
			return PlanningItinerary{}, err
		}
		stopRows.Close()

		itinerary.Days = append(itinerary.Days, day)
	}
	if err := dayRows.Err(); err != nil {
		return PlanningItinerary{}, err
	}

	return itinerary, nil
}

func (r *PostgresRepository) CreatePlanningRouteDay(ctx context.Context, planningID uuid.UUID, input CreatePlanningRouteDayInput) (PlanningItineraryDay, error) {
	day := PlanningItineraryDay{}
	err := r.pool.QueryRow(ctx, `
		WITH next_order AS (
			SELECT COALESCE(MAX(sort_order), -1) + 1 AS value
			FROM public.planning_route_days
			WHERE planning_id = $1
		)
		INSERT INTO public.planning_route_days (planning_id, label, date_label, sort_order)
		VALUES ($1, $2, $3, (SELECT value FROM next_order))
		RETURNING id, label, date_label
	`, planningID, input.Label, input.DateLabel).Scan(&day.ID, &day.Label, &day.DateLabel)
	if err != nil {
		return PlanningItineraryDay{}, err
	}

	day.Stops = []PlanningItineraryStop{}
	return day, nil
}

func (r *PostgresRepository) DeletePlanningRouteDay(ctx context.Context, planningID uuid.UUID, dayID uuid.UUID) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var dayCount int
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM public.planning_route_days
		WHERE planning_id = $1
	`, planningID).Scan(&dayCount); err != nil {
		return err
	}
	if dayCount == 0 {
		return ErrPlanningNotFound
	}
	if dayCount == 1 {
		return ErrConflict
	}

	result, err := tx.Exec(ctx, `
		DELETE FROM public.planning_route_days
		WHERE planning_id = $1 AND id = $2
	`, planningID, dayID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrPlanningNotFound
	}

	return tx.Commit(ctx)
}

func (r *PostgresRepository) GetPlanningRouteByID(ctx context.Context, planningID uuid.UUID, routeID uuid.UUID) (PlanningItineraryStop, error) {
	stop := PlanningItineraryStop{}
	var imageURL *string
	err := r.pool.QueryRow(ctx, `
		SELECT id, title, description, address, time_label, duration_minutes, notes, cost_estimate, currency, external_url, image_url, status, category, is_optional, latitude, longitude
		FROM public.planning_route_stops
		WHERE planning_id = $1 AND id = $2
	`, planningID, routeID).Scan(
		&stop.ID,
		&stop.Title,
		&stop.Description,
		&stop.Address,
		&stop.TimeLabel,
		&stop.DurationMinutes,
		&stop.Notes,
		&stop.CostEstimate,
		&stop.Currency,
		&stop.ExternalURL,
		&imageURL,
		&stop.Status,
		&stop.Category,
		&stop.IsOptional,
		&stop.Coordinates.Latitude,
		&stop.Coordinates.Longitude,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PlanningItineraryStop{}, ErrPlanningNotFound
		}
		return PlanningItineraryStop{}, err
	}
	stop.ImageURL = imageURL
	stop.Bookings, err = r.listRouteStopBookings(ctx, stop.ID)
	if err != nil {
		return PlanningItineraryStop{}, err
	}
	return stop, nil
}

func (r *PostgresRepository) listRouteStopBookings(ctx context.Context, stopID uuid.UUID) ([]PlanningRouteBooking, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, title, type, status, occurs_at
		FROM public.bookings
		WHERE route_stop_id = $1
		ORDER BY occurs_at NULLS LAST, created_at ASC
	`, stopID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	bookings := make([]PlanningRouteBooking, 0)
	for rows.Next() {
		booking := PlanningRouteBooking{}
		if err := rows.Scan(&booking.ID, &booking.Title, &booking.Type, &booking.Status, &booking.OccursAt); err != nil {
			return nil, err
		}
		bookings = append(bookings, booking)
	}
	return bookings, rows.Err()
}

func (r *PostgresRepository) CreatePlanningRouteStop(ctx context.Context, planningID uuid.UUID, input CreatePlanningRouteStopInput) (PlanningItineraryStop, error) {
	if err := r.assertRouteDayBelongsToPlanning(ctx, planningID, input.DayID); err != nil {
		return PlanningItineraryStop{}, err
	}

	stop := PlanningItineraryStop{}
	var imageURL *string
	err := r.pool.QueryRow(ctx, `
		WITH next_order AS (
			SELECT COALESCE(MAX(sort_order), -1) + 1 AS value
			FROM public.planning_route_stops
			WHERE planning_id = $1 AND day_id = $2
		)
		INSERT INTO public.planning_route_stops (
			id, planning_id, day_id, title, description, address, time_label, duration_minutes,
			notes, cost_estimate, currency, external_url, image_url, status, category, is_optional,
			latitude, longitude, sort_order
		)
		VALUES (
			gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11, $12, $13, $14, $15,
			$16, $17, (SELECT value FROM next_order)
		)
		RETURNING id, title, description, address, time_label, duration_minutes, notes, cost_estimate, currency,
			external_url, image_url, status, category, is_optional, latitude, longitude
	`,
		planningID,
		input.DayID,
		input.Title,
		input.Description,
		input.Address,
		input.TimeLabel,
		input.DurationMinutes,
		input.Notes,
		input.CostEstimate,
		defaultString(input.Currency, "EUR"),
		input.ExternalURL,
		input.ImageURL,
		defaultString(input.Status, "pending"),
		defaultString(input.Category, "landmark"),
		input.IsOptional,
		input.Coordinates.Latitude,
		input.Coordinates.Longitude,
	).Scan(
		&stop.ID,
		&stop.Title,
		&stop.Description,
		&stop.Address,
		&stop.TimeLabel,
		&stop.DurationMinutes,
		&stop.Notes,
		&stop.CostEstimate,
		&stop.Currency,
		&stop.ExternalURL,
		&imageURL,
		&stop.Status,
		&stop.Category,
		&stop.IsOptional,
		&stop.Coordinates.Latitude,
		&stop.Coordinates.Longitude,
	)
	if err != nil {
		return PlanningItineraryStop{}, err
	}

	stop.ImageURL = imageURL
	return stop, nil
}

func (r *PostgresRepository) UpdatePlanningRouteStop(ctx context.Context, planningID uuid.UUID, routeID uuid.UUID, input UpdatePlanningRouteStopInput) (PlanningItineraryStop, error) {
	mutableStop, err := r.getMutablePlanningRouteStop(ctx, planningID, routeID)
	if err != nil {
		return PlanningItineraryStop{}, err
	}

	dayID := mutableStop.DayID
	if input.DayID != nil {
		if err := r.assertRouteDayBelongsToPlanning(ctx, planningID, *input.DayID); err != nil {
			return PlanningItineraryStop{}, err
		}
		dayID = *input.DayID
	}

	if input.Title != nil {
		mutableStop.Title = *input.Title
	}
	if input.Description != nil {
		mutableStop.Description = *input.Description
	}
	if input.Address != nil {
		mutableStop.Address = *input.Address
	}
	if input.TimeLabel != nil {
		mutableStop.TimeLabel = *input.TimeLabel
	}
	if input.DurationMinutes != nil {
		mutableStop.DurationMinutes = *input.DurationMinutes
	}
	if input.Notes != nil {
		mutableStop.Notes = *input.Notes
	}
	if input.CostEstimate != nil {
		mutableStop.CostEstimate = *input.CostEstimate
	}
	if input.Currency != nil {
		mutableStop.Currency = *input.Currency
	}
	if input.ExternalURL != nil {
		mutableStop.ExternalURL = *input.ExternalURL
	}
	if input.ImageURL != nil {
		trimmedImageURL := strings.TrimSpace(*input.ImageURL)
		if trimmedImageURL == "" {
			mutableStop.ImageURL = nil
		} else {
			mutableStop.ImageURL = &trimmedImageURL
		}
	}
	if input.Status != nil {
		mutableStop.Status = *input.Status
	}
	if input.Category != nil {
		mutableStop.Category = *input.Category
	}
	if input.IsOptional != nil {
		mutableStop.IsOptional = *input.IsOptional
	}
	if input.Coordinates != nil {
		mutableStop.Coordinates = *input.Coordinates
	}

	updatedStop := PlanningItineraryStop{}
	var updatedImageURL *string
	err = r.pool.QueryRow(ctx, `
		UPDATE public.planning_route_stops
		SET day_id = $3,
			title = $4,
			description = $5,
			address = $6,
			time_label = $7,
			duration_minutes = $8,
			notes = $9,
			cost_estimate = $10,
			currency = $11,
			external_url = $12,
			image_url = $13,
			status = $14,
			category = $15,
			is_optional = $16,
			latitude = $17,
			longitude = $18,
			updated_at = now()
		WHERE planning_id = $1 AND id = $2
		RETURNING id, title, description, address, time_label, duration_minutes, notes, cost_estimate, currency,
			external_url, image_url, status, category, is_optional, latitude, longitude
	`,
		planningID,
		routeID,
		dayID,
		mutableStop.Title,
		mutableStop.Description,
		mutableStop.Address,
		mutableStop.TimeLabel,
		mutableStop.DurationMinutes,
		mutableStop.Notes,
		mutableStop.CostEstimate,
		defaultString(mutableStop.Currency, "EUR"),
		mutableStop.ExternalURL,
		mutableStop.ImageURL,
		defaultString(mutableStop.Status, "pending"),
		defaultString(mutableStop.Category, "landmark"),
		mutableStop.IsOptional,
		mutableStop.Coordinates.Latitude,
		mutableStop.Coordinates.Longitude,
	).Scan(
		&updatedStop.ID,
		&updatedStop.Title,
		&updatedStop.Description,
		&updatedStop.Address,
		&updatedStop.TimeLabel,
		&updatedStop.DurationMinutes,
		&updatedStop.Notes,
		&updatedStop.CostEstimate,
		&updatedStop.Currency,
		&updatedStop.ExternalURL,
		&updatedImageURL,
		&updatedStop.Status,
		&updatedStop.Category,
		&updatedStop.IsOptional,
		&updatedStop.Coordinates.Latitude,
		&updatedStop.Coordinates.Longitude,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PlanningItineraryStop{}, ErrPlanningNotFound
		}
		return PlanningItineraryStop{}, err
	}

	updatedStop.ImageURL = updatedImageURL
	return updatedStop, nil
}

func (r *PostgresRepository) DeletePlanningRouteStop(ctx context.Context, planningID uuid.UUID, routeID uuid.UUID) error {
	result, err := r.pool.Exec(ctx, `
		DELETE FROM public.planning_route_stops
		WHERE planning_id = $1 AND id = $2
	`, planningID, routeID)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrPlanningNotFound
	}

	return nil
}

func (r *PostgresRepository) ReorderPlanningRouteStops(ctx context.Context, planningID uuid.UUID, dayID uuid.UUID, stopIDs []uuid.UUID) error {
	if err := r.assertRouteDayBelongsToPlanning(ctx, planningID, dayID); err != nil {
		return err
	}

	existingRows, err := r.pool.Query(ctx, `
		SELECT id
		FROM public.planning_route_stops
		WHERE planning_id = $1 AND day_id = $2
	`, planningID, dayID)
	if err != nil {
		return err
	}
	defer existingRows.Close()

	existing := make(map[uuid.UUID]bool)
	for existingRows.Next() {
		var stopID uuid.UUID
		if scanErr := existingRows.Scan(&stopID); scanErr != nil {
			return scanErr
		}
		existing[stopID] = true
	}
	if err := existingRows.Err(); err != nil {
		return err
	}

	if len(existing) != len(stopIDs) {
		return ValidationError{Details: []response.ErrorDetail{{Field: "stop_ids", Message: "stop_ids must include all route stops of selected day"}}}
	}

	for _, stopID := range stopIDs {
		if !existing[stopID] {
			return ValidationError{Details: []response.ErrorDetail{{Field: "stop_ids", Message: "stop_ids contain stop outside selected day"}}}
		}
	}

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for index, stopID := range stopIDs {
		if _, execErr := tx.Exec(ctx, `
			UPDATE public.planning_route_stops
			SET sort_order = $1, updated_at = now()
			WHERE planning_id = $2 AND day_id = $3 AND id = $4
		`, index, planningID, dayID, stopID); execErr != nil {
			return execErr
		}
	}

	return tx.Commit(ctx)
}

type planningMutableRouteStop struct {
	PlanningItineraryStop
	DayID uuid.UUID
}

func (r *PostgresRepository) getMutablePlanningRouteStop(ctx context.Context, planningID uuid.UUID, routeID uuid.UUID) (planningMutableRouteStop, error) {
	stop := planningMutableRouteStop{}
	var imageURL *string
	err := r.pool.QueryRow(ctx, `
		SELECT id, day_id, title, description, address, time_label, duration_minutes, notes, cost_estimate,
			currency, external_url, image_url, status, category, is_optional, latitude, longitude
		FROM public.planning_route_stops
		WHERE planning_id = $1 AND id = $2
	`, planningID, routeID).Scan(
		&stop.ID,
		&stop.DayID,
		&stop.Title,
		&stop.Description,
		&stop.Address,
		&stop.TimeLabel,
		&stop.DurationMinutes,
		&stop.Notes,
		&stop.CostEstimate,
		&stop.Currency,
		&stop.ExternalURL,
		&imageURL,
		&stop.Status,
		&stop.Category,
		&stop.IsOptional,
		&stop.Coordinates.Latitude,
		&stop.Coordinates.Longitude,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return planningMutableRouteStop{}, ErrPlanningNotFound
		}
		return planningMutableRouteStop{}, err
	}

	stop.ImageURL = imageURL
	return stop, nil
}

func (r *PostgresRepository) assertRouteDayBelongsToPlanning(ctx context.Context, planningID uuid.UUID, dayID uuid.UUID) error {
	var existingDayID uuid.UUID
	err := r.pool.QueryRow(ctx, `
		SELECT id
		FROM public.planning_route_days
		WHERE planning_id = $1 AND id = $2
	`, planningID, dayID).Scan(&existingDayID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ValidationError{Details: []response.ErrorDetail{{Field: "day_id", Message: "day_id not found for selected planning"}}}
		}
		return err
	}

	return nil
}

func defaultString(value string, fallback string) string {
	trimmedValue := strings.TrimSpace(value)
	if trimmedValue == "" {
		return fallback
	}

	return trimmedValue
}

func (r *PostgresRepository) addMemberTx(ctx context.Context, tx pgx.Tx, actorUserID uuid.UUID, planningID uuid.UUID, input AddPlanningMemberInput) (PlanningMember, error) {
	member, err := scanPlanningMember(tx.QueryRow(ctx, `
		INSERT INTO public.planning_members (id, planning_id, user_id, role, invited_by_user_id, joined_at)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, now())
		RETURNING id, planning_id, user_id, role, invited_by_user_id, joined_at, created_at, updated_at
	`, planningID, input.UserID, input.Role, actorUserID))
	if err != nil {
		var pgError *pgconn.PgError
		if errors.As(err, &pgError) && pgError.Code == "23505" {
			return PlanningMember{}, ErrConflict
		}

		return PlanningMember{}, err
	}

	return member, nil
}

func (r *PostgresRepository) insertActivityTx(ctx context.Context, tx pgx.Tx, planningID uuid.UUID, actorUserID *uuid.UUID, eventType string, entityType string, entityID *uuid.UUID, metadata map[string]any) error {
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO public.planning_activity (id, planning_id, actor_user_id, event_type, entity_type, entity_id, metadata)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6::jsonb)
	`, planningID, actorUserID, eventType, entityType, entityID, string(metadataJSON))
	return err
}

type planningScanner interface {
	Scan(dest ...any) error
}

func scanPlanning(scanner planningScanner) (Planning, error) {
	planning := Planning{}
	var groupID *string
	err := scanner.Scan(
		&planning.ID,
		&groupID,
		&planning.OwnerUserID,
		&planning.Name,
		&planning.Description,
		&planning.DestinationName,
		&planning.StartsAt,
		&planning.EndsAt,
		&planning.Status,
		&planning.CoverImageURL,
		&planning.IsArchived,
		&planning.CreatedAt,
		&planning.UpdatedAt,
	)
	if err != nil {
		return Planning{}, err
	}

	if groupID != nil {
		parsedGroupID, parseErr := uuid.Parse(*groupID)
		if parseErr == nil {
			planning.GroupID = &parsedGroupID
		}
	}

	return planning, nil
}

func scanPlanningMember(scanner planningScanner) (PlanningMember, error) {
	member := PlanningMember{}
	err := scanner.Scan(
		&member.ID,
		&member.PlanningID,
		&member.UserID,
		&member.Role,
		&member.InvitedByUserID,
		&member.JoinedAt,
		&member.CreatedAt,
		&member.UpdatedAt,
	)
	return member, err
}

func scanPlanningActivity(scanner planningScanner) (PlanningActivity, error) {
	activity := PlanningActivity{}
	var metadataBytes []byte
	err := scanner.Scan(
		&activity.ID,
		&activity.PlanningID,
		&activity.ActorUserID,
		&activity.EventType,
		&activity.EntityType,
		&activity.EntityID,
		&metadataBytes,
		&activity.CreatedAt,
	)
	if err != nil {
		return PlanningActivity{}, err
	}

	activity.Metadata = map[string]any{}
	if len(metadataBytes) > 0 {
		if err := json.Unmarshal(metadataBytes, &activity.Metadata); err != nil {
			return PlanningActivity{}, err
		}
	}

	return activity, nil
}

func metadataOptionalString(metadata map[string]any, key string) *string {
	value, ok := metadata[key]
	if !ok || value == nil {
		return nil
	}

	text, ok := value.(string)
	if !ok {
		return nil
	}

	trimmed := text
	return &trimmed
}

func metadataString(metadata map[string]any, key string) string {
	if value := metadataOptionalString(metadata, key); value != nil {
		return *value
	}

	return ""
}

func metadataTime(metadata map[string]any, key string, fallback time.Time) time.Time {
	value, ok := metadata[key]
	if !ok || value == nil {
		return fallback
	}

	text, ok := value.(string)
	if !ok {
		return fallback
	}

	parsed, err := time.Parse(time.RFC3339, text)
	if err != nil {
		return fallback
	}

	return parsed
}

func metadataInt(metadata map[string]any, key string) int {
	value, ok := metadata[key]
	if !ok || value == nil {
		return 0
	}

	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	default:
		return 0
	}
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

// GetUserIDByEmail retrieves the UUID of a user by their email address.
func (r *PostgresRepository) GetUserIDByEmail(ctx context.Context, email string) (uuid.UUID, error) {
	trimmedEmail := strings.TrimSpace(email)
	if trimmedEmail == "" {
		return uuid.Nil, ErrUserNotFound
	}

	var userID uuid.UUID
	err := r.pool.QueryRow(ctx, `
		SELECT id
		FROM auth.users
		WHERE LOWER(email) = LOWER($1)
		LIMIT 1
	`, trimmedEmail).Scan(&userID)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, ErrUserNotFound
		}
		return uuid.Nil, err
	}

	return userID, nil
}
