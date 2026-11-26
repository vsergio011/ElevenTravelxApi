package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/eleventravel/eleventravel-api/internal/http/middleware"
	"github.com/eleventravel/eleventravel-api/internal/http/response"
)

type NotificationService interface {
	ListForUser(ctx context.Context, recipientUserID uuid.UUID, limit int) (ListResult, error)
	CountUnread(ctx context.Context, recipientUserID uuid.UUID) (int, error)
	MarkRead(ctx context.Context, recipientUserID uuid.UUID, notificationID uuid.UUID) error
	MarkAllRead(ctx context.Context, recipientUserID uuid.UUID) error
	PushConfig() (bool, string)
	SavePushSubscription(ctx context.Context, userID uuid.UUID, endpoint string, p256dh string, auth string, userAgent string) error
	DeletePushSubscription(ctx context.Context, userID uuid.UUID, endpoint string) error
}

type Handler struct {
	service NotificationService
}

func NewHandler(service NotificationService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) listNotifications(writer http.ResponseWriter, request *http.Request) {
	actorUser, ok := middleware.AuthUserFromContext(request.Context())
	if !ok {
		response.WriteError(writer, http.StatusUnauthorized, "unauthorized", "Authenticated user not found in request context", nil)
		return
	}

	limit := parseIntQueryParam(request, "limit", 30)
	result, err := h.service.ListForUser(request.Context(), actorUser.UserID, limit)
	if err != nil {
		h.writeDomainError(writer, err)
		return
	}

	response.WriteJSON(writer, http.StatusOK, map[string]NotificationListResponse{
		"data": toNotificationListResponse(result),
	})
}

func (h *Handler) getUnreadCount(writer http.ResponseWriter, request *http.Request) {
	actorUser, ok := middleware.AuthUserFromContext(request.Context())
	if !ok {
		response.WriteError(writer, http.StatusUnauthorized, "unauthorized", "Authenticated user not found in request context", nil)
		return
	}

	count, err := h.service.CountUnread(request.Context(), actorUser.UserID)
	if err != nil {
		h.writeDomainError(writer, err)
		return
	}

	response.WriteJSON(writer, http.StatusOK, map[string]UnreadCountResponse{
		"data": {UnreadCount: count},
	})
}

func (h *Handler) markRead(writer http.ResponseWriter, request *http.Request) {
	actorUser, ok := middleware.AuthUserFromContext(request.Context())
	if !ok {
		response.WriteError(writer, http.StatusUnauthorized, "unauthorized", "Authenticated user not found in request context", nil)
		return
	}

	notificationID, err := parseUUIDParam(request, "notificationId")
	if err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_path_param", err.Error(), nil)
		return
	}

	if err := h.service.MarkRead(request.Context(), actorUser.UserID, notificationID); err != nil {
		h.writeDomainError(writer, err)
		return
	}

	response.WriteJSON(writer, http.StatusOK, map[string]string{"status": "read"})
}

func (h *Handler) markAllRead(writer http.ResponseWriter, request *http.Request) {
	actorUser, ok := middleware.AuthUserFromContext(request.Context())
	if !ok {
		response.WriteError(writer, http.StatusUnauthorized, "unauthorized", "Authenticated user not found in request context", nil)
		return
	}

	if err := h.service.MarkAllRead(request.Context(), actorUser.UserID); err != nil {
		h.writeDomainError(writer, err)
		return
	}

	response.WriteJSON(writer, http.StatusOK, map[string]string{"status": "all_read"})
}

func (h *Handler) getPushConfig(writer http.ResponseWriter, request *http.Request) {
	if _, ok := middleware.AuthUserFromContext(request.Context()); !ok {
		response.WriteError(writer, http.StatusUnauthorized, "unauthorized", "Authenticated user not found in request context", nil)
		return
	}

	enabled, publicKey := h.service.PushConfig()
	response.WriteJSON(writer, http.StatusOK, map[string]PushConfigResponse{
		"data": {
			Enabled:        enabled,
			VAPIDPublicKey: publicKey,
		},
	})
}

func (h *Handler) upsertPushSubscription(writer http.ResponseWriter, request *http.Request) {
	actorUser, ok := middleware.AuthUserFromContext(request.Context())
	if !ok {
		response.WriteError(writer, http.StatusUnauthorized, "unauthorized", "Authenticated user not found in request context", nil)
		return
	}

	var body UpsertPushSubscriptionRequest
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_json", "Invalid request body", nil)
		return
	}

	if err := h.service.SavePushSubscription(
		request.Context(),
		actorUser.UserID,
		body.Endpoint,
		body.Keys.P256DH,
		body.Keys.Auth,
		request.UserAgent(),
	); err != nil {
		h.writeDomainError(writer, err)
		return
	}

	response.WriteJSON(writer, http.StatusCreated, map[string]string{"status": "subscribed"})
}

func (h *Handler) deletePushSubscription(writer http.ResponseWriter, request *http.Request) {
	actorUser, ok := middleware.AuthUserFromContext(request.Context())
	if !ok {
		response.WriteError(writer, http.StatusUnauthorized, "unauthorized", "Authenticated user not found in request context", nil)
		return
	}

	var body DeletePushSubscriptionRequest
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_json", "Invalid request body", nil)
		return
	}

	if err := h.service.DeletePushSubscription(request.Context(), actorUser.UserID, body.Endpoint); err != nil {
		h.writeDomainError(writer, err)
		return
	}

	response.WriteJSON(writer, http.StatusOK, map[string]string{"status": "unsubscribed"})
}

func (h *Handler) writeDomainError(writer http.ResponseWriter, err error) {
	var validationErr ValidationError
	switch {
	case errors.As(err, &validationErr):
		response.WriteError(writer, http.StatusBadRequest, "validation_error", "Validation failed", validationErr.Details)
	case errors.Is(err, ErrNotFound):
		response.WriteError(writer, http.StatusNotFound, "not_found", "Notification not found", nil)
	default:
		log.Printf("notifications internal error: %v", err)
		response.WriteError(writer, http.StatusInternalServerError, "internal_error", "Unexpected server error", nil)
	}
}

func parseUUIDParam(request *http.Request, name string) (uuid.UUID, error) {
	raw := strings.TrimSpace(chi.URLParam(request, name))
	parsed, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, errors.New(name + " must be a valid UUID")
	}

	return parsed, nil
}

func parseIntQueryParam(request *http.Request, key string, fallback int) int {
	raw := strings.TrimSpace(request.URL.Query().Get(key))
	if raw == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}

	return parsed
}
