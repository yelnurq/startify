package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
	"github.com/redis/go-redis/v9"
	"github.com/riverqueue/river"

	"github.com/yelnurq/startify/backend/internal/ai/llm"
	"github.com/yelnurq/startify/backend/internal/ai/mock"
	"github.com/yelnurq/startify/backend/internal/apigen"
	"github.com/yelnurq/startify/backend/internal/files"
	"github.com/yelnurq/startify/backend/internal/mail"
	"github.com/yelnurq/startify/backend/internal/platform/config"
	"github.com/yelnurq/startify/backend/internal/platform/httpx"
	"github.com/yelnurq/startify/backend/internal/platform/metrics"
	"github.com/yelnurq/startify/backend/internal/platform/secrets"
	"github.com/yelnurq/startify/backend/internal/platform/testpg"
)

var pg *testpg.Server

func TestMain(m *testing.M) { testpg.Main(m, &pg) }

const testSecret = "test-secret-key-for-qadam-integration-tests-0123456789"

// captureQueue records enqueued jobs instead of storing them.
type captureQueue struct {
	mu   sync.Mutex
	jobs []river.JobArgs
}

func (q *captureQueue) Enqueue(_ context.Context, args river.JobArgs, _ *river.InsertOpts) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.jobs = append(q.jobs, args)
	return nil
}

type sentMail struct {
	Template mail.Template
	Locale   string
	To       string
	Data     map[string]string
}

type harness struct {
	handler  http.Handler
	router   routers.Router
	services *Services
	queue    *captureQueue
	storage  *files.MemoryStorage
	keys     *secrets.Keyring
	db       *testpg.Database
	redis    *miniredis.Miniredis
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	cfg, err := config.FromEnviron([]string{
		"QADAM_ENV=test",
		"QADAM_DATABASE_URL=postgres://unused@localhost/unused",
		"QADAM_AUTH_SECRET_KEY=" + testSecret,
		"QADAM_TENANT_BASE_DOMAIN=localhost",
		"QADAM_APP_PUBLIC_URL=http://localhost:3000",
	})
	if err != nil {
		t.Fatal(err)
	}
	database := pg.NewDB(t)
	mr := miniredis.RunT(t)
	h := &harness{queue: &captureQueue{}, storage: files.NewMemoryStorage(), db: database, redis: mr}
	h.keys, err = secrets.NewKeyring(testSecret)
	if err != nil {
		t.Fatal(err)
	}
	api := &API{
		Config:    cfg,
		Log:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		DB:        database.App,
		Cache:     redis.NewClient(&redis.Options{Addr: mr.Addr()}),
		Metrics:   metrics.New(),
		Storage:   h.storage,
		Queue:     h.queue,
		Providers: []llm.Provider{&mock.Provider{}},
	}
	h.handler, h.services, err = api.Handler(context.Background())
	if err != nil {
		t.Fatal(err)
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
	h.router, err = gorillamux.NewRouter(spec)
	if err != nil {
		t.Fatalf("router: %v", err)
	}
	return h
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
		Options:                &openapi3filter.Options{IncludeResponseStatus: true},
	}
	if err := openapi3filter.ValidateResponse(context.Background(), in); err != nil {
		t.Fatalf("response violates openapi.yaml: %v\nbody: %s", err, rec.Body.String())
	}
}

// mails decodes the letters enqueued so far.
func (h *harness) mails(t *testing.T) []sentMail {
	t.Helper()
	h.queue.mu.Lock()
	defer h.queue.mu.Unlock()
	var out []sentMail
	for _, j := range h.queue.jobs {
		args, ok := j.(mail.SendArgs)
		if !ok {
			continue
		}
		raw, err := h.keys.Open(args.Sealed)
		if err != nil {
			t.Fatalf("open letter: %v", err)
		}
		var p struct {
			To   string            `json:"to"`
			Data map[string]string `json:"data"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			t.Fatal(err)
		}
		out = append(out, sentMail{Template: args.Template, Locale: args.Locale, To: p.To, Data: p.Data})
	}
	return out
}

// lastMail returns the latest letter of a template to an address.
func (h *harness) lastMail(t *testing.T, to string, tpl mail.Template) sentMail {
	t.Helper()
	all := h.mails(t)
	for i := len(all) - 1; i >= 0; i-- {
		if all[i].To == to && all[i].Template == tpl {
			return all[i]
		}
	}
	t.Fatalf("no %s letter to %s", tpl, to)
	return sentMail{}
}

func linkToken(t *testing.T, m sentMail) string {
	t.Helper()
	u, err := url.Parse(m.Data["link"])
	if err != nil {
		t.Fatal(err)
	}
	token := u.Query().Get("token")
	if token == "" {
		t.Fatalf("no token in %s", m.Data["link"])
	}
	return token
}

// browser sends requests as a web client on a university host would: with
// its cookies, Origin, Sec-Fetch-Site and the CSRF header.
type browser struct {
	t    *testing.T
	h    *harness
	host string
	jar  map[string]*http.Cookie
}

func (h *harness) browser(t *testing.T, host string) *browser {
	t.Helper()
	return &browser{t: t, h: h, host: host, jar: map[string]*http.Cookie{}}
}

type form struct {
	contentType string
	body        []byte
}

func multipartForm(t *testing.T, fields map[string]string, fileField, fileName string, content []byte) form {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			t.Fatal(err)
		}
	}
	part, err := w.CreateFormFile(fileField, fileName)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return form{contentType: w.FormDataContentType(), body: buf.Bytes()}
}

func (b *browser) do(method, path string, body any, mods ...func(*http.Request)) *httptest.ResponseRecorder {
	b.t.Helper()
	var reader io.Reader = http.NoBody
	contentType := ""
	switch v := body.(type) {
	case nil:
	case form:
		reader, contentType = bytes.NewReader(v.body), v.contentType
	default:
		raw, err := json.Marshal(v)
		if err != nil {
			b.t.Fatal(err)
		}
		reader, contentType = bytes.NewReader(raw), "application/json"
	}
	req := httptest.NewRequest(method, "http://"+b.host+path, reader)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("Origin", "http://"+b.host)
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("User-Agent", "qadam-test")
	for _, c := range b.jar {
		if strings.HasPrefix(path, c.Path) {
			req.AddCookie(&http.Cookie{Name: c.Name, Value: c.Value})
		}
	}
	if method != http.MethodGet {
		if c, ok := b.jar["qadam_csrf"]; ok {
			req.Header.Set("X-CSRF-Token", c.Value)
		}
	}
	for _, m := range mods {
		m(req)
	}
	rec := httptest.NewRecorder()
	b.h.handler.ServeHTTP(rec, req)
	for _, c := range rec.Result().Cookies() {
		if c.MaxAge < 0 {
			delete(b.jar, c.Name)
		} else {
			b.jar[c.Name] = c
		}
	}
	return rec
}

func expect(t *testing.T, rec *httptest.ResponseRecorder, status int) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d: %s", rec.Code, status, rec.Body)
	}
}

func expectProblem(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) httpx.Problem {
	t.Helper()
	expect(t, rec, status)
	var p httpx.Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("not a problem: %s", rec.Body)
	}
	if p.Code != code {
		t.Fatalf("code = %q, want %q: %s", p.Code, code, rec.Body)
	}
	return p
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode %T: %v: %s", v, err, rec.Body)
	}
	return v
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
