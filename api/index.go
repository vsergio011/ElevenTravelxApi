package api

import (
	"context"
	"log"
	"net/http"
	"sync"

	"github.com/eleventravel/eleventravel-api/internal/app"
	"github.com/eleventravel/eleventravel-api/internal/config"
)

var application struct {
	sync.Once
	handler http.Handler
	err     error
}

func Handler(writer http.ResponseWriter, request *http.Request) {
	application.Do(func() {
		cfg, err := config.Load()
		if err != nil {
			application.err = err
			return
		}

		instance, err := app.New(context.Background(), cfg)
		if err != nil {
			application.err = err
			return
		}

		application.handler = instance.Handler
	})

	if application.err != nil {
		log.Printf("create application: %v", application.err)
		http.Error(writer, "internal server error", http.StatusInternalServerError)
		return
	}

	application.handler.ServeHTTP(writer, request)
}
