package bookings

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/eleventravel/eleventravel-api/internal/http/middleware"
	"github.com/eleventravel/eleventravel-api/internal/http/response"
)

type BookingService interface {
	CreateBooking(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, input CreateBookingInput) (Booking, error)
	GetBooking(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, bookingID uuid.UUID) (Booking, error)
	ListBookings(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, filters BookingFilters) (PageResult[Booking], error)
	UpdateBooking(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, bookingID uuid.UUID, input UpdateBookingInput) (Booking, error)
	ConfirmBooking(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, bookingID uuid.UUID) (Booking, error)
	DeleteBooking(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, bookingID uuid.UUID) error
	GetBudget(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID) ([]BudgetLine, error)
}

func (h *Handler) getBudget(writer http.ResponseWriter, request *http.Request) {
	actor, ok := middleware.AuthUserFromContext(request.Context())
	if !ok {
		response.WriteError(writer, http.StatusUnauthorized, "unauthorized", "Authenticated user not found in request context", nil)
		return
	}
	planningID, err := parseUUIDParam(request, "planningId")
	if err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_path_param", err.Error(), nil)
		return
	}
	lines, err := h.service.GetBudget(request.Context(), actor.UserID, planningID)
	if err != nil {
		h.writeDomainError(writer, err)
		return
	}
	result := make([]BudgetLineResponse, 0, len(lines))
	for _, line := range lines {
		result = append(result, BudgetLineResponse{UserID: line.UserID, Cents: line.Cents})
	}
	response.WriteJSON(writer, http.StatusOK, BudgetResponse{Data: result})
}

type Handler struct {
	service BookingService
}

func NewHandler(service BookingService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) createBooking(writer http.ResponseWriter, request *http.Request) {
	actorUser, ok := middleware.AuthUserFromContext(request.Context())
	if !ok {
		response.WriteError(writer, http.StatusUnauthorized, "unauthorized", "Authenticated user not found in request context", nil)
		return
	}

	planningID, err := parseUUIDParam(request, "planningId")
	if err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_path_param", err.Error(), nil)
		return
	}

	var requestBody CreateBookingRequest
	if err := decodeJSON(request, &requestBody); err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_json", "Invalid request body", nil)
		return
	}

	input, err := validateCreateBookingRequest(requestBody)
	if err != nil {
		h.writeDomainError(writer, err)
		return
	}

	booking, err := h.service.CreateBooking(request.Context(), actorUser.UserID, planningID, input)
	if err != nil {
		h.writeDomainError(writer, err)
		return
	}

	response.WriteJSON(writer, http.StatusCreated, map[string]BookingResponse{"data": NewBookingResponse(booking)})
}

func (h *Handler) listBookings(writer http.ResponseWriter, request *http.Request) {
	actorUser, ok := middleware.AuthUserFromContext(request.Context())
	if !ok {
		response.WriteError(writer, http.StatusUnauthorized, "unauthorized", "Authenticated user not found in request context", nil)
		return
	}

	planningID, err := parseUUIDParam(request, "planningId")
	if err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_path_param", err.Error(), nil)
		return
	}

	filters, err := parseBookingFilters(request)
	if err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_query_params", err.Error(), nil)
		return
	}

	result, err := h.service.ListBookings(request.Context(), actorUser.UserID, planningID, filters)
	if err != nil {
		h.writeDomainError(writer, err)
		return
	}

	bookingResponses := make([]BookingResponse, 0, len(result.Items))
	for _, booking := range result.Items {
		bookingResponses = append(bookingResponses, NewBookingResponse(booking))
	}

	response.WriteJSON(writer, http.StatusOK, response.ListEnvelope[BookingResponse]{
		Data: bookingResponses,
		Pagination: response.Pagination{
			Page:       result.Page,
			PageSize:   result.PageSize,
			Total:      result.Total,
			TotalPages: result.TotalPages,
		},
	})
}

func (h *Handler) getBooking(writer http.ResponseWriter, request *http.Request) {
	actorUser, ok := middleware.AuthUserFromContext(request.Context())
	if !ok {
		response.WriteError(writer, http.StatusUnauthorized, "unauthorized", "Authenticated user not found in request context", nil)
		return
	}

	planningID, err := parseUUIDParam(request, "planningId")
	if err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_path_param", err.Error(), nil)
		return
	}

	bookingID, err := parseUUIDParam(request, "bookingId")
	if err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_path_param", err.Error(), nil)
		return
	}

	booking, err := h.service.GetBooking(request.Context(), actorUser.UserID, planningID, bookingID)
	if err != nil {
		h.writeDomainError(writer, err)
		return
	}

	response.WriteJSON(writer, http.StatusOK, map[string]BookingResponse{"data": NewBookingResponse(booking)})
}

func (h *Handler) updateBooking(writer http.ResponseWriter, request *http.Request) {
	actorUser, ok := middleware.AuthUserFromContext(request.Context())
	if !ok {
		response.WriteError(writer, http.StatusUnauthorized, "unauthorized", "Authenticated user not found in request context", nil)
		return
	}

	planningID, err := parseUUIDParam(request, "planningId")
	if err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_path_param", err.Error(), nil)
		return
	}

	bookingID, err := parseUUIDParam(request, "bookingId")
	if err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_path_param", err.Error(), nil)
		return
	}

	var requestBody UpdateBookingRequest
	if err := decodeJSON(request, &requestBody); err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_json", "Invalid request body", nil)
		return
	}

	input, err := validateUpdateBookingRequest(requestBody)
	if err != nil {
		h.writeDomainError(writer, err)
		return
	}

	booking, err := h.service.UpdateBooking(request.Context(), actorUser.UserID, planningID, bookingID, input)
	if err != nil {
		h.writeDomainError(writer, err)
		return
	}

	response.WriteJSON(writer, http.StatusOK, map[string]BookingResponse{"data": NewBookingResponse(booking)})
}

func (h *Handler) confirmBooking(writer http.ResponseWriter, request *http.Request) {
	actorUser, ok := middleware.AuthUserFromContext(request.Context())
	if !ok {
		response.WriteError(writer, http.StatusUnauthorized, "unauthorized", "Authenticated user not found in request context", nil)
		return
	}

	planningID, err := parseUUIDParam(request, "planningId")
	if err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_path_param", err.Error(), nil)
		return
	}

	bookingID, err := parseUUIDParam(request, "bookingId")
	if err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_path_param", err.Error(), nil)
		return
	}

	booking, err := h.service.ConfirmBooking(request.Context(), actorUser.UserID, planningID, bookingID)
	if err != nil {
		h.writeDomainError(writer, err)
		return
	}

	response.WriteJSON(writer, http.StatusOK, map[string]BookingResponse{"data": NewBookingResponse(booking)})
}

func (h *Handler) deleteBooking(writer http.ResponseWriter, request *http.Request) {
	actorUser, ok := middleware.AuthUserFromContext(request.Context())
	if !ok {
		response.WriteError(writer, http.StatusUnauthorized, "unauthorized", "Authenticated user not found in request context", nil)
		return
	}

	planningID, err := parseUUIDParam(request, "planningId")
	if err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_path_param", err.Error(), nil)
		return
	}

	bookingID, err := parseUUIDParam(request, "bookingId")
	if err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_path_param", err.Error(), nil)
		return
	}

	if err := h.service.DeleteBooking(request.Context(), actorUser.UserID, planningID, bookingID); err != nil {
		h.writeDomainError(writer, err)
		return
	}

	response.WriteJSON(writer, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *Handler) writeDomainError(writer http.ResponseWriter, err error) {
	var validationErr ValidationError
	if errors.As(err, &validationErr) {
		response.WriteError(writer, http.StatusBadRequest, "validation_error", "Invalid request body", validationErr.Details)
		return
	}

	switch {
	case errors.Is(err, ErrBookingNotFound):
		response.WriteError(writer, http.StatusNotFound, "booking_not_found", "Booking not found", nil)
	case errors.Is(err, ErrForbidden):
		response.WriteError(writer, http.StatusForbidden, "forbidden", "You do not have permission to perform this action", nil)
	case errors.Is(err, ErrConflict):
		response.WriteError(writer, http.StatusConflict, "conflict", "Resource already exists", nil)
	default:
		response.WriteError(writer, http.StatusInternalServerError, "internal_error", "Unexpected server error", nil)
	}
}

func parseBookingFilters(request *http.Request) (BookingFilters, error) {
	page := normalizePage(parseIntQueryParam(request, "page", 1), parseIntQueryParam(request, "page_size", defaultPageSize))
	filters := BookingFilters{Page: page}

	status := request.URL.Query().Get("status")
	if status != "" {
		if !isValidBookingStatus(status) {
			return BookingFilters{}, errors.New("status must be proposal or confirmed")
		}
		filters.Status = &status
	}

	bookingType := request.URL.Query().Get("type")
	if bookingType != "" {
		if !isValidBookingType(bookingType) {
			return BookingFilters{}, errors.New("type must be hotel, restaurant, attraction, car or activity")
		}
		filters.Type = &bookingType
	}

	return filters, nil
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
