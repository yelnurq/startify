// Package health serves liveness (/healthz) and readiness (/readyz) probes.
package health

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/yelnurq/startify/backend/internal/platform/httpx"
)

// Check probes one dependency.
type Check struct {
	Name string
	Fn   func(ctx context.Context) error
}

// Handler runs the configured checks.
type Handler struct {
	checks  []Check
	timeout time.Duration
}

// New returns a handler that gives each check at most timeout to answer.
func New(timeout time.Duration, checks ...Check) *Handler {
	return &Handler{checks: checks, timeout: timeout}
}

// Live reports that the process is running; it never touches dependencies.
func (h *Handler) Live(w http.ResponseWriter, _ *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Ready reports whether every dependency answers. Failure reasons are logged,
// not returned: the endpoint is reachable by anyone who can reach the port.
func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	results := make(map[string]string, len(h.checks))
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, c := range h.checks {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
			defer cancel()
			state := "ok"
			if err := c.Fn(ctx); err != nil {
				state = "fail"
				slog.WarnContext(r.Context(), "readiness check failed", slog.String("check", c.Name), slog.Any("error", err))
			}
			mu.Lock()
			results[c.Name] = state
			mu.Unlock()
		})
	}
	wg.Wait()

	status, overall := http.StatusOK, "ok"
	for _, state := range results {
		if state != "ok" {
			status, overall = http.StatusServiceUnavailable, "fail"
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.WriteJSON(w, status, map[string]any{"status": overall, "checks": results})
}
