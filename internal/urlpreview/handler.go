package urlpreview

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/eleventravel/eleventravel-api/internal/http/middleware"
	"github.com/eleventravel/eleventravel-api/internal/http/response"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(router chi.Router) {
	router.Post("/plannings/{planningId}/url-preview", h.preview)
}

func (h *Handler) preview(w http.ResponseWriter, r *http.Request) {
	actor, ok := middleware.AuthUserFromContext(r.Context())
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "unauthorized", "Authenticated user not found in request context", nil)
		return
	}
	planningID, err := uuid.Parse(chi.URLParam(r, "planningId"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid_path_param", "planningId must be a valid UUID", nil)
		return
	}
	var body PreviewRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid_json", "Invalid request body", nil)
		return
	}

	result, err := h.service.Preview(r.Context(), actor.UserID, planningID, body.URL)
	if err != nil {
		switch {
		case errors.Is(err, ErrForbidden):
			response.WriteError(w, http.StatusForbidden, "forbidden", "You do not have permission to perform this action", nil)
		case errors.Is(err, ErrInvalidURL):
			response.WriteError(w, http.StatusBadRequest, "validation_error", "url must be a valid public http(s) URL", nil)
		default:
			log.Printf("urlpreview: unexpected error: %v", err)
			response.WriteError(w, http.StatusInternalServerError, "internal_error", "Unexpected server error", nil)
		}
		return
	}

	response.WriteJSON(w, http.StatusOK, map[string]PreviewResponse{"data": result})
}
