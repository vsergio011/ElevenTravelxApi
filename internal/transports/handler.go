package transports

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

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

func (h *Handler) RegisterRoutes(router chi.Router) {
	router.Route("/plannings/{planningId}/transports", func(r chi.Router) {
		r.Get("/", h.list)
		r.Post("/", h.create)
		r.Get("/{transportId}", h.get)
		r.Patch("/{transportId}", h.update)
		r.Post("/{transportId}/confirm", h.confirm)
		r.Delete("/{transportId}", h.delete)
	})
}

func ids(w http.ResponseWriter, r *http.Request, requireID bool) (uuid.UUID, uuid.UUID, uuid.UUID, bool) {
	actor, ok := middleware.AuthUserFromContext(r.Context())
	if !ok {
		response.WriteError(w, 401, "unauthorized", "Authenticated user not found", nil)
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	planning, err := uuid.Parse(chi.URLParam(r, "planningId"))
	if err != nil {
		response.WriteError(w, 400, "invalid_path_param", "Invalid planning ID", nil)
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	id := uuid.Nil
	if requireID {
		id, err = uuid.Parse(chi.URLParam(r, "transportId"))
		if err != nil {
			response.WriteError(w, 400, "invalid_path_param", "Invalid transport ID", nil)
			return uuid.Nil, uuid.Nil, uuid.Nil, false
		}
	}
	return actor.UserID, planning, id, true
}

func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		response.WriteError(w, 404, "transport_not_found", "Transport not found", nil)
	case errors.Is(err, ErrForbidden):
		response.WriteError(w, 403, "forbidden", "You do not have permission to perform this action", nil)
	case errors.Is(err, ErrValidation):
		response.WriteError(w, 400, "validation_error", err.Error(), nil)
	default:
		log.Printf("transports: unexpected error: %v", err)
		response.WriteError(w, 500, "internal_error", "Unexpected server error", nil)
	}
}

func readInput(w http.ResponseWriter, r *http.Request) (Input, bool) {
	var input Input
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		response.WriteError(w, 400, "invalid_json", "Invalid request body", nil)
		return input, false
	}
	return input, true
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	actor, planning, _, ok := ids(w, r, false)
	if !ok {
		return
	}
	items, err := h.service.List(r.Context(), actor, planning)
	if err != nil {
		writeError(w, err)
		return
	}
	response.WriteJSON(w, 200, map[string][]Transport{"data": items})
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	actor, planning, _, ok := ids(w, r, false)
	if !ok {
		return
	}
	input, ok := readInput(w, r)
	if !ok {
		return
	}
	item, err := h.service.Create(r.Context(), actor, planning, input)
	if err != nil {
		writeError(w, err)
		return
	}
	response.WriteJSON(w, 201, map[string]Transport{"data": item})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	actor, planning, id, ok := ids(w, r, true)
	if !ok {
		return
	}
	item, err := h.service.Get(r.Context(), actor, planning, id)
	if err != nil {
		writeError(w, err)
		return
	}
	response.WriteJSON(w, 200, map[string]Transport{"data": item})
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	actor, planning, id, ok := ids(w, r, true)
	if !ok {
		return
	}
	current, err := h.service.Get(r.Context(), actor, planning, id)
	if err != nil {
		writeError(w, err)
		return
	}
	input := current.Input
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		response.WriteError(w, 400, "invalid_json", "Invalid request body", nil)
		return
	}
	item, err := h.service.Update(r.Context(), actor, planning, id, input)
	if err != nil {
		writeError(w, err)
		return
	}
	response.WriteJSON(w, 200, map[string]Transport{"data": item})
}

func (h *Handler) confirm(w http.ResponseWriter, r *http.Request) {
	actor, planning, id, ok := ids(w, r, true)
	if !ok {
		return
	}
	item, err := h.service.Confirm(r.Context(), actor, planning, id)
	if err != nil {
		writeError(w, err)
		return
	}
	response.WriteJSON(w, 200, map[string]Transport{"data": item})
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	actor, planning, id, ok := ids(w, r, true)
	if !ok {
		return
	}
	if err := h.service.Delete(r.Context(), actor, planning, id); err != nil {
		writeError(w, err)
		return
	}
	response.WriteJSON(w, 200, map[string]string{"status": "deleted"})
}
