package auth

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/riverqueue/river"

	"github.com/yelnurq/startify/backend/internal/auth/authdb"
	"github.com/yelnurq/startify/backend/internal/platform/db"
)

// CleanupArgs schedules removal of expired sessions and one-time tokens.
type CleanupArgs struct{}

// Kind implements river.JobArgs.
func (CleanupArgs) Kind() string { return "auth.cleanup" }

// InsertOpts deduplicates the job.
func (CleanupArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{UniqueOpts: river.UniqueOpts{ByPeriod: 30 * time.Minute}}
}

// CleanupWorker deletes sessions and tokens nobody can use any more.
type CleanupWorker struct {
	river.WorkerDefaults[CleanupArgs]
	DB *db.DB
}

// Work implements river.Worker.
func (w *CleanupWorker) Work(ctx context.Context, _ *river.Job[CleanupArgs]) error {
	ctx = db.WithSystem(ctx)
	return w.DB.Tx(ctx, func(ctx context.Context) error {
		q := authdb.New(w.DB.Conn(ctx))
		sessions, err := q.DeleteStaleSessions(ctx)
		if err != nil {
			return fmt.Errorf("auth: cleanup sessions: %w", err)
		}
		tokens, err := q.DeleteStaleAuthTokens(ctx)
		if err != nil {
			return fmt.Errorf("auth: cleanup tokens: %w", err)
		}
		slog.InfoContext(ctx, "expired credentials removed", slog.Int64("sessions", sessions), slog.Int64("tokens", tokens))
		return nil
	})
}

// PeriodicJobs returns the cleanup schedule.
func PeriodicJobs() []*river.PeriodicJob {
	return []*river.PeriodicJob{
		river.NewPeriodicJob(river.PeriodicInterval(time.Hour),
			func() (river.JobArgs, *river.InsertOpts) { return CleanupArgs{}, nil },
			&river.PeriodicJobOpts{RunOnStart: true}),
	}
}
