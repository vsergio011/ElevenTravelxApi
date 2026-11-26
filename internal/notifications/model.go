package notifications

import (
	"context"
	"time"

	"github.com/google/uuid"
)

const (
	TypeNewFollower = "new_follower"
)

type NotificationActor struct {
	UserID    uuid.UUID
	Username  string
	FullName  string
	AvatarURL *string
}

type Notification struct {
	ID              uuid.UUID
	RecipientUserID uuid.UUID
	ActorUserID     *uuid.UUID
	Type            string
	Title           string
	Body            string
	Data            map[string]any
	ReadAt          *time.Time
	CreatedAt       time.Time
	Actor           *NotificationActor
}

type CreateNotificationInput struct {
	RecipientUserID uuid.UUID
	ActorUserID     *uuid.UUID
	Type            string
	Title           string
	Body            string
	Data            map[string]any
}

type ListResult struct {
	Items       []Notification
	UnreadCount int
}

// PushDispatcher is an optional hook for delivering Web Push (or native push)
// notifications when an in-app notification is created.
type PushDispatcher interface {
	Dispatch(ctx context.Context, notification Notification) error
	PublicKey() string
}

type PushSubscription struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Endpoint  string
	P256DH    string
	Auth      string
	UserAgent *string
}

type UpsertPushSubscriptionInput struct {
	UserID    uuid.UUID
	Endpoint  string
	P256DH    string
	Auth      string
	UserAgent *string
}
