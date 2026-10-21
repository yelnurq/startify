package httpx

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type memStore struct {
	mu   sync.Mutex
	recs map[string]IdempotencyRecord
}

func newMemStore() *memStore { return &memStore{recs: map[string]IdempotencyRecord{}} }

func (s *memStore) Reserve(_ context.Context, scope, key, fp string, _ time.Duration) (IdempotencyRecord, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if rec, ok := s.recs[scope+"|"+key]; ok {
		return rec, false, nil
	}
	s.recs[scope+"|"+key] = IdempotencyRecord{Fingerprint: fp}
	return IdempotencyRecord{}, true, nil
}

func (s *memStore) Complete(_ context.Context, scope, key string, rec IdempotencyRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recs[scope+"|"+key] = rec
	return nil
}

func (s *memStore) Release(_ context.Context, scope, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.recs, scope+"|"+key)
	return nil
}

func fixedScope(*http.Request) (string, bool) { return "tenant-a:user-1", true }

func newIdempotentHandler(store IdempotencyStore, calls *atomic.Int32, status int) http.Handler {
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Location", "/api/v1/things/1")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]int32{"call": n})
	})
	return Idempotency(store, fixedScope, time.Hour)(h)
}

func post(h http.Handler, key, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/things", strings.NewReader(body))
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestIdempotencyReplaysStoredResponse(t *testing.T) {
	var calls atomic.Int32
	h := newIdempotentHandler(newMemStore(), &calls, http.StatusCreated)

	first := post(h, "key-0000001", `{"name":"a"}`)
	second := post(h, "key-0000001", `{"name":"a"}`)

	if calls.Load() != 1 {
		t.Fatalf("handler called %d times, want 1", calls.Load())
	}
	if second.Code != http.StatusCreated || second.Body.String() != first.Body.String() {
		t.Fatalf("replay mismatch: %d %q vs %d %q", second.Code, second.Body, first.Code, first.Body)
	}
	if second.Header().Get(HeaderIdempotentReplayed) != "true" || second.Header().Get("Location") != "/api/v1/things/1" {
		t.Fatalf("replayed headers missing: %v", second.Header())
	}
}

func TestIdempotencyRejectsKeyReuseWithDifferentBody(t *testing.T) {
	var calls atomic.Int32
	h := newIdempotentHandler(newMemStore(), &calls, http.StatusCreated)
	post(h, "key-0000002", `{"name":"a"}`)
	rec := post(h, "key-0000002", `{"name":"b"}`)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "idempotency.key_reused") {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}
}

func TestIdempotencyReleasesKeyAfterServerError(t *testing.T) {
	var calls atomic.Int32
	store := newMemStore()
	h := newIdempotentHandler(store, &calls, http.StatusInternalServerError)
	post(h, "key-0000003", `{}`)
	post(h, "key-0000003", `{}`)
	if calls.Load() != 2 {
		t.Fatalf("after a 5xx the retry must reach the handler; calls = %d", calls.Load())
	}
}

func TestIdempotencyInProgress(t *testing.T) {
	store := newMemStore()
	_, _, _ = store.Reserve(context.Background(), "tenant-a:user-1", "key-0000004", requestFingerprintFor(`{}`), time.Hour)
	var calls atomic.Int32
	rec := post(newIdempotentHandler(store, &calls, http.StatusCreated), "key-0000004", `{}`)
	if rec.Code != http.StatusConflict || calls.Load() != 0 {
		t.Fatalf("got %d, calls %d", rec.Code, calls.Load())
	}
}

func TestIdempotencyValidatesKeyAndSkipsWithoutKey(t *testing.T) {
	var calls atomic.Int32
	h := newIdempotentHandler(newMemStore(), &calls, http.StatusCreated)
	if rec := post(h, "short", `{}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("short key: got %d", rec.Code)
	}
	post(h, "", `{}`)
	post(h, "", `{}`)
	if calls.Load() != 2 {
		t.Fatalf("requests without key must not be deduplicated; calls = %d", calls.Load())
	}
}

func requestFingerprintFor(body string) string {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/things", http.NoBody)
	return requestFingerprint(req, []byte(body))
}
