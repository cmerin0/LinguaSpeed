// Package db provides a PostgreSQL connection pool factory using pgx/v5.
// Call New once at startup; share the returned pool across all repositories.
package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// New opens a pgx connection pool using the given DSN and verifies connectivity
// with a Ping. It returns an error if the connection cannot be established within
// the context deadline, rather than panicking — the caller (main) decides whether
// to exit.
func New(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("db.New: parse config: %w", err)
	}

	// why: Ping verifies the pool can actually reach the database. pgxpool.New
	// only parses the DSN; a bad host/credentials would only surface on the first
	// real query otherwise, far from the startup sequence where it belongs.
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db.New: ping: %w", err)
	}

	return pool, nil
}
