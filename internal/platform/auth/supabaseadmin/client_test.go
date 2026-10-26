package supabaseadmin

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateUserSendsAdminRequestAndReturnsID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		assert.Equal(t, http.MethodPost, request.Method)
		assert.Equal(t, "/auth/v1/admin/users", request.URL.Path)
		assert.Equal(t, "service-key", request.Header.Get("apikey"))
		assert.Equal(t, "Bearer service-key", request.Header.Get("Authorization"))

		body, err := io.ReadAll(request.Body)
		require.NoError(t, err)
		var payload map[string]any
		require.NoError(t, json.Unmarshal(body, &payload))
		assert.Equal(t, "ana@example.com", payload["email"])
		assert.Equal(t, true, payload["email_confirm"])

		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"id":"11111111-1111-1111-1111-111111111111"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL+"/", "service-key")
	id, err := client.CreateUser(context.Background(), CreateUserInput{Email: "ana@example.com", Password: "supersecret", Username: "ana"})

	require.NoError(t, err)
	assert.Equal(t, "11111111-1111-1111-1111-111111111111", id)
}

func TestCreateUserMapsEmailExists(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = writer.Write([]byte(`{"code":422,"error_code":"email_exists","msg":"A user with this email address has already been registered"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "service-key")
	_, err := client.CreateUser(context.Background(), CreateUserInput{Email: "ana@example.com", Password: "supersecret", Username: "ana"})

	assert.ErrorIs(t, err, ErrEmailAlreadyRegistered)
}

func TestCreateUserMapsWeakPasswordAsRejection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = writer.Write([]byte(`{"code":422,"error_code":"weak_password","msg":"Password is too weak"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "service-key")
	_, err := client.CreateUser(context.Background(), CreateUserInput{Email: "ana@example.com", Password: "supersecret", Username: "ana"})

	assert.ErrorIs(t, err, ErrUserRejected)
	var rejected *RejectedError
	require.ErrorAs(t, err, &rejected)
	assert.Equal(t, "Password is too weak", rejected.Message)
}

func TestCreateUserReturnsGenericErrorOnServerFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL, "service-key")
	_, err := client.CreateUser(context.Background(), CreateUserInput{Email: "ana@example.com", Password: "supersecret", Username: "ana"})

	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrEmailAlreadyRegistered))
	assert.False(t, errors.Is(err, ErrUserRejected))
}

func TestDeleteUserCallsAdminEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		assert.Equal(t, http.MethodDelete, request.Method)
		assert.Equal(t, "/auth/v1/admin/users/abc", request.URL.Path)
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "service-key")

	assert.NoError(t, client.DeleteUser(context.Background(), "abc"))
}
