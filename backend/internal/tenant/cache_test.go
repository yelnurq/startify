package tenant

import (
	"fmt"
	"testing"
	"time"
)

func TestResolverCacheIsBounded(t *testing.T) {
	s := &Service{cache: map[string]cacheEntry{}, cacheLimit: 8}
	expired := cacheEntry{expires: time.Now().Add(-time.Second)}
	live := cacheEntry{expires: time.Now().Add(time.Minute), found: true}
	for i := range 4 {
		s.remember(fmt.Sprintf("old-%d.example", i), expired)
	}
	for i := range 4 {
		s.remember(fmt.Sprintf("live-%d.example", i), live)
	}
	// A full cache drops expired entries before live ones.
	s.remember("next.example", live)
	if len(s.cache) != 5 {
		t.Fatalf("expired entries kept: %d entries", len(s.cache))
	}
	for i := range 1000 {
		s.remember(fmt.Sprintf("random-%d.example", i), live)
		if len(s.cache) > s.cacheLimit {
			t.Fatalf("cache grew to %d entries", len(s.cache))
		}
	}
	if _, ok := s.cache["random-999.example"]; !ok {
		t.Fatal("the newest host is not cached")
	}
}

func TestNormalizeHost(t *testing.T) {
	for in, want := range map[string]string{
		"KazUTB.qadam.kz":       "kazutb.qadam.kz",
		"kazutb.qadam.kz:3000":  "kazutb.qadam.kz",
		"kazutb.qadam.kz.":      "kazutb.qadam.kz",
		"[::1]:8080":            "::1",
		" localhost:8080 ":      "localhost",
		"innovation.kazutb.kz.": "innovation.kazutb.kz",
	} {
		if got := normalizeHost(in); got != want {
			t.Errorf("normalizeHost(%q) = %q, want %q", in, got, want)
		}
	}
}
