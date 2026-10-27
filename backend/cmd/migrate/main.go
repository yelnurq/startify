// Command migrate manages the database schema.
//
//	migrate [up|down|status]
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/yelnurq/startify/backend/internal/platform/config"
	"github.com/yelnurq/startify/backend/internal/platform/logger"
	"github.com/yelnurq/startify/backend/internal/platform/migrate"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "migrate:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	command := "up"
	if len(args) > 0 {
		command = args[0]
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := logger.New(cfg.Log, os.Stderr)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch command {
	case "up":
		return migrate.Up(ctx, cfg.Database.MigrateURL, log)
	case "down":
		return migrate.Down(ctx, cfg.Database.MigrateURL, log)
	case "status":
		return migrate.Status(ctx, cfg.Database.MigrateURL, os.Stdout)
	default:
		return fmt.Errorf("unknown command %q (want up, down or status)", command)
	}
}
