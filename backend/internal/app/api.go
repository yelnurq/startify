// Package app assembles the modules into the API handler and the worker.
package app

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/yelnurq/startify/backend/internal/apigen"
	"github.com/yelnurq/startify/backend/internal/platform/apperr"
	"github.com/yelnurq/startify/backend/internal/platform/config"
	"github.com/yelnurq/startify/backend/internal/platform/db"
	"github.com/yelnurq/startify/backend/internal/platform/health"
	"github.com/yelnurq/startify/backend/internal/platform/httpx"
	"github.com/yelnurq/startify/backend/internal/platform/idempotency"
	"github.com/yelnurq/startify/backend/internal/platform/metrics"
	"github.com/yelnurq/startify/backend/internal/platform/telemetry"
	"github.com/yelnurq/startify/backend/internal/system"
)

// IdempotencyTTL is how long a stored response can be replayed.
const IdempotencyTTL = 24 * time.Hour

// API holds the dependencies of the HTTP server.
type API struct {
	Config  *config.Config
	Log     *slog.Logger
	DB      *db.DB
	Cache   *redis.Client
	Metrics *metrics.Metrics
}

// server composes the module handlers into the generated strict interface.
// Each module implements the operations of its OpenAPI tag.
type server struct {
	*system.Handler
}

var _ apigen.StrictServerInterface = (*server)(nil)

// Handler builds the root HTTP handler.
func (a *API) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(
		httpx.RequestID,
		httpx.ClientIP(a.Config.HTTP.TrustedPrefixes()),
		httpx.Recoverer,
		telemetry.RouteSpanName,
		a.Metrics.Middleware,
		httpx.AccessLog(a.Log),
		httpx.SecurityHeaders(a.Config.IsProduction()),
	)
	r.NotFound(httpx.NotFoundHandler)
	r.MethodNotAllowed(httpx.MethodNotAllowedHandler)

	probes := health.New(2*time.Second,
		health.Check{Name: "postgres", Fn: a.DB.Ping},
		health.Check{Name: "valkey", Fn: func(ctx context.Context) error { return a.Cache.Ping(ctx).Err() }},
	)
	r.Get("/healthz", probes.Live)
	r.Get("/readyz", probes.Ready)
	r.Method(http.MethodGet, "/metrics", a.Metrics.Handler())

	srv := &server{
		Handler: system.NewHandler(a.Config.App.DefaultLocale),
	}
	strict := apigen.NewStrictHandlerWithOptions(srv, nil, apigen.StrictHTTPServerOptions{
		RequestErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			httpx.WriteError(w, r, apperr.BadRequest("request.malformed", err.Error()))
		},
		ResponseErrorHandlerFunc: httpx.WriteError,
	})

	r.Route("/api/v1", func(api chi.Router) {
		api.Use(
			httpx.Timeout(a.Config.HTTP.RequestTimeout),
			httpx.MaxBody(a.Config.HTTP.MaxBodyBytes),
			httpx.Idempotency(idempotency.NewStore(a.DB), idempotency.ScopeByClientIP, IdempotencyTTL),
		)
		api.NotFound(httpx.NotFoundHandler)
		api.MethodNotAllowed(httpx.MethodNotAllowedHandler)
		apigen.HandlerWithOptions(strict, apigen.ChiServerOptions{
			BaseRouter: api,
			ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
				httpx.WriteError(w, r, apperr.BadRequest("request.invalid_parameter", err.Error()))
			},
		})
	})

	return otelhttp.NewHandler(r, "http.server",
		otelhttp.WithFilter(func(r *http.Request) bool { return r.URL.Path != "/healthz" && r.URL.Path != "/readyz" }))
}
