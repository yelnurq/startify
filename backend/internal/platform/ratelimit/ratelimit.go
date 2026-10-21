// Package ratelimit throttles abuse-prone operations (sign-in, codes, AI) with
// the GCRA algorithm on Valkey, shared by every API instance.
package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis_rate/v10"
	"github.com/redis/go-redis/v9"

	"github.com/yelnurq/startify/backend/internal/platform/apperr"
)

// Limit is an allowance: Rate events per Period with bursts up to Burst.
type Limit struct {
	Rate   int
	Burst  int
	Period time.Duration
}

// PerMinute allows n events a minute.
func PerMinute(n int) Limit { return Limit{Rate: n, Burst: n, Period: time.Minute} }

// PerHour allows n events an hour.
func PerHour(n int) Limit { return Limit{Rate: n, Burst: n, Period: time.Hour} }

// Limiter checks allowances.
type Limiter struct {
	l *redis_rate.Limiter
}

// New returns a limiter on the Valkey client.
func New(client *redis.Client) *Limiter { return &Limiter{l: redis_rate.NewLimiter(client)} }

// Allow consumes one event of key; when the allowance is exhausted it returns
// a rate-limited error with the code and a Retry-After hint.
func (r *Limiter) Allow(ctx context.Context, key string, limit Limit, code string) error {
	res, err := r.l.Allow(ctx, "rl:"+key, redis_rate.Limit{Rate: limit.Rate, Burst: limit.Burst, Period: limit.Period})
	if err != nil {
		return apperr.Internal(fmt.Errorf("ratelimit: %w", err))
	}
	if res.Allowed == 0 {
		wait := res.RetryAfter
		if wait < time.Second {
			wait = time.Second
		}
		return apperr.RateLimited(code, "too many requests", wait).
			WithParams(map[string]any{"retry_after_seconds": int(wait.Seconds() + 0.999)})
	}
	return nil
}

// Reset forgets the history of key (e.g. failed sign-ins after a success).
func (r *Limiter) Reset(ctx context.Context, key string) error {
	return r.l.Reset(ctx, "rl:"+key)
}
