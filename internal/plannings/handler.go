package plannings

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/eleventravel/eleventravel-api/internal/http/middleware"
	"github.com/eleventravel/eleventravel-api/internal/http/response"
)

type PlanningService interface {
	CreatePlanning(ctx context.Context, actorUserID uuid.UUID, input CreatePlanningInput) (Planning, error)
	GetPlanning(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID) (Planning, error)
	GetSummary(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID) (PlanningSummary, error)
	GetDashboardActivity(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID) (PlanningDashboardActivity, error)
	ListPlannings(ctx context.Context, actorUserID uuid.UUID, filters PlanningFilters) (PageResult[Planning], error)
	UpdatePlanning(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, input UpdatePlanningInput) (Planning, error)
	ArchivePlanning(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID) (Planning, error)
	UnarchivePlanning(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID) (Planning, error)
	ListMembers(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID) ([]PlanningMember, error)
	AddMember(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, input AddPlanningMemberInput) (PlanningMember, error)
	UpdateMemberRole(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, targetUserID uuid.UUID, input UpdatePlanningMemberRoleInput) (PlanningMember, error)
	RemoveMember(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, targetUserID uuid.UUID) error
	ListActivity(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, page Page) (PageResult[PlanningActivity], error)
	GetPlanningItinerary(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID) (PlanningItinerary, error)
	CreatePlanningRouteDay(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, input CreatePlanningRouteDayInput) (PlanningItineraryDay, error)
	DeletePlanningRouteDay(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, dayID uuid.UUID) error
	GetPlanningRouteByID(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, routeID uuid.UUID) (PlanningItineraryStop, error)
	CreatePlanningRouteStop(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, input CreatePlanningRouteStopInput) (PlanningItineraryStop, error)
	UpdatePlanningRouteStop(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, routeID uuid.UUID, input UpdatePlanningRouteStopInput) (PlanningItineraryStop, error)
	DeletePlanningRouteStop(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, routeID uuid.UUID) error
	ReorderPlanningRouteStops(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, input ReorderPlanningRouteStopsInput) error
}

type Handler struct {
	service PlanningService
}

func NewHandler(service PlanningService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) createPlanning(writer http.ResponseWriter, request *http.Request) {
	actorUser, ok := middleware.AuthUserFromContext(request.Context())
	if !ok {
		response.WriteError(writer, http.StatusUnauthorized, "unauthorized", "Authenticated user not found in request context", nil)
		return
	}

	var requestBody CreatePlanningRequest
	if err := decodeJSON(request, &requestBody); err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_json", "Invalid request body", nil)
		return
	}

	input, err := validateCreatePlanningRequest(requestBody)
	if err != nil {
		h.writeDomainError(writer, err)
		return
	}

	planning, err := h.service.CreatePlanning(request.Context(), actorUser.UserID, input)
	if err != nil {
		h.writeDomainError(writer, err)
		return
	}

	response.WriteJSON(writer, http.StatusCreated, map[string]PlanningResponse{"data": NewPlanningResponse(planning)})
}

func (h *Handler) getPlanning(writer http.ResponseWriter, request *http.Request) {
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

	planning, err := h.service.GetPlanning(request.Context(), actorUser.UserID, planningID)
	if err != nil {
		h.writeDomainError(writer, err)
		return
	}

	response.WriteJSON(writer, http.StatusOK, map[string]PlanningResponse{"data": NewPlanningResponse(planning)})
}

func (h *Handler) getSummary(writer http.ResponseWriter, request *http.Request) {
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

	summary, err := h.service.GetSummary(request.Context(), actorUser.UserID, planningID)
	if err != nil {
		h.writeDomainError(writer, err)
		return
	}

	response.WriteJSON(writer, http.StatusOK, map[string]PlanningSummaryResponse{"data": NewPlanningSummaryResponse(summary)})
}

func (h *Handler) listPlannings(writer http.ResponseWriter, request *http.Request) {
	actorUser, ok := middleware.AuthUserFromContext(request.Context())
	if !ok {
		response.WriteError(writer, http.StatusUnauthorized, "unauthorized", "Authenticated user not found in request context", nil)
		return
	}

	filters, err := parsePlanningFilters(request)
	if err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_query_params", err.Error(), nil)
		return
	}

	h.writePlanningList(writer, request.Context(), actorUser.UserID, filters)
}

func (h *Handler) listGroupPlannings(writer http.ResponseWriter, request *http.Request) {
	actorUser, ok := middleware.AuthUserFromContext(request.Context())
	if !ok {
		response.WriteError(writer, http.StatusUnauthorized, "unauthorized", "Authenticated user not found in request context", nil)
		return
	}

	groupID, err := parseUUIDParam(request, "groupId")
	if err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_path_param", err.Error(), nil)
		return
	}

	filters, err := parsePlanningFilters(request)
	if err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_query_params", err.Error(), nil)
		return
	}
	filters.GroupID = &groupID

	h.writePlanningList(writer, request.Context(), actorUser.UserID, filters)
}

func (h *Handler) writePlanningList(writer http.ResponseWriter, ctx context.Context, actorUserID uuid.UUID, filters PlanningFilters) {
	startedAt := time.Now()
	result, err := h.service.ListPlannings(ctx, actorUserID, filters)
	if err != nil {
		log.Printf("planning.list actor_user_id=%s page=%d page_size=%d error=%v", actorUserID, filters.Page.Number, filters.Page.Size, err)
		h.writeDomainError(writer, err)
		return
	}

	planningResponses := make([]PlanningResponse, 0, len(result.Items))
	for _, planning := range result.Items {
		planningResponses = append(planningResponses, NewPlanningResponse(planning))
	}

	response.WriteJSON(writer, http.StatusOK, response.ListEnvelope[PlanningResponse]{
		Data: planningResponses,
		Pagination: response.Pagination{
			Page:       result.Page,
			PageSize:   result.PageSize,
			Total:      result.Total,
			TotalPages: result.TotalPages,
		},
	})

	log.Printf("planning.list actor_user_id=%s page=%d page_size=%d total=%d items=%d duration=%s", actorUserID, result.Page, result.PageSize, result.Total, len(result.Items), time.Since(startedAt))
}

func (h *Handler) updatePlanning(writer http.ResponseWriter, request *http.Request) {
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

	var requestBody UpdatePlanningRequest
	if err := decodeJSON(request, &requestBody); err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_json", "Invalid request body", nil)
		return
	}

	input, err := validateUpdatePlanningRequest(requestBody)
	if err != nil {
		h.writeDomainError(writer, err)
		return
	}

	planning, err := h.service.UpdatePlanning(request.Context(), actorUser.UserID, planningID, input)
	if err != nil {
		h.writeDomainError(writer, err)
		return
	}

	response.WriteJSON(writer, http.StatusOK, map[string]PlanningResponse{"data": NewPlanningResponse(planning)})
}

func (h *Handler) archivePlanning(writer http.ResponseWriter, request *http.Request) {
	h.setPlanningArchived(writer, request, true)
}

func (h *Handler) unarchivePlanning(writer http.ResponseWriter, request *http.Request) {
	h.setPlanningArchived(writer, request, false)
}

func (h *Handler) setPlanningArchived(writer http.ResponseWriter, request *http.Request, archived bool) {
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

	var planning Planning
	if archived {
		planning, err = h.service.ArchivePlanning(request.Context(), actorUser.UserID, planningID)
	} else {
		planning, err = h.service.UnarchivePlanning(request.Context(), actorUser.UserID, planningID)
	}
	if err != nil {
		h.writeDomainError(writer, err)
		return
	}

	response.WriteJSON(writer, http.StatusOK, map[string]PlanningResponse{"data": NewPlanningResponse(planning)})
}

func (h *Handler) listMembers(writer http.ResponseWriter, request *http.Request) {
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

	members, err := h.service.ListMembers(request.Context(), actorUser.UserID, planningID)
	if err != nil {
		h.writeDomainError(writer, err)
		return
	}

	memberResponses := make([]PlanningMemberResponse, 0, len(members))
	for _, member := range members {
		memberResponses = append(memberResponses, NewPlanningMemberResponse(member))
	}

	response.WriteJSON(writer, http.StatusOK, map[string][]PlanningMemberResponse{"data": memberResponses})
}

func (h *Handler) addMember(writer http.ResponseWriter, request *http.Request) {
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

	var requestBody AddPlanningMemberRequest
	if err := decodeJSON(request, &requestBody); err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_json", "Invalid request body", nil)
		return
	}

	input, err := validateAddPlanningMemberRequest(requestBody)
	if err != nil {
		h.writeDomainError(writer, err)
		return
	}

	member, err := h.service.AddMember(request.Context(), actorUser.UserID, planningID, input)
	if err != nil {
		h.writeDomainError(writer, err)
		return
	}

	response.WriteJSON(writer, http.StatusCreated, map[string]PlanningMemberResponse{"data": NewPlanningMemberResponse(member)})
}

func (h *Handler) updateMemberRole(writer http.ResponseWriter, request *http.Request) {
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

	targetUserID, err := parseUUIDParam(request, "userId")
	if err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_path_param", err.Error(), nil)
		return
	}

	var requestBody UpdatePlanningMemberRoleRequest
	if err := decodeJSON(request, &requestBody); err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_json", "Invalid request body", nil)
		return
	}

	input, err := validateUpdatePlanningMemberRoleRequest(requestBody)
	if err != nil {
		h.writeDomainError(writer, err)
		return
	}

	member, err := h.service.UpdateMemberRole(request.Context(), actorUser.UserID, planningID, targetUserID, input)
	if err != nil {
		h.writeDomainError(writer, err)
		return
	}

	response.WriteJSON(writer, http.StatusOK, map[string]PlanningMemberResponse{"data": NewPlanningMemberResponse(member)})
}

func (h *Handler) removeMember(writer http.ResponseWriter, request *http.Request) {
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

	targetUserID, err := parseUUIDParam(request, "userId")
	if err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_path_param", err.Error(), nil)
		return
	}

	if err := h.service.RemoveMember(request.Context(), actorUser.UserID, planningID, targetUserID); err != nil {
		h.writeDomainError(writer, err)
		return
	}

	response.WriteJSON(writer, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *Handler) listActivity(writer http.ResponseWriter, request *http.Request) {
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

	activity, err := h.service.GetDashboardActivity(request.Context(), actorUser.UserID, planningID)
	if err != nil {
		h.writeDomainError(writer, err)
		return
	}

	response.WriteJSON(writer, http.StatusOK, map[string]PlanningDashboardActivityResponse{"data": NewPlanningDashboardActivityResponse(activity)})
}

func (h *Handler) getPlanningRoutes(writer http.ResponseWriter, request *http.Request) {
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

	itinerary, err := h.service.GetPlanningItinerary(request.Context(), actorUser.UserID, planningID)
	if err != nil {
		h.writeDomainError(writer, err)
		return
	}

	response.WriteJSON(writer, http.StatusOK, map[string]PlanningItineraryResponse{"data": NewPlanningItineraryResponse(itinerary)})
}

func (h *Handler) createPlanningRouteDay(writer http.ResponseWriter, request *http.Request) {
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

	var requestBody CreatePlanningRouteDayRequest
	if err := decodeJSON(request, &requestBody); err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_json", "Invalid request body", nil)
		return
	}
	if requestBody.Label == "" || requestBody.DateLabel == "" {
		h.writeDomainError(writer, ValidationError{})
		return
	}

	day, err := h.service.CreatePlanningRouteDay(request.Context(), actorUser.UserID, planningID, CreatePlanningRouteDayInput{
		Label:     requestBody.Label,
		DateLabel: requestBody.DateLabel,
	})
	if err != nil {
		h.writeDomainError(writer, err)
		return
	}

	response.WriteJSON(writer, http.StatusCreated, map[string]PlanningItineraryDayResponse{"data": toPlanningItineraryDayResponse(day)})
}

func (h *Handler) deletePlanningRouteDay(writer http.ResponseWriter, request *http.Request) {
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
	dayID, err := parseUUIDParam(request, "dayId")
	if err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_path_param", err.Error(), nil)
		return
	}

	if err := h.service.DeletePlanningRouteDay(request.Context(), actorUser.UserID, planningID, dayID); err != nil {
		h.writeDomainError(writer, err)
		return
	}

	response.WriteJSON(writer, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *Handler) getPlanningRoute(writer http.ResponseWriter, request *http.Request) {
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

	routeID, err := parseUUIDParam(request, "routeId")
	if err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_path_param", err.Error(), nil)
		return
	}

	stop, err := h.service.GetPlanningRouteByID(request.Context(), actorUser.UserID, planningID, routeID)
	if err != nil {
		h.writeDomainError(writer, err)
		return
	}

	response.WriteJSON(writer, http.StatusOK, map[string]PlanningItineraryStopResponse{"data": toPlanningItineraryStopResponse(stop)})
}

func (h *Handler) createPlanningRoute(writer http.ResponseWriter, request *http.Request) {
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

	var requestBody CreatePlanningRouteStopRequest
	if err := decodeJSON(request, &requestBody); err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_json", "Invalid request body", nil)
		return
	}

	input, err := validateCreatePlanningRouteStopRequest(requestBody)
	if err != nil {
		h.writeDomainError(writer, err)
		return
	}

	stop, err := h.service.CreatePlanningRouteStop(request.Context(), actorUser.UserID, planningID, input)
	if err != nil {
		h.writeDomainError(writer, err)
		return
	}

	response.WriteJSON(writer, http.StatusCreated, map[string]PlanningItineraryStopResponse{"data": toPlanningItineraryStopResponse(stop)})
}

func (h *Handler) updatePlanningRoute(writer http.ResponseWriter, request *http.Request) {
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

	routeID, err := parseUUIDParam(request, "routeId")
	if err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_path_param", err.Error(), nil)
		return
	}

	var requestBody UpdatePlanningRouteStopRequest
	if err := decodeJSON(request, &requestBody); err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_json", "Invalid request body", nil)
		return
	}

	input, err := validateUpdatePlanningRouteStopRequest(requestBody)
	if err != nil {
		h.writeDomainError(writer, err)
		return
	}

	stop, err := h.service.UpdatePlanningRouteStop(request.Context(), actorUser.UserID, planningID, routeID, input)
	if err != nil {
		h.writeDomainError(writer, err)
		return
	}

	response.WriteJSON(writer, http.StatusOK, map[string]PlanningItineraryStopResponse{"data": toPlanningItineraryStopResponse(stop)})
}

func (h *Handler) deletePlanningRoute(writer http.ResponseWriter, request *http.Request) {
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

	routeID, err := parseUUIDParam(request, "routeId")
	if err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_path_param", err.Error(), nil)
		return
	}

	if err := h.service.DeletePlanningRouteStop(request.Context(), actorUser.UserID, planningID, routeID); err != nil {
		h.writeDomainError(writer, err)
		return
	}

	response.WriteJSON(writer, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *Handler) reorderPlanningRoutes(writer http.ResponseWriter, request *http.Request) {
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

	var requestBody ReorderPlanningRouteStopsRequest
	if err := decodeJSON(request, &requestBody); err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_json", "Invalid request body", nil)
		return
	}

	input, err := validateReorderPlanningRouteStopsRequest(requestBody)
	if err != nil {
		h.writeDomainError(writer, err)
		return
	}

	if err := h.service.ReorderPlanningRouteStops(request.Context(), actorUser.UserID, planningID, input); err != nil {
		h.writeDomainError(writer, err)
		return
	}

	response.WriteJSON(writer, http.StatusOK, map[string]string{"status": "reordered"})
}

func toPlanningItineraryStopResponse(stop PlanningItineraryStop) PlanningItineraryStopResponse {
	return PlanningItineraryStopResponse{
		ID:              stop.ID,
		Title:           stop.Title,
		Description:     stop.Description,
		Address:         stop.Address,
		TimeLabel:       stop.TimeLabel,
		DurationMinutes: stop.DurationMinutes,
		Notes:           stop.Notes,
		CostEstimate:    stop.CostEstimate,
		Currency:        stop.Currency,
		ExternalURL:     stop.ExternalURL,
		ImageURL:        stop.ImageURL,
		Status:          stop.Status,
		Category:        stop.Category,
		IsOptional:      stop.IsOptional,
		Coordinates: PlanningItineraryCoordinatesResponse{
			Latitude:  stop.Coordinates.Latitude,
			Longitude: stop.Coordinates.Longitude,
		},
		Bookings: toPlanningRouteBookingResponses(stop.Bookings),
	}
}

func (h *Handler) writeDomainError(writer http.ResponseWriter, err error) {
	var validationErr ValidationError
	if errors.As(err, &validationErr) {
		response.WriteError(writer, http.StatusBadRequest, "validation_error", "Invalid request body", validationErr.Details)
		return
	}

	switch {
	case errors.Is(err, ErrPlanningNotFound):
		response.WriteError(writer, http.StatusNotFound, "planning_not_found", "Planning not found", nil)
	case errors.Is(err, ErrMemberNotFound):
		response.WriteError(writer, http.StatusNotFound, "planning_member_not_found", "Planning member not found", nil)
	case errors.Is(err, ErrUserNotFound):
		response.WriteError(writer, http.StatusNotFound, "user_not_found", "User not found", nil)
	case errors.Is(err, ErrForbidden):
		response.WriteError(writer, http.StatusForbidden, "forbidden", "You do not have permission to perform this action", nil)
	case errors.Is(err, ErrConflict):
		response.WriteError(writer, http.StatusConflict, "conflict", "Resource already exists", nil)
	default:
		response.WriteError(writer, http.StatusInternalServerError, "internal_error", "Unexpected server error", nil)
	}
}

func parsePlanningFilters(request *http.Request) (PlanningFilters, error) {
	page := normalizePage(parseIntQueryParam(request, "page", 1), parseIntQueryParam(request, "page_size", defaultPageSize))
	filters := PlanningFilters{Page: page}

	groupIDRaw := request.URL.Query().Get("group_id")
	if groupIDRaw != "" {
		groupID, err := uuid.Parse(groupIDRaw)
		if err != nil {
			return PlanningFilters{}, errors.New("group_id must be a valid UUID")
		}
		filters.GroupID = &groupID
	}

	status := request.URL.Query().Get("status")
	if status != "" {
		if !isValidPlanningStatus(status) {
			return PlanningFilters{}, errors.New("status must be draft, active or completed")
		}
		filters.Status = &status
	}

	archived := request.URL.Query().Get("archived")
	if archived != "" {
		parsedArchived, err := strconv.ParseBool(archived)
		if err != nil {
			return PlanningFilters{}, errors.New("archived must be true or false")
		}
		filters.Archived = &parsedArchived
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
