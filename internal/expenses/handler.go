package expenses

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

type ExpenseService interface {
	CreateExpense(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, input CreateExpenseInput) (Expense, error)
	GetExpense(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, expenseID uuid.UUID) (Expense, error)
	ListExpenses(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID) ([]Expense, error)
	UpdateExpense(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, expenseID uuid.UUID, input UpdateExpenseInput) (Expense, error)
	DeleteExpense(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, expenseID uuid.UUID) error
	GetBudget(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID) (Budget, error)
}

type Handler struct {
	service ExpenseService
}

func NewHandler(service ExpenseService) *Handler {
	return &Handler{service: service}
}

// context resolves the authenticated user and the planning path param, writing the error response itself.
func (h *Handler) context(writer http.ResponseWriter, request *http.Request) (uuid.UUID, uuid.UUID, bool) {
	actorUser, ok := middleware.AuthUserFromContext(request.Context())
	if !ok {
		response.WriteError(writer, http.StatusUnauthorized, "unauthorized", "Authenticated user not found in request context", nil)
		return uuid.Nil, uuid.Nil, false
	}
	planningID, err := parseUUIDParam(request, "planningId")
	if err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_path_param", err.Error(), nil)
		return uuid.Nil, uuid.Nil, false
	}
	return actorUser.UserID, planningID, true
}

func (h *Handler) expenseContext(writer http.ResponseWriter, request *http.Request) (uuid.UUID, uuid.UUID, uuid.UUID, bool) {
	actorID, planningID, ok := h.context(writer, request)
	if !ok {
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	expenseID, err := parseUUIDParam(request, "expenseId")
	if err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_path_param", err.Error(), nil)
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	return actorID, planningID, expenseID, true
}

func (h *Handler) createExpense(writer http.ResponseWriter, request *http.Request) {
	actorID, planningID, ok := h.context(writer, request)
	if !ok {
		return
	}
	var body CreateExpenseRequest
	if err := decodeJSON(request, &body); err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_json", "Invalid request body", nil)
		return
	}
	input, err := validateCreateRequest(body)
	if err != nil {
		h.writeDomainError(writer, err)
		return
	}
	expense, err := h.service.CreateExpense(request.Context(), actorID, planningID, input)
	if err != nil {
		h.writeDomainError(writer, err)
		return
	}
	response.WriteJSON(writer, http.StatusCreated, map[string]ExpenseResponse{"data": NewExpenseResponse(expense)})
}

func (h *Handler) getExpense(writer http.ResponseWriter, request *http.Request) {
	actorID, planningID, expenseID, ok := h.expenseContext(writer, request)
	if !ok {
		return
	}
	expense, err := h.service.GetExpense(request.Context(), actorID, planningID, expenseID)
	if err != nil {
		h.writeDomainError(writer, err)
		return
	}
	response.WriteJSON(writer, http.StatusOK, map[string]ExpenseResponse{"data": NewExpenseResponse(expense)})
}

func (h *Handler) listExpenses(writer http.ResponseWriter, request *http.Request) {
	actorID, planningID, ok := h.context(writer, request)
	if !ok {
		return
	}
	expenses, err := h.service.ListExpenses(request.Context(), actorID, planningID)
	if err != nil {
		h.writeDomainError(writer, err)
		return
	}
	items := make([]ExpenseResponse, 0, len(expenses))
	for _, expense := range expenses {
		items = append(items, NewExpenseResponse(expense))
	}
	response.WriteJSON(writer, http.StatusOK, map[string][]ExpenseResponse{"data": items})
}

func (h *Handler) updateExpense(writer http.ResponseWriter, request *http.Request) {
	actorID, planningID, expenseID, ok := h.expenseContext(writer, request)
	if !ok {
		return
	}
	var body UpdateExpenseRequest
	if err := decodeJSON(request, &body); err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_json", "Invalid request body", nil)
		return
	}
	input, err := validateUpdateRequest(body)
	if err != nil {
		h.writeDomainError(writer, err)
		return
	}
	expense, err := h.service.UpdateExpense(request.Context(), actorID, planningID, expenseID, input)
	if err != nil {
		h.writeDomainError(writer, err)
		return
	}
	response.WriteJSON(writer, http.StatusOK, map[string]ExpenseResponse{"data": NewExpenseResponse(expense)})
}

func (h *Handler) deleteExpense(writer http.ResponseWriter, request *http.Request) {
	actorID, planningID, expenseID, ok := h.expenseContext(writer, request)
	if !ok {
		return
	}
	if err := h.service.DeleteExpense(request.Context(), actorID, planningID, expenseID); err != nil {
		h.writeDomainError(writer, err)
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (h *Handler) getBudget(writer http.ResponseWriter, request *http.Request) {
	actorID, planningID, ok := h.context(writer, request)
	if !ok {
		return
	}
	budget, err := h.service.GetBudget(request.Context(), actorID, planningID)
	if err != nil {
		h.writeDomainError(writer, err)
		return
	}
	response.WriteJSON(writer, http.StatusOK, map[string]BudgetResponse{"data": NewBudgetResponse(budget)})
}

func (h *Handler) writeDomainError(writer http.ResponseWriter, err error) {
	var validationErr ValidationError
	if errors.As(err, &validationErr) {
		response.WriteError(writer, http.StatusBadRequest, "validation_error", "Invalid request body", validationErr.Details)
		return
	}
	switch {
	case errors.Is(err, ErrExpenseNotFound):
		response.WriteError(writer, http.StatusNotFound, "expense_not_found", "Expense not found", nil)
	case errors.Is(err, ErrForbidden):
		response.WriteError(writer, http.StatusForbidden, "forbidden", "You do not have permission to perform this action", nil)
	default:
		response.WriteError(writer, http.StatusInternalServerError, "internal_error", "Unexpected server error", nil)
	}
}

func parseUUIDParam(request *http.Request, name string) (uuid.UUID, error) {
	parsedValue, err := uuid.Parse(chi.URLParam(request, name))
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
