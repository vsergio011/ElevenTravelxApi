package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCORSAllowsConfiguredOriginAndHandlesPreflight(t *testing.T) {
	t.Parallel()

	handler := CORS([]string{"http://localhost:8081"})(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
	}))

	request := httptest.NewRequest(http.MethodOptions, "/v1/plannings", nil)
	request.Header.Set("Origin", "http://localhost:8081")
	request.Header.Set("Access-Control-Request-Headers", "Authorization, Content-Type")
	responseRecorder := httptest.NewRecorder()

	handler.ServeHTTP(responseRecorder, request)

	require.Equal(t, http.StatusNoContent, responseRecorder.Code)
	require.Equal(t, "http://localhost:8081", responseRecorder.Header().Get("Access-Control-Allow-Origin"))
	require.Contains(t, responseRecorder.Header().Get("Access-Control-Allow-Headers"), "Authorization")
	require.Contains(t, responseRecorder.Header().Get("Access-Control-Allow-Methods"), "OPTIONS")
}
