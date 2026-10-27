package httpx

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNoStoreByDefault(t *testing.T) {
	for _, c := range []struct {
		name    string
		handler http.HandlerFunc
		want    string
	}{
		{"handler without caching", func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"ok":true}`))
		}, "private, no-store"},
		{"handler with its own caching", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			w.WriteHeader(http.StatusOK)
		}, "public, max-age=31536000, immutable"},
		{"error", func(w http.ResponseWriter, r *http.Request) {
			WriteError(w, r, errBoom)
		}, "no-store"},
	} {
		rec := httptest.NewRecorder()
		NoStoreByDefault(c.handler).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/me", http.NoBody))
		if got := rec.Header().Values("Cache-Control"); len(got) != 1 || got[0] != c.want {
			t.Errorf("%s: Cache-Control %q, want %q", c.name, got, c.want)
		}
	}
}

var errBoom = errors.New("boom")
