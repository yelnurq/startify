// Command api runs the Qadam HTTP API.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/yelnurq/startify/backend/internal/app"
	"github.com/yelnurq/startify/backend/internal/platform/buildinfo"
	"github.com/yelnurq/startify/backend/internal/platform/cache"
	"github.com/yelnurq/startify/backend/internal/platform/config"
	"github.com/yelnurq/startify/backend/internal/platform/db"
	"github.com/yelnurq/startify/backend/internal/platform/health"
	"github.com/yelnurq/startify/backend/internal/platform/logger"
	"github.com/yelnurq/startify/backend/internal/platform/metrics"
	"github.com/yelnurq/startify/backend/internal/platform/telemetry"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		addr := os.Getenv("QADAM_HTTP_ADDR")
		if addr == "" {
			addr = ":8080"
		}
		if err := health.Probe(addr); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "api:", err)
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
	log := logger.New(cfg.Log, os.Stdout)
	slog.SetDefault(log)

	shutdownTelemetry, err := telemetry.Setup(ctx, cfg.OTel, "qadam-api")
	if err != nil {
		return err
	}
	defer func() { _ = shutdownTelemetry(context.WithoutCancel(ctx)) }()

	database, err := db.Connect(ctx, cfg.Database)
	if err != nil {
		return err
	}
	defer database.Close()

	valkey, err := cache.Connect(ctx, cfg.Valkey)
	if err != nil {
		return err
	}
	defer func() { _ = valkey.Close() }()

	api := &app.API{Config: cfg, Log: log, DB: database, Cache: valkey, Metrics: metrics.New()}
	handler, _, err := api.Handler(ctx)
	if err != nil {
		return err
	}
	srv := &http.Server{
		Addr:              cfg.HTTP.Addr,
		Handler:           handler,
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelWarn),
	}

	serveErr := make(chan error, 1)
	go func() {
		log.Info("api listening", slog.String("addr", cfg.HTTP.Addr),
			slog.String("version", buildinfo.Version), slog.String("commit", buildinfo.Commit))
		serveErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve: %w", err)
		}
		return nil
	case <-ctx.Done():
	}

	log.Info("shutting down", slog.Duration("grace", cfg.HTTP.ShutdownTimeout))
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cfg.HTTP.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	return nil
}
