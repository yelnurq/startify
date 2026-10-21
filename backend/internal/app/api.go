// Package app assembles the modules into the API handler and the worker.
package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/yelnurq/startify/backend/internal/apigen"
	"github.com/yelnurq/startify/backend/internal/audit"
	"github.com/yelnurq/startify/backend/internal/auth"
	"github.com/yelnurq/startify/backend/internal/files"
	"github.com/yelnurq/startify/backend/internal/imports"
	"github.com/yelnurq/startify/backend/internal/mail"
	"github.com/yelnurq/startify/backend/internal/org"
	"github.com/yelnurq/startify/backend/internal/platform/apperr"
	"github.com/yelnurq/startify/backend/internal/platform/authz"
	"github.com/yelnurq/startify/backend/internal/platform/config"
	"github.com/yelnurq/startify/backend/internal/platform/db"
	"github.com/yelnurq/startify/backend/internal/platform/health"
	"github.com/yelnurq/startify/backend/internal/platform/httpx"
	"github.com/yelnurq/startify/backend/internal/platform/idempotency"
	"github.com/yelnurq/startify/backend/internal/platform/jobs"
	"github.com/yelnurq/startify/backend/internal/platform/metrics"
	"github.com/yelnurq/startify/backend/internal/platform/ratelimit"
	"github.com/yelnurq/startify/backend/internal/platform/secrets"
	"github.com/yelnurq/startify/backend/internal/platform/telemetry"
	"github.com/yelnurq/startify/backend/internal/system"
	"github.com/yelnurq/startify/backend/internal/team"
	"github.com/yelnurq/startify/backend/internal/tenant"
	"github.com/yelnurq/startify/backend/internal/user"
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
	// Storage and Queue default to S3 and River; tests replace them.
	Storage files.Storage
	Queue   jobs.Enqueuer
	// Scanner replaces the ClamAV client (tests).
	Scanner files.Scanner
}

// Every module names its handler Handler; aliases give the embedded fields
// distinct names.
type (
	systemHandler  = system.Handler
	tenantHandler  = tenant.Handler
	authHandler    = auth.Handler
	userHandler    = user.Handler
	orgHandler     = org.Handler
	filesHandler   = files.Handler
	teamHandler    = team.Handler
	importsHandler = imports.Handler
)

// server composes the module handlers into the generated strict interface.
// Each module implements the operations of its OpenAPI tag.
type server struct {
	*systemHandler
	*tenantHandler
	*authHandler
	*userHandler
	*orgHandler
	*filesHandler
	*teamHandler
	*importsHandler
}

var _ apigen.StrictServerInterface = (*server)(nil)

// Services are the module services, exposed for tests and tools.
type Services struct {
	Tenants *tenant.Service
	Auth    *auth.Service
	Users   *user.Service
	Org     *org.Service
	Files   *files.Service
	Team    *team.Service
	Imports *imports.Service
}

func (a *API) services(ctx context.Context) (*Services, error) {
	cfg := a.Config
	keys, err := secrets.NewKeyring(cfg.Auth.SecretKey)
	if err != nil {
		return nil, err
	}
	if a.Storage == nil {
		if a.Storage, err = files.NewStorage(ctx, cfg.S3); err != nil {
			return nil, err
		}
	}
	if a.Queue == nil {
		if a.Queue, err = jobs.NewQueue(a.DB); err != nil {
			return nil, err
		}
	}
	tenants, err := tenant.NewService(a.DB, cfg.Tenant, cfg.App.PublicURL)
	if err != nil {
		return nil, err
	}
	auditLog := audit.NewLogger(a.DB)
	limiter := ratelimit.New(a.Cache)
	mailer := mail.NewMailer(a.Queue, keys)
	revocations := auth.NewRevocations(a.Cache, a.DB, cfg.Auth.AccessTTL)
	orgs := org.NewService(a.DB)
	fileSvc := files.NewService(a.DB, a.Storage, cfg.Files, a.Log)
	if a.Scanner != nil {
		fileSvc.WithScanner(a.Scanner)
	}
	users := user.NewService(a.DB, orgs, fileSvc, tenants, auditLog, revocations)
	authSvc := auth.NewService(auth.Deps{
		Config: cfg.Auth, PublicURL: cfg.App.PublicURL, DB: a.DB, Keys: keys, Revocations: revocations,
		Limiter: limiter, Mailer: mailer, Users: users, Tenants: tenants, Audit: auditLog,
	})
	return &Services{
		Tenants: tenants,
		Auth:    authSvc,
		Users:   users,
		Org:     orgs,
		Files:   fileSvc,
		Team:    team.NewService(a.DB, users, mailer, limiter, auditLog),
		Imports: imports.NewService(a.DB, orgs, users, authSvc, auditLog),
	}, nil
}

// idempotencyScope keys stored responses by the signed-in member, or by the
// client address for anonymous requests.
func idempotencyScope(r *http.Request) (string, bool) {
	if p, ok := authz.FromContext(r.Context()); ok {
		return "user:" + p.TenantID.String() + ":" + p.UserID.String(), true
	}
	return idempotency.ScopeByClientIP(r)
}

// Handler builds the root HTTP handler.
func (a *API) Handler(ctx context.Context) (http.Handler, *Services, error) {
	svc, err := a.services(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("app: %w", err)
	}

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
		systemHandler:  system.NewHandler(a.Config.App.DefaultLocale),
		tenantHandler:  tenant.NewHandler(svc.Tenants),
		authHandler:    auth.NewHandler(svc.Auth),
		userHandler:    user.NewHandler(svc.Users),
		orgHandler:     org.NewHandler(svc.Org),
		filesHandler:   files.NewHandler(svc.Files),
		teamHandler:    team.NewHandler(svc.Team),
		importsHandler: imports.NewHandler(svc.Imports),
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
			httpx.MaxBody(a.Config.HTTP.MaxBodyBytes, a.Config.Files.MaxBytes+1<<20),
			httpx.KeepRequest,
			svc.Tenants.Middleware,
			auth.CSRF,
			svc.Auth.Authenticate,
			httpx.Idempotency(idempotency.NewStore(a.DB), idempotencyScope, IdempotencyTTL),
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

	handler := otelhttp.NewHandler(r, "http.server",
		otelhttp.WithFilter(func(r *http.Request) bool { return r.URL.Path != "/healthz" && r.URL.Path != "/readyz" }))
	return handler, svc, nil
}
