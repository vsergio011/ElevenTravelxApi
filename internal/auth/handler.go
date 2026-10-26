package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/eleventravel/eleventravel-api/internal/http/response"
)

const maxRegisterBodyBytes = 16 << 10

type RegistrationService interface {
	Settings() Settings
	Register(ctx context.Context, input RegisterInput) (RegisterResult, error)
}

type Handler struct {
	service RegistrationService
}

type registrationSettingsResponse struct {
	Enabled            bool `json:"enabled"`
	AccessCodeRequired bool `json:"access_code_required"`
}

type registerRequest struct {
	Email      string `json:"email"`
	Password   string `json:"password"`
	Username   string `json:"username"`
	AccessCode string `json:"access_code"`
}

type registerResponse struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

func NewHandler(service RegistrationService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(router chi.Router) {
	router.Get("/auth/registration-settings", h.getRegistrationSettings)
	router.Post("/auth/register", h.register)
}

func (h *Handler) getRegistrationSettings(writer http.ResponseWriter, request *http.Request) {
	settings := h.service.Settings()

	response.WriteJSON(writer, http.StatusOK, map[string]registrationSettingsResponse{
		"data": {
			Enabled:            settings.Enabled,
			AccessCodeRequired: settings.AccessCodeRequired,
		},
	})
}

func (h *Handler) register(writer http.ResponseWriter, request *http.Request) {
	request.Body = http.MaxBytesReader(writer, request.Body, maxRegisterBodyBytes)

	var body registerRequest
	if err := decodeJSON(request, &body); err != nil {
		response.WriteError(writer, http.StatusBadRequest, "invalid_body", "Request body must be valid JSON with the expected fields", nil)
		return
	}

	result, err := h.service.Register(request.Context(), RegisterInput{
		Email:      body.Email,
		Password:   body.Password,
		Username:   body.Username,
		AccessCode: body.AccessCode,
	})
	if err != nil {
		h.writeDomainError(writer, err)
		return
	}

	response.WriteJSON(writer, http.StatusCreated, map[string]registerResponse{
		"data": {
			UserID:   result.UserID,
			Username: result.Username,
			Email:    result.Email,
		},
	})
}

func (h *Handler) writeDomainError(writer http.ResponseWriter, err error) {
	var validationErr ValidationError
	if errors.As(err, &validationErr) {
		response.WriteError(writer, http.StatusBadRequest, "validation_error", "Invalid request body", validationErr.Details)
		return
	}

	switch {
	case errors.Is(err, ErrRegistrationDisabled):
		response.WriteError(writer, http.StatusForbidden, "registration_disabled", "Registration is currently disabled", nil)
	case errors.Is(err, ErrInvalidAccessCode):
		response.WriteError(writer, http.StatusForbidden, "invalid_access_code", "The access code is not valid", nil)
	case errors.Is(err, ErrUsernameTaken):
		response.WriteError(writer, http.StatusConflict, "username_taken", "The username is already in use", nil)
	case errors.Is(err, ErrEmailTaken):
		response.WriteError(writer, http.StatusConflict, "email_taken", "The email is already registered", nil)
	case errors.Is(err, ErrAuthProvider):
		response.WriteError(writer, http.StatusBadGateway, "auth_provider_error", "The authentication provider could not complete the request", nil)
	default:
		response.WriteError(writer, http.StatusInternalServerError, "internal_error", "Unexpected server error", nil)
	}
}

func decodeJSON(request *http.Request, destination any) error {
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(destination)
}
