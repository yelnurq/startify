package showcase

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// A release of the web client replaces its hashed scripts at once: the next
// page must already point at the new ones.
func TestShellFollowsANewRelease(t *testing.T) {
	var release, unchanged atomic.Int32
	release.Store(1)
	web := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tag := fmt.Sprintf(`"release-%d"`, release.Load())
		if r.Header.Get("If-None-Match") == tag {
			unchanged.Add(1)
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", tag)
		_, _ = fmt.Fprintf(w, `<html><head><script src="/assets/index-%d.js"></script></head><body><div id="root"></div></body></html>`, release.Load())
	}))
	defer web.Close()
	s := newShell(web.URL)
	ctx := context.Background()

	for range 2 {
		if page := string(s.get(ctx)); !strings.Contains(page, "index-1.js") {
			t.Fatalf("first release: %s", page)
		}
	}
	if unchanged.Load() != 1 {
		t.Fatalf("the second page asked %d times with the ETag, want 1", unchanged.Load())
	}
	release.Store(2)
	if page := string(s.get(ctx)); !strings.Contains(page, "index-2.js") {
		t.Fatalf("a new release is not picked up at once: %s", page)
	}
	web.Close()
	if page := string(s.get(ctx)); !strings.Contains(page, "index-2.js") {
		t.Fatalf("without the web server the last copy is served: %s", page)
	}
}
