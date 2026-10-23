package app

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSpecRoutes(t *testing.T) {
	streams, uploads, err := specRoutes()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		method, path   string
		stream, upload bool
	}{
		{http.MethodPost, "/api/v1/admin/ai/check", true, false},
		{http.MethodPost, "/api/v1/projects/0198/steps/problem/mentor", true, false},
		{http.MethodGet, "/api/v1/projects/0198/steps/problem/mentor", false, false},
		{http.MethodPut, "/api/v1/me/avatar", false, true},
		{http.MethodPost, "/api/v1/admin/imports", false, true},
		{http.MethodPost, "/api/v1/admin/knowledge", false, true},
		{http.MethodPut, "/api/v1/projects/0198/showcase/cover", false, true},
		{http.MethodPost, "/api/v1/projects", false, false},
		{http.MethodPost, "/api/v1/auth/login", false, false},
		{http.MethodDelete, "/api/v1/me/avatar", false, false},
	} {
		r := httptest.NewRequest(c.method, c.path, http.NoBody)
		if got := streams.Match(r); got != c.stream {
			t.Errorf("%s %s: event stream %v, want %v", c.method, c.path, got, c.stream)
		}
		if got := uploads.Match(r); got != c.upload {
			t.Errorf("%s %s: upload %v, want %v", c.method, c.path, got, c.upload)
		}
	}
}
