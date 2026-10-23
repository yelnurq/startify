// Command seed fills the database with the demonstration data: the KazUTB
// structure with its seal and campus photo, demo accounts of every role,
// requests for teammates and student projects. It is idempotent and refuses to run in production.
//
//	seed            # the demo data
//	seed clean-e2e  # remove what end-to-end tests left (development only)
//
// It connects as the worker's role (QADAM_DATABASE_WORKER_URL): universities
// are created in the system scope.
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
	database, err := db.ConnectWorker(ctx, cfg.Database)
	if err != nil {
		return err
	}
	defer database.Close()

	if len(os.Args) > 1 {
		if os.Args[1] != "clean-e2e" {
			return fmt.Errorf("unknown command %q (want none for the demo data, or clean-e2e)", os.Args[1])
		}
		c, err := seed.CleanE2E(ctx, database)
		if err != nil {
			return err
		}
		fmt.Printf("Removed what end-to-end tests left: %d accounts, %d projects, %d requests for teammates, "+
			"%d contact requests, %d knowledge documents, %d imports; %d files go with the next purge.\n",
			c.Accounts, c.Projects, c.Requests, c.Contacts, c.Documents, c.Imports, c.Files)
		return nil
	}

	if err := seed.Demo(ctx, database); err != nil {
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
	return nil
}
