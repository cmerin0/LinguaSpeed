// Package cache provides a Redis client factory using go-redis/v9.
// Call New once at startup; share the returned client across all cache repositories.
package cache

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// New creates a go-redis Client for the given address and verifies connectivity
// with a Ping. It returns an error if Redis is unreachable, rather than panicking.
func New(ctx context.Context, addr string) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr: addr,
	})

	// why: Ping verifies the client can reach Redis at startup. Without this,
	// a misconfigured REDIS_ADDR would only surface when the first game session
	// is created, making the failure hard to distinguish from a game logic bug.
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("cache.New: ping %s: %w", addr, err)
	}

	return client, nil
}
