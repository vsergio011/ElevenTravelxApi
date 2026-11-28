package flights

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type LookupCache interface {
	Get(ctx context.Context, flightNumber string, flightDate time.Time) (FlightLookup, bool, error)
	Set(ctx context.Context, flightNumber string, flightDate time.Time, value FlightLookup, ttl time.Duration) error
}

type PostgresLookupCache struct {
	pool *pgxpool.Pool
}

func NewPostgresLookupCache(pool *pgxpool.Pool) *PostgresLookupCache {
	return &PostgresLookupCache{pool: pool}
}

func (c *PostgresLookupCache) Get(ctx context.Context, flightNumber string, flightDate time.Time) (FlightLookup, bool, error) {
	if c == nil || c.pool == nil {
		return FlightLookup{}, false, nil
	}
	var raw []byte
	var isEstimated bool
	var matchedDate pgtype.Date
	err := c.pool.QueryRow(
		ctx,
		`SELECT payload, is_estimated, matched_date
		 FROM public.flight_lookup_cache
		 WHERE flight_number = $1 AND flight_date = $2::date AND expires_at > now()`,
		NormalizeFlightNumber(flightNumber),
		flightDate.Format("2006-01-02"),
	).Scan(&raw, &isEstimated, &matchedDate)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return FlightLookup{}, false, nil
		}
		return FlightLookup{}, false, err
	}
	var result FlightLookup
	if err := json.Unmarshal(raw, &result); err != nil {
		return FlightLookup{}, false, err
	}
	result.IsEstimated = isEstimated
	if matchedDate.Valid {
		result.MatchedDate = matchedDate.Time.Format("2006-01-02")
	}
	return result, true, nil
}

func (c *PostgresLookupCache) Set(ctx context.Context, flightNumber string, flightDate time.Time, value FlightLookup, ttl time.Duration) error {
	if c == nil || c.pool == nil {
		return nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	var matchedDate *string
	if value.MatchedDate != "" {
		matchedDate = &value.MatchedDate
	}
	expiresAt := time.Now().UTC().Add(ttl)
	_, err = c.pool.Exec(
		ctx,
		`INSERT INTO public.flight_lookup_cache (flight_number, flight_date, payload, is_estimated, matched_date, fetched_at, expires_at)
		 VALUES ($1, $2::date, $3::jsonb, $4, $5::date, now(), $6)
		 ON CONFLICT (flight_number, flight_date) DO UPDATE SET
			payload = EXCLUDED.payload,
			is_estimated = EXCLUDED.is_estimated,
			matched_date = EXCLUDED.matched_date,
			fetched_at = now(),
			expires_at = EXCLUDED.expires_at`,
		NormalizeFlightNumber(flightNumber),
		flightDate.Format("2006-01-02"),
		raw,
		value.IsEstimated,
		matchedDate,
		expiresAt,
	)
	return err
}
