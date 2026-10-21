// Package tenancy carries the university resolved from the request host.
package tenancy

import (
	"context"

	"github.com/google/uuid"

	"github.com/yelnurq/startify/backend/internal/platform/apperr"
)

// Tenant is the university a request is addressed to.
type Tenant struct {
	ID     uuid.UUID
	Slug   string
	Locale string
	// Origin is the scheme and host the user sees (https://kazutb.qadam.kz);
	// links in e-mails point there.
	Origin string
}

type tenantKey struct{}

// With stores the tenant in ctx.
func With(ctx context.Context, t Tenant) context.Context {
	return context.WithValue(ctx, tenantKey{}, t)
}

// FromContext returns the tenant of the request.
func FromContext(ctx context.Context) (Tenant, bool) {
	t, ok := ctx.Value(tenantKey{}).(Tenant)
	return t, ok
}

// Require returns the tenant or a not-found error for hosts that belong to none.
func Require(ctx context.Context) (Tenant, error) {
	t, ok := FromContext(ctx)
	if !ok {
		return Tenant{}, apperr.NotFound("tenant.not_found", "no university is served on this address")
	}
	return t, nil
}
