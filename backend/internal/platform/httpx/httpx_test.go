package httpx

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/yelnurq/startify/backend/internal/platform/apperr"
)

func decodeProblem(t *testing.T, rec *httptest.ResponseRecorder) Problem {
	t.Helper()
	if ct := rec.Header().Get("Content-Type"); ct != ContentTypeProblem {
		t.Fatalf("Content-Type = %q, want %q", ct, ContentTypeProblem)
	}
	var p Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("decode problem: %v", err)
	}
	return p
}

func TestWriteErrorDomainError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/42", http.NoBody)
	rec := httptest.NewRecorder()
	WriteError(rec, req, apperr.NotFound("project.not_found", "project does not exist").
		WithParams(map[string]any{"id": "42"}))

	p := decodeProblem(t, rec)
	if p.Status != http.StatusNotFound || p.Code != "project.not_found" || p.Title != "Not Found" {
		t.Fatalf("unexpected problem: %+v", p)
	}
	if p.Type != "https://qadam.kz/problems/project.not_found" || p.Instance != "/api/v1/projects/42" {
		t.Fatalf("unexpected type/instance: %+v", p)
	}
	if p.Params["id"] != "42" || p.Detail == "" {
		t.Fatalf("params/detail lost: %+v", p)
	}
}

func TestWriteErrorHidesInternalDetails(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()
	WriteError(rec, req, errors.New("pq: password authentication failed for user secret"))
	p := decodeProblem(t, rec)
	if p.Status != http.StatusInternalServerError || p.Code != "internal" {
		t.Fatalf("unexpected problem: %+v", p)
	}
	if strings.Contains(rec.Body.String(), "secret") {
		t.Fatalf("internal error leaked to client: %s", rec.Body)
	}
}

func TestWriteErrorValidationAndRetryAfter(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", http.NoBody)
	rec := httptest.NewRecorder()
	WriteError(rec, req, apperr.Validation(apperr.FieldError{Field: "title", Code: "required"}))
	p := decodeProblem(t, rec)
	if p.Status != http.StatusUnprocessableEntity || len(p.Errors) != 1 || p.Errors[0].Field != "title" {
		t.Fatalf("unexpected problem: %+v", p)
	}

	rec = httptest.NewRecorder()
	WriteError(rec, req, apperr.RateLimited("ai.limit_exceeded", "limit", 90*time.Second+time.Millisecond))
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") != "91" {
		t.Fatalf("got %d Retry-After=%q", rec.Code, rec.Header().Get("Retry-After"))
	}
}

func TestMaxBodyProducesTooLarge(t *testing.T) {
	h := MaxBody(1024)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 4096)
		_, err := r.Body.Read(buf)
		for err == nil {
			_, err = r.Body.Read(buf)
		}
		WriteError(w, r, err)
	}))
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(strings.Repeat("x", 4096)))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if p := decodeProblem(t, rec); p.Status != http.StatusRequestEntityTooLarge || p.Code != "request.too_large" {
		t.Fatalf("unexpected problem: %+v", p)
	}
}

func TestRequestIDPropagation(t *testing.T) {
	var seen string
	h := RequestID(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { seen = RequestIDFrom(r.Context()) }))

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.Header.Set("X-Request-ID", "trace-12345678")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if seen != "trace-12345678" || rec.Header().Get("X-Request-ID") != seen {
		t.Fatalf("incoming id not propagated: %q", seen)
	}

	req = httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.Header.Set("X-Request-ID", "bad id with spaces")
	h.ServeHTTP(httptest.NewRecorder(), req)
	if seen == "bad id with spaces" || len(seen) != 36 {
		t.Fatalf("malformed id must be replaced by a UUID, got %q", seen)
	}
}

func TestResolveClientIP(t *testing.T) {
	trusted := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
	cases := []struct {
		name   string
		remote string
		xff    string
		want   string
	}{
		{"untrusted peer ignores header", "203.0.113.7:4000", "1.2.3.4", "203.0.113.7"},
		{"trusted peer uses last untrusted hop", "10.0.0.2:4000", "1.2.3.4, 198.51.100.9, 10.0.0.5", "198.51.100.9"},
		{"only trusted hops", "10.0.0.2:4000", "10.0.0.9", "10.0.0.9"},
		{"garbage stops the walk", "10.0.0.2:4000", "1.2.3.4, nonsense", "10.0.0.2"},
		{"ipv6 peer", "[2001:db8::1]:443", "", "2001:db8::1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			req.RemoteAddr = tc.remote
			if tc.xff != "" {
				req.Header.Set("X-Forwarded-For", tc.xff)
			}
			if got := resolveClientIP(req, trusted).String(); got != tc.want {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
		})
	}
}

func TestRecovererReturnsProblem(t *testing.T) {
	h := Recoverer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", http.NoBody))
	if p := decodeProblem(t, rec); p.Status != http.StatusInternalServerError {
		t.Fatalf("unexpected problem: %+v", p)
	}
}

func TestTimeoutSkipsEventStreams(t *testing.T) {
	var hasDeadline bool
	h := Timeout(time.Second)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, hasDeadline = r.Context().Deadline()
	}))
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	h.ServeHTTP(httptest.NewRecorder(), req)
	if !hasDeadline {
		t.Fatal("regular request must get a deadline")
	}
	req.Header.Set("Accept", "text/event-stream")
	h.ServeHTTP(httptest.NewRecorder(), req)
	if hasDeadline {
		t.Fatal("event stream must not get a deadline")
	}
}
