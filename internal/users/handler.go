package users

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/eleventravel/eleventravel-api/internal/http/middleware"
	"github.com/eleventravel/eleventravel-api/internal/http/response"
)

type UserProfileService interface {
	GetMyProfile(ctx context.Context, actorUserID uuid.UUID) (UserProfileView, error)
	GetProfileByUsername(ctx context.Context, actorUserID uuid.UUID, username string) (UserProfileView, error)
	UpdateMyProfile(ctx context.Context, actorUserID uuid.UUID, input UpdateUserProfileInput) (UserProfileView, error)
	FollowUser(ctx context.Context, actorUserID uuid.UUID, targetUserID uuid.UUID) error
	UnfollowUser(ctx context.Context, actorUserID uuid.UUID, targetUserID uuid.UUID) error
	ListFollowers(ctx context.Context, actorUserID uuid.UUID, targetUserID uuid.UUID, limit int) ([]UserSummary, error)
	ListFollowing(ctx context.Context, actorUserID uuid.UUID, targetUserID uuid.UUID, limit int) ([]UserSummary, error)
	SearchUsers(ctx context.Context, actorUserID uuid.UUID, query string, limit int) ([]UserSearchResult, error)
	AddLocation(ctx context.Context, actorUserID uuid.UUID, targetUserID uuid.UUID, input CreateUserLocationInput) (UserLocationPin, error)
	UpdateLocation(ctx context.Context, actorUserID uuid.UUID, targetUserID uuid.UUID, locationID uuid.UUID, input UpdateUserLocationInput) (UserLocationPin, error)
	DeleteLocation(ctx context.Context, actorUserID uuid.UUID, targetUserID uuid.UUID, locationID uuid.UUID) error
	AddMedia(ctx context.Context, actorUserID uuid.UUID, targetUserID uuid.UUID, input CreateUserMediaInput) (UserMediaAsset, error)
	VerifyAchievement(ctx context.Context, actorUserID uuid.UUID, targetUserID uuid.UUID, input VerifyAchievementInput) (UserBadge, error)
}

type Handler struct {
	service UserProfileService
}

func NewHandler(service UserProfileService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) getMyProfile(writer http.ResponseWriter, request *http.Request) {
	actorUser, ok := middleware.AuthUserFromContext(request.Context())
	if !ok {
		response.WriteError(writer, http.StatusUnauthorized, "unauthorized", "Authenticated user not found in request context", nil)
		return
	}

	view, err := h.service.GetMyProfile(request.Context(), actorUser.UserID)
	if err != nil {
		h.writeDomainError(writer, err)
		return
	}

	response.WriteJSON(writer, http.StatusOK, map[string]UserProfileResponse{"data": toProfileResponse(view)})
}

func (h *Handler) updateMyProfile(writer http.ResponseWriter, request *http.Request) {
	actorUser, ok := middleware.AuthUserFromContext(request.Context())
	if !ok {
		response.WriteError(writer, http.StatusUnauthorized, "unauthorized", "Authenticated user not found in request context", nil)
		return
	}

	var body UpdateUserProfileRequest
	if err := decodeJSON(request, &body); err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_json", "Invalid request body", nil)
		return
	}

	input := UpdateUserProfileInput{
		FullName:             body.FullName,
		Bio:                  body.Bio,
		AvatarURL:            body.AvatarURL,
		MultimediaVisibility: normalizeOptionalString(body.MultimediaVisibility),
		BadgesVisibility:     normalizeOptionalString(body.BadgesVisibility),
	}

	view, err := h.service.UpdateMyProfile(request.Context(), actorUser.UserID, input)
	if err != nil {
		h.writeDomainError(writer, err)
		return
	}

	response.WriteJSON(writer, http.StatusOK, map[string]UserProfileResponse{"data": toProfileResponse(view)})
}

func (h *Handler) getProfileByUsername(writer http.ResponseWriter, request *http.Request) {
	actorUser, ok := middleware.AuthUserFromContext(request.Context())
	if !ok {
		response.WriteError(writer, http.StatusUnauthorized, "unauthorized", "Authenticated user not found in request context", nil)
		return
	}

	username := strings.TrimSpace(chi.URLParam(request, "username"))
	if !isValidUsername(username) {
		response.WriteError(writer, http.StatusBadRequest, "invalid_path_param", "username must be 3-30 chars using letters, numbers, _, -, .", nil)
		return
	}

	view, err := h.service.GetProfileByUsername(request.Context(), actorUser.UserID, username)
	if err != nil {
		h.writeDomainError(writer, err)
		return
	}

	response.WriteJSON(writer, http.StatusOK, map[string]UserProfileResponse{"data": toProfileResponse(view)})
}

func (h *Handler) followUser(writer http.ResponseWriter, request *http.Request) {
	actorUser, ok := middleware.AuthUserFromContext(request.Context())
	if !ok {
		response.WriteError(writer, http.StatusUnauthorized, "unauthorized", "Authenticated user not found in request context", nil)
		return
	}

	targetUserID, err := parseUUIDParam(request, "userId")
	if err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_path_param", err.Error(), nil)
		return
	}

	if err := h.service.FollowUser(request.Context(), actorUser.UserID, targetUserID); err != nil {
		h.writeDomainError(writer, err)
		return
	}

	response.WriteJSON(writer, http.StatusCreated, map[string]string{"status": "following"})
}

func (h *Handler) unfollowUser(writer http.ResponseWriter, request *http.Request) {
	actorUser, ok := middleware.AuthUserFromContext(request.Context())
	if !ok {
		response.WriteError(writer, http.StatusUnauthorized, "unauthorized", "Authenticated user not found in request context", nil)
		return
	}

	targetUserID, err := parseUUIDParam(request, "userId")
	if err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_path_param", err.Error(), nil)
		return
	}

	if err := h.service.UnfollowUser(request.Context(), actorUser.UserID, targetUserID); err != nil {
		h.writeDomainError(writer, err)
		return
	}

	response.WriteJSON(writer, http.StatusOK, map[string]string{"status": "unfollowed"})
}

func (h *Handler) listFollowers(writer http.ResponseWriter, request *http.Request) {
	h.listFollows(writer, request, true)
}

func (h *Handler) listFollowing(writer http.ResponseWriter, request *http.Request) {
	h.listFollows(writer, request, false)
}

func (h *Handler) searchUsers(writer http.ResponseWriter, request *http.Request) {
	actorUser, ok := middleware.AuthUserFromContext(request.Context())
	if !ok {
		response.WriteError(writer, http.StatusUnauthorized, "unauthorized", "Authenticated user not found in request context", nil)
		return
	}

	query := strings.TrimSpace(request.URL.Query().Get("q"))
	if len(query) < 2 {
		response.WriteError(writer, http.StatusBadRequest, "validation_error", "q must be at least 2 characters", []response.ErrorDetail{{Field: "q", Message: "must be at least 2 characters"}})
		return
	}

	items, err := h.service.SearchUsers(request.Context(), actorUser.UserID, query, parseIntQueryParam(request, "limit", 20))
	if err != nil {
		h.writeDomainError(writer, err)
		return
	}

	response.WriteJSON(writer, http.StatusOK, map[string][]UserSearchResultResponse{"data": toUserSearchResultResponse(items)})
}

func (h *Handler) listFollows(writer http.ResponseWriter, request *http.Request, followers bool) {
	actorUser, ok := middleware.AuthUserFromContext(request.Context())
	if !ok {
		response.WriteError(writer, http.StatusUnauthorized, "unauthorized", "Authenticated user not found in request context", nil)
		return
	}

	targetUserID, err := parseUUIDParam(request, "userId")
	if err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_path_param", err.Error(), nil)
		return
	}

	limit := parseIntQueryParam(request, "limit", 20)

	var items []UserSummary
	if followers {
		items, err = h.service.ListFollowers(request.Context(), actorUser.UserID, targetUserID, limit)
	} else {
		items, err = h.service.ListFollowing(request.Context(), actorUser.UserID, targetUserID, limit)
	}
	if err != nil {
		h.writeDomainError(writer, err)
		return
	}

	response.WriteJSON(writer, http.StatusOK, map[string][]UserSummaryResponse{"data": toUserSummaryResponse(items)})
}

func (h *Handler) createLocation(writer http.ResponseWriter, request *http.Request) {
	actorUser, ok := middleware.AuthUserFromContext(request.Context())
	if !ok {
		response.WriteError(writer, http.StatusUnauthorized, "unauthorized", "Authenticated user not found in request context", nil)
		return
	}

	targetUserID, err := parseUUIDParam(request, "userId")
	if err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_path_param", err.Error(), nil)
		return
	}

	var body CreateUserLocationRequest
	if err := decodeJSON(request, &body); err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_json", "Invalid request body", nil)
		return
	}

	var visitedAt *time.Time
	if body.VisitedAt != nil {
		parsedAt, parseErr := time.Parse(time.RFC3339, *body.VisitedAt)
		if parseErr != nil {
			response.WriteError(writer, http.StatusBadRequest, "validation_error", "Invalid request body", []response.ErrorDetail{{Field: "visited_at", Message: "must be RFC3339 date-time"}})
			return
		}
		visitedAt = &parsedAt
	}

	item, err := h.service.AddLocation(request.Context(), actorUser.UserID, targetUserID, CreateUserLocationInput{
		Name:        body.Name,
		Latitude:    body.Latitude,
		Longitude:   body.Longitude,
		CountryCode: body.CountryCode,
		VisitedAt:   visitedAt,
	})
	if err != nil {
		h.writeDomainError(writer, err)
		return
	}

	response.WriteJSON(writer, http.StatusCreated, map[string]UserLocationPinResponse{"data": {
		LocationID:               item.LocationID,
		Name:                     item.Name,
		Latitude:                 item.Latitude,
		Longitude:                item.Longitude,
		CountryCode:              item.CountryCode,
		ThumbnailURL:             item.ThumbnailURL,
		MediaCount:               item.MediaCount,
		HasVerifiedAchievement:   item.HasVerifiedAchievement,
		VerifiedAchievementCount: item.VerifiedAchievementCount,
	}})
}

func (h *Handler) updateLocation(writer http.ResponseWriter, request *http.Request) {
	actorUser, ok := middleware.AuthUserFromContext(request.Context())
	if !ok {
		response.WriteError(writer, http.StatusUnauthorized, "unauthorized", "Authenticated user not found in request context", nil)
		return
	}

	targetUserID, err := parseUUIDParam(request, "userId")
	if err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_path_param", err.Error(), nil)
		return
	}

	locationID, err := parseUUIDParam(request, "locationId")
	if err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_path_param", err.Error(), nil)
		return
	}

	var body UpdateUserLocationRequest
	if err := decodeJSON(request, &body); err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_json", "Invalid request body", nil)
		return
	}

	item, err := h.service.UpdateLocation(request.Context(), actorUser.UserID, targetUserID, locationID, UpdateUserLocationInput{
		Name:        body.Name,
		Latitude:    body.Latitude,
		Longitude:   body.Longitude,
		CountryCode: body.CountryCode,
	})
	if err != nil {
		h.writeDomainError(writer, err)
		return
	}

	response.WriteJSON(writer, http.StatusOK, map[string]UserLocationPinResponse{"data": {
		LocationID:               item.LocationID,
		Name:                     item.Name,
		Latitude:                 item.Latitude,
		Longitude:                item.Longitude,
		CountryCode:              item.CountryCode,
		ThumbnailURL:             item.ThumbnailURL,
		MediaCount:               item.MediaCount,
		HasVerifiedAchievement:   item.HasVerifiedAchievement,
		VerifiedAchievementCount: item.VerifiedAchievementCount,
	}})
}

func (h *Handler) deleteLocation(writer http.ResponseWriter, request *http.Request) {
	actorUser, ok := middleware.AuthUserFromContext(request.Context())
	if !ok {
		response.WriteError(writer, http.StatusUnauthorized, "unauthorized", "Authenticated user not found in request context", nil)
		return
	}

	targetUserID, err := parseUUIDParam(request, "userId")
	if err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_path_param", err.Error(), nil)
		return
	}

	locationID, err := parseUUIDParam(request, "locationId")
	if err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_path_param", err.Error(), nil)
		return
	}

	if err := h.service.DeleteLocation(request.Context(), actorUser.UserID, targetUserID, locationID); err != nil {
		h.writeDomainError(writer, err)
		return
	}

	response.WriteJSON(writer, http.StatusOK, map[string]any{"status": "ok"})
}

func (h *Handler) createMedia(writer http.ResponseWriter, request *http.Request) {
	actorUser, ok := middleware.AuthUserFromContext(request.Context())
	if !ok {
		response.WriteError(writer, http.StatusUnauthorized, "unauthorized", "Authenticated user not found in request context", nil)
		return
	}

	targetUserID, err := parseUUIDParam(request, "userId")
	if err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_path_param", err.Error(), nil)
		return
	}

	var body CreateUserMediaRequest
	if err := decodeJSON(request, &body); err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_json", "Invalid request body", nil)
		return
	}

	locationID, err := uuid.Parse(body.LocationID)
	if err != nil {
		response.WriteError(writer, http.StatusBadRequest, "validation_error", "Invalid request body", []response.ErrorDetail{{Field: "location_id", Message: "must be a valid UUID"}})
		return
	}

	item, err := h.service.AddMedia(request.Context(), actorUser.UserID, targetUserID, CreateUserMediaInput{
		LocationID:   locationID,
		MediaType:    body.MediaType,
		URL:          body.URL,
		ThumbnailURL: body.ThumbnailURL,
		Caption:      body.Caption,
	})
	if err != nil {
		h.writeDomainError(writer, err)
		return
	}

	response.WriteJSON(writer, http.StatusCreated, map[string]UserMediaAssetResponse{"data": {
		ID:           item.ID,
		LocationID:   item.LocationID,
		MediaType:    item.MediaType,
		URL:          item.URL,
		ThumbnailURL: item.ThumbnailURL,
		Caption:      item.Caption,
		CreatedAt:    item.CreatedAt,
	}})
}

func (h *Handler) verifyAchievement(writer http.ResponseWriter, request *http.Request) {
	actorUser, ok := middleware.AuthUserFromContext(request.Context())
	if !ok {
		response.WriteError(writer, http.StatusUnauthorized, "unauthorized", "Authenticated user not found in request context", nil)
		return
	}

	targetUserID, err := parseUUIDParam(request, "userId")
	if err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_path_param", err.Error(), nil)
		return
	}

	var body VerifyAchievementRequest
	if err := decodeJSON(request, &body); err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_json", "Invalid request body", nil)
		return
	}

	locationID, err := uuid.Parse(body.LocationID)
	if err != nil {
		response.WriteError(writer, http.StatusBadRequest, "validation_error", "Invalid request body", []response.ErrorDetail{{Field: "location_id", Message: "must be a valid UUID"}})
		return
	}

	badgeID, err := uuid.Parse(body.BadgeID)
	if err != nil {
		response.WriteError(writer, http.StatusBadRequest, "validation_error", "Invalid request body", []response.ErrorDetail{{Field: "badge_id", Message: "must be a valid UUID"}})
		return
	}

	badge, err := h.service.VerifyAchievement(request.Context(), actorUser.UserID, targetUserID, VerifyAchievementInput{
		LocationID: locationID,
		BadgeID:    badgeID,
	})
	if err != nil {
		h.writeDomainError(writer, err)
		return
	}

	response.WriteJSON(writer, http.StatusCreated, map[string]UserBadgeResponse{"data": {
		ID:           badge.ID,
		BadgeID:      badge.BadgeID,
		Code:         badge.Code,
		Title:        badge.Title,
		IconURL:      badge.IconURL,
		LocationID:   badge.LocationID,
		LocationName: badge.LocationName,
		UnlockedAt:   badge.UnlockedAt,
	}})
}

func (h *Handler) writeDomainError(writer http.ResponseWriter, err error) {
	var validationErr ValidationError
	if errors.As(err, &validationErr) {
		response.WriteError(writer, http.StatusBadRequest, "validation_error", "Invalid request body", validationErr.Details)
		return
	}

	switch {
	case errors.Is(err, ErrProfileNotFound):
		response.WriteError(writer, http.StatusNotFound, "profile_not_found", "Profile not found", nil)
	case errors.Is(err, ErrLocationNotFound):
		response.WriteError(writer, http.StatusNotFound, "location_not_found", "Location not found", nil)
	case errors.Is(err, ErrBadgeNotFound):
		response.WriteError(writer, http.StatusNotFound, "badge_not_found", "Badge not found", nil)
	case errors.Is(err, ErrBadgeUnavailable):
		response.WriteError(writer, http.StatusConflict, "badge_unavailable", "Badge is not available", nil)
	case errors.Is(err, ErrBadgeLocationMismatch):
		response.WriteError(writer, http.StatusBadRequest, "badge_location_mismatch", "Location does not qualify for this badge", nil)
	case errors.Is(err, ErrForbidden):
		response.WriteError(writer, http.StatusForbidden, "forbidden", "You do not have permission to perform this action", nil)
	case errors.Is(err, ErrConflict):
		response.WriteError(writer, http.StatusConflict, "conflict", "Resource already exists", nil)
	case errors.Is(err, ErrInvalidPrivacyValue):
		response.WriteError(writer, http.StatusBadRequest, "validation_error", "Invalid privacy value", nil)
	default:
		response.WriteError(writer, http.StatusInternalServerError, "internal_error", "Unexpected server error", nil)
	}
}

func parseUUIDParam(request *http.Request, name string) (uuid.UUID, error) {
	value := chi.URLParam(request, name)
	parsedValue, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil, errors.New(name + " must be a valid UUID")
	}

	return parsedValue, nil
}

func decodeJSON(request *http.Request, destination any) error {
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(destination)
}

func parseIntQueryParam(request *http.Request, name string, fallback int) int {
	value := request.URL.Query().Get(name)
	if value == "" {
		return fallback
	}

	parsedValue, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}

	return parsedValue
}

func normalizeOptionalString(value *string) *string {
	if value == nil {
		return nil
	}

	normalized := strings.TrimSpace(*value)
	return &normalized
}
