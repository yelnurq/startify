package seed

import (
	"context"
	"log/slog"
	"regexp"
	"testing"

	"github.com/yelnurq/startify/backend/internal/auth"
	"github.com/yelnurq/startify/backend/internal/files"
	"github.com/yelnurq/startify/backend/internal/platform/config"
	"github.com/yelnurq/startify/backend/internal/platform/db"
	"github.com/yelnurq/startify/backend/internal/tenant/tenantdb"
)

func TestBootstrapPreparesARealInstallation(t *testing.T) {
	database := pg.NewDB(t)
	ctx := context.Background()
	store := files.NewService(database.App, files.NewMemoryStorage(), config.Files{Scan: files.ScanOff, MaxBytes: 1 << 20}, slog.Default())
	in := Installation{
		Domain: "qadam.kaztbu.edu.kz", SelfSignup: true, EmailDomains: []string{"kaztbu.edu.kz"},
		AdminEmail: " Admin@KazTBU.edu.kz ", AdminName: "Ерлан Сейтказиев", AdminLocale: "ru",
	}
	first, err := Bootstrap(ctx, database.Worker, store, in)
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^[a-zA-Z2-9]{5}(-[a-zA-Z2-9]{5}){3}$`).MatchString(first) {
		t.Fatalf("generated password %q", first)
	}

	q := tenantdb.New(database.App.Conn(ctx))
	kazutb, err := q.GetTenantBySlug(ctx, KazUTB.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if kazutb.LogoFileID == nil || kazutb.CoverFileID == nil {
		t.Fatal("the seal and the campus photo are missing")
	}
	var domain string
	var signup bool
	var mailDomains []string
	err = database.App.ReadTx(db.WithTenant(ctx, kazutb.ID), func(ctx context.Context) error {
		conn := database.App.Conn(ctx)
		if err := conn.QueryRow(ctx, "SELECT domain FROM tenant_domains WHERE tenant_id = $1 AND is_primary", kazutb.ID).Scan(&domain); err != nil {
			return err
		}
		return conn.QueryRow(ctx, "SELECT self_signup, email_domains FROM tenant_settings WHERE tenant_id = $1", kazutb.ID).
			Scan(&signup, &mailDomains)
	})
	if err != nil {
		t.Fatal(err)
	}
	if domain != "qadam.kaztbu.edu.kz" || !signup || len(mailDomains) != 1 || mailDomains[0] != "kaztbu.edu.kz" {
		t.Fatalf("settings: domain %q, sign-up %v, mail domains %v", domain, signup, mailDomains)
	}

	admin := func() (verified bool, roles []string, hash string) {
		t.Helper()
		err := database.App.ReadTx(db.WithTenant(ctx, kazutb.ID), func(ctx context.Context) error {
			return database.App.Conn(ctx).QueryRow(ctx, `
				SELECT u.email_verified_at IS NOT NULL, m.roles, i.password_hash
				FROM users u
				JOIN memberships m ON m.user_id = u.id AND m.tenant_id = $1
				JOIN user_identities i ON i.user_id = u.id AND i.provider = 'password'
				WHERE u.email = 'admin@kaztbu.edu.kz'`, kazutb.ID).Scan(&verified, &roles, &hash)
		})
		if err != nil {
			t.Fatal(err)
		}
		return verified, roles, hash
	}
	verified, roles, hash := admin()
	if !verified || len(roles) != 1 || roles[0] != "tenant_admin" {
		t.Fatalf("administrator: verified %v, roles %v", verified, roles)
	}
	if ok, err := auth.VerifyPassword(hash, first); err != nil || !ok {
		t.Fatalf("the printed password does not sign in: %v", err)
	}

	// Running it again recovers a lost password: a new one, the old one stops working.
	second, err := Bootstrap(ctx, database.Worker, store, in)
	if err != nil {
		t.Fatal(err)
	}
	_, _, hash = admin()
	if second == first {
		t.Fatal("the same password twice")
	}
	if ok, _ := auth.VerifyPassword(hash, first); ok {
		t.Fatal("the old password still signs in")
	}
	if ok, err := auth.VerifyPassword(hash, second); err != nil || !ok {
		t.Fatal("the new password does not sign in")
	}
	c := counter{t: t, db: database.App}
	if n := c.rows(db.WithTenant(ctx, kazutb.ID), "memberships"); n != 1 {
		t.Fatalf("%d memberships after two runs, want the administrator only", n)
	}
	if n := c.rows(db.WithTenant(ctx, kazutb.ID), "projects"); n != 0 {
		t.Fatalf("%d projects: a real installation starts without demo data", n)
	}
}

func TestBootstrapRefusesBadInput(t *testing.T) {
	database := pg.NewDB(t)
	ctx := context.Background()
	store := files.NewService(database.App, files.NewMemoryStorage(), config.Files{Scan: files.ScanOff, MaxBytes: 1 << 20}, slog.Default())
	good := Installation{Domain: "qadam.example.kz", AdminEmail: "admin@example.kz", AdminName: "Admin Adminov"}
	for name, mutate := range map[string]func(*Installation){
		"no e-mail":       func(in *Installation) { in.AdminEmail = "" },
		"bad e-mail":      func(in *Installation) { in.AdminEmail = "admin" },
		"no name":         func(in *Installation) { in.AdminName = " " },
		"url, not a host": func(in *Installation) { in.Domain = "https://qadam.example.kz" },
		"weak password":   func(in *Installation) { in.AdminPassword = "12345678" },
	} {
		in := good
		mutate(&in)
		if _, err := Bootstrap(ctx, database.Worker, store, in); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestGeneratedPasswordsPassThePolicy(t *testing.T) {
	seen := map[string]bool{}
	for range 200 {
		p := GeneratePassword()
		if err := auth.CheckPassword(p, "admin@example.kz", "Admin Adminov"); err != nil {
			t.Fatalf("%q: %v", p, err)
		}
		if seen[p] {
			t.Fatalf("%q twice", p)
		}
		seen[p] = true
	}
}
