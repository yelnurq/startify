package db_test

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yelnurq/startify/backend/internal/platform/config"
	"github.com/yelnurq/startify/backend/internal/platform/db"
	"github.com/yelnurq/startify/backend/internal/platform/testpg"
)

var pg *testpg.Server

func TestMain(m *testing.M) { testpg.Main(m, &pg) }

// rlsFixture creates a tenant-scoped table protected by the same policy shape
// every business table uses, with one row per tenant.
func rlsFixture(t *testing.T) (*testpg.Database, uuid.UUID, uuid.UUID) {
	t.Helper()
	d := pg.NewDB(t)
	d.Exec(t,
		`CREATE TABLE notes (
		   id uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
		   tenant_id uuid NOT NULL,
		   body text NOT NULL)`,
		`ALTER TABLE notes ENABLE ROW LEVEL SECURITY`,
		`CREATE POLICY tenant_isolation ON notes
		   USING (tenant_id = app_current_tenant() OR app_is_system())
		   WITH CHECK (tenant_id = app_current_tenant() OR app_is_system())`,
	)
	a, b := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	d.Exec(t,
		`INSERT INTO notes (tenant_id, body) VALUES ('`+a.String()+`', 'a-1'), ('`+a.String()+`', 'a-2'), ('`+b.String()+`', 'b-1')`,
	)
	return d, a, b
}

func countNotes(ctx context.Context, t *testing.T, database *db.DB) int {
	t.Helper()
	var n int
	err := database.ReadTx(ctx, func(ctx context.Context) error {
		return database.Conn(ctx).QueryRow(ctx, "SELECT count(*) FROM notes").Scan(&n)
	})
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	return n
}

func TestRowLevelSecurityIsolatesTenants(t *testing.T) {
	d, tenantA, tenantB := rlsFixture(t)
	ctx := context.Background()

	if n := countNotes(db.WithTenant(ctx, tenantA), t, d.App); n != 2 {
		t.Errorf("tenant A sees %d notes, want 2", n)
	}
	if n := countNotes(db.WithTenant(ctx, tenantB), t, d.App); n != 1 {
		t.Errorf("tenant B sees %d notes, want 1", n)
	}
	if n := countNotes(ctx, t, d.App); n != 0 {
		t.Errorf("unscoped transaction sees %d notes, want 0 (fail closed)", n)
	}
	if n := countNotes(db.WithSystem(ctx), t, d.Worker); n != 3 {
		t.Errorf("system scope of the worker sees %d notes, want 3", n)
	}
	// The API's role cannot enter the system scope, however it sets
	// app.system: through the context, or a statement of its own (an
	// injected one) in a university's transaction.
	if n := countNotes(db.WithSystem(ctx), t, d.App); n != 0 {
		t.Errorf("system scope of the API sees %d notes, want 0", n)
	}
	var injected int
	err := d.App.ReadTx(db.WithTenant(ctx, tenantB), func(ctx context.Context) error {
		if _, err := d.App.Conn(ctx).Exec(ctx, "SELECT set_config('app.system', 'on', true)"); err != nil {
			return err
		}
		return d.App.Conn(ctx).QueryRow(ctx, "SELECT count(*) FROM notes").Scan(&injected)
	})
	if err != nil || injected != 1 {
		t.Errorf("the API with app.system=on sees %d notes (err %v), want its own 1", injected, err)
	}

	// Outside any transaction the pool has no scope at all.
	var n int
	if err := d.App.Conn(ctx).QueryRow(ctx, "SELECT count(*) FROM notes").Scan(&n); err != nil || n != 0 {
		t.Errorf("pool query sees %d notes (err %v), want 0", n, err)
	}
}

func TestRowLevelSecurityRejectsForeignWrites(t *testing.T) {
	d, tenantA, tenantB := rlsFixture(t)
	ctx := db.WithTenant(context.Background(), tenantA)

	err := d.App.Tx(ctx, func(ctx context.Context) error {
		_, err := d.App.Conn(ctx).Exec(ctx, "INSERT INTO notes (tenant_id, body) VALUES ($1, 'smuggled')", tenantB)
		return err
	})
	if err == nil {
		t.Fatal("tenant A must not insert rows of tenant B")
	}

	var updated int64
	err = d.App.Tx(ctx, func(ctx context.Context) error {
		tag, err := d.App.Conn(ctx).Exec(ctx, "UPDATE notes SET body = 'hijacked' WHERE tenant_id = $1", tenantB)
		updated = tag.RowsAffected()
		return err
	})
	if err != nil || updated != 0 {
		t.Fatalf("tenant A updated %d rows of tenant B (err %v)", updated, err)
	}
}

func TestNestedTxUsesSavepointsAndRestoresScope(t *testing.T) {
	d, tenantA, _ := rlsFixture(t)
	ctx := db.WithTenant(context.Background(), tenantA)
	errInner := errors.New("inner failed")

	err := d.App.Tx(ctx, func(ctx context.Context) error {
		if _, err := d.App.Conn(ctx).Exec(ctx, "INSERT INTO notes (tenant_id, body) VALUES ($1, 'kept')", tenantA); err != nil {
			return err
		}
		innerErr := d.App.Tx(ctx, func(ctx context.Context) error {
			if _, err := d.App.Conn(ctx).Exec(ctx, "INSERT INTO notes (tenant_id, body) VALUES ($1, 'rolled back')", tenantA); err != nil {
				return err
			}
			return errInner
		})
		if !errors.Is(innerErr, errInner) {
			t.Errorf("inner error = %v", innerErr)
		}

		// A nested block of another scope must not leak it into the outer
		// transaction. (The system scope needs the worker's pool; for the
		// API's role a nested scope of no university shows the same.)
		var none int
		if err := d.App.Tx(db.WithTenant(context.Background(), uuid.Nil), func(ctx context.Context) error {
			return d.App.Conn(ctx).QueryRow(ctx, "SELECT count(*) FROM notes").Scan(&none)
		}); err != nil {
			return err
		}
		if none != 0 {
			t.Errorf("a block without a university sees %d notes, want 0", none)
		}
		var mine int
		if err := d.App.Conn(ctx).QueryRow(ctx, "SELECT count(*) FROM notes").Scan(&mine); err != nil {
			return err
		}
		if mine != 3 {
			t.Errorf("after the nested block tenant A sees %d notes, want 3", mine)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("outer tx: %v", err)
	}

	var bodies []string
	_ = d.App.ReadTx(ctx, func(ctx context.Context) error {
		rows, _ := d.App.Conn(ctx).Query(ctx, "SELECT body FROM notes ORDER BY id")
		bodies, err = pgx.CollectRows(rows, pgx.RowTo[string])
		return err
	})
	if len(bodies) != 3 || bodies[2] != "kept" {
		t.Fatalf("bodies = %v, want a-1 a-2 kept", bodies)
	}
}

func TestUUIDv7IsTimeOrdered(t *testing.T) {
	d := pg.NewDB(t)
	ctx := context.Background()
	var ids []uuid.UUID
	err := d.App.ReadTx(ctx, func(ctx context.Context) error {
		rows, err := d.App.Conn(ctx).Query(ctx, "SELECT uuid_generate_v7() FROM generate_series(1, 50)")
		if err != nil {
			return err
		}
		ids, err = pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	for i, id := range ids {
		if id.Version() != 7 || id.Variant() != uuid.RFC4122 {
			t.Fatalf("id %s: version %d variant %v", id, id.Version(), id.Variant())
		}
		if i > 0 && ids[i-1].Time() > id.Time() {
			t.Fatalf("ids not time ordered: %s before %s", ids[i-1], id)
		}
	}
}

func TestIsUniqueViolation(t *testing.T) {
	d := pg.NewDB(t)
	d.Exec(t, "CREATE TABLE uniq (v text CONSTRAINT uniq_v_key UNIQUE)", "INSERT INTO uniq VALUES ('x')")
	ctx := db.WithSystem(context.Background())
	err := d.App.Tx(ctx, func(ctx context.Context) error {
		_, err := d.App.Conn(ctx).Exec(ctx, "INSERT INTO uniq VALUES ('x')")
		return err
	})
	if !db.IsUniqueViolation(err, "uniq_v_key") || !db.IsUniqueViolation(err, "") || db.IsUniqueViolation(err, "other") {
		t.Fatalf("IsUniqueViolation misclassified %v", err)
	}
}

// connectAs opens a pool to the test database as another login role.
func connectAs(t *testing.T, d *testpg.Database, role string) *db.DB {
	t.Helper()
	u, err := url.Parse(d.OwnerURL)
	if err != nil {
		t.Fatal(err)
	}
	u.User = url.UserPassword(role, "role-check")
	pcfg, err := pgxpool.ParseConfig(u.String())
	if err != nil {
		t.Fatal(err)
	}
	conn, err := db.Open(context.Background(), pcfg)
	if err != nil {
		t.Fatalf("connect as %s: %v", role, err)
	}
	t.Cleanup(conn.Close)
	return conn
}

// The worker's pool restores the outer scope after a nested system block too.
func TestNestedSystemBlockOfTheWorker(t *testing.T) {
	d, tenantA, _ := rlsFixture(t)
	ctx := db.WithTenant(context.Background(), tenantA)
	err := d.Worker.Tx(ctx, func(ctx context.Context) error {
		var all, mine int
		if err := d.Worker.Tx(db.WithSystem(ctx), func(ctx context.Context) error {
			return d.Worker.Conn(ctx).QueryRow(ctx, "SELECT count(*) FROM notes").Scan(&all)
		}); err != nil {
			return err
		}
		if err := d.Worker.Conn(ctx).QueryRow(ctx, "SELECT count(*) FROM notes").Scan(&mine); err != nil {
			return err
		}
		if all != 3 || mine != 2 {
			t.Errorf("system block sees %d notes, then tenant A %d; want 3 and 2", all, mine)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// The API refuses the worker's role and the worker refuses the API's: only
// the worker may enter the system scope (audit Н-38).
func TestConnectChecksTheSystemRole(t *testing.T) {
	d := pg.NewDB(t)
	ctx := context.Background()
	for _, c := range []struct {
		name    string
		connect func(context.Context, config.Database) (*db.DB, error)
		cfg     config.Database
		want    string
	}{
		{"api as qadam_app", db.Connect, config.Database{URL: d.AppURL, MaxConns: 2}, ""},
		{"api as qadam_worker", db.Connect, config.Database{URL: d.WorkerURL, MaxConns: 2}, "may enter the system scope"},
		{"worker as qadam_worker", db.ConnectWorker, config.Database{URL: d.AppURL, WorkerURL: d.WorkerURL, MaxConns: 2}, ""},
		{"worker as qadam_app", db.ConnectWorker, config.Database{URL: d.AppURL, MaxConns: 2}, "QADAM_DATABASE_WORKER_URL"},
	} {
		conn, err := c.connect(ctx, c.cfg)
		switch {
		case c.want == "" && err != nil:
			t.Errorf("%s: %v", c.name, err)
		case c.want != "" && (err == nil || !strings.Contains(err.Error(), c.want)):
			t.Errorf("%s: %v, want %q", c.name, err, c.want)
		}
		if conn != nil {
			conn.Close()
		}
	}
}

func TestRequireRestrictedRole(t *testing.T) {
	d := pg.NewDB(t)
	ctx := context.Background()

	if err := d.App.RequireRestrictedRole(ctx); err != nil {
		t.Fatalf("the application role must pass: %v", err)
	}

	// The API and the worker refuse the schema owner (here a superuser) at start-up.
	_, err := db.Connect(ctx, config.Database{URL: d.OwnerURL, MaxConns: 2})
	if err == nil || !strings.Contains(err.Error(), "is a superuser") || !strings.Contains(err.Error(), "QADAM_DATABASE_MIGRATE_URL") {
		t.Fatalf("superuser: %v", err)
	}

	// Roles belong to the whole server: the names must not clash between runs.
	suffix := strings.ReplaceAll(uuid.Must(uuid.NewV7()).String()[24:], "-", "")
	bypass, owner, member := "bypass_"+suffix, "owner_"+suffix, "member_"+suffix
	d.Exec(t,
		"CREATE ROLE "+bypass+" LOGIN BYPASSRLS PASSWORD 'role-check'",
		"CREATE ROLE "+owner+" LOGIN PASSWORD 'role-check'",
		"CREATE ROLE "+member+" LOGIN PASSWORD 'role-check' IN ROLE "+owner,
		"CREATE TABLE owned_notes (id int)",
		"ALTER TABLE owned_notes OWNER TO "+owner,
	)
	for role, want := range map[string]string{
		bypass: "BYPASSRLS",
		owner:  "owns tables such as public.owned_notes",
		// A member of the owner can SET ROLE to it.
		member: "owns tables such as public.owned_notes",
	} {
		err := connectAs(t, d, role).RequireRestrictedRole(ctx)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v, want %q", role, err, want)
		}
	}
}
