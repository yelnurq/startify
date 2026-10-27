// Command worker runs background jobs (AI evaluations, documents, e-mail,
// digests, housekeeping) from the River queue stored in PostgreSQL.
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
	"time"
	_ "time/tzdata" // daily AI limits reset at local midnight on any host

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"github.com/yelnurq/startify/backend/internal/app"
	"github.com/yelnurq/startify/backend/internal/platform/buildinfo"
	"github.com/yelnurq/startify/backend/internal/platform/config"
	"github.com/yelnurq/startify/backend/internal/platform/db"
	"github.com/yelnurq/startify/backend/internal/platform/health"
	"github.com/yelnurq/startify/backend/internal/platform/logger"
	"github.com/yelnurq/startify/backend/internal/platform/metrics"
	"github.com/yelnurq/startify/backend/internal/platform/telemetry"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		addr := os.Getenv("QADAM_WORKER_ADDR")
		if addr == "" {
			addr = ":8081"
		}
		if err := health.Probe(addr); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "worker:", err)
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

	shutdownTelemetry, err := telemetry.Setup(ctx, cfg.OTel, "qadam-worker")
	if err != nil {
		return err
	}
	defer func() { _ = shutdownTelemetry(context.WithoutCancel(ctx)) }()

	database, err := db.ConnectWorker(ctx, cfg.Database)
	if err != nil {
		return err
	}
	defer database.Close()

	worker := &app.Worker{Config: cfg, DB: database, Log: log}
	workers, err := worker.Workers(ctx)
	if err != nil {
		return err
	}
	client, err := river.NewClient(riverpgxv5.New(database.Pool()), &river.Config{
		Logger:       log,
		Queues:       worker.Queues(),
		Workers:      workers,
		PeriodicJobs: worker.PeriodicJobs(),
	})
	if err != nil {
		return fmt.Errorf("river client: %w", err)
	}
	if err := client.Start(ctx); err != nil {
		return fmt.Errorf("river start: %w", err)
	}

	m := metrics.New()
	probes := health.New(2*time.Second, health.Check{Name: "postgres", Fn: database.Ping})
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", probes.Live)
	mux.HandleFunc("GET /readyz", probes.Ready)
	mux.Handle("GET /metrics", m.Handler())
	probeSrv := &http.Server{Addr: cfg.Worker.Addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if err := probeSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("probe server stopped", slog.Any("error", err))
		}
	}()

	log.Info("worker started", slog.Int("concurrency", cfg.Worker.Concurrency),
		slog.String("version", buildinfo.Version), slog.String("probes", cfg.Worker.Addr))
	<-ctx.Done()

	log.Info("stopping worker: finishing running jobs")
	stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	if err := client.Stop(stopCtx); err != nil {
		log.Warn("graceful stop timed out, cancelling jobs", slog.Any("error", err))
		cancelCtx, cancelHard := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancelHard()
		_ = client.StopAndCancel(cancelCtx)
	}
	return probeSrv.Shutdown(stopCtx)
}
