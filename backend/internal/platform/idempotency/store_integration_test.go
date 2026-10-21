package idempotency_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/yelnurq/startify/backend/internal/platform/httpx"
	"github.com/yelnurq/startify/backend/internal/platform/idempotency"
	"github.com/yelnurq/startify/backend/internal/platform/testpg"
)

var pg *testpg.Server

func TestMain(m *testing.M) { testpg.Main(m, &pg) }

func TestStoreLifecycle(t *testing.T) {
	d := pg.NewDB(t)
	store := idempotency.NewStore(d.App)
	ctx := context.Background()

	_, fresh, err := store.Reserve(ctx, "t:u", "key-1", "fp-1", time.Hour)
	if err != nil || !fresh {
		t.Fatalf("first reserve: fresh=%v err=%v", fresh, err)
	}

	rec, fresh, err := store.Reserve(ctx, "t:u", "key-1", "fp-1", time.Hour)
	if err != nil || fresh || rec.Completed || rec.Fingerprint != "fp-1" {
		t.Fatalf("in-flight reserve: %+v fresh=%v err=%v", rec, fresh, err)
	}

	h := http.Header{}
	h.Set("Content-Type", "application/json")
	h.Set("Location", "/api/v1/projects/1")
	if err := store.Complete(ctx, "t:u", "key-1", httpx.IdempotencyRecord{
		Status: http.StatusCreated, Header: h, Body: []byte(`{"id":1}`),
	}); err != nil {
		t.Fatal(err)
	}

	rec, fresh, err = store.Reserve(ctx, "t:u", "key-1", "fp-1", time.Hour)
	if err != nil || fresh || !rec.Completed || rec.Status != http.StatusCreated ||
		string(rec.Body) != `{"id":1}` || rec.Header.Get("Location") != "/api/v1/projects/1" {
		t.Fatalf("completed reserve: %+v fresh=%v err=%v", rec, fresh, err)
	}

	// Another scope never sees the key.
	if _, fresh, _ := store.Reserve(ctx, "t:other", "key-1", "fp-1", time.Hour); !fresh {
		t.Fatal("keys must be namespaced by scope")
	}

	if err := store.Release(ctx, "t:u", "key-1"); err != nil {
		t.Fatal(err)
	}
	if _, fresh, _ := store.Reserve(ctx, "t:u", "key-1", "fp-2", time.Hour); !fresh {
		t.Fatal("released key must be reusable")
	}
}

func TestExpiredKeyIsRecycled(t *testing.T) {
	d := pg.NewDB(t)
	store := idempotency.NewStore(d.App)
	ctx := context.Background()
	if _, fresh, err := store.Reserve(ctx, "t:u", "key-2", "fp-1", -time.Minute); err != nil || !fresh {
		t.Fatalf("reserve: %v", err)
	}
	if _, fresh, err := store.Reserve(ctx, "t:u", "key-2", "fp-2", time.Hour); err != nil || !fresh {
		t.Fatalf("expired key must be recycled: fresh=%v err=%v", fresh, err)
	}
}
