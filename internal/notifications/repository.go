package notifications

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	UpsertUnread(ctx context.Context, input CreateNotificationInput) (Notification, error)
	ListForUser(ctx context.Context, recipientUserID uuid.UUID, limit int) ([]Notification, error)
	CountUnread(ctx context.Context, recipientUserID uuid.UUID) (int, error)
	MarkRead(ctx context.Context, recipientUserID uuid.UUID, notificationID uuid.UUID) error
	MarkAllRead(ctx context.Context, recipientUserID uuid.UUID) error
	GetActorProfile(ctx context.Context, userID uuid.UUID) (*NotificationActor, error)
	UpsertPushSubscription(ctx context.Context, input UpsertPushSubscriptionInput) error
	DeletePushSubscription(ctx context.Context, userID uuid.UUID, endpoint string) error
	DeletePushSubscriptionByEndpoint(ctx context.Context, endpoint string) error
	ListPushSubscriptionsForUser(ctx context.Context, userID uuid.UUID) ([]PushSubscription, error)
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) UpsertUnread(ctx context.Context, input CreateNotificationInput) (Notification, error) {
	payload := input.Data
	if payload == nil {
		payload = map[string]any{}
	}
	rawData, err := json.Marshal(payload)
	if err != nil {
		return Notification{}, err
	}
	jsonText := string(rawData)

	var item Notification
	var returnedData string

	if input.ActorUserID != nil {
		actorID := *input.ActorUserID
		err = r.pool.QueryRow(ctx, `
			UPDATE public.notifications
			SET title = $4, body = $5, data = $6::jsonb, created_at = NOW()
			WHERE id = (
				SELECT id
				FROM public.notifications
				WHERE recipient_user_id = $1::uuid
				  AND actor_user_id = $2::uuid
				  AND type = $3
				  AND read_at IS NULL
				ORDER BY created_at DESC
				LIMIT 1
			)
			RETURNING id, recipient_user_id, actor_user_id, type, title, body, data::text, read_at, created_at
		`, input.RecipientUserID, actorID, input.Type, input.Title, input.Body, jsonText).Scan(
			&item.ID,
			&item.RecipientUserID,
			&item.ActorUserID,
			&item.Type,
			&item.Title,
			&item.Body,
			&returnedData,
			&item.ReadAt,
			&item.CreatedAt,
		)
		if errors.Is(err, pgx.ErrNoRows) {
			err = r.pool.QueryRow(ctx, `
				INSERT INTO public.notifications (recipient_user_id, actor_user_id, type, title, body, data)
				VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6::jsonb)
				RETURNING id, recipient_user_id, actor_user_id, type, title, body, data::text, read_at, created_at
			`, input.RecipientUserID, actorID, input.Type, input.Title, input.Body, jsonText).Scan(
				&item.ID,
				&item.RecipientUserID,
				&item.ActorUserID,
				&item.Type,
				&item.Title,
				&item.Body,
				&returnedData,
				&item.ReadAt,
				&item.CreatedAt,
			)
		}
	} else {
		err = r.pool.QueryRow(ctx, `
			INSERT INTO public.notifications (recipient_user_id, actor_user_id, type, title, body, data)
			VALUES ($1::uuid, NULL, $2, $3, $4, $5::jsonb)
			RETURNING id, recipient_user_id, actor_user_id, type, title, body, data::text, read_at, created_at
		`, input.RecipientUserID, input.Type, input.Title, input.Body, jsonText).Scan(
			&item.ID,
			&item.RecipientUserID,
			&item.ActorUserID,
			&item.Type,
			&item.Title,
			&item.Body,
			&returnedData,
			&item.ReadAt,
			&item.CreatedAt,
		)
	}

	if err != nil {
		return Notification{}, err
	}

	if returnedData != "" {
		_ = json.Unmarshal([]byte(returnedData), &item.Data)
	}
	if item.Data == nil {
		item.Data = map[string]any{}
	}

	return item, nil
}

func (r *PostgresRepository) ListForUser(ctx context.Context, recipientUserID uuid.UUID, limit int) ([]Notification, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT
			n.id,
			n.recipient_user_id,
			n.actor_user_id,
			n.type,
			n.title,
			n.body,
			n.data::text,
			n.read_at,
			n.created_at,
			p.user_id,
			p.username,
			p.full_name,
			p.avatar_url
		FROM public.notifications n
		LEFT JOIN public.user_profiles p ON p.user_id = n.actor_user_id
		WHERE n.recipient_user_id = $1::uuid
		ORDER BY n.created_at DESC
		LIMIT $2
	`, recipientUserID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Notification, 0)
	for rows.Next() {
		var item Notification
		var rawData string
		var actorProfileID *uuid.UUID
		var actorUsername *string
		var actorFullName *string
		var actorAvatarURL *string

		if scanErr := rows.Scan(
			&item.ID,
			&item.RecipientUserID,
			&item.ActorUserID,
			&item.Type,
			&item.Title,
			&item.Body,
			&rawData,
			&item.ReadAt,
			&item.CreatedAt,
			&actorProfileID,
			&actorUsername,
			&actorFullName,
			&actorAvatarURL,
		); scanErr != nil {
			return nil, scanErr
		}

		if rawData != "" {
			_ = json.Unmarshal([]byte(rawData), &item.Data)
		}
		if item.Data == nil {
			item.Data = map[string]any{}
		}

		if actorProfileID != nil && actorUsername != nil && actorFullName != nil {
			item.Actor = &NotificationActor{
				UserID:    *actorProfileID,
				Username:  *actorUsername,
				FullName:  *actorFullName,
				AvatarURL: actorAvatarURL,
			}
		}

		items = append(items, item)
	}

	return items, rows.Err()
}

func (r *PostgresRepository) CountUnread(ctx context.Context, recipientUserID uuid.UUID) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT count(*)::int
		FROM public.notifications
		WHERE recipient_user_id = $1 AND read_at IS NULL
	`, recipientUserID).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (r *PostgresRepository) MarkRead(ctx context.Context, recipientUserID uuid.UUID, notificationID uuid.UUID) error {
	command, err := r.pool.Exec(ctx, `
		UPDATE public.notifications
		SET read_at = COALESCE(read_at, NOW())
		WHERE id = $1 AND recipient_user_id = $2
	`, notificationID, recipientUserID)
	if err != nil {
		return err
	}

	if command.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

func (r *PostgresRepository) MarkAllRead(ctx context.Context, recipientUserID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE public.notifications
		SET read_at = NOW()
		WHERE recipient_user_id = $1 AND read_at IS NULL
	`, recipientUserID)
	return err
}

func (r *PostgresRepository) GetActorProfile(ctx context.Context, userID uuid.UUID) (*NotificationActor, error) {
	var actor NotificationActor
	err := r.pool.QueryRow(ctx, `
		SELECT user_id, username, full_name, avatar_url
		FROM public.user_profiles
		WHERE user_id = $1
	`, userID).Scan(&actor.UserID, &actor.Username, &actor.FullName, &actor.AvatarURL)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &actor, nil
}

func (r *PostgresRepository) UpsertPushSubscription(ctx context.Context, input UpsertPushSubscriptionInput) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO public.push_subscriptions (user_id, endpoint, p256dh, auth, user_agent)
		VALUES ($1::uuid, $2, $3, $4, $5)
		ON CONFLICT (endpoint) DO UPDATE SET
			user_id = EXCLUDED.user_id,
			p256dh = EXCLUDED.p256dh,
			auth = EXCLUDED.auth,
			user_agent = EXCLUDED.user_agent,
			updated_at = NOW()
	`, input.UserID, input.Endpoint, input.P256DH, input.Auth, input.UserAgent)
	return err
}

func (r *PostgresRepository) DeletePushSubscription(ctx context.Context, userID uuid.UUID, endpoint string) error {
	_, err := r.pool.Exec(ctx, `
		DELETE FROM public.push_subscriptions
		WHERE user_id = $1::uuid AND endpoint = $2
	`, userID, endpoint)
	return err
}

func (r *PostgresRepository) DeletePushSubscriptionByEndpoint(ctx context.Context, endpoint string) error {
	_, err := r.pool.Exec(ctx, `
		DELETE FROM public.push_subscriptions
		WHERE endpoint = $1
	`, endpoint)
	return err
}

func (r *PostgresRepository) ListPushSubscriptionsForUser(ctx context.Context, userID uuid.UUID) ([]PushSubscription, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, endpoint, p256dh, auth, user_agent
		FROM public.push_subscriptions
		WHERE user_id = $1::uuid
		ORDER BY updated_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]PushSubscription, 0)
	for rows.Next() {
		var item PushSubscription
		if scanErr := rows.Scan(&item.ID, &item.UserID, &item.Endpoint, &item.P256DH, &item.Auth, &item.UserAgent); scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}

	return items, rows.Err()
}
