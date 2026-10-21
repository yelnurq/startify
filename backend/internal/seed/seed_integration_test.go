package seed

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/yelnurq/startify/backend/internal/platform/db"
	"github.com/yelnurq/startify/backend/internal/platform/testpg"
	"github.com/yelnurq/startify/backend/internal/tenant/tenantdb"
)

var pg *testpg.Server

func TestMain(m *testing.M) { testpg.Main(m, &pg) }

// tenantTables are the tables isolated by row-level security.
var tenantTables = []string{
	"tenant_settings", "tenant_oidc_providers", "faculties", "departments", "study_groups", "memberships",
	"sessions", "auth_tokens", "files", "audit_log", "import_jobs", "team_requests", "team_responses",
}

// counter counts the rows a data scope can see.
type counter struct {
	t  *testing.T
	db *db.DB
}

func (c counter) rows(ctx context.Context, table string) int {
	c.t.Helper()
	var n int
	err := c.db.ReadTx(ctx, func(ctx context.Context) error {
		return c.db.Conn(ctx).QueryRow(ctx, "SELECT count(*) FROM "+pgx.Identifier{table}.Sanitize()).Scan(&n)
	})
	if err != nil {
		c.t.Fatalf("count %s: %v", table, err)
	}
	return n
}

func TestDemoIsIdempotentAndIsolated(t *testing.T) {
	database := pg.NewDB(t)
	ctx := context.Background()
	if err := Demo(ctx, database.App); err != nil {
		t.Fatal(err)
	}
	if err := Demo(ctx, database.App); err != nil {
		t.Fatalf("second run: %v", err)
	}

	var kazutb, sandbox tenantdb.Tenant
	q := tenantdb.New(database.App.Conn(ctx))
	kazutb, err := q.GetTenantBySlug(ctx, "kazutb")
	if err != nil {
		t.Fatal(err)
	}
	if sandbox, err = q.GetTenantBySlug(ctx, "sandbox"); err != nil {
		t.Fatal(err)
	}

	c := counter{t: t, db: database.App}
	k := db.WithTenant(ctx, kazutb.ID)
	s := db.WithTenant(ctx, sandbox.ID)
	if n := c.rows(k, "memberships"); n != len(AllKazUTBMembers()) {
		t.Fatalf("kazutb members: %d", n)
	}
	if n := c.rows(k, "team_requests"); n != len(KazUTBRequests) {
		t.Fatalf("team requests after two runs: %d", n)
	}
	if n := c.rows(k, "study_groups"); n == 0 {
		t.Fatal("no groups")
	}

	// Every tenant table shows only the rows of the scope; without a scope, none.
	for _, table := range tenantTables {
		all := c.rows(db.WithSystem(ctx), table)
		mine := c.rows(k, table)
		theirs := c.rows(s, table)
		if mine+theirs != all {
			t.Errorf("%s: kazutb %d + sandbox %d != all %d", table, mine, theirs, all)
		}
		if n := c.rows(ctx, table); n != 0 {
			t.Errorf("%s: %d rows visible without a tenant", table, n)
		}
	}
	if c.rows(s, "memberships") != 1 || c.rows(s, "faculties") != 0 {
		t.Fatal("sandbox sees KazUTB data")
	}

	// Rows cannot be written into another tenant either.
	err = database.App.Tx(s, func(ctx context.Context) error {
		_, err := database.App.Conn(ctx).Exec(ctx,
			"INSERT INTO faculties (tenant_id, code, name) VALUES ($1, 'X', '{}')", kazutb.ID)
		return err
	})
	if err == nil {
		t.Fatal("cross-tenant insert succeeded")
	}
}
