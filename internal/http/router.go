package http

import (
	stdhttp "net/http"

	"github.com/go-chi/chi/v5"

	"github.com/eleventravel/eleventravel-api/internal/auth"
	"github.com/eleventravel/eleventravel-api/internal/bookings"
	"github.com/eleventravel/eleventravel-api/internal/checklists"
	"github.com/eleventravel/eleventravel-api/internal/expenses"
	"github.com/eleventravel/eleventravel-api/internal/flights"
	"github.com/eleventravel/eleventravel-api/internal/http/middleware"
	"github.com/eleventravel/eleventravel-api/internal/http/response"
	"github.com/eleventravel/eleventravel-api/internal/plannings"
	"github.com/eleventravel/eleventravel-api/internal/platform/auth/supabasejwt"
	"github.com/eleventravel/eleventravel-api/internal/users"
)

func NewRouter(tokenValidator supabasejwt.TokenValidator, authHandler *auth.Handler, planningHandler *plannings.Handler, userHandler *users.Handler, bookingHandler *bookings.Handler, flightHandler *flights.Handler, checklistHandler *checklists.Handler, expenseHandler *expenses.Handler, allowedOrigins []string) stdhttp.Handler {
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.Logging)
	router.Use(middleware.CORS(allowedOrigins))

	router.Get("/health", func(writer stdhttp.ResponseWriter, request *stdhttp.Request) {
		response.WriteJSON(writer, stdhttp.StatusOK, map[string]string{"status": "ok"})
	})

	router.Route("/v1", func(api chi.Router) {
		authHandler.RegisterRoutes(api)

		api.Group(func(protected chi.Router) {
			protected.Use(middleware.Authenticate(tokenValidator))
			planningHandler.RegisterRoutes(protected)
			userHandler.RegisterRoutes(protected)
			bookingHandler.RegisterRoutes(protected)
			flightHandler.RegisterRoutes(protected)
			checklistHandler.RegisterRoutes(protected)
			expenseHandler.RegisterRoutes(protected)
		})
	})

	return router
}
