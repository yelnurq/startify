package migrate_test

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/yelnurq/startify/backend/internal/platform/migrate"
	"github.com/yelnurq/startify/backend/internal/platform/testpg"
)

var pg *testpg.Server

func TestMain(m *testing.M) { testpg.Main(m, &pg) }

// Every migration must roll back cleanly and apply again.
func TestMigrationsRoundTrip(t *testing.T) {
	d := pg.NewDB(t)
	ctx := context.Background()
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))

	var before bytes.Buffer
	if err := migrate.Status(ctx, d.OwnerURL, &before); err != nil {
		t.Fatal(err)
	}
	applied := strings.Count(before.String(), "applied")
	if applied == 0 || strings.Contains(before.String(), "pending") {
		t.Fatalf("template should be fully migrated:\n%s", before.String())
	}

	for range applied {
		if err := migrate.Down(ctx, d.OwnerURL, quiet); err != nil {
			t.Fatalf("down: %v", err)
		}
	}
	if err := migrate.Up(ctx, d.OwnerURL, quiet); err != nil {
		t.Fatalf("up after full rollback: %v", err)
	}

	var after bytes.Buffer
	if err := migrate.Status(ctx, d.OwnerURL, &after); err != nil {
		t.Fatal(err)
	}
	if strings.Count(after.String(), "applied") != applied {
		t.Fatalf("status after round trip:\n%s", after.String())
	}
}
