package migrate_test

import (
	"context"
	"io"
	"log/slog"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/yelnurq/startify/backend/internal/platform/migrate"
)

// strict is the condition of a university table: its own rows, or every row
// in the system scope (which only the worker's role can enter).
const strict = "((tenant_id = app_current_tenant()) OR app_is_system())"

// sharedReads are the tables whose rows other universities may read, with
// the whole condition of their SELECT policy: public files (avatars, covers)
// and published public pages (the national showcase) on top of the strict
// one; the registry of domains for everyone, since the university is
// resolved from the request host before any scope exists.
var sharedReads = map[string]string{
	"files":            "((tenant_id = app_current_tenant()) OR app_is_system() OR ((visibility = 'public'::text) AND (deleted_at IS NULL)))",
	"showcase_entries": "((tenant_id = app_current_tenant()) OR app_is_system() OR ((status = 'published'::text) AND (visibility = 'public'::text)))",
	"tenant_domains":   "true",
	"tenants":          "true",
}

// registries are written only in the system scope.
var registries = map[string]bool{"tenants": true, "tenant_domains": true}

// globalTables have no tenant_id. A new table either has tenant_id (and is
// checked below) or is added here with the reason it needs none.
var globalTables = map[string]string{
	"tenants":          "the registry of universities: everyone reads it, only the system scope writes it",
	"users":            "accounts are shared by their universities (D-007); services show them only through memberships",
	"user_identities":  "the sign-in identities of accounts (D-007)",
	"user_profiles":    "the profiles of accounts (D-007)",
	"idempotency_keys": "stored responses keyed by university and member (scope)",
	"goose_db_version": "the migrations applied",
}

type policy struct {
	table, name, cmd string
	permissive       string
	roles            []string
	qual, check      *string
}

// TestEveryTenantTableIsIsolated reads the catalog: every table with a
// tenant_id column forces row-level security and has, for each command,
// exactly one policy with the strict condition; only the tables listed above
// share reads, and the registries take writes only from the system scope
// (audit С-22, Н-37, Н-39).
func TestEveryTenantTableIsIsolated(t *testing.T) {
	d := pg.NewDB(t)
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, d.OwnerURL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close(ctx) }()

	type table struct {
		name                 string
		rls, forced, tenancy bool
	}
	rows, err := conn.Query(ctx, `
SELECT c.relname::text, c.relrowsecurity, c.relforcerowsecurity,
       EXISTS (SELECT 1 FROM pg_attribute a WHERE a.attrelid = c.oid AND a.attname = 'tenant_id' AND NOT a.attisdropped)
  FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
 WHERE n.nspname = 'public' AND c.relkind IN ('r', 'p')
 ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	tables, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (table, error) {
		var tb table
		return tb, r.Scan(&tb.name, &tb.rls, &tb.forced, &tb.tenancy)
	})
	if err != nil {
		t.Fatal(err)
	}
	rows, err = conn.Query(ctx, `
SELECT tablename::text, policyname::text, cmd, permissive, roles::text[], qual, with_check
  FROM pg_policies WHERE schemaname = 'public'`)
	if err != nil {
		t.Fatal(err)
	}
	policies, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (policy, error) {
		var p policy
		return p, r.Scan(&p.table, &p.name, &p.cmd, &p.permissive, &p.roles, &p.qual, &p.check)
	})
	if err != nil {
		t.Fatal(err)
	}

	isolated := 0
	for _, tb := range tables {
		_, global := globalTables[tb.name]
		switch {
		case tb.tenancy:
			isolated++
		case strings.HasPrefix(tb.name, "river_"):
			// River's job queue: the worker reads the jobs of every university.
			continue
		case !global:
			t.Errorf("%s has no tenant_id and is not a known global table", tb.name)
			continue
		case !registries[tb.name]:
			continue
		}
		if !tb.rls || !tb.forced {
			t.Errorf("%s: row-level security enabled %v, forced %v", tb.name, tb.rls, tb.forced)
		}
		var own []policy
		for _, p := range policies {
			if p.table == tb.name {
				own = append(own, p)
			}
		}
		checkPolicies(t, tb.name, own)
	}
	if isolated < 30 {
		t.Fatalf("only %d tables with tenant_id", isolated)
	}
}

// checkPolicies checks the policies of one table, command by command.
func checkPolicies(t *testing.T, table string, policies []policy) {
	t.Helper()
	read, write := strict, strict
	if shared, ok := sharedReads[table]; ok {
		read = shared
	}
	if registries[table] {
		write = "app_is_system()"
	}
	for _, p := range policies {
		if p.permissive != "PERMISSIVE" || !slices.Equal(p.roles, []string{"public"}) {
			t.Errorf("%s.%s: %s for %v, want a permissive policy for everyone", table, p.name, p.permissive, p.roles)
		}
	}
	for _, cmd := range []string{"SELECT", "INSERT", "UPDATE", "DELETE"} {
		var applies []policy
		for _, p := range policies {
			if p.cmd == cmd || p.cmd == "ALL" {
				applies = append(applies, p)
			}
		}
		if len(applies) != 1 {
			t.Errorf("%s: %d policies for %s, want exactly one", table, len(applies), cmd)
			continue
		}
		p := applies[0]
		// FOR ALL applies its USING to reads and writes alike: a widened
		// read there would let other universities change or delete rows.
		using, check := p.qual, p.check
		if p.cmd == "ALL" && check == nil {
			check = using
		}
		switch cmd {
		case "SELECT":
			expect(t, table, p.name, cmd, "USING", using, read)
		case "INSERT":
			expect(t, table, p.name, cmd, "WITH CHECK", check, write)
		case "UPDATE":
			expect(t, table, p.name, cmd, "USING", using, write)
			expect(t, table, p.name, cmd, "WITH CHECK", check, write)
		case "DELETE":
			expect(t, table, p.name, cmd, "USING", using, write)
		}
	}
}

func expect(t *testing.T, table, name, cmd, clause string, got *string, want string) {
	t.Helper()
	if got == nil || *got != want {
		g := "<none>"
		if got != nil {
			g = *got
		}
		t.Errorf("%s.%s: %s %s is %s, want %s", table, name, cmd, clause, g, want)
	}
}

// Migrations run in the system scope, so one that changes data sees the rows
// of every university even when its owner is not a superuser and the tables
// force row-level security on it. The Down of 00007 deletes the campus
// photos before it narrows the purposes of files: without the system scope
// it deleted nothing and then failed (audit Н-42). The whole set of
// migrations must also work for such an owner, the usual on-premises setup.
func TestMigrationsWorkForAnOwnerWithoutSuperuser(t *testing.T) {
	ctx := context.Background()
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	suffix := strings.ReplaceAll(uuid.Must(uuid.NewV7()).String()[24:], "-", "")
	owner, name := "owner_"+suffix, "plain_"+suffix

	admin := func(database string, statements ...string) {
		t.Helper()
		conn, err := pgx.Connect(ctx, pg.Admin(database))
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = conn.Close(ctx) }()
		for _, s := range statements {
			if _, err := conn.Exec(ctx, s); err != nil {
				t.Fatalf("%s: %v", s, err)
			}
		}
	}
	// The DBA creates the roles (as deploy/postgres/roles.sh does), the
	// owner and its database; extensions need a superuser.
	admin("postgres",
		"CREATE ROLE "+owner+" LOGIN PASSWORD 'plain-owner'",
		"GRANT qadam_worker TO "+owner,
		"CREATE DATABASE "+name+" OWNER "+owner)
	t.Cleanup(func() {
		conn, err := pgx.Connect(context.Background(), pg.Admin("postgres"))
		if err != nil {
			return
		}
		defer func() { _ = conn.Close(context.Background()) }()
		_, _ = conn.Exec(context.Background(), "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
		_, _ = conn.Exec(context.Background(), "DROP ROLE IF EXISTS "+owner)
	})
	admin(name, "CREATE EXTENSION pgcrypto", "CREATE EXTENSION citext", "CREATE EXTENSION pg_trgm", "CREATE EXTENSION vector")

	u, err := url.Parse(pg.Admin(name))
	if err != nil {
		t.Fatal(err)
	}
	u.User = url.UserPassword(owner, "plain-owner")
	ownerURL := u.String()
	if err := migrate.Up(ctx, ownerURL, quiet); err != nil {
		t.Fatalf("up as a plain owner: %v", err)
	}

	// A university with its campus photo (00007) and the photo it replaced,
	// deleted and waiting for the purge: no university's scope sees that one.
	tenant := uuid.Must(uuid.NewV7())
	admin(name,
		"INSERT INTO tenants (id, slug, name, short_name) VALUES ('"+tenant.String()+"', 'plain', '{}', '{}')",
		`INSERT INTO files (tenant_id, purpose, visibility, object_key, file_name, content_type, size_bytes, sha256, scan_status, deleted_at)
		 VALUES ('`+tenant.String()+`', 'tenant_cover', 'public', 'plain/cover', 'cover.jpg', 'image/jpeg', 1, 'x', 'clean', NULL),
		        ('`+tenant.String()+`', 'tenant_cover', 'public', 'plain/old-cover', 'cover.jpg', 'image/jpeg', 1, 'x', 'clean', now())`)
	for {
		var version int64
		conn, err := pgx.Connect(ctx, pg.Admin(name))
		if err != nil {
			t.Fatal(err)
		}
		err = conn.QueryRow(ctx, "SELECT max(version_id) FROM goose_db_version WHERE is_applied").Scan(&version)
		_ = conn.Close(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if version < 7 {
			break
		}
		if err := migrate.Down(ctx, ownerURL, quiet); err != nil {
			t.Fatalf("down from %d as a plain owner: %v", version, err)
		}
	}
}
