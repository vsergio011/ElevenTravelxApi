package checklists

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/eleventravel/eleventravel-api/internal/http/middleware"
	"github.com/eleventravel/eleventravel-api/internal/http/response"
)

type ChecklistTaskService interface {
	CreateTask(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, input CreateChecklistTaskInput) (ChecklistTask, error)
	GetTask(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, taskID uuid.UUID) (ChecklistTask, error)
	ListTasks(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, filters ChecklistTaskFilters) ([]ChecklistTask, error)
	UpdateTask(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, taskID uuid.UUID, input UpdateChecklistTaskInput) (ChecklistTask, error)
	DeleteTask(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, taskID uuid.UUID) error
	ToggleCompletion(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, taskID uuid.UUID) (ChecklistTask, error)
}

type Handler struct {
	service ChecklistTaskService
}

func NewHandler(service ChecklistTaskService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) createTask(writer http.ResponseWriter, request *http.Request) {
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

	var requestBody CreateChecklistTaskRequest
	if err := decodeJSON(request, &requestBody); err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_json", "Invalid request body", nil)
		return
	}

	input, err := validateCreateChecklistTaskRequest(requestBody)
	if err != nil {
		h.writeDomainError(writer, err)
		return
	}

	task, err := h.service.CreateTask(request.Context(), actorUser.UserID, planningID, input)
	if err != nil {
		h.writeDomainError(writer, err)
		return
	}

	response.WriteJSON(writer, http.StatusCreated, map[string]ChecklistTaskResponse{"data": NewChecklistTaskResponse(task)})
}

func (h *Handler) getTask(writer http.ResponseWriter, request *http.Request) {
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

	taskID, err := parseUUIDParam(request, "taskId")
	if err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_path_param", err.Error(), nil)
		return
	}

	task, err := h.service.GetTask(request.Context(), actorUser.UserID, planningID, taskID)
	if err != nil {
		h.writeDomainError(writer, err)
		return
	}

	response.WriteJSON(writer, http.StatusOK, map[string]ChecklistTaskResponse{"data": NewChecklistTaskResponse(task)})
}

func (h *Handler) listTasks(writer http.ResponseWriter, request *http.Request) {
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

	filters := ChecklistTaskFilters{}
	if taskType := request.URL.Query().Get("type"); taskType != "" {
		filters.Type = &taskType
	}

	tasks, err := h.service.ListTasks(request.Context(), actorUser.UserID, planningID, filters)
	if err != nil {
		h.writeDomainError(writer, err)
		return
	}

	taskResponses := make([]ChecklistTaskResponse, 0, len(tasks))
	for _, task := range tasks {
		taskResponses = append(taskResponses, NewChecklistTaskResponse(task))
	}

	response.WriteJSON(writer, http.StatusOK, map[string][]ChecklistTaskResponse{"data": taskResponses})
}

func (h *Handler) updateTask(writer http.ResponseWriter, request *http.Request) {
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

	taskID, err := parseUUIDParam(request, "taskId")
	if err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_path_param", err.Error(), nil)
		return
	}

	var requestBody UpdateChecklistTaskRequest
	if err := decodeJSON(request, &requestBody); err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_json", "Invalid request body", nil)
		return
	}

	input, err := validateUpdateChecklistTaskRequest(requestBody)
	if err != nil {
		h.writeDomainError(writer, err)
		return
	}

	task, err := h.service.UpdateTask(request.Context(), actorUser.UserID, planningID, taskID, input)
	if err != nil {
		h.writeDomainError(writer, err)
		return
	}

	response.WriteJSON(writer, http.StatusOK, map[string]ChecklistTaskResponse{"data": NewChecklistTaskResponse(task)})
}

func (h *Handler) deleteTask(writer http.ResponseWriter, request *http.Request) {
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

	taskID, err := parseUUIDParam(request, "taskId")
	if err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_path_param", err.Error(), nil)
		return
	}

	if err := h.service.DeleteTask(request.Context(), actorUser.UserID, planningID, taskID); err != nil {
		h.writeDomainError(writer, err)
		return
	}

	writer.WriteHeader(http.StatusNoContent)
}

func (h *Handler) toggleTaskCompletion(writer http.ResponseWriter, request *http.Request) {
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

	taskID, err := parseUUIDParam(request, "taskId")
	if err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_path_param", err.Error(), nil)
		return
	}

	task, err := h.service.ToggleCompletion(request.Context(), actorUser.UserID, planningID, taskID)
	if err != nil {
		h.writeDomainError(writer, err)
		return
	}

	response.WriteJSON(writer, http.StatusOK, map[string]ChecklistTaskResponse{"data": NewChecklistTaskResponse(task)})
}

func (h *Handler) writeDomainError(writer http.ResponseWriter, err error) {
	var validationErr ValidationError
	if errors.As(err, &validationErr) {
		response.WriteError(writer, http.StatusBadRequest, "validation_error", "Invalid request body", validationErr.Details)
		return
	}

	switch {
	case errors.Is(err, ErrTaskNotFound):
		response.WriteError(writer, http.StatusNotFound, "checklist_task_not_found", "Checklist task not found", nil)
	case errors.Is(err, ErrForbidden):
		response.WriteError(writer, http.StatusForbidden, "forbidden", "You do not have permission to perform this action", nil)
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
