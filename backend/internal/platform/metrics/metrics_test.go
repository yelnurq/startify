package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

// methods returns the method label of every recorded request series.
func methods(t *testing.T, m *Metrics) map[string]bool {
	t.Helper()
	families, err := m.Registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, f := range families {
		if f.GetName() != "qadam_http_request_duration_seconds" {
			continue
		}
		for _, metric := range f.GetMetric() {
			for _, l := range metric.GetLabel() {
				if l.GetName() == "method" {
					seen[l.GetValue()] = true
				}
			}
		}
	}
	return seen
}

func TestMethodLabelIsBounded(t *testing.T) {
	m := New()
	r := chi.NewRouter()
	r.Use(m.Middleware)
	r.Get("/ping", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })

	for _, method := range []string{http.MethodGet, http.MethodPost, "FOO123", "PROPFIND", "get", "X-RANDOM-1", "X-RANDOM-2"} {
		r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(method, "/ping", http.NoBody))
	}

	seen := methods(t, m)
	want := map[string]bool{http.MethodGet: true, http.MethodPost: true, "OTHER": true}
	if len(seen) != len(want) {
		t.Fatalf("method labels %v, want %v", seen, want)
	}
	for label := range want {
		if !seen[label] {
			t.Fatalf("method labels %v, want %v", seen, want)
		}
	}
}
