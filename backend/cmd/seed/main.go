// Command seed fills the database with the demonstration data: the KazUTB
// structure, demo accounts of every role and requests for teammates. It is
// idempotent and refuses to run in production.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

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
	database, err := db.Connect(ctx, cfg.Database)
	if err != nil {
		return err
	}
	defer database.Close()

	if err := seed.Demo(ctx, database); err != nil {
		return err
	}
	fmt.Println("Demo data is ready. Sign in at", cfg.App.PublicURL, "with any account below, password", seed.DemoPassword)
	for _, list := range [][]seed.Member{seed.KazUTBMembers, seed.KazUTBStaff} {
		for i := range list {
			fmt.Printf("  %-26s %v\n", list[i].Email, list[i].Roles)
		}
	}
	fmt.Printf("  and %d more students: <name>.<surname>@qadam.test\n", seed.GeneratedStudents)
	return nil
}
