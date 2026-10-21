package app

import (
	"github.com/riverqueue/river"

	"github.com/yelnurq/startify/backend/internal/platform/config"
	"github.com/yelnurq/startify/backend/internal/platform/db"
	"github.com/yelnurq/startify/backend/internal/platform/maintenance"
)

// Worker holds the dependencies of background jobs.
type Worker struct {
	Config *config.Config
	DB     *db.DB
}

// Workers registers the job handlers of every module.
func (w *Worker) Workers() *river.Workers {
	workers := river.NewWorkers()
	river.AddWorker(workers, &maintenance.CleanupWorker{DB: w.DB})
	return workers
}

// PeriodicJobs returns the schedule of recurring jobs.
func (w *Worker) PeriodicJobs() []*river.PeriodicJob {
	return maintenance.PeriodicJobs()
}
