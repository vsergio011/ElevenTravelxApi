package storage

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/eleventravel/eleventravel-api/internal/http/middleware"
	"github.com/eleventravel/eleventravel-api/internal/http/response"
)

type ImageService interface {
	CreateUpload(ctx context.Context, userID uuid.UUID, input CreateUploadInput) (UploadTicket, error)
	Complete(ctx context.Context, userID uuid.UUID, imageID uuid.UUID) (Image, string, error)
	GetURL(ctx context.Context, actorID uuid.UUID, imageID uuid.UUID) (ImageURL, error)
	Delete(ctx context.Context, actorID uuid.UUID, imageID uuid.UUID) error
}

type Handler struct {
	service ImageService
}

func NewHandler(service ImageService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(router chi.Router) {
	router.Route("/images", func(images chi.Router) {
		images.Post("/uploads", h.createUpload)
		images.Post("/{imageId}/complete", h.complete)
		images.Get("/{imageId}/url", h.getURL)
		images.Delete("/{imageId}", h.deleteImage)
	})
}

type createUploadRequest struct {
	Purpose   string `json:"purpose"`
	MimeType  string `json:"mime_type"`
	SizeBytes int64  `json:"size_bytes"`
}

type imageResponse struct {
	ID          uuid.UUID  `json:"id"`
	Purpose     string     `json:"purpose"`
	Visibility  string     `json:"visibility"`
	MimeType    string     `json:"mime_type"`
	SizeBytes   *int64     `json:"size_bytes"`
	Width       *int       `json:"width"`
	Height      *int       `json:"height"`
	Status      string     `json:"status"`
	URL         *string    `json:"url"`
	CreatedAt   time.Time  `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at"`
}

func toImageResponse(img Image, url string) imageResponse {
	var urlPtr *string
	if url != "" {
		urlPtr = &url
	}
	return imageResponse{
		ID: img.ID, Purpose: img.Purpose, Visibility: img.Visibility, MimeType: img.MimeType,
		SizeBytes: img.SizeBytes, Width: img.Width, Height: img.Height, Status: img.Status,
		URL: urlPtr, CreatedAt: img.CreatedAt, CompletedAt: img.CompletedAt,
	}
}

func (h *Handler) createUpload(writer http.ResponseWriter, request *http.Request) {
	user, ok := middleware.AuthUserFromContext(request.Context())
	if !ok {
		response.WriteError(writer, http.StatusUnauthorized, "unauthorized", "Authenticated user not found in request context", nil)
		return
	}

	var body createUploadRequest
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_json", "Invalid request body", nil)
		return
	}

	ticket, err := h.service.CreateUpload(request.Context(), user.UserID, CreateUploadInput{
		Purpose: body.Purpose, MimeType: body.MimeType, SizeBytes: body.SizeBytes,
	})
	if err != nil {
		h.writeDomainError(writer, err)
		return
	}

	response.WriteJSON(writer, http.StatusCreated, map[string]any{"data": map[string]any{
		"image_id":   ticket.Image.ID,
		"upload_url": ticket.UploadURL,
		"method":     http.MethodPut,
		"headers":    ticket.Headers,
		"expires_at": ticket.Image.ExpiresAt,
		"image":      toImageResponse(ticket.Image, ""),
	}})
}

func (h *Handler) complete(writer http.ResponseWriter, request *http.Request) {
	user, imageID, ok := h.authAndID(writer, request)
	if !ok {
		return
	}

	img, url, err := h.service.Complete(request.Context(), user, imageID)
	if err != nil {
		h.writeDomainError(writer, err)
		return
	}

	response.WriteJSON(writer, http.StatusOK, map[string]imageResponse{"data": toImageResponse(img, url)})
}

func (h *Handler) getURL(writer http.ResponseWriter, request *http.Request) {
	user, imageID, ok := h.authAndID(writer, request)
	if !ok {
		return
	}

	result, err := h.service.GetURL(request.Context(), user, imageID)
	if err != nil {
		h.writeDomainError(writer, err)
		return
	}

	response.WriteJSON(writer, http.StatusOK, map[string]any{"data": map[string]any{
		"url": result.URL, "expires_at": result.ExpiresAt,
	}})
}

func (h *Handler) deleteImage(writer http.ResponseWriter, request *http.Request) {
	user, imageID, ok := h.authAndID(writer, request)
	if !ok {
		return
	}

	if err := h.service.Delete(request.Context(), user, imageID); err != nil {
		h.writeDomainError(writer, err)
		return
	}

	response.WriteJSON(writer, http.StatusOK, map[string]any{"status": "ok"})
}

func (h *Handler) authAndID(writer http.ResponseWriter, request *http.Request) (uuid.UUID, uuid.UUID, bool) {
	user, ok := middleware.AuthUserFromContext(request.Context())
	if !ok {
		response.WriteError(writer, http.StatusUnauthorized, "unauthorized", "Authenticated user not found in request context", nil)
		return uuid.Nil, uuid.Nil, false
	}

	imageID, err := uuid.Parse(chi.URLParam(request, "imageId"))
	if err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_path_param", "imageId must be a valid UUID", nil)
		return uuid.Nil, uuid.Nil, false
	}

	return user.UserID, imageID, true
}

func (h *Handler) writeDomainError(writer http.ResponseWriter, err error) {
	var validationErr ValidationError
	if errors.As(err, &validationErr) {
		response.WriteError(writer, http.StatusBadRequest, "validation_error", "Invalid request body", validationErr.Details)
		return
	}

	switch {
	case errors.Is(err, ErrNotFound):
		response.WriteError(writer, http.StatusNotFound, "image_not_found", "Image not found", nil)
	case errors.Is(err, ErrProfileMissing):
		response.WriteError(writer, http.StatusNotFound, "profile_not_found", "Profile not found", nil)
	case errors.Is(err, ErrForbidden):
		response.WriteError(writer, http.StatusForbidden, "forbidden", "You do not have permission to perform this action", nil)
	case errors.Is(err, ErrUploadMissing):
		response.WriteError(writer, http.StatusConflict, "upload_not_found", "The file has not been uploaded yet or the upload URL expired", nil)
	case errors.Is(err, ErrTooManyPending):
		response.WriteError(writer, http.StatusTooManyRequests, "too_many_pending_uploads", "Too many pending uploads", nil)
	case errors.Is(err, ErrUnavailable):
		response.WriteError(writer, http.StatusServiceUnavailable, "storage_unavailable", "Image storage is not available", nil)
	default:
		response.WriteError(writer, http.StatusInternalServerError, "internal_error", "Unexpected server error", nil)
	}
}
