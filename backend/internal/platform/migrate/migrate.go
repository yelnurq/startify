// Package migrate applies the schema migrations (goose) and the job queue
// migrations (River). Migrations run as the schema owner, not the app role.
package migrate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
	"github.com/pressly/goose/v3"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"

	"github.com/yelnurq/startify/backend/db"
)

// Up applies all pending migrations.
func Up(ctx context.Context, dsn string, log *slog.Logger) error {
	err := withProvider(dsn, func(p *goose.Provider) error {
		results, err := p.Up(ctx)
		for _, r := range results {
			log.InfoContext(ctx, "migration applied",
				slog.Int64("version", r.Source.Version), slog.String("file", r.Source.Path), slog.Duration("took", r.Duration))
		}
		if err != nil {
			return fmt.Errorf("migrate: up: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	return riverUp(ctx, dsn, log)
}

// Down rolls back the most recent schema migration.
func Down(ctx context.Context, dsn string, log *slog.Logger) error {
	return withProvider(dsn, func(p *goose.Provider) error {
		r, err := p.Down(ctx)
		if err != nil {
			return fmt.Errorf("migrate: down: %w", err)
		}
		if r != nil {
			log.InfoContext(ctx, "migration rolled back", slog.Int64("version", r.Source.Version))
		}
		return nil
	})
}

// Status prints the state of every schema migration.
func Status(ctx context.Context, dsn string, w io.Writer) error {
	return withProvider(dsn, func(p *goose.Provider) error {
		statuses, err := p.Status(ctx)
		if err != nil {
			return fmt.Errorf("migrate: status: %w", err)
		}
		for _, s := range statuses {
			applied := "pending"
			if s.State == goose.StateApplied {
				applied = "applied " + s.AppliedAt.Format("2006-01-02 15:04:05")
			}
			if _, err := fmt.Fprintf(w, "%05d  %-40s %s\n", s.Source.Version, s.Source.Path, applied); err != nil {
				return err
			}
		}
		return nil
	})
}

func withProvider(dsn string, fn func(*goose.Provider) error) (err error) {
	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("migrate: open: %w", err)
	}
	defer func() { err = errors.Join(err, sqlDB.Close()) }()

	fsys, err := fs.Sub(db.Migrations, "migrations")
	if err != nil {
		return fmt.Errorf("migrate: migrations fs: %w", err)
	}
	p, err := goose.NewProvider(goose.DialectPostgres, sqlDB, fsys)
	if err != nil {
		return fmt.Errorf("migrate: provider: %w", err)
	}
	return fn(p)
}

func riverUp(ctx context.Context, dsn string, log *slog.Logger) error {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return fmt.Errorf("migrate: river pool: %w", err)
	}
	defer pool.Close()

	migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		return fmt.Errorf("migrate: river migrator: %w", err)
	}
	res, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil)
	if err != nil {
		return fmt.Errorf("migrate: river: %w", err)
	}
	for _, v := range res.Versions {
		log.InfoContext(ctx, "river migration applied", slog.Int("version", v.Version))
	}
	return nil
}
