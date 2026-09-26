package users

import (
	"context"
	"errors"
	"math"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	EnsureProfile(ctx context.Context, userID uuid.UUID, usernameHint string) (UserProfile, error)
	GetProfileByUserID(ctx context.Context, userID uuid.UUID) (UserProfile, error)
	GetProfileByUsername(ctx context.Context, username string) (UserProfile, error)
	UpdateProfile(ctx context.Context, userID uuid.UUID, input UpdateUserProfileInput) (UserProfile, error)
	GetFollowStats(ctx context.Context, actorUserID uuid.UUID, profileUserID uuid.UUID) (UserFollowStats, error)
	Follow(ctx context.Context, actorUserID uuid.UUID, targetUserID uuid.UUID) error
	Unfollow(ctx context.Context, actorUserID uuid.UUID, targetUserID uuid.UUID) error
	ListFollowers(ctx context.Context, userID uuid.UUID, limit int) ([]UserSummary, error)
	ListFollowing(ctx context.Context, userID uuid.UUID, limit int) ([]UserSummary, error)
	ListLocationPins(ctx context.Context, userID uuid.UUID) ([]UserLocationPin, error)
	ListMedia(ctx context.Context, userID uuid.UUID, limit int) ([]UserMediaAsset, error)
	ListBadges(ctx context.Context, userID uuid.UUID, limit int) ([]UserBadge, error)
	CreateLocation(ctx context.Context, userID uuid.UUID, input CreateUserLocationInput) (UserLocationPin, error)
	UpdateLocation(ctx context.Context, userID uuid.UUID, locationID uuid.UUID, input UpdateUserLocationInput) (UserLocationPin, error)
	DeleteLocation(ctx context.Context, userID uuid.UUID, locationID uuid.UUID) error
	CreateMedia(ctx context.Context, userID uuid.UUID, input CreateUserMediaInput) (UserMediaAsset, error)
	UpsertVerifiedAchievement(ctx context.Context, userID uuid.UUID, input VerifyAchievementInput) (UserBadge, error)
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) EnsureProfile(ctx context.Context, userID uuid.UUID, usernameHint string) (UserProfile, error) {
	normalizedUsername := normalizeUsername(usernameHint)
	if normalizedUsername == "" {
		normalizedUsername = "user-" + strings.ToLower(userID.String()[:8])
	}
	defaultFullName := "Traveler " + strings.ToUpper(userID.String()[:6])

	_, err := r.pool.Exec(ctx, `
		INSERT INTO public.user_profiles (user_id, username, full_name)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id) DO NOTHING
	`, userID, normalizedUsername, defaultFullName)
	if err != nil {
		if isUniqueViolation(err) {
			// Username collision can happen with generated defaults; retry with UUID suffix.
			_, err = r.pool.Exec(ctx, `
				INSERT INTO public.user_profiles (user_id, username, full_name)
				VALUES ($1, $2, $3)
				ON CONFLICT (user_id) DO NOTHING
			`, userID, normalizedUsername+"-"+strings.ToLower(userID.String()[:4]), defaultFullName)
		}
		if err != nil {
			return UserProfile{}, err
		}
	}

	return r.GetProfileByUserID(ctx, userID)
}

func (r *PostgresRepository) GetProfileByUserID(ctx context.Context, userID uuid.UUID) (UserProfile, error) {
	profile := UserProfile{}
	err := r.pool.QueryRow(ctx, `
		SELECT user_id, username, full_name, bio, avatar_url, multimedia_visibility, badges_visibility, created_at, updated_at
		FROM public.user_profiles
		WHERE user_id = $1
	`, userID).Scan(
		&profile.UserID,
		&profile.Username,
		&profile.FullName,
		&profile.Bio,
		&profile.AvatarURL,
		&profile.MultimediaVisibility,
		&profile.BadgesVisibility,
		&profile.CreatedAt,
		&profile.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return UserProfile{}, ErrProfileNotFound
		}
		return UserProfile{}, err
	}

	return profile, nil
}

func (r *PostgresRepository) GetProfileByUsername(ctx context.Context, username string) (UserProfile, error) {
	profile := UserProfile{}
	err := r.pool.QueryRow(ctx, `
		SELECT user_id, username, full_name, bio, avatar_url, multimedia_visibility, badges_visibility, created_at, updated_at
		FROM public.user_profiles
		WHERE LOWER(username) = LOWER($1)
	`, username).Scan(
		&profile.UserID,
		&profile.Username,
		&profile.FullName,
		&profile.Bio,
		&profile.AvatarURL,
		&profile.MultimediaVisibility,
		&profile.BadgesVisibility,
		&profile.CreatedAt,
		&profile.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return UserProfile{}, ErrProfileNotFound
		}
		return UserProfile{}, err
	}

	return profile, nil
}

func (r *PostgresRepository) UpdateProfile(ctx context.Context, userID uuid.UUID, input UpdateUserProfileInput) (UserProfile, error) {
	command, err := r.pool.Exec(ctx, `
		UPDATE public.user_profiles
		SET
			full_name = COALESCE($2, full_name),
			bio = CASE WHEN $3::text IS NULL THEN bio ELSE $3 END,
			avatar_url = CASE WHEN $4::text IS NULL THEN avatar_url ELSE $4 END,
			multimedia_visibility = COALESCE($5, multimedia_visibility),
			badges_visibility = COALESCE($6, badges_visibility),
			updated_at = NOW()
		WHERE user_id = $1
	`, userID, input.FullName, input.Bio, input.AvatarURL, input.MultimediaVisibility, input.BadgesVisibility)
	if err != nil {
		if isCheckViolation(err) {
			return UserProfile{}, ErrInvalidPrivacyValue
		}

		return UserProfile{}, err
	}

	if command.RowsAffected() == 0 {
		return UserProfile{}, ErrProfileNotFound
	}

	return r.GetProfileByUserID(ctx, userID)
}

func (r *PostgresRepository) GetFollowStats(ctx context.Context, actorUserID uuid.UUID, profileUserID uuid.UUID) (UserFollowStats, error) {
	stats := UserFollowStats{IsMe: actorUserID == profileUserID}
	err := r.pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*)::int FROM public.user_follows WHERE following_user_id = $1) AS followers_count,
			(SELECT count(*)::int FROM public.user_follows WHERE follower_user_id = $1) AS following_count,
			EXISTS(SELECT 1 FROM public.user_follows WHERE follower_user_id = $2 AND following_user_id = $1) AS is_following
	`, profileUserID, actorUserID).Scan(&stats.FollowersCount, &stats.FollowingCount, &stats.IsFollowing)
	if err != nil {
		return UserFollowStats{}, err
	}

	return stats, nil
}

func (r *PostgresRepository) Follow(ctx context.Context, actorUserID uuid.UUID, targetUserID uuid.UUID) error {
	if actorUserID == targetUserID {
		return ErrConflict
	}

	_, err := r.pool.Exec(ctx, `
		INSERT INTO public.user_follows (follower_user_id, following_user_id)
		VALUES ($1, $2)
		ON CONFLICT (follower_user_id, following_user_id) DO NOTHING
	`, actorUserID, targetUserID)
	if err != nil {
		return err
	}

	return nil
}

func (r *PostgresRepository) Unfollow(ctx context.Context, actorUserID uuid.UUID, targetUserID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		DELETE FROM public.user_follows
		WHERE follower_user_id = $1 AND following_user_id = $2
	`, actorUserID, targetUserID)
	return err
}

func (r *PostgresRepository) ListFollowers(ctx context.Context, userID uuid.UUID, limit int) ([]UserSummary, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT p.user_id, p.username, p.full_name, p.avatar_url, f.created_at
		FROM public.user_follows f
		INNER JOIN public.user_profiles p ON p.user_id = f.follower_user_id
		WHERE f.following_user_id = $1
		ORDER BY f.created_at DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]UserSummary, 0)
	for rows.Next() {
		var item UserSummary
		if scanErr := rows.Scan(&item.UserID, &item.Username, &item.FullName, &item.AvatarURL, &item.FollowedAt); scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}

	return items, rows.Err()
}

func (r *PostgresRepository) ListFollowing(ctx context.Context, userID uuid.UUID, limit int) ([]UserSummary, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT p.user_id, p.username, p.full_name, p.avatar_url, f.created_at
		FROM public.user_follows f
		INNER JOIN public.user_profiles p ON p.user_id = f.following_user_id
		WHERE f.follower_user_id = $1
		ORDER BY f.created_at DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]UserSummary, 0)
	for rows.Next() {
		var item UserSummary
		if scanErr := rows.Scan(&item.UserID, &item.Username, &item.FullName, &item.AvatarURL, &item.FollowedAt); scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}

	return items, rows.Err()
}

func (r *PostgresRepository) ListLocationPins(ctx context.Context, userID uuid.UUID) ([]UserLocationPin, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT
			l.id,
			l.name,
			l.latitude,
			l.longitude,
			l.country_code,
			thumb.thumbnail_url,
			COALESCE(media.media_count, 0) AS media_count,
			COALESCE((badge.achievement_count > 0), false) AS has_verified_achievement,
			COALESCE(badge.achievement_count, 0) AS verified_count
		FROM public.user_locations l
		LEFT JOIN LATERAL (
			SELECT m.thumbnail_url
			FROM public.user_media_assets m
			WHERE m.location_id = l.id AND m.thumbnail_url IS NOT NULL
			ORDER BY m.created_at DESC
			LIMIT 1
		) AS thumb ON true
		LEFT JOIN LATERAL (
			SELECT count(*)::int AS media_count
			FROM public.user_media_assets m2
			WHERE m2.location_id = l.id
		) AS media ON true
		LEFT JOIN LATERAL (
			SELECT
				count(*)::int AS achievement_count
			FROM public.user_achievements a
			WHERE a.user_id = $1
				AND a.location_id = l.id
		) AS badge ON true
		WHERE l.user_id = $1
		ORDER BY l.created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	pins := make([]UserLocationPin, 0)
	for rows.Next() {
		var pin UserLocationPin
		if scanErr := rows.Scan(
			&pin.LocationID,
			&pin.Name,
			&pin.Latitude,
			&pin.Longitude,
			&pin.CountryCode,
			&pin.ThumbnailURL,
			&pin.MediaCount,
			&pin.HasVerifiedAchievement,
			&pin.VerifiedAchievementCount,
		); scanErr != nil {
			return nil, scanErr
		}
		pins = append(pins, pin)
	}

	return pins, rows.Err()
}

func (r *PostgresRepository) ListMedia(ctx context.Context, userID uuid.UUID, limit int) ([]UserMediaAsset, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, location_id, media_type, url, thumbnail_url, caption, created_at
		FROM public.user_media_assets
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]UserMediaAsset, 0)
	for rows.Next() {
		var item UserMediaAsset
		if scanErr := rows.Scan(&item.ID, &item.UserID, &item.LocationID, &item.MediaType, &item.URL, &item.ThumbnailURL, &item.Caption, &item.CreatedAt); scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}

	return items, rows.Err()
}

func (r *PostgresRepository) ListBadges(ctx context.Context, userID uuid.UUID, limit int) ([]UserBadge, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT a.id, b.id, b.code, b.title, b.icon_url, a.obtained_at, a.location_id, b.location_name
		FROM public.user_achievements a
		INNER JOIN public.badges b ON b.id = a.badge_id
		WHERE a.user_id = $1
		ORDER BY a.obtained_at DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		if isUndefinedRelationOrColumn(err) {
			return r.listBadgesLegacy(ctx, userID, limit)
		}
		return nil, err
	}
	defer rows.Close()

	items := make([]UserBadge, 0)
	for rows.Next() {
		var item UserBadge
		if scanErr := rows.Scan(&item.ID, &item.BadgeID, &item.Code, &item.Title, &item.IconURL, &item.UnlockedAt, &item.LocationID, &item.LocationName); scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}

	return items, rows.Err()
}

func (r *PostgresRepository) listBadgesLegacy(ctx context.Context, userID uuid.UUID, limit int) ([]UserBadge, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT b.id, b.code, b.label, b.icon_url, b.unlocked_at, b.location_id, l.name
		FROM public.user_badges b
		INNER JOIN public.user_locations l ON l.id = b.location_id
		WHERE b.user_id = $1
		ORDER BY b.unlocked_at DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]UserBadge, 0)
	for rows.Next() {
		var item UserBadge
		if scanErr := rows.Scan(&item.ID, &item.Code, &item.Title, &item.IconURL, &item.UnlockedAt, &item.LocationID, &item.LocationName); scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}

	return items, rows.Err()
}

func (r *PostgresRepository) CreateLocation(ctx context.Context, userID uuid.UUID, input CreateUserLocationInput) (UserLocationPin, error) {
	var locationID uuid.UUID
	err := r.pool.QueryRow(ctx, `
		INSERT INTO public.user_locations (user_id, name, latitude, longitude, country_code, visited_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`, userID, input.Name, input.Latitude, input.Longitude, input.CountryCode, input.VisitedAt).Scan(&locationID)
	if err != nil {
		return UserLocationPin{}, err
	}

	pin := UserLocationPin{
		LocationID:  locationID,
		Name:        input.Name,
		Latitude:    input.Latitude,
		Longitude:   input.Longitude,
		CountryCode: input.CountryCode,
		MediaCount:  0,
	}
	return pin, nil
}

func (r *PostgresRepository) UpdateLocation(ctx context.Context, userID uuid.UUID, locationID uuid.UUID, input UpdateUserLocationInput) (UserLocationPin, error) {
	var updated UserLocationPin
	err := r.pool.QueryRow(ctx, `
		UPDATE public.user_locations
		SET name = $3, latitude = $4, longitude = $5, country_code = $6
		WHERE id = $1 AND user_id = $2
		RETURNING id, name, latitude, longitude, country_code
	`, locationID, userID, input.Name, input.Latitude, input.Longitude, input.CountryCode).Scan(
		&updated.LocationID,
		&updated.Name,
		&updated.Latitude,
		&updated.Longitude,
		&updated.CountryCode,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return UserLocationPin{}, ErrNotFound
		}
		return UserLocationPin{}, err
	}

	return updated, nil
}

func (r *PostgresRepository) DeleteLocation(ctx context.Context, userID uuid.UUID, locationID uuid.UUID) error {
	command, err := r.pool.Exec(ctx, `
		DELETE FROM public.user_locations
		WHERE id = $1 AND user_id = $2
	`, locationID, userID)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PostgresRepository) CreateMedia(ctx context.Context, userID uuid.UUID, input CreateUserMediaInput) (UserMediaAsset, error) {
	var ownerID uuid.UUID
	err := r.pool.QueryRow(ctx, `
		SELECT user_id
		FROM public.user_locations
		WHERE id = $1
	`, input.LocationID).Scan(&ownerID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return UserMediaAsset{}, ErrLocationNotFound
		}
		return UserMediaAsset{}, err
	}

	if ownerID != userID {
		return UserMediaAsset{}, ErrForbidden
	}

	item := UserMediaAsset{}
	err = r.pool.QueryRow(ctx, `
		INSERT INTO public.user_media_assets (user_id, location_id, media_type, url, thumbnail_url, caption)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, user_id, location_id, media_type, url, thumbnail_url, caption, created_at
	`, userID, input.LocationID, input.MediaType, input.URL, input.ThumbnailURL, input.Caption).Scan(
		&item.ID,
		&item.UserID,
		&item.LocationID,
		&item.MediaType,
		&item.URL,
		&item.ThumbnailURL,
		&item.Caption,
		&item.CreatedAt,
	)
	if err != nil {
		return UserMediaAsset{}, err
	}

	return item, nil
}

func (r *PostgresRepository) UpsertVerifiedAchievement(ctx context.Context, userID uuid.UUID, input VerifyAchievementInput) (UserBadge, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return UserBadge{}, err
	}
	defer tx.Rollback(ctx)

	var ownerID uuid.UUID
	var locationLatitude float64
	var locationLongitude float64
	err = tx.QueryRow(ctx, `
		SELECT user_id, latitude, longitude
		FROM public.user_locations
		WHERE id = $1
	`, input.LocationID).Scan(&ownerID, &locationLatitude, &locationLongitude)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return UserBadge{}, ErrLocationNotFound
		}
		return UserBadge{}, err
	}
	if ownerID != userID {
		return UserBadge{}, ErrForbidden
	}

	badge := UserBadge{}
	var badgeLatitude float64
	var badgeLongitude float64
	var isAvailable bool
	err = tx.QueryRow(ctx, `
		SELECT id, code, title, icon_url, location_name, latitude, longitude, is_available
		FROM public.badges
		WHERE id = $1
	`, input.BadgeID).Scan(
		&badge.ID,
		&badge.Code,
		&badge.Title,
		&badge.IconURL,
		&badge.LocationName,
		&badgeLatitude,
		&badgeLongitude,
		&isAvailable,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return UserBadge{}, ErrBadgeNotFound
		}
		return UserBadge{}, err
	}

	if !isAvailable {
		return UserBadge{}, ErrBadgeUnavailable
	}

	if distanceMeters(locationLatitude, locationLongitude, badgeLatitude, badgeLongitude) > 250 {
		return UserBadge{}, ErrBadgeLocationMismatch
	}

	err = tx.QueryRow(ctx, `
		INSERT INTO public.user_achievements (user_id, badge_id, location_id, obtained_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (user_id, badge_id)
		DO UPDATE SET
			location_id = EXCLUDED.location_id,
			obtained_at = NOW()
		RETURNING id, badge_id, location_id, obtained_at
	`, userID, input.BadgeID, input.LocationID).Scan(
		&badge.ID,
		&badge.BadgeID,
		&badge.LocationID,
		&badge.UnlockedAt,
	)
	if err != nil {
		return UserBadge{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return UserBadge{}, err
	}

	return badge, nil
}

func distanceMeters(lat1 float64, lon1 float64, lat2 float64, lon2 float64) float64 {
	const earthRadiusMeters = 6371000.0

	phi1 := lat1 * math.Pi / 180
	phi2 := lat2 * math.Pi / 180
	deltaPhi := (lat2 - lat1) * math.Pi / 180
	deltaLambda := (lon2 - lon1) * math.Pi / 180

	a := math.Sin(deltaPhi/2)*math.Sin(deltaPhi/2) +
		math.Cos(phi1)*math.Cos(phi2)*math.Sin(deltaLambda/2)*math.Sin(deltaLambda/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadiusMeters * c
}

func normalizeUsername(raw string) string {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	normalized = strings.ReplaceAll(normalized, " ", "")
	return normalized
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}

func isCheckViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23514"
	}
	return false
}

func isUndefinedRelationOrColumn(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "42P01" || pgErr.Code == "42703"
	}
	return false
}
