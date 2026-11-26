package notifications

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/google/uuid"

	"github.com/eleventravel/eleventravel-api/internal/http/response"
)

type Service struct {
	repository Repository
	push       PushDispatcher
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) WithPushDispatcher(dispatcher PushDispatcher) *Service {
	s.push = dispatcher
	return s
}

func (s *Service) PushConfig() (bool, string) {
	if s.push == nil {
		return false, ""
	}
	pubKey := strings.TrimSpace(s.push.PublicKey())
	return pubKey != "", pubKey
}

func (s *Service) NotifyNewFollower(ctx context.Context, actorUserID uuid.UUID, recipientUserID uuid.UUID) error {
	if actorUserID == recipientUserID {
		return nil
	}

	actor, err := s.repository.GetActorProfile(ctx, actorUserID)
	if err != nil {
		return err
	}

	displayName := "Alguien"
	data := map[string]any{
		"actor_user_id": actorUserID.String(),
	}

	if actor != nil {
		if trimmed := strings.TrimSpace(actor.FullName); trimmed != "" {
			displayName = trimmed
		} else if trimmed := strings.TrimSpace(actor.Username); trimmed != "" {
			displayName = "@" + trimmed
		}

		if trimmedUsername := strings.TrimSpace(actor.Username); trimmedUsername != "" {
			data["actor_username"] = trimmedUsername
			data["route"] = "/profile/" + trimmedUsername
		}
	}

	created, err := s.repository.UpsertUnread(ctx, CreateNotificationInput{
		RecipientUserID: recipientUserID,
		ActorUserID:     &actorUserID,
		Type:            TypeNewFollower,
		Title:           "Nuevo seguidor",
		Body:            fmt.Sprintf("%s empezó a seguirte", displayName),
		Data:            data,
	})
	if err != nil {
		return err
	}
	created.Actor = actor
	log.Printf("notifications: saved %s notification %s for recipient %s", created.Type, created.ID, recipientUserID)

	if s.push != nil {
		if pushErr := s.push.Dispatch(ctx, created); pushErr != nil {
			log.Printf("notifications: push dispatch failed for notification %s: %v", created.ID, pushErr)
		}
	}

	return nil
}

func (s *Service) ListForUser(ctx context.Context, recipientUserID uuid.UUID, limit int) (ListResult, error) {
	clampedLimit := clampLimit(limit, 50)

	items, err := s.repository.ListForUser(ctx, recipientUserID, clampedLimit)
	if err != nil {
		return ListResult{}, err
	}

	unreadCount, err := s.repository.CountUnread(ctx, recipientUserID)
	if err != nil {
		return ListResult{}, err
	}

	return ListResult{
		Items:       items,
		UnreadCount: unreadCount,
	}, nil
}

func (s *Service) CountUnread(ctx context.Context, recipientUserID uuid.UUID) (int, error) {
	return s.repository.CountUnread(ctx, recipientUserID)
}

func (s *Service) MarkRead(ctx context.Context, recipientUserID uuid.UUID, notificationID uuid.UUID) error {
	return s.repository.MarkRead(ctx, recipientUserID, notificationID)
}

func (s *Service) MarkAllRead(ctx context.Context, recipientUserID uuid.UUID) error {
	return s.repository.MarkAllRead(ctx, recipientUserID)
}

func (s *Service) SavePushSubscription(ctx context.Context, userID uuid.UUID, endpoint string, p256dh string, auth string, userAgent string) error {
	trimmedEndpoint := strings.TrimSpace(endpoint)
	trimmedP256DH := strings.TrimSpace(p256dh)
	trimmedAuth := strings.TrimSpace(auth)

	var details []response.ErrorDetail
	if !strings.HasPrefix(trimmedEndpoint, "https://") {
		details = append(details, response.ErrorDetail{Field: "endpoint", Message: "must be a valid https URL"})
	}
	if trimmedP256DH == "" {
		details = append(details, response.ErrorDetail{Field: "keys.p256dh", Message: "is required"})
	}
	if trimmedAuth == "" {
		details = append(details, response.ErrorDetail{Field: "keys.auth", Message: "is required"})
	}
	if len(details) > 0 {
		return ValidationError{Details: details}
	}

	var optionalUA *string
	if trimmedUA := strings.TrimSpace(userAgent); trimmedUA != "" {
		if len(trimmedUA) > 512 {
			trimmedUA = trimmedUA[:512]
		}
		optionalUA = &trimmedUA
	}

	return s.repository.UpsertPushSubscription(ctx, UpsertPushSubscriptionInput{
		UserID:    userID,
		Endpoint:  trimmedEndpoint,
		P256DH:    trimmedP256DH,
		Auth:      trimmedAuth,
		UserAgent: optionalUA,
	})
}

func (s *Service) DeletePushSubscription(ctx context.Context, userID uuid.UUID, endpoint string) error {
	trimmedEndpoint := strings.TrimSpace(endpoint)
	if trimmedEndpoint == "" {
		return ValidationError{Details: []response.ErrorDetail{{Field: "endpoint", Message: "is required"}}}
	}

	return s.repository.DeletePushSubscription(ctx, userID, trimmedEndpoint)
}

func clampLimit(value int, max int) int {
	if value <= 0 {
		return 30
	}
	if value > max {
		return max
	}
	return value
}
