// Package idempotency stores Idempotency-Key outcomes in PostgreSQL.
package idempotency

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/yelnurq/startify/backend/internal/platform/db"
	"github.com/yelnurq/startify/backend/internal/platform/httpx"
	"github.com/yelnurq/startify/backend/internal/platform/platformdb"
)

// Store implements httpx.IdempotencyStore. The table has no row-level security
// (the scope column already namespaces keys by tenant and user).
type Store struct {
	db *db.DB
}

var _ httpx.IdempotencyStore = (*Store)(nil)

// NewStore returns a PostgreSQL-backed store.
func NewStore(database *db.DB) *Store { return &Store{db: database} }

func (s *Store) q(ctx context.Context) *platformdb.Queries { return platformdb.New(s.db.Conn(ctx)) }

// Reserve implements httpx.IdempotencyStore.
func (s *Store) Reserve(ctx context.Context, scope, key, fingerprint string, ttl time.Duration) (httpx.IdempotencyRecord, bool, error) {
	_, err := s.q(ctx).ReserveIdempotencyKey(ctx, platformdb.ReserveIdempotencyKeyParams{
		Scope:       scope,
		Key:         key,
		Fingerprint: fingerprint,
		ExpiresAt:   time.Now().Add(ttl),
	})
	if err == nil {
		return httpx.IdempotencyRecord{}, true, nil
	}
	if !db.IsNoRows(err) {
		return httpx.IdempotencyRecord{}, false, fmt.Errorf("idempotency: reserve: %w", err)
	}

	row, err := s.q(ctx).GetIdempotencyKey(ctx, platformdb.GetIdempotencyKeyParams{Scope: scope, Key: key})
	if err != nil {
		return httpx.IdempotencyRecord{}, false, fmt.Errorf("idempotency: load: %w", err)
	}
	rec := httpx.IdempotencyRecord{
		Fingerprint: row.Fingerprint,
		Completed:   row.CompletedAt != nil,
		Body:        row.ResponseBody,
		Header:      http.Header{},
	}
	if row.StatusCode != nil {
		rec.Status = int(*row.StatusCode)
	}
	if len(row.ResponseHeaders) > 0 {
		if err := json.Unmarshal(row.ResponseHeaders, &rec.Header); err != nil {
			return httpx.IdempotencyRecord{}, false, fmt.Errorf("idempotency: decode headers: %w", err)
		}
	}
	return rec, false, nil
}

// Complete implements httpx.IdempotencyStore.
func (s *Store) Complete(ctx context.Context, scope, key string, rec httpx.IdempotencyRecord) error {
	headers, err := json.Marshal(rec.Header)
	if err != nil {
		return fmt.Errorf("idempotency: encode headers: %w", err)
	}
	status := int32(rec.Status) //nolint:gosec // HTTP status codes fit in int32.
	err = s.q(ctx).CompleteIdempotencyKey(ctx, platformdb.CompleteIdempotencyKeyParams{
		StatusCode:      &status,
		ResponseHeaders: headers,
		ResponseBody:    rec.Body,
		Scope:           scope,
		Key:             key,
	})
	if err != nil {
		return fmt.Errorf("idempotency: complete: %w", err)
	}
	return nil
}

// Release implements httpx.IdempotencyStore.
func (s *Store) Release(ctx context.Context, scope, key string) error {
	if err := s.q(ctx).DeleteIdempotencyKey(ctx, platformdb.DeleteIdempotencyKeyParams{Scope: scope, Key: key}); err != nil {
		return fmt.Errorf("idempotency: release: %w", err)
	}
	return nil
}

// ScopeByClientIP namespaces keys of anonymous requests by client address.
// Authenticated routes replace it with a tenant and user scope.
func ScopeByClientIP(r *http.Request) (string, bool) {
	ip := httpx.ClientIPFrom(r.Context())
	if !ip.IsValid() {
		return "", false
	}
	return "anon:" + ip.String(), true
}
