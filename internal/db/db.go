package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool configures, initializes, and verifies a PostgreSQL connection pool.
// It applies explicit pool limits and pings the database within the provided context.
func NewPool(
	ctx context.Context,
	connString string,
	maxConns int32,
	minConns int32,
	maxIdleTime time.Duration,
	maxLifetime time.Duration,
) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, err
	}

	config.MaxConns = maxConns
	config.MinConns = minConns
	config.MaxConnIdleTime = maxIdleTime
	config.MaxConnLifetime = maxLifetime

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}

	// Verify connectivity within the bounded context.
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}

	return pool, nil
}
