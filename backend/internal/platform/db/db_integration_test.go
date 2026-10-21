package db_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

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
	if n := countNotes(db.WithSystem(ctx), t, d.App); n != 3 {
		t.Errorf("system scope sees %d notes, want 3", n)
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

		// A nested system-scope block must not leak its scope into the outer transaction.
		var all int
		if err := d.App.Tx(db.WithSystem(ctx), func(ctx context.Context) error {
			return d.App.Conn(ctx).QueryRow(ctx, "SELECT count(*) FROM notes").Scan(&all)
		}); err != nil {
			return err
		}
		if all != 4 {
			t.Errorf("system block sees %d notes, want 4", all)
		}
		var mine int
		if err := d.App.Conn(ctx).QueryRow(ctx, "SELECT count(*) FROM notes").Scan(&mine); err != nil {
			return err
		}
		if mine != 3 {
			t.Errorf("after the system block tenant A sees %d notes, want 3", mine)
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
