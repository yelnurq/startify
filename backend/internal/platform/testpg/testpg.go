// Package testpg runs integration tests against a disposable PostgreSQL with
// pgvector. One container is started per test package; every test gets its own
// database cloned from a migrated template, so tests are isolated and fast.
//
// Usage:
//
//	var pg *testpg.Server
//
//	func TestMain(m *testing.M) { testpg.Main(m, &pg) }
//
//	func TestSomething(t *testing.T) {
//		database := pg.NewDB(t) // skips the test in -short mode
//		...
//	}
//
// database.App connects as the API does (qadam_app); database.Worker as the
// worker and the seed do (qadam_worker), the only role that may enter the
// system scope.
package testpg

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/yelnurq/startify/backend/internal/platform/db"
	"github.com/yelnurq/startify/backend/internal/platform/migrate"
)

const (
	image          = "pgvector/pgvector:pg17"
	templateName   = "qadam_template"
	appRole        = "qadam_app"
	appPassword    = "qadam_app"
	workerRole     = "qadam_worker"
	workerPassword = "qadam_worker"
)

// Server is a running PostgreSQL container with a migrated template database.
type Server struct {
	container *postgres.PostgresContainer
	adminURL  *url.URL
	seq       atomic.Int64
}

// Main starts the server (unless -short is set), runs the tests and exits.
func Main(m *testing.M, srv **Server) {
	flag.Parse()
	if testing.Short() {
		os.Exit(m.Run())
	}
	ctx := context.Background()
	s, err := Start(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "testpg:", err)
		os.Exit(1)
	}
	*srv = s
	code := m.Run()
	_ = s.Terminate(ctx)
	os.Exit(code)
}

// Start launches the container and prepares the migrated template.
func Start(ctx context.Context) (*Server, error) {
	c, err := postgres.Run(ctx, image,
		postgres.WithDatabase("postgres"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres"),
		postgres.BasicWaitStrategies(),
		testcontainers.WithCmdArgs("-c", "fsync=off", "-c", "full_page_writes=off", "-c", "max_connections=300"),
	)
	if err != nil {
		return nil, fmt.Errorf("start container: %w", err)
	}
	dsn, err := c.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return nil, err
	}
	admin, err := url.Parse(dsn)
	if err != nil {
		return nil, err
	}
	s := &Server{container: c, adminURL: admin}

	if err := s.exec(ctx, "postgres",
		fmt.Sprintf("CREATE ROLE %s LOGIN PASSWORD '%s'", appRole, appPassword),
		fmt.Sprintf("CREATE ROLE %s LOGIN PASSWORD '%s'", workerRole, workerPassword),
		"CREATE DATABASE "+templateName,
	); err != nil {
		return nil, err
	}
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := migrate.Up(ctx, s.url(templateName, ""), quiet); err != nil {
		return nil, fmt.Errorf("migrate template: %w", err)
	}
	return s, nil
}

// Terminate stops the container.
func (s *Server) Terminate(ctx context.Context) error {
	return s.container.Terminate(ctx)
}

// Database is a per-test database.
type Database struct {
	// OwnerURL connects as the schema owner (superuser): RLS does not apply.
	OwnerURL string
	// AppURL connects as the restricted application role: RLS applies.
	AppURL string
	// App is a pool for AppURL, closed when the test ends.
	App *db.DB
	// WorkerURL connects as the worker role: RLS applies, and the system
	// scope (db.WithSystem) works only for it.
	WorkerURL string
	// Worker is a pool for WorkerURL, closed when the test ends.
	Worker *db.DB
}

// NewDB creates a fresh migrated database for the test. In -short mode, or
// when the server was not started, the test is skipped.
func (s *Server) NewDB(tb testing.TB) *Database {
	tb.Helper()
	if s == nil {
		tb.Skip("integration test: needs Docker (run without -short)")
	}
	ctx := context.Background()
	name := fmt.Sprintf("test_%d_%d", time.Now().UnixNano()%1_000_000, s.seq.Add(1))
	if err := s.exec(ctx, "postgres", fmt.Sprintf("CREATE DATABASE %s TEMPLATE %s", name, templateName)); err != nil {
		tb.Fatalf("create database: %v", err)
	}

	app, err := openPool(ctx, s.url(name, appRole))
	if err != nil {
		tb.Fatalf("open app pool: %v", err)
	}
	worker, err := openPool(ctx, s.url(name, workerRole))
	if err != nil {
		app.Close()
		tb.Fatalf("open worker pool: %v", err)
	}
	tb.Cleanup(func() {
		app.Close()
		worker.Close()
		_ = s.exec(context.Background(), "postgres", fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH (FORCE)", name))
	})
	return &Database{
		OwnerURL: s.url(name, ""), AppURL: s.url(name, appRole), App: app,
		WorkerURL: s.url(name, workerRole), Worker: worker,
	}
}

func openPool(ctx context.Context, dsn string) (*db.DB, error) {
	pcfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	pcfg.MaxConns = 10
	return db.Open(ctx, pcfg)
}

// Admin returns the URL of a database of the server as its superuser, for
// tests that create roles or databases of their own.
func (s *Server) Admin(database string) string { return s.url(database, "") }

// Exec runs statements as the owner in the test database.
func (d *Database) Exec(tb testing.TB, statements ...string) {
	tb.Helper()
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, d.OwnerURL)
	if err != nil {
		tb.Fatal(err)
	}
	defer func() { _ = conn.Close(ctx) }()
	for _, stmt := range statements {
		if _, err := conn.Exec(ctx, stmt); err != nil {
			tb.Fatalf("exec %q: %v", stmt, err)
		}
	}
}

// url connects to a database as role, or as the superuser when role is empty.
func (s *Server) url(database, role string) string {
	u := *s.adminURL
	u.Path = "/" + database
	switch role {
	case appRole:
		u.User = url.UserPassword(appRole, appPassword)
	case workerRole:
		u.User = url.UserPassword(workerRole, workerPassword)
	}
	return u.String()
}

func (s *Server) exec(ctx context.Context, database string, statements ...string) error {
	conn, err := pgx.Connect(ctx, s.url(database, ""))
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer func() { _ = conn.Close(ctx) }()
	for _, stmt := range statements {
		if _, err := conn.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("exec %q: %w", stmt, err)
		}
	}
	return nil
}
