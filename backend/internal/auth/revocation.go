package auth

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/yelnurq/startify/backend/internal/auth/authdb"
	"github.com/yelnurq/startify/backend/internal/platform/db"
)

// Revocations ends access before access tokens expire. A revoked session is
// put on a deny-list for the lifetime of its last access token; a change of
// roles bumps a version that every access token must match (D-022).
type Revocations struct {
	cache     *redis.Client
	db        *db.DB
	accessTTL time.Duration
}

// NewRevocations returns the revocation store.
func NewRevocations(cache *redis.Client, database *db.DB, accessTTL time.Duration) *Revocations {
	return &Revocations{cache: cache, db: database, accessTTL: accessTTL}
}

func deniedKey(sid uuid.UUID) string { return "auth:revoked:" + sid.String() }

func roleKey(tenantID, userID uuid.UUID) string {
	return "auth:rv:" + tenantID.String() + ":" + userID.String()
}

// current reports whether the session of the claims is live and its roles current.
func (r *Revocations) current(ctx context.Context, c Claims, userID uuid.UUID) (bool, error) {
	vals, err := r.cache.MGet(ctx, deniedKey(c.SessionID), roleKey(c.TenantID, userID)).Result()
	if err != nil {
		return false, fmt.Errorf("auth: revocation check: %w", err)
	}
	if vals[0] != nil {
		return false, nil
	}
	var version int64
	if s, ok := vals[1].(string); ok {
		version, _ = strconv.ParseInt(s, 10, 64)
	}
	return version == c.RoleVersion, nil
}

// roleVersion returns the current role version of a member.
func (r *Revocations) roleVersion(ctx context.Context, tenantID, userID uuid.UUID) (int64, error) {
	v, err := r.cache.Get(ctx, roleKey(tenantID, userID)).Int64()
	if errors.Is(err, redis.Nil) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("auth: role version: %w", err)
	}
	return v, nil
}

// deny puts sessions on the deny-list.
func (r *Revocations) deny(ctx context.Context, sids ...uuid.UUID) error {
	if len(sids) == 0 {
		return nil
	}
	pipe := r.cache.Pipeline()
	for _, sid := range sids {
		pipe.Set(ctx, deniedKey(sid), "1", r.accessTTL+time.Minute)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("auth: deny sessions: %w", err)
	}
	return nil
}

// RolesChanged implements user.AccessRevoker.
func (r *Revocations) RolesChanged(ctx context.Context, tenantID, userID uuid.UUID) error {
	if err := r.cache.Incr(ctx, roleKey(tenantID, userID)).Err(); err != nil {
		return fmt.Errorf("auth: bump role version: %w", err)
	}
	return nil
}

// RevokeAll implements user.AccessRevoker.
func (r *Revocations) RevokeAll(ctx context.Context, _, userID uuid.UUID, reason string) error {
	var sids []uuid.UUID
	err := r.db.Tx(ctx, func(ctx context.Context) error {
		var err error
		sids, err = authdb.New(r.db.Conn(ctx)).RevokeUserSessions(ctx, authdb.RevokeUserSessionsParams{Reason: &reason, UserID: userID})
		return err
	})
	if err != nil {
		return fmt.Errorf("auth: revoke sessions: %w", err)
	}
	return r.deny(ctx, sids...)
}
