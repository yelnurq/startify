// Package cache connects to Valkey (Redis protocol), used for rate limits,
// presence and pub/sub fan-out between API instances.
package cache

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"

	"github.com/yelnurq/startify/backend/internal/platform/config"
)

// Connect opens a client and verifies connectivity.
func Connect(ctx context.Context, cfg config.Valkey) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("cache: ping %s: %w", cfg.Addr, err)
	}
	return client, nil
}
