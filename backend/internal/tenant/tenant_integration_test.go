package tenant_test

import (
	"context"
	"testing"

	"github.com/yelnurq/startify/backend/internal/platform/config"
	"github.com/yelnurq/startify/backend/internal/platform/testpg"
	"github.com/yelnurq/startify/backend/internal/seed"
	"github.com/yelnurq/startify/backend/internal/tenant"
)

var pg *testpg.Server

func TestMain(m *testing.M) { testpg.Main(m, &pg) }

func TestResolve(t *testing.T) {
	database := pg.NewDB(t)
	ctx := context.Background()
	spec := seed.KazUTB
	spec.Domains = []string{"innovation.kazutb.kz"}
	k, err := seed.Tenant(ctx, database.App, spec)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := seed.Tenant(ctx, database.App, seed.Sandbox); err != nil {
		t.Fatal(err)
	}
	svc, err := tenant.NewService(database.App, config.Tenant{BaseDomain: "qadam.kz", Default: "kazutb"}, "https://qadam.kz")
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		host, slug, origin string
	}{
		{"innovation.kazutb.kz", "kazutb", "https://innovation.kazutb.kz"},
		{"INNOVATION.kazutb.kz:443", "kazutb", "https://INNOVATION.kazutb.kz:443"},
		{"sandbox.qadam.kz", "sandbox", "https://sandbox.qadam.kz"},
		{"qadam.kz", "kazutb", "https://qadam.kz"},
		// Unknown hosts fall back to the default tenant, but links in letters
		// never point at a host the platform does not own.
		{"evil.example", "kazutb", "https://qadam.kz"},
		{"a.b.qadam.kz", "kazutb", "https://qadam.kz"},
	}
	for _, c := range cases {
		got, ok, err := svc.Resolve(ctx, c.host)
		if err != nil || !ok {
			t.Fatalf("%s: %v %v", c.host, ok, err)
		}
		if got.Slug != c.slug || got.Origin != c.origin {
			t.Errorf("%s: got %s %s, want %s %s", c.host, got.Slug, got.Origin, c.slug, c.origin)
		}
	}

	database.Exec(t, "UPDATE tenants SET status = 'suspended' WHERE slug = 'sandbox'")
	fresh, _ := tenant.NewService(database.App, config.Tenant{BaseDomain: "qadam.kz"}, "https://qadam.kz")
	if _, ok, _ := fresh.Resolve(ctx, "sandbox.qadam.kz"); ok {
		t.Fatal("suspended tenant resolved")
	}
	if _, ok, _ := fresh.Resolve(ctx, "unknown.example"); ok {
		t.Fatal("no default tenant configured, yet resolved")
	}
	brief, err := svc.Brief(ctx, k.ID)
	if err != nil || brief.Slug != "kazutb" || *brief.ShortName.Kk != "ҚазТБУ" {
		t.Fatalf("brief: %+v %v", brief, err)
	}
}
