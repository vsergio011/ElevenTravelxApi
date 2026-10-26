package supabaseadmin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var (
	ErrEmailAlreadyRegistered = errors.New("email already registered")
	ErrUserRejected           = errors.New("user rejected by auth provider")
)

const maxResponseBytes = 1 << 20

type Client struct {
	baseURL        string
	serviceRoleKey string
	httpClient     *http.Client
}

type CreateUserInput struct {
	Email    string
	Password string
	Username string
}

type RejectedError struct {
	Message string
}

func (e *RejectedError) Error() string {
	return e.Message
}

func (e *RejectedError) Is(target error) bool {
	return target == ErrUserRejected
}

type providerError struct {
	ErrorCode string `json:"error_code"`
	Message   string `json:"msg"`
}

func NewClient(baseURL, serviceRoleKey string) *Client {
	return &Client{
		baseURL:        strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		serviceRoleKey: serviceRoleKey,
		httpClient:     &http.Client{Timeout: 15 * time.Second},
	}
}

// CreateUser creates an auto-confirmed Supabase Auth user and returns its ID.
func (c *Client) CreateUser(ctx context.Context, input CreateUserInput) (string, error) {
	payload := map[string]any{
		"email":         input.Email,
		"password":      input.Password,
		"email_confirm": true,
		"user_metadata": map[string]string{"username": input.Username},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal create user payload: %w", err)
	}

	request, err := c.newRequest(ctx, http.MethodPost, "/auth/v1/admin/users", bytes.NewReader(body))
	if err != nil {
		return "", err
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("create supabase user: %w", err)
	}
	defer response.Body.Close()

	data, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes))
	if err != nil {
		return "", fmt.Errorf("read supabase create user response: %w", err)
	}

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return "", mapProviderError(response.StatusCode, data)
	}

	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &created); err != nil {
		return "", fmt.Errorf("decode supabase create user response: %w", err)
	}
	if created.ID == "" {
		return "", errors.New("supabase create user response has no id")
	}

	return created.ID, nil
}

// DeleteUser removes an Auth user. It is used to roll back a registration when the profile cannot be created.
func (c *Client) DeleteUser(ctx context.Context, userID string) error {
	request, err := c.newRequest(ctx, http.MethodDelete, "/auth/v1/admin/users/"+userID, nil)
	if err != nil {
		return err
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("delete supabase user: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		data, _ := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes))
		return mapProviderError(response.StatusCode, data)
	}

	return nil
}

func (c *Client) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("build supabase request: %w", err)
	}

	request.Header.Set("apikey", c.serviceRoleKey)
	request.Header.Set("Authorization", "Bearer "+c.serviceRoleKey)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	return request, nil
}

func mapProviderError(status int, data []byte) error {
	var failure providerError
	_ = json.Unmarshal(data, &failure)

	switch failure.ErrorCode {
	case "email_exists", "user_already_exists":
		return ErrEmailAlreadyRegistered
	case "weak_password", "validation_failed", "email_address_invalid":
		return &RejectedError{Message: failure.Message}
	}

	if status >= http.StatusBadRequest && status < http.StatusInternalServerError {
		return fmt.Errorf("supabase admin request rejected with status %d: %s", status, failure.Message)
	}

	return fmt.Errorf("supabase admin request failed with status %d", status)
}
