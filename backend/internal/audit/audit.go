// Package audit records who did what, when and from where. Events are written
// in the caller's transaction, so a rolled-back change leaves no trace.
package audit

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/yelnurq/startify/backend/internal/audit/auditdb"
	"github.com/yelnurq/startify/backend/internal/platform/authz"
	"github.com/yelnurq/startify/backend/internal/platform/db"
	"github.com/yelnurq/startify/backend/internal/platform/httpx"
	"github.com/yelnurq/startify/backend/internal/platform/tenancy"
)

// Event is one auditable action, e.g. "auth.login" or "member.roles_changed".
type Event struct {
	Action     string
	TargetType string
	TargetID   string
	// ActorID overrides the principal of the request (sign-in, where no
	// principal exists yet).
	ActorID  uuid.UUID
	Metadata map[string]any
}

// Logger writes events.
type Logger struct {
	db *db.DB
}

// NewLogger returns a logger on the database.
func NewLogger(database *db.DB) *Logger { return &Logger{db: database} }

// Record stores the event with the tenant, actor, IP and user agent of ctx.
func (l *Logger) Record(ctx context.Context, e Event) error {
	params := auditdb.InsertAuditEventParams{Action: e.Action, Metadata: json.RawMessage("{}")}
	if t, ok := tenancy.FromContext(ctx); ok {
		params.TenantID = &t.ID
	}
	switch {
	case e.ActorID != uuid.Nil:
		params.ActorID = &e.ActorID
	default:
		if p, ok := authz.FromContext(ctx); ok {
			params.ActorID = &p.UserID
		}
	}
	if e.TargetType != "" {
		params.TargetType = &e.TargetType
	}
	if e.TargetID != "" {
		params.TargetID = &e.TargetID
	}
	if ip := httpx.ClientIPFrom(ctx); ip.IsValid() {
		params.Ip = &ip
	}
	if ua := httpx.UserAgentFrom(ctx); ua != "" {
		params.UserAgent = &ua
	}
	if len(e.Metadata) > 0 {
		raw, err := json.Marshal(e.Metadata)
		if err != nil {
			return fmt.Errorf("audit: metadata: %w", err)
		}
		params.Metadata = raw
	}
	if err := auditdb.New(l.db.Conn(ctx)).InsertAuditEvent(ctx, params); err != nil {
		return fmt.Errorf("audit: insert %s: %w", e.Action, err)
	}
	return nil
}
