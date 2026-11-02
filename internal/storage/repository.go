package storage

import (
	"context"
	"errors"
	"time"

	"github.com/eleventravel/eleventravel-api/internal/platform/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	CreatePending(ctx context.Context, image Image) (Image, error)
	CountPending(ctx context.Context, ownerID uuid.UUID) (int, error)
	Get(ctx context.Context, id uuid.UUID) (Image, error)
	MarkCompleted(ctx context.Context, id uuid.UUID, size int64, width int, height int) (Image, error)
	Delete(ctx context.Context, id uuid.UUID) error
	DeleteMediaForImage(ctx context.Context, id uuid.UUID) error
	ListExpiredPending(ctx context.Context, now time.Time, limit int) ([]Image, error)
	ListOtherAvatars(ctx context.Context, ownerID uuid.UUID, exceptID uuid.UUID) ([]Image, error)
	ListPublicProfileMedia(ctx context.Context) ([]Image, error)
	UpdateBucket(ctx context.Context, id uuid.UUID, bucket string, visibility string) error
	MultimediaVisibility(ctx context.Context, userID uuid.UUID) (string, error)
	IsFollowing(ctx context.Context, followerID uuid.UUID, followingID uuid.UUID) (bool, error)
	SetAvatarURL(ctx context.Context, userID uuid.UUID, avatarURL *string) error
	ListByIDs(ctx context.Context, ids []uuid.UUID) ([]Image, error)
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

const imageColumns = `id, owner_user_id, purpose, visibility, bucket, object_key, mime_type, declared_size_bytes,
	size_bytes, width, height, status, expires_at, created_at, completed_at`

func scanImage(row pgx.Row) (Image, error) {
	var image Image
	err := row.Scan(&image.ID, &image.OwnerUserID, &image.Purpose, &image.Visibility, &image.Bucket, &image.ObjectKey,
		&image.MimeType, &image.DeclaredSizeBytes, &image.SizeBytes, &image.Width, &image.Height, &image.Status,
		&image.ExpiresAt, &image.CreatedAt, &image.CompletedAt)
	return image, err
}

func (r *PostgresRepository) queryImages(ctx context.Context, query string, args ...any) ([]Image, error) {
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	images := make([]Image, 0)
	for rows.Next() {
		image, scanErr := scanImage(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		images = append(images, image)
	}

	return images, rows.Err()
}

func (r *PostgresRepository) CreatePending(ctx context.Context, image Image) (Image, error) {
	return scanImage(r.pool.QueryRow(ctx, `
		INSERT INTO public.images (id, owner_user_id, purpose, visibility, bucket, object_key, mime_type, declared_size_bytes, status, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'pending', $9)
		RETURNING `+imageColumns,
		image.ID, image.OwnerUserID, image.Purpose, image.Visibility, image.Bucket, image.ObjectKey, image.MimeType,
		image.DeclaredSizeBytes, image.ExpiresAt))
}

func (r *PostgresRepository) CountPending(ctx context.Context, ownerID uuid.UUID) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT count(*)::int FROM public.images WHERE owner_user_id = $1 AND status = 'pending'
	`, ownerID).Scan(&count)
	return count, err
}

func (r *PostgresRepository) Get(ctx context.Context, id uuid.UUID) (Image, error) {
	image, err := scanImage(r.pool.QueryRow(ctx, `SELECT `+imageColumns+` FROM public.images WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Image{}, ErrNotFound
	}
	return image, err
}

func (r *PostgresRepository) MarkCompleted(ctx context.Context, id uuid.UUID, size int64, width int, height int) (Image, error) {
	image, err := scanImage(r.pool.QueryRow(ctx, `
		UPDATE public.images
		SET status = 'completed', size_bytes = $2, width = $3, height = $4, completed_at = NOW()
		WHERE id = $1
		RETURNING `+imageColumns, id, size, width, height))
	if errors.Is(err, pgx.ErrNoRows) {
		return Image{}, ErrNotFound
	}
	return image, err
}

func (r *PostgresRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM public.images WHERE id = $1`, id)
	return err
}

func (r *PostgresRepository) DeleteMediaForImage(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM public.user_media_assets WHERE image_id = $1`, id)
	return err
}

func (r *PostgresRepository) ListExpiredPending(ctx context.Context, now time.Time, limit int) ([]Image, error) {
	return r.queryImages(ctx, `
		SELECT `+imageColumns+` FROM public.images
		WHERE status = 'pending' AND expires_at < $1
		ORDER BY expires_at
		LIMIT $2
	`, now.Add(-time.Hour), limit)
}

func (r *PostgresRepository) ListOtherAvatars(ctx context.Context, ownerID uuid.UUID, exceptID uuid.UUID) ([]Image, error) {
	return r.queryImages(ctx, `
		SELECT `+imageColumns+` FROM public.images
		WHERE owner_user_id = $1 AND purpose = 'avatar' AND status = 'completed' AND id <> $2
	`, ownerID, exceptID)
}

func (r *PostgresRepository) ListPublicProfileMedia(ctx context.Context) ([]Image, error) {
	return r.queryImages(ctx, `
		SELECT `+imageColumns+` FROM public.images
		WHERE purpose = 'profile_media' AND visibility = 'public' AND status = 'completed'
	`)
}

func (r *PostgresRepository) ListByIDs(ctx context.Context, ids []uuid.UUID) ([]Image, error) {
	return r.queryImages(ctx, `
		SELECT `+imageColumns+` FROM public.images WHERE id = ANY($1::uuid[]) AND status = 'completed'
	`, db.UUIDArray(ids))
}

func (r *PostgresRepository) UpdateBucket(ctx context.Context, id uuid.UUID, bucket string, visibility string) error {
	_, err := r.pool.Exec(ctx, `UPDATE public.images SET bucket = $2, visibility = $3 WHERE id = $1`, id, bucket, visibility)
	return err
}

func (r *PostgresRepository) MultimediaVisibility(ctx context.Context, userID uuid.UUID) (string, error) {
	var visibility string
	err := r.pool.QueryRow(ctx, `SELECT multimedia_visibility FROM public.user_profiles WHERE user_id = $1`, userID).Scan(&visibility)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrProfileMissing
	}
	return visibility, err
}

func (r *PostgresRepository) IsFollowing(ctx context.Context, followerID uuid.UUID, followingID uuid.UUID) (bool, error) {
	var following bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM public.user_follows WHERE follower_user_id = $1 AND following_user_id = $2)
	`, followerID, followingID).Scan(&following)
	return following, err
}

func (r *PostgresRepository) SetAvatarURL(ctx context.Context, userID uuid.UUID, avatarURL *string) error {
	command, err := r.pool.Exec(ctx, `
		UPDATE public.user_profiles SET avatar_url = $2, updated_at = NOW() WHERE user_id = $1
	`, userID, avatarURL)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return ErrProfileMissing
	}
	return nil
}
