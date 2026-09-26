package middleware

import (
	"log"
	"net/http"
	"time"
)

func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		startedAt := time.Now()
		next.ServeHTTP(writer, request)
		log.Printf("method=%s path=%s duration=%s request_id=%s", request.Method, request.URL.Path, time.Since(startedAt), GetRequestID(request.Context()))
	})
}
