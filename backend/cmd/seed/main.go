// Command seed prepares universities and their data. It connects as the
// worker's role (QADAM_DATABASE_WORKER_URL): universities are created in the
// system scope.
//
//	seed                    # the demo data (development)
//	seed demo               # the same; in production only with SEED_DEMO_PASSWORD
//	seed bootstrap -admin-email … -admin-name …
//	                        # a real installation: KazUTB, its address and first administrator
//	seed clean-e2e          # remove what end-to-end tests left (development only)
//
// Every command is idempotent.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"

	"github.com/yelnurq/startify/backend/internal/auth"
	"github.com/yelnurq/startify/backend/internal/files"
	"github.com/yelnurq/startify/backend/internal/platform/config"
	"github.com/yelnurq/startify/backend/internal/platform/db"
	"github.com/yelnurq/startify/backend/internal/platform/secrets"
	"github.com/yelnurq/startify/backend/internal/seed"
)

// The seed's own variables. They must not start with QADAM_: the
// configuration refuses unknown QADAM_* variables.
const (
	envAdminPassword = "SEED_ADMIN_PASSWORD"
	envDemoPassword  = "SEED_DEMO_PASSWORD" //nolint:gosec // the name of a variable, not a password
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "seed:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	command := "demo"
	if len(args) > 0 {
		command, args = args[0], args[1:]
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	switch command {
	case "demo":
		return demo(ctx, cfg, args)
	case "bootstrap":
		return bootstrap(ctx, cfg, args)
	case "clean-e2e":
		return cleanE2E(ctx, cfg)
	default:
		return fmt.Errorf("unknown command %q (want demo, bootstrap or clean-e2e)", command)
	}
}

func connect(ctx context.Context, cfg *config.Config) (*db.DB, *files.Service, error) {
	database, err := db.ConnectWorker(ctx, cfg.Database)
	if err != nil {
		return nil, nil, err
	}
	storage, err := files.NewStorage(ctx, cfg.S3)
	if err != nil {
		database.Close()
		return nil, nil, err
	}
	return database, files.NewService(database, storage, cfg.Files, slog.Default()), nil
}

// publicHost is the host of the public URL: the university's address.
func publicHost(cfg *config.Config) (string, error) {
	u, err := url.Parse(cfg.App.PublicURL)
	if err != nil || u.Hostname() == "" {
		return "", fmt.Errorf("QADAM_APP_PUBLIC_URL %q has no host", cfg.App.PublicURL)
	}
	return u.Hostname(), nil
}

func bootstrap(ctx context.Context, cfg *config.Config, args []string) error {
	fs := flag.NewFlagSet("bootstrap", flag.ContinueOnError)
	email := fs.String("admin-email", "", "e-mail of the university's first administrator (required)")
	name := fs.String("admin-name", "", "full name of the first administrator (required)")
	locale := fs.String("admin-locale", "ru", "interface language of the administrator: kk, ru or en")
	domain := fs.String("domain", "", "the university's host (default: the host of QADAM_APP_PUBLIC_URL)")
	signup := fs.Bool("self-signup", true, "students create accounts themselves (with a confirmed e-mail)")
	mailDomains := fs.String("email-domains", "", "comma-separated mail domains allowed to sign up, e.g. kaztbu.edu.kz (default: any)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *email == "" || *name == "" {
		fs.Usage()
		return errors.New("bootstrap needs -admin-email and -admin-name")
	}
	if !slices.Contains([]string{"kk", "ru", "en"}, *locale) {
		return fmt.Errorf("-admin-locale must be kk, ru or en, got %q", *locale)
	}
	host := *domain
	if host == "" {
		var err error
		if host, err = publicHost(cfg); err != nil {
			return err
		}
	}
	database, store, err := connect(ctx, cfg)
	if err != nil {
		return err
	}
	defer database.Close()

	password, err := seed.Bootstrap(ctx, database, store, seed.Installation{
		Domain: host, SelfSignup: *signup, EmailDomains: splitList(*mailDomains),
		AdminEmail: *email, AdminName: *name, AdminLocale: *locale,
		// A password typed on the command line stays in the shell history:
		// it is taken from the environment only, and generated otherwise.
		AdminPassword: os.Getenv(envAdminPassword),
	})
	if err != nil {
		return err
	}
	fmt.Printf("KazUTB is ready at %s (host %s).\n", cfg.App.PublicURL, host)
	fmt.Printf("First administrator: %s\n", strings.ToLower(strings.TrimSpace(*email)))
	if os.Getenv(envAdminPassword) == "" {
		fmt.Printf("Password:            %s\n", password)
		fmt.Println("The password is shown once: sign in and change it (profile menu → Security).")
	}
	return nil
}

func demo(ctx context.Context, cfg *config.Config, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("demo takes no arguments, got %q", strings.Join(args, " "))
	}
	opts := seed.DemoOptions{Password: seed.DemoPassword, Sandbox: true}
	if cfg.IsProduction() {
		// A presentation server: the demo accounts must not share the
		// password printed in the repository, and the sandbox university and
		// the demo directory do not exist there.
		password := os.Getenv(envDemoPassword)
		if password == "" || password == seed.DemoPassword {
			return errors.New("demo data in production needs " + envDemoPassword + ": " +
				"a password of your own for every demo account (at least 10 characters)")
		}
		if err := auth.CheckPassword(password, "", ""); err != nil {
			return fmt.Errorf("%s is too weak: %w", envDemoPassword, err)
		}
		host, err := publicHost(cfg)
		if err != nil {
			return err
		}
		opts = seed.DemoOptions{Password: password, Domains: []string{host}}
	}
	database, store, err := connect(ctx, cfg)
	if err != nil {
		return err
	}
	defer database.Close()

	if err := seed.DemoWith(ctx, database, opts); err != nil {
		return err
	}
	if !cfg.IsProduction() {
		keys, err := secrets.NewKeyring(cfg.Auth.SecretKey)
		if err != nil {
			return err
		}
		if err := seed.Directory(ctx, database, keys, seed.DemoDirectoryURL); err != nil {
			return err
		}
	}
	if err := seed.Branding(ctx, database, store); err != nil {
		return err
	}
	shown := opts.Password
	if cfg.IsProduction() {
		shown = "of " + envDemoPassword
	}
	fmt.Println("Demo data is ready. Sign in at", cfg.App.PublicURL, "with any account below, password", shown)
	for _, list := range [][]seed.Member{seed.KazUTBMembers, seed.KazUTBStaff} {
		for i := range list {
			fmt.Printf("  %-26s %v\n", list[i].Email, list[i].Roles)
		}
	}
	fmt.Printf("  and %d more students: <name>.<surname>@qadam.test\n", seed.GeneratedStudents)
	fmt.Printf("KazUTB has %d demo projects, from a first draft of the problem to a finished pitch.\n", len(seed.KazUTBProjects))
	if !cfg.IsProduction() {
		fmt.Println("Directory sign-in points at the demo directory", seed.DemoDirectoryURL,
			"and is off: check it and turn it on at /app/admin/directory.")
	}
	return nil
}

func cleanE2E(ctx context.Context, cfg *config.Config) error {
	if cfg.IsProduction() {
		return errors.New("clean-e2e is for development databases only")
	}
	database, err := db.ConnectWorker(ctx, cfg.Database)
	if err != nil {
		return err
	}
	defer database.Close()
	c, err := seed.CleanE2E(ctx, database)
	if err != nil {
		return err
	}
	fmt.Printf("Removed what end-to-end tests left: %d accounts, %d projects, %d requests for teammates, "+
		"%d contact requests, %d knowledge documents, %d imports; %d files go with the next purge.\n",
		c.Accounts, c.Projects, c.Requests, c.Contacts, c.Documents, c.Imports, c.Files)
	return nil
}

func splitList(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if part = strings.ToLower(strings.TrimSpace(part)); part != "" {
			out = append(out, part)
		}
	}
	return out
}
