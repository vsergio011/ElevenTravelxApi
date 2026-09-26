package flights

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/eleventravel/eleventravel-api/internal/http/middleware"
	"github.com/eleventravel/eleventravel-api/internal/http/response"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"net/http"
	"strings"
	"time"
)

type FlightService interface {
	Create(context.Context, uuid.UUID, uuid.UUID, CreateInput) (Flight, error)
	Get(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (Flight, error)
	List(context.Context, uuid.UUID, uuid.UUID, Filters) ([]Flight, error)
	Lookup(context.Context, uuid.UUID, uuid.UUID, string, time.Time) (FlightLookup, error)
	Update(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, UpdateInput) (Flight, error)
	Confirm(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (Flight, error)
	Delete(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error
}
type Handler struct{ service FlightService }

func NewHandler(service FlightService) *Handler { return &Handler{service: service} }

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	actor, planning, ok := h.actorAndPlanning(w, r)
	if !ok {
		return
	}
	var body CreateRequest
	if err := decode(r, &body); err != nil {
		response.WriteError(w, 400, "invalid_json", "Invalid request body", nil)
		return
	}
	status := StatusDraft
	if body.Status != nil {
		status = *body.Status
	}
	currency := "EUR"
	if body.Currency != nil {
		currency = *body.Currency
	}
	distribution := DistributionGroup
	if body.CostDistribution != nil {
		distribution = *body.CostDistribution
	}
	cost := int64(0)
	if body.CostCents != nil {
		cost = *body.CostCents
	}
	input := CreateInput{Status: status, Airline: body.Airline, FlightNumber: body.FlightNumber, ReservationCode: body.ReservationCode, Notes: body.Notes, OfferURL: body.OfferURL, CabinClass: body.CabinClass, Baggage: body.Baggage, CostCents: cost, Currency: currency, CostDistribution: distribution, ParticipantUserIDs: body.ParticipantUserIDs, Segments: toSegments(body.Segments)}
	flight, err := h.service.Create(r.Context(), actor, planning, input)
	if err != nil {
		h.writeError(w, err)
		return
	}
	response.WriteJSON(w, 201, map[string]FlightResponse{"data": toResponse(flight)})
}
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	actor, planning, ok := h.actorAndPlanning(w, r)
	if !ok {
		return
	}
	filters := Filters{}
	if status := r.URL.Query().Get("status"); status != "" {
		filters.Status = &status
	}
	items, err := h.service.List(r.Context(), actor, planning, filters)
	if err != nil {
		h.writeError(w, err)
		return
	}
	data := make([]FlightResponse, len(items))
	for i, item := range items {
		data[i] = toResponse(item)
	}
	response.WriteJSON(w, 200, map[string]any{"data": data})
}
func (h *Handler) lookup(w http.ResponseWriter, r *http.Request) {
	actor, planning, ok := h.actorAndPlanning(w, r)
	if !ok {
		return
	}
	flightNumber := strings.TrimSpace(r.URL.Query().Get("flight_number"))
	if flightNumber == "" {
		response.WriteError(w, 400, "validation_error", "flight_number is required", nil)
		return
	}
	flightDate, err := time.Parse("2006-01-02", r.URL.Query().Get("flight_date"))
	if err != nil {
		response.WriteError(w, 400, "validation_error", "flight_date must use YYYY-MM-DD", nil)
		return
	}
	lookup, err := h.service.Lookup(r.Context(), actor, planning, flightNumber, flightDate)
	if err != nil {
		h.writeError(w, err)
		return
	}
	response.WriteJSON(w, 200, map[string]FlightLookup{"data": lookup})
}
func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	actor, planning, id, ok := h.ids(w, r)
	if !ok {
		return
	}
	flight, err := h.service.Get(r.Context(), actor, planning, id)
	if err != nil {
		h.writeError(w, err)
		return
	}
	response.WriteJSON(w, 200, map[string]FlightResponse{"data": toResponse(flight)})
}
func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	actor, planning, id, ok := h.ids(w, r)
	if !ok {
		return
	}
	var body UpdateRequest
	if err := decode(r, &body); err != nil {
		response.WriteError(w, 400, "invalid_json", "Invalid request body", nil)
		return
	}
	input := UpdateInput{Status: body.Status, Airline: body.Airline, FlightNumber: body.FlightNumber, ReservationCode: body.ReservationCode, Notes: body.Notes, OfferURL: body.OfferURL, CabinClass: body.CabinClass, Baggage: body.Baggage, CostCents: body.CostCents, Currency: body.Currency, CostDistribution: body.CostDistribution, ParticipantUserIDs: body.ParticipantUserIDs}
	if body.Segments != nil {
		segments := toSegments(*body.Segments)
		input.Segments = &segments
	}
	flight, err := h.service.Update(r.Context(), actor, planning, id, input)
	if err != nil {
		h.writeError(w, err)
		return
	}
	response.WriteJSON(w, 200, map[string]FlightResponse{"data": toResponse(flight)})
}
func (h *Handler) confirm(w http.ResponseWriter, r *http.Request) {
	actor, planning, id, ok := h.ids(w, r)
	if !ok {
		return
	}
	flight, err := h.service.Confirm(r.Context(), actor, planning, id)
	if err != nil {
		h.writeError(w, err)
		return
	}
	response.WriteJSON(w, 200, map[string]FlightResponse{"data": toResponse(flight)})
}
func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	actor, planning, id, ok := h.ids(w, r)
	if !ok {
		return
	}
	if err := h.service.Delete(r.Context(), actor, planning, id); err != nil {
		h.writeError(w, err)
		return
	}
	response.WriteJSON(w, 200, map[string]string{"status": "deleted"})
}
func (h *Handler) actorAndPlanning(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	actor, ok := middleware.AuthUserFromContext(r.Context())
	if !ok {
		response.WriteError(w, 401, "unauthorized", "Authenticated user not found in request context", nil)
		return uuid.Nil, uuid.Nil, false
	}
	planning, err := uuid.Parse(chi.URLParam(r, "planningId"))
	if err != nil {
		response.WriteError(w, 400, "invalid_path_param", "planningId must be a valid UUID", nil)
		return uuid.Nil, uuid.Nil, false
	}
	return actor.UserID, planning, true
}
func (h *Handler) ids(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, uuid.UUID, bool) {
	actor, planning, ok := h.actorAndPlanning(w, r)
	if !ok {
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	id, err := uuid.Parse(chi.URLParam(r, "flightId"))
	if err != nil {
		response.WriteError(w, 400, "invalid_path_param", "flightId must be a valid UUID", nil)
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	return actor, planning, id, true
}
func (h *Handler) writeError(w http.ResponseWriter, err error) {
	var validation responseValidationError
	if errors.As(err, &validation) {
		response.WriteError(w, 400, "validation_error", "Invalid request body", validation.details)
		return
	}
	switch {
	case errors.Is(err, ErrFlightNotFound):
		response.WriteError(w, 404, "flight_not_found", "Flight not found", nil)
	case errors.Is(err, ErrFlightLookupNotFound):
		response.WriteError(w, 404, "flight_lookup_not_found", "Flight not found for the selected date", nil)
	case errors.Is(err, ErrFlightLookupUnavailable):
		response.WriteError(w, 503, "flight_lookup_unavailable", "Flight lookup is not available", nil)
	case errors.Is(err, ErrForbidden):
		response.WriteError(w, 403, "forbidden", "You do not have permission to perform this action", nil)
	default:
		response.WriteError(w, 500, "internal_error", "Unexpected server error", nil)
	}
}
func decode(r *http.Request, destination any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(destination)
}
