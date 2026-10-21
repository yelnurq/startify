package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestReady(t *testing.T) {
	ok := Check{Name: "postgres", Fn: func(context.Context) error { return nil }}
	broken := Check{Name: "valkey", Fn: func(context.Context) error { return errors.New("connection refused") }}
	slow := Check{Name: "slow", Fn: func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	}}

	cases := []struct {
		name   string
		checks []Check
		status int
		want   map[string]string
	}{
		{"all healthy", []Check{ok}, http.StatusOK, map[string]string{"postgres": "ok"}},
		{"one broken", []Check{ok, broken}, http.StatusServiceUnavailable, map[string]string{"postgres": "ok", "valkey": "fail"}},
		{"timeout", []Check{slow}, http.StatusServiceUnavailable, map[string]string{"slow": "fail"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			New(50*time.Millisecond, tc.checks...).Ready(rec, httptest.NewRequest(http.MethodGet, "/readyz", http.NoBody))
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d", rec.Code, tc.status)
			}
			var body struct {
				Checks map[string]string `json:"checks"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			for k, v := range tc.want {
				if body.Checks[k] != v {
					t.Errorf("check %s = %q, want %q", k, body.Checks[k], v)
				}
			}
		})
	}
}

func TestLive(t *testing.T) {
	rec := httptest.NewRecorder()
	New(time.Second).Live(rec, httptest.NewRequest(http.MethodGet, "/healthz", http.NoBody))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
}
