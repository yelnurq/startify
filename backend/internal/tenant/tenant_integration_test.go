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
	k, err := seed.Tenant(ctx, database.Worker, spec)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := seed.Tenant(ctx, database.Worker, seed.Sandbox); err != nil {
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
		// The origin of links in letters never repeats the port or spelling the client sent.
		{"INNOVATION.kazutb.kz:8443", "kazutb", "https://innovation.kazutb.kz"},
		{"sandbox.qadam.kz", "sandbox", "https://sandbox.qadam.kz"},
		{"Sandbox.Qadam.kz.:444", "sandbox", "https://sandbox.qadam.kz"},
		{"qadam.kz", "kazutb", "https://qadam.kz"},
		{"localhost:8080", "kazutb", "https://qadam.kz"},
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
	// Unknown hosts (a forged Host header sent straight to the API) belong to
	// no university, not to the default one.
	for _, host := range []string{"evil.example", "a.b.qadam.kz", "qadam.kz.evil.example", ".qadam.kz", ""} {
		if got, ok, err := svc.Resolve(ctx, host); err != nil || ok {
			t.Errorf("%q resolved to %+v (%v)", host, got, err)
		}
	}

	// The public URL's port is part of the addresses of every university.
	dev, _ := tenant.NewService(database.App, config.Tenant{BaseDomain: "localhost", Default: "kazutb"}, "http://localhost:3000")
	for host, origin := range map[string]string{
		"localhost:3000":         "http://localhost:3000",
		"kazutb.localhost:9999":  "http://kazutb.localhost:3000",
		"sandbox.localhost:3000": "http://sandbox.localhost:3000",
	} {
		if got, ok, _ := dev.Resolve(ctx, host); !ok || got.Origin != origin {
			t.Errorf("%s: origin %q, want %q", host, got.Origin, origin)
		}
	}

	database.Exec(t, "UPDATE tenants SET status = 'suspended' WHERE slug = 'sandbox'")
	fresh, _ := tenant.NewService(database.App, config.Tenant{BaseDomain: "qadam.kz"}, "https://qadam.kz")
	if _, ok, _ := fresh.Resolve(ctx, "sandbox.qadam.kz"); ok {
		t.Fatal("suspended tenant resolved")
	}
	if _, ok, _ := fresh.Resolve(ctx, "qadam.kz"); ok {
		t.Fatal("no default tenant configured, yet resolved")
	}
	brief, err := svc.Brief(ctx, k.ID)
	if err != nil || brief.Slug != "kazutb" || *brief.ShortName.Kk != "ҚазТБУ" {
		t.Fatalf("brief: %+v %v", brief, err)
	}
}
