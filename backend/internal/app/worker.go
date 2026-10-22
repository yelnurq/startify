package app

import (
	"context"
	"log/slog"

	"github.com/riverqueue/river"

	"github.com/yelnurq/startify/backend/internal/ai"
	"github.com/yelnurq/startify/backend/internal/ai/llm"
	"github.com/yelnurq/startify/backend/internal/auth"
	"github.com/yelnurq/startify/backend/internal/files"
	"github.com/yelnurq/startify/backend/internal/knowledge"
	"github.com/yelnurq/startify/backend/internal/mail"
	"github.com/yelnurq/startify/backend/internal/platform/config"
	"github.com/yelnurq/startify/backend/internal/platform/db"
	"github.com/yelnurq/startify/backend/internal/platform/jobs"
	"github.com/yelnurq/startify/backend/internal/platform/maintenance"
	"github.com/yelnurq/startify/backend/internal/platform/secrets"
)

// Worker holds the dependencies of background jobs.
type Worker struct {
	Config *config.Config
	DB     *db.DB
	Log    *slog.Logger
	// Sender, Storage and Providers default to SMTP, S3 and the configured
	// language models; tests replace them.
	Sender    mail.Sender
	Storage   files.Storage
	Providers []llm.Provider
}

// Workers registers the job handlers of every module.
func (w *Worker) Workers(ctx context.Context) (*river.Workers, error) {
	keys, err := secrets.NewKeyring(w.Config.Auth.SecretKey)
	if err != nil {
		return nil, err
	}
	if w.Sender == nil {
		w.Sender = mail.NewSender(w.Config.SMTP, w.Log)
	}
	if w.Storage == nil {
		if w.Storage, err = files.NewStorage(ctx, w.Config.S3); err != nil {
			return nil, err
		}
	}
	if w.Providers == nil {
		w.Providers = ai.ConfiguredProviders(w.Config.AI, 0)
	}
	gateway, err := ai.NewGateway(w.DB, w.Providers, w.Config.AI, w.Log)
	if err != nil {
		return nil, err
	}
	queue, err := jobs.NewQueue(w.DB)
	if err != nil {
		return nil, err
	}
	workers := river.NewWorkers()
	river.AddWorker(workers, &knowledge.IndexWorker{DB: w.DB, Gateway: gateway})
	river.AddWorker(workers, &knowledge.ReindexWorker{DB: w.DB, Gateway: gateway, Queue: queue})
	river.AddWorker(workers, &maintenance.CleanupWorker{DB: w.DB})
	river.AddWorker(workers, &auth.CleanupWorker{DB: w.DB})
	river.AddWorker(workers, &files.PurgeWorker{DB: w.DB, Store: w.Storage})
	river.AddWorker(workers, &mail.SendWorker{Sender: w.Sender, Keys: keys})
	return workers, nil
}

// Queues returns the queues the worker processes. Letters have their own
// queue so that a slow mail relay never delays other jobs.
func (w *Worker) Queues() map[string]river.QueueConfig {
	return map[string]river.QueueConfig{
		river.QueueDefault: {MaxWorkers: w.Config.Worker.Concurrency},
		mail.QueueName:     {MaxWorkers: 5},
	}
}

// PeriodicJobs returns the schedule of recurring jobs.
func (w *Worker) PeriodicJobs() []*river.PeriodicJob {
	periodic := maintenance.PeriodicJobs()
	periodic = append(periodic, auth.PeriodicJobs()...)
	periodic = append(periodic, files.PeriodicJobs()...)
	return append(periodic, knowledge.PeriodicJobs()...)
}
