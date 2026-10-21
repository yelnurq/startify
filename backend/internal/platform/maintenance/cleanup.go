// Package maintenance holds periodic housekeeping jobs.
package maintenance

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/riverqueue/river"

	"github.com/yelnurq/startify/backend/internal/platform/db"
	"github.com/yelnurq/startify/backend/internal/platform/platformdb"
)

// CleanupArgs schedules removal of expired platform records.
type CleanupArgs struct{}

// Kind implements river.JobArgs.
func (CleanupArgs) Kind() string { return "platform.cleanup" }

// InsertOpts deduplicates the job so that several schedulers never pile it up.
func (CleanupArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{UniqueOpts: river.UniqueOpts{ByPeriod: 30 * time.Minute}}
}

// CleanupWorker deletes expired idempotency keys.
type CleanupWorker struct {
	river.WorkerDefaults[CleanupArgs]
	DB *db.DB
}

// Work implements river.Worker.
func (w *CleanupWorker) Work(ctx context.Context, _ *river.Job[CleanupArgs]) error {
	ctx = db.WithSystem(ctx)
	return w.DB.Tx(ctx, func(ctx context.Context) error {
		n, err := platformdb.New(w.DB.Conn(ctx)).DeleteExpiredIdempotencyKeys(ctx)
		if err != nil {
			return fmt.Errorf("cleanup: idempotency keys: %w", err)
		}
		slog.InfoContext(ctx, "expired records removed", slog.String("table", "idempotency_keys"), slog.Int64("rows", n))
		return nil
	})
}

// PeriodicJobs returns the housekeeping schedule.
func PeriodicJobs() []*river.PeriodicJob {
	return []*river.PeriodicJob{
		river.NewPeriodicJob(river.PeriodicInterval(time.Hour),
			func() (river.JobArgs, *river.InsertOpts) { return CleanupArgs{}, nil },
			&river.PeriodicJobOpts{RunOnStart: true}),
	}
}
