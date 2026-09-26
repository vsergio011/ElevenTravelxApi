package middleware

import (
	"net/http"
	"strings"
)

func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	allowedOriginSet := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		trimmedOrigin := strings.TrimSpace(origin)
		if trimmedOrigin != "" {
			allowedOriginSet[trimmedOrigin] = struct{}{}
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			origin := strings.TrimSpace(request.Header.Get("Origin"))
			if origin != "" {
				if _, allowed := allowedOriginSet[origin]; allowed {
					headers := writer.Header()
					headers.Set("Access-Control-Allow-Origin", origin)
					headers.Add("Vary", "Origin")
					headers.Set("Access-Control-Allow-Credentials", "true")
					headers.Set("Access-Control-Allow-Methods", "GET,POST,PATCH,PUT,DELETE,OPTIONS")
					requestedHeaders := strings.TrimSpace(request.Header.Get("Access-Control-Request-Headers"))
					if requestedHeaders == "" {
						requestedHeaders = "Authorization,Content-Type,Accept,Origin,X-Requested-With"
					}
					headers.Set("Access-Control-Allow-Headers", requestedHeaders)
					headers.Set("Access-Control-Max-Age", "600")
				}
			}

			if request.Method == http.MethodOptions {
				writer.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(writer, request)
		})
	}
}
