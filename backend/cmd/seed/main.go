// Command seed fills the database with the demonstration data: the KazUTB
// structure with its seal and campus photo, demo accounts of every role,
// requests for teammates and student projects. It is idempotent and refuses to run in production.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/yelnurq/startify/backend/internal/files"
	"github.com/yelnurq/startify/backend/internal/platform/config"
	"github.com/yelnurq/startify/backend/internal/platform/db"
	"github.com/yelnurq/startify/backend/internal/platform/secrets"
	"github.com/yelnurq/startify/backend/internal/seed"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "seed:", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.IsProduction() {
		return errors.New("demo data is not for production")
	}
	database, err := db.Connect(ctx, cfg.Database)
	if err != nil {
		return err
	}
	defer database.Close()

	if err := seed.Demo(ctx, database); err != nil {
		return err
	}
	keys, err := secrets.NewKeyring(cfg.Auth.SecretKey)
	if err != nil {
		return err
	}
	if err := seed.Directory(ctx, database, keys, seed.DemoDirectoryURL); err != nil {
		return err
	}
	storage, err := files.NewStorage(ctx, cfg.S3)
	if err != nil {
		return err
	}
	if err := seed.Branding(ctx, database, files.NewService(database, storage, cfg.Files, slog.Default())); err != nil {
		return err
	}
	fmt.Println("Demo data is ready. Sign in at", cfg.App.PublicURL, "with any account below, password", seed.DemoPassword)
	for _, list := range [][]seed.Member{seed.KazUTBMembers, seed.KazUTBStaff} {
		for i := range list {
			fmt.Printf("  %-26s %v\n", list[i].Email, list[i].Roles)
		}
	}
	fmt.Printf("  and %d more students: <name>.<surname>@qadam.test\n", seed.GeneratedStudents)
	fmt.Printf("KazUTB has %d demo projects, from a first draft of the problem to a finished pitch.\n", len(seed.KazUTBProjects))
	fmt.Println("Directory sign-in points at the demo directory", seed.DemoDirectoryURL,
		"and is off: check it and turn it on at /app/admin/directory.")
	return nil
}
