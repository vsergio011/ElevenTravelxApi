package app

import (
	"context"
	"net/http"

	"github.com/eleventravel/eleventravel-api/internal/bookings"
	"github.com/eleventravel/eleventravel-api/internal/checklists"
	"github.com/eleventravel/eleventravel-api/internal/config"
	"github.com/eleventravel/eleventravel-api/internal/expenses"
	"github.com/eleventravel/eleventravel-api/internal/flights"
	apihttp "github.com/eleventravel/eleventravel-api/internal/http"
	"github.com/eleventravel/eleventravel-api/internal/plannings"
	"github.com/eleventravel/eleventravel-api/internal/platform/auth/supabasejwt"
	"github.com/eleventravel/eleventravel-api/internal/platform/db"
	"github.com/eleventravel/eleventravel-api/internal/users"
)

type Application struct {
	Handler http.Handler
	Close   func()
}

func New(ctx context.Context, cfg config.Config) (*Application, error) {
	pool, err := db.NewPostgresPool(ctx, cfg.DatabaseURL, cfg.DatabaseMaxConns)
	if err != nil {
		return nil, err
	}

	tokenValidator, err := supabasejwt.NewValidator(cfg.SupabaseJWTSecret, cfg.SupabaseJWTIssuer, cfg.SupabaseJWTAudience)
	if err != nil {
		pool.Close()
		return nil, err
	}

	planningRepository := plannings.NewPostgresRepository(pool)
	planningService := plannings.NewService(planningRepository)
	planningHandler := plannings.NewHandler(planningService)
	userRepository := users.NewPostgresRepository(pool)
	userService := users.NewService(userRepository)
	userHandler := users.NewHandler(userService)
	bookingRepository := bookings.NewPostgresRepository(pool)
	flightRepository := flights.NewPostgresRepository(pool)
	flightLookup := flights.NewAeroDataBoxClient(cfg.AeroDataBoxAPIKey, cfg.AeroDataBoxHost)
	flightService := flights.NewService(flightRepository, planningRepository, flightLookup)
	bookingService := bookings.NewService(bookingRepository, planningRepository, flightService)
	bookingHandler := bookings.NewHandler(bookingService)
	flightHandler := flights.NewHandler(flightService)
	checklistRepository := checklists.NewPostgresRepository(pool)
	checklistService := checklists.NewService(checklistRepository, planningRepository)
	checklistHandler := checklists.NewHandler(checklistService)
	expenseRepository := expenses.NewPostgresRepository(pool)
	expenseService := expenses.NewService(expenseRepository, planningRepository, bookingService, flightService)
	expenseHandler := expenses.NewHandler(expenseService)

	return &Application{
		Handler: apihttp.NewRouter(tokenValidator, planningHandler, userHandler, bookingHandler, flightHandler, checklistHandler, expenseHandler, cfg.CORSAllowedOrigins),
		Close:   pool.Close,
	}, nil
}
