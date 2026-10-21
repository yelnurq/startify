package app

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
	"github.com/redis/go-redis/v9"

	"github.com/yelnurq/startify/backend/internal/apigen"
	"github.com/yelnurq/startify/backend/internal/platform/config"
	"github.com/yelnurq/startify/backend/internal/platform/metrics"
	"github.com/yelnurq/startify/backend/internal/platform/testpg"
)

var pg *testpg.Server

func TestMain(m *testing.M) { testpg.Main(m, &pg) }

type harness struct {
	handler http.Handler
	router  routers.Router
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	cfg, err := config.FromEnviron([]string{"QADAM_DATABASE_URL=postgres://unused@localhost/unused", "QADAM_ENV=test"})
	if err != nil {
		t.Fatal(err)
	}
	mr := miniredis.RunT(t)
	api := &API{
		Config:  cfg,
		Log:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		DB:      pg.NewDB(t).App,
		Cache:   redis.NewClient(&redis.Options{Addr: mr.Addr()}),
		Metrics: metrics.New(),
	}

	spec, err := apigen.GetSpec()
	if err != nil {
		t.Fatalf("load embedded spec: %v", err)
	}
	spec.Servers = nil // match request paths that include /api/v1 directly
	for _, path := range spec.Paths.InMatchingOrder() {
		item := spec.Paths.Value(path)
		spec.Paths.Delete(path)
		spec.Paths.Set("/api/v1"+path, item)
	}
	router, err := gorillamux.NewRouter(spec)
	if err != nil {
		t.Fatalf("router: %v", err)
	}
	return &harness{handler: api.Handler(), router: router}
}

func (h *harness) do(t *testing.T, method, target string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, http.NoBody)
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, req)
	return rec
}

// validate checks a response against the operation's response schema in openapi.yaml.
func (h *harness) validate(t *testing.T, method, target string, rec *httptest.ResponseRecorder) {
	t.Helper()
	req := httptest.NewRequest(method, target, http.NoBody)
	route, params, err := h.router.FindRoute(req)
	if err != nil {
		t.Fatalf("route %s %s not in spec: %v", method, target, err)
	}
	in := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: &openapi3filter.RequestValidationInput{Request: req, PathParams: params, Route: route},
		Status:                 rec.Code,
		Header:                 rec.Header(),
		Body:                   io.NopCloser(bytes.NewReader(rec.Body.Bytes())),
	}
	if err := openapi3filter.ValidateResponse(context.Background(), in); err != nil {
		t.Fatalf("response violates openapi.yaml: %v\nbody: %s", err, rec.Body.String())
	}
}

func TestGetMetaMatchesContract(t *testing.T) {
	h := newHarness(t)
	rec := h.do(t, http.MethodGet, "/api/v1/meta")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	h.validate(t, http.MethodGet, "/api/v1/meta", rec)
	if !strings.Contains(rec.Body.String(), `"default_locale":"kk"`) {
		t.Errorf("unexpected body: %s", rec.Body)
	}
}

func TestUnknownRouteIsProblem(t *testing.T) {
	h := newHarness(t)
	for _, target := range []string{"/api/v1/nope", "/nope"} {
		rec := h.do(t, http.MethodGet, target)
		if rec.Code != http.StatusNotFound || rec.Header().Get("Content-Type") != "application/problem+json" {
			t.Fatalf("%s: status %d content-type %q", target, rec.Code, rec.Header().Get("Content-Type"))
		}
		if !strings.Contains(rec.Body.String(), `"code":"route.not_found"`) {
			t.Fatalf("%s: body %s", target, rec.Body)
		}
	}
	rec := h.do(t, http.MethodDelete, "/api/v1/meta")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("DELETE /meta: status %d", rec.Code)
	}
}

func TestProbesMetricsAndHeaders(t *testing.T) {
	h := newHarness(t)
	if rec := h.do(t, http.MethodGet, "/healthz"); rec.Code != http.StatusOK {
		t.Fatalf("/healthz: %d", rec.Code)
	}
	if rec := h.do(t, http.MethodGet, "/readyz"); rec.Code != http.StatusOK {
		t.Fatalf("/readyz: %d %s", rec.Code, rec.Body)
	}

	rec := h.do(t, http.MethodGet, "/api/v1/meta")
	for _, header := range []string{"X-Request-ID", "X-Content-Type-Options", "Content-Security-Policy", "Referrer-Policy"} {
		if rec.Header().Get(header) == "" {
			t.Errorf("missing header %s", header)
		}
	}

	metricsRec := h.do(t, http.MethodGet, "/metrics")
	if !strings.Contains(metricsRec.Body.String(), `qadam_http_request_duration_seconds_count{method="GET",route="/api/v1/meta",status="200"}`) {
		t.Fatalf("route-labelled metric missing:\n%s", metricsRec.Body)
	}
}
