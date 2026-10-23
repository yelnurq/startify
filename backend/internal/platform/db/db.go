// Package db owns the PostgreSQL pool and the transaction discipline that makes
// row-level security the second line of tenant isolation.
//
// Every tenant-scoped table has an RLS policy comparing tenant_id with the
// transaction-local setting app.tenant_id. Tx copies the data scope found in the
// context (WithTenant / WithSystem) into that setting, so a query that forgets a
// tenant filter still cannot read another tenant's rows. Outside a transaction
// no scope is set and RLS hides every tenant row: the failure mode is "empty",
// never "leak".
package db

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/exaring/otelpgx"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yelnurq/startify/backend/internal/platform/config"
)

// Querier is the query interface used by sqlc-generated code. It is satisfied by
// both *pgxpool.Pool and pgx.Tx.
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	CopyFrom(ctx context.Context, table pgx.Identifier, columns []string, src pgx.CopyFromSource) (int64, error)
	SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults
}

// DB wraps the connection pool.
type DB struct {
	pool *pgxpool.Pool
}

// Connect opens the pool of the application role, verifies connectivity and
// refuses a role that row-level security does not bind (RequireRestrictedRole).
func Connect(ctx context.Context, cfg config.Database) (*DB, error) {
	pcfg, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("db: parse url: %w", err)
	}
	pcfg.MaxConns = cfg.MaxConns
	pcfg.MinConns = cfg.MinConns
	pcfg.ConnConfig.Tracer = otelpgx.NewTracer()
	d, err := Open(ctx, pcfg)
	if err != nil {
		return nil, err
	}
	if err := d.RequireRestrictedRole(ctx); err != nil {
		d.Close()
		return nil, err
	}
	return d, nil
}

// restrictedRoleQuery reports why the current role is not bound by row-level
// security. Superusers and BYPASSRLS roles skip every policy; a table owner
// skips the policies of its tables. Membership counts as well: a member of
// such a role can SET ROLE to it.
const restrictedRoleQuery = `
SELECT current_user::text,
       EXISTS (SELECT 1 FROM pg_roles r WHERE r.rolsuper AND pg_has_role(current_user, r.oid, 'MEMBER')),
       EXISTS (SELECT 1 FROM pg_roles r WHERE r.rolbypassrls AND pg_has_role(current_user, r.oid, 'MEMBER')),
       coalesce((SELECT string_agg(t.name, ', ')
                   FROM (SELECT n.nspname || '.' || c.relname AS name
                           FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
                          WHERE c.relkind IN ('r', 'p')
                            AND n.nspname <> 'information_schema' AND n.nspname NOT LIKE 'pg\_%'
                            AND pg_has_role(current_user, c.relowner, 'MEMBER')
                          ORDER BY 1
                          LIMIT 3) t), '')`

// RequireRestrictedRole fails unless the pool's role is subject to row-level
// security: not a superuser, without BYPASSRLS and owning no table, directly
// or through role membership. Without this check, putting the schema owner
// into QADAM_DATABASE_URL "to make it work" would silently switch off tenant
// isolation in the database.
func (d *DB) RequireRestrictedRole(ctx context.Context) error {
	var (
		role              string
		superuser, bypass bool
		owned             string
	)
	if err := d.pool.QueryRow(ctx, restrictedRoleQuery).Scan(&role, &superuser, &bypass, &owned); err != nil {
		return fmt.Errorf("db: check role: %w", err)
	}
	var reasons []string
	switch {
	case superuser:
		// A superuser is a member of every role: it also bypasses and owns.
		reasons = append(reasons, "is a superuser")
	case bypass:
		reasons = append(reasons, "bypasses row-level security (BYPASSRLS)")
	}
	if owned != "" && !superuser {
		reasons = append(reasons, "owns tables such as "+owned)
	}
	if len(reasons) == 0 {
		return nil
	}
	return fmt.Errorf("db: QADAM_DATABASE_URL connects as %q, which %s, so row-level security would not isolate universities; "+
		"connect as the restricted application role (qadam_app) and keep the schema owner in QADAM_DATABASE_MIGRATE_URL",
		role, strings.Join(reasons, " and "))
}

// Open opens a pool from a prepared configuration.
func Open(ctx context.Context, pcfg *pgxpool.Config) (*DB, error) {
	pool, err := pgxpool.NewWithConfig(ctx, pcfg)
	if err != nil {
		return nil, fmt.Errorf("db: open pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: ping: %w", err)
	}
	return &DB{pool: pool}, nil
}

// Pool exposes the underlying pool for infrastructure that needs it (job queue).
func (d *DB) Pool() *pgxpool.Pool { return d.pool }

// Ping checks connectivity.
func (d *DB) Ping(ctx context.Context) error { return d.pool.Ping(ctx) }

// Close closes the pool.
func (d *DB) Close() { d.pool.Close() }

// Scope selects which rows a transaction may see.
type Scope struct {
	TenantID uuid.UUID
	// System grants cross-tenant access for platform operations (super admin,
	// national showcase, background maintenance). Use sparingly.
	System bool
}

type scopeKey struct{}

type txKey struct{}

// WithTenant scopes data access in ctx to one tenant.
func WithTenant(ctx context.Context, tenantID uuid.UUID) context.Context {
	return context.WithValue(ctx, scopeKey{}, Scope{TenantID: tenantID})
}

// WithSystem grants ctx cross-tenant data access.
func WithSystem(ctx context.Context) context.Context {
	s := ScopeFrom(ctx)
	s.System = true
	return context.WithValue(ctx, scopeKey{}, s)
}

// ScopeFrom returns the data scope stored in ctx.
func ScopeFrom(ctx context.Context) Scope {
	s, _ := ctx.Value(scopeKey{}).(Scope)
	return s
}

type scopedTx struct {
	tx    pgx.Tx
	scope Scope
}

// Tx runs fn in a transaction bound to the data scope of ctx. Inside fn, use
// Conn(ctx) for queries. A nested call opens a savepoint in the outer
// transaction, so an inner failure can be handled without aborting it.
func (d *DB) Tx(ctx context.Context, fn func(ctx context.Context) error) error {
	return d.tx(ctx, pgx.TxOptions{}, fn)
}

// ReadTx is Tx with a read-only transaction.
func (d *DB) ReadTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return d.tx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly}, fn)
}

func (d *DB) tx(ctx context.Context, opts pgx.TxOptions, fn func(ctx context.Context) error) error {
	scope := ScopeFrom(ctx)
	if outer, ok := ctx.Value(txKey{}).(scopedTx); ok {
		return pgx.BeginFunc(ctx, outer.tx, func(tx pgx.Tx) error {
			if scope == outer.scope {
				return fn(context.WithValue(ctx, txKey{}, scopedTx{tx: tx, scope: scope}))
			}
			if err := applyScope(ctx, tx, scope); err != nil {
				return err
			}
			err := fn(context.WithValue(ctx, txKey{}, scopedTx{tx: tx, scope: scope}))
			// Settings outlive a released savepoint: restore the outer scope.
			return errors.Join(err, applyScope(ctx, tx, outer.scope))
		})
	}
	return pgx.BeginTxFunc(ctx, d.pool, opts, func(tx pgx.Tx) error {
		if err := applyScope(ctx, tx, scope); err != nil {
			return err
		}
		return fn(context.WithValue(ctx, txKey{}, scopedTx{tx: tx, scope: scope}))
	})
}

func applyScope(ctx context.Context, tx pgx.Tx, s Scope) error {
	tenant := ""
	if s.TenantID != uuid.Nil {
		tenant = s.TenantID.String()
	}
	system := "off"
	if s.System {
		system = "on"
	}
	_, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true), set_config('app.system', $2, true)", tenant, system)
	if err != nil {
		return fmt.Errorf("db: apply scope: %w", err)
	}
	return nil
}

// Conn returns the transaction bound to ctx, or the pool outside a transaction.
func (d *DB) Conn(ctx context.Context) Querier {
	if s, ok := ctx.Value(txKey{}).(scopedTx); ok {
		return s.tx
	}
	return d.pool
}

// IsUniqueViolation reports whether err is a unique constraint violation,
// optionally of the named constraint.
func IsUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		return false
	}
	return constraint == "" || pgErr.ConstraintName == constraint
}

// IsNoRows reports whether err means "no rows in result set".
func IsNoRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }
