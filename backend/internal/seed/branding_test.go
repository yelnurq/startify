package seed

import (
	"context"
	"log/slog"
	"testing"

	"github.com/yelnurq/startify/backend/internal/files"
	"github.com/yelnurq/startify/backend/internal/platform/config"
	"github.com/yelnurq/startify/backend/internal/platform/db"
	"github.com/yelnurq/startify/backend/internal/tenant/tenantdb"
)

func TestBrandingStoresTheSealAndThePhotoOnce(t *testing.T) {
	database := pg.NewDB(t)
	ctx := context.Background()
	if err := Demo(ctx, database.App); err != nil {
		t.Fatal(err)
	}
	storage := files.NewMemoryStorage()
	store := files.NewService(database.App, storage, config.Files{Scan: files.ScanOff, MaxBytes: 1 << 20}, slog.Default())
	for range 2 {
		if err := Branding(ctx, database.App, store); err != nil {
			t.Fatal(err)
		}
	}

	q := tenantdb.New(database.App.Conn(ctx))
	kazutb, err := q.GetTenantBySlug(ctx, KazUTB.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if kazutb.LogoFileID == nil || kazutb.CoverFileID == nil {
		t.Fatalf("branding not set: %+v", kazutb)
	}
	sandbox, err := q.GetTenantBySlug(ctx, Sandbox.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if sandbox.LogoFileID != nil || sandbox.CoverFileID != nil {
		t.Fatal("the second university got KazUTB's branding")
	}
	c := counter{t: t, db: database.App}
	if n := c.rows(db.WithTenant(ctx, kazutb.ID), "files"); n != 2 {
		t.Fatalf("%d files after two runs, want the seal and the photo", n)
	}
	for _, id := range []string{kazutb.LogoFileID.String(), kazutb.CoverFileID.String()} {
		var visibility string
		err := database.App.ReadTx(db.WithTenant(ctx, kazutb.ID), func(ctx context.Context) error {
			return database.App.Conn(ctx).QueryRow(ctx, "SELECT visibility FROM files WHERE id = $1", id).Scan(&visibility)
		})
		if err != nil || visibility != files.VisibilityPublic {
			t.Fatalf("file %s: %q %v", id, visibility, err)
		}
	}
}
