package middleware

import (
	"context"
	"log"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/eleventravel/eleventravel-api/internal/http/response"
	"github.com/eleventravel/eleventravel-api/internal/platform/auth/supabasejwt"
)

const authUserContextKey contextKey = "auth_user"

type AuthUser struct {
	UserID uuid.UUID
	Email  string
	Role   string
}

func Authenticate(tokenValidator supabasejwt.TokenValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			authorizationHeader := request.Header.Get("Authorization")
			if authorizationHeader == "" {
				log.Printf("auth.failed request_id=%s method=%s path=%s reason=%s", GetRequestID(request.Context()), request.Method, request.URL.Path, "missing bearer token")
				response.WriteError(writer, http.StatusUnauthorized, "unauthorized", "Missing bearer token", nil)
				return
			}

			parts := strings.SplitN(authorizationHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
				log.Printf("auth.failed request_id=%s method=%s path=%s reason=%s", GetRequestID(request.Context()), request.Method, request.URL.Path, "invalid authorization header")
				response.WriteError(writer, http.StatusUnauthorized, "unauthorized", "Invalid authorization header", nil)
				return
			}

			claims, err := tokenValidator.ValidateToken(request.Context(), parts[1])
			if err != nil {
				log.Printf("auth.failed request_id=%s method=%s path=%s reason=%v", GetRequestID(request.Context()), request.Method, request.URL.Path, err)
				response.WriteError(writer, http.StatusUnauthorized, "unauthorized", "Invalid or expired token", nil)
				return
			}

			userID, err := uuid.Parse(claims.Subject)
			if err != nil {
				log.Printf("auth.failed request_id=%s method=%s path=%s reason=%s subject=%s", GetRequestID(request.Context()), request.Method, request.URL.Path, "token subject is invalid uuid", claims.Subject)
				response.WriteError(writer, http.StatusUnauthorized, "unauthorized", "Token subject is invalid", nil)
				return
			}

			ctx := context.WithValue(request.Context(), authUserContextKey, AuthUser{
				UserID: userID,
				Email:  claims.Email,
				Role:   claims.Role,
			})

			next.ServeHTTP(writer, request.WithContext(ctx))
		})
	}
}

func AuthUserFromContext(ctx context.Context) (AuthUser, bool) {
	user, ok := ctx.Value(authUserContextKey).(AuthUser)
	return user, ok
}
