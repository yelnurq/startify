// Package authz carries the authenticated principal through the request
// context and answers role questions. It is deliberately tiny and dependency
// free so every module can use it.
package authz

import (
	"context"
	"slices"

	"github.com/google/uuid"

	"github.com/yelnurq/startify/backend/internal/platform/apperr"
)

// Role of a member within a university.
type Role string

// Tenant roles.
const (
	RoleStudent           Role = "student"
	RoleSupervisor        Role = "supervisor"
	RoleMentor            Role = "mentor"
	RoleJury              Role = "jury"
	RoleInnovationManager Role = "innovation_manager"
	RoleTenantAdmin       Role = "tenant_admin"
	RoleLeadership        Role = "leadership"
	RolePartner           Role = "partner"
)

// AllRoles lists every tenant role.
var AllRoles = []Role{
	RoleStudent, RoleSupervisor, RoleMentor, RoleJury,
	RoleInnovationManager, RoleTenantAdmin, RoleLeadership, RolePartner,
}

// Valid reports whether r is a known role.
func (r Role) Valid() bool { return slices.Contains(AllRoles, r) }

// Principal is the signed-in user acting within one tenant.
type Principal struct {
	UserID     uuid.UUID
	TenantID   uuid.UUID
	SessionID  uuid.UUID
	Roles      []Role
	SuperAdmin bool
}

// Has reports whether the principal holds any of the roles.
func (p Principal) Has(roles ...Role) bool {
	for _, r := range roles {
		if slices.Contains(p.Roles, r) {
			return true
		}
	}
	return false
}

type principalKey struct{}

// WithPrincipal stores the principal in ctx.
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// FromContext returns the principal, if the request is authenticated.
func FromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok
}

// ErrUnauthenticated is returned when a request needs a signed-in user.
var ErrUnauthenticated = apperr.Unauthorized("auth.unauthenticated", "sign-in required")

// Require returns the principal or ErrUnauthenticated.
func Require(ctx context.Context) (Principal, error) {
	p, ok := FromContext(ctx)
	if !ok {
		return Principal{}, ErrUnauthenticated
	}
	return p, nil
}

// RequireRole returns the principal if it holds one of the roles; super
// administrators pass every role check.
func RequireRole(ctx context.Context, roles ...Role) (Principal, error) {
	p, err := Require(ctx)
	if err != nil {
		return p, err
	}
	if p.SuperAdmin || p.Has(roles...) {
		return p, nil
	}
	return p, apperr.Forbidden("rbac.forbidden", "the action requires another role")
}
