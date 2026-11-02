package main

import (
	"context"
	"log"

	"github.com/eleventravel/eleventravel-api/internal/config"
	"github.com/eleventravel/eleventravel-api/internal/platform/db"
	"github.com/eleventravel/eleventravel-api/internal/storage"
)

// Moves existing public profile media to the private R2 bucket. Run once per environment.
func main() {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	if !cfg.R2Configured() {
		log.Fatal("R2 is not configured")
	}

	pool, err := db.NewPostgresPool(ctx, cfg.DatabaseURL, cfg.DatabaseMaxConns)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	defer pool.Close()

	service := storage.NewService(
		storage.NewPostgresRepository(pool),
		storage.NewR2Store(storage.R2Settings{
			AccessKeyID:     cfg.R2AccessKeyID,
			SecretAccessKey: cfg.R2SecretAccessKey,
			Endpoint:        cfg.R2Endpoint,
			Region:          cfg.R2Region,
		}),
		storage.Settings{
			PublicBucket:  cfg.R2PublicBucket,
			PrivateBucket: cfg.R2PrivateBucket,
			PublicDomain:  cfg.R2PublicDomain,
		},
	)

	moved, err := service.MigratePublicProfileMedia(ctx)
	if err != nil {
		log.Fatalf("migrate profile media after %d objects: %v", moved, err)
	}
	log.Printf("migrated %d profile media objects to the private bucket", moved)
}
