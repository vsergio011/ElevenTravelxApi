package notifications

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type stubRepository struct {
	actor       *NotificationActor
	upserted    []CreateNotificationInput
	items       []Notification
	unreadCount int
	markReadID  uuid.UUID
	markedAll   bool
}

func (s *stubRepository) UpsertUnread(_ context.Context, input CreateNotificationInput) (Notification, error) {
	s.upserted = append(s.upserted, input)
	return Notification{
		ID:              uuid.New(),
		RecipientUserID: input.RecipientUserID,
		ActorUserID:     input.ActorUserID,
		Type:            input.Type,
		Title:           input.Title,
		Body:            input.Body,
		Data:            input.Data,
		CreatedAt:       time.Now().UTC(),
	}, nil
}

func (s *stubRepository) ListForUser(_ context.Context, _ uuid.UUID, _ int) ([]Notification, error) {
	return s.items, nil
}

func (s *stubRepository) CountUnread(_ context.Context, _ uuid.UUID) (int, error) {
	return s.unreadCount, nil
}

func (s *stubRepository) MarkRead(_ context.Context, _ uuid.UUID, notificationID uuid.UUID) error {
	s.markReadID = notificationID
	return nil
}

func (s *stubRepository) MarkAllRead(_ context.Context, _ uuid.UUID) error {
	s.markedAll = true
	return nil
}

func (s *stubRepository) GetActorProfile(_ context.Context, _ uuid.UUID) (*NotificationActor, error) {
	return s.actor, nil
}

func (s *stubRepository) UpsertPushSubscription(_ context.Context, _ UpsertPushSubscriptionInput) error {
	return nil
}

func (s *stubRepository) DeletePushSubscription(_ context.Context, _ uuid.UUID, _ string) error {
	return nil
}

func (s *stubRepository) DeletePushSubscriptionByEndpoint(_ context.Context, _ string) error {
	return nil
}

func (s *stubRepository) ListPushSubscriptionsForUser(_ context.Context, _ uuid.UUID) ([]PushSubscription, error) {
	return nil, nil
}

type stubPushDispatcher struct {
	dispatched []Notification
}

func (s *stubPushDispatcher) Dispatch(_ context.Context, n Notification) error {
	s.dispatched = append(s.dispatched, n)
	return nil
}

func (s *stubPushDispatcher) PublicKey() string {
	return "test-vapid-public-key"
}

func TestNotifyNewFollowerCreatesNotificationAndDispatchesPush(t *testing.T) {
	actorID := uuid.New()
	recipientID := uuid.New()
	repo := &stubRepository{
		actor: &NotificationActor{
			UserID:   actorID,
			Username: "maria_viajera",
			FullName: "María López",
		},
	}
	push := &stubPushDispatcher{}
	service := NewService(repo).WithPushDispatcher(push)

	err := service.NotifyNewFollower(context.Background(), actorID, recipientID)
	require.NoError(t, err)
	require.Len(t, repo.upserted, 1)

	created := repo.upserted[0]
	require.Equal(t, recipientID, created.RecipientUserID)
	require.NotNil(t, created.ActorUserID)
	require.Equal(t, actorID, *created.ActorUserID)
	require.Equal(t, TypeNewFollower, created.Type)
	require.Equal(t, "Nuevo seguidor", created.Title)
	require.Equal(t, "María López empezó a seguirte", created.Body)
	require.Equal(t, "maria_viajera", created.Data["actor_username"])
	require.Equal(t, "/profile/maria_viajera", created.Data["route"])

	require.Len(t, push.dispatched, 1)
	require.Equal(t, TypeNewFollower, push.dispatched[0].Type)
}

func TestNotifyNewFollowerIgnoresSelfFollow(t *testing.T) {
	userID := uuid.New()
	repo := &stubRepository{}
	service := NewService(repo)

	err := service.NotifyNewFollower(context.Background(), userID, userID)
	require.NoError(t, err)
	require.Empty(t, repo.upserted)
}
