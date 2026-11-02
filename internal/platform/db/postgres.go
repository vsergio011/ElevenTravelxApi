package db

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// UUIDArray renders ids as a uuid[] literal. The simple protocol cannot encode []uuid.UUID, so queries pass this string with ANY($1::uuid[]).
func UUIDArray(ids []uuid.UUID) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = id.String()
	}
	return "{" + strings.Join(parts, ",") + "}"
}

// Simple protocol avoids prepared statements, which the Supabase transaction pooler (port 6543) does not support.
func NewPostgresPool(ctx context.Context, databaseURL string, maxConns int32) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, err
	}

	config.MaxConns = maxConns
	config.MaxConnLifetime = time.Hour
	config.MaxConnIdleTime = time.Minute
	config.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	config.HealthCheckPeriod = time.Minute

	return pgxpool.NewWithConfig(ctx, config)
}
