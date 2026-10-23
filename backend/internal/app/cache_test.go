package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yelnurq/startify/backend/internal/apigen"
)

func cacheControl(t *testing.T, rec *httptest.ResponseRecorder, path, want string) {
	t.Helper()
	if got := rec.Header().Values("Cache-Control"); len(got) != 1 || got[0] != want {
		t.Errorf("%s: Cache-Control %q, want %q", path, got, want)
	}
}

// Answers for a signed-in member are never stored by the browser or a shared
// cache; what is meant to be cached says so itself.
func TestSignedInAnswersAreNotStored(t *testing.T) {
	h := newHarness(t)
	ts := seedTenants(t, h)
	b, _ := account(t, h, ts, "cache@qadam.test", "Кэш Тестов", "student")
	for _, path := range []string{"/api/v1/me", "/api/v1/projects", "/api/v1/auth/sessions", "/api/v1/team-requests", "/api/v1/meta"} {
		rec := b.do(http.MethodGet, path, nil)
		expect(t, rec, http.StatusOK)
		cacheControl(t, rec, path, "private, no-store")
	}
	// A replayed creation too.
	create := apigen.ProjectCreate{Title: "Кэш", Industry: apigen.IndustryEdtech, IpPolicyVersion: ptr("2026-09")}
	key := func(r *http.Request) { r.Header.Set("Idempotency-Key", "cache-project-0001") }
	expect(t, b.do(http.MethodPost, "/api/v1/projects", create, key), http.StatusCreated)
	replay := b.do(http.MethodPost, "/api/v1/projects", create, key)
	expect(t, replay, http.StatusCreated)
	if replay.Header().Get("Idempotent-Replayed") != "true" {
		t.Fatal("not replayed")
	}
	cacheControl(t, replay, "replayed POST /api/v1/projects", "private, no-store")

	// The showcase revalidates per visitor; a public picture never changes.
	rec := b.do(http.MethodGet, "/api/v1/showcase", nil)
	expect(t, rec, http.StatusOK)
	cacheControl(t, rec, "/api/v1/showcase", "private, no-cache")
	if !strings.Contains(strings.Join(rec.Header().Values("Vary"), ","), "Cookie") {
		t.Errorf("showcase Vary %q", rec.Header().Values("Vary"))
	}
	rec = b.do(http.MethodPut, "/api/v1/me/avatar", multipartForm(t, nil, "file", "photo.png", pngImage(t, 64, 64)))
	expect(t, rec, http.StatusOK)
	avatar := decode[apigen.Me](t, rec).User.AvatarUrl
	if avatar == nil {
		t.Fatal("no avatar")
	}
	rec = b.do(http.MethodGet, *avatar, nil)
	expect(t, rec, http.StatusOK)
	cacheControl(t, rec, *avatar, "public, max-age=31536000, immutable")
}
