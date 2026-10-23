package maintenance

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/riverqueue/river"
)

// Reaper gives up the records of one kind whose background job is gone (a
// crashed or redeployed worker, a job River discarded): an evaluation left
// "running" would block the next evaluation of its project forever. Each
// module reaps its own tables; Fn returns how many records it gave up.
type Reaper struct {
	Name string
	Fn   func(ctx context.Context, now time.Time) (int64, error)
}

// StaleArgs schedules giving up stale background work.
type StaleArgs struct{}

// Kind implements river.JobArgs.
func (StaleArgs) Kind() string { return "platform.fail_stale" }

// InsertOpts deduplicates the job so that several schedulers never pile it up.
func (StaleArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{UniqueOpts: river.UniqueOpts{ByPeriod: 5 * time.Minute}}
}

// StaleWorker runs the reapers of every module.
type StaleWorker struct {
	river.WorkerDefaults[StaleArgs]
	Reapers []Reaper
	// Now is replaceable in tests.
	Now func() time.Time
}

// Work implements river.Worker. A failing reaper does not stop the others.
func (w *StaleWorker) Work(ctx context.Context, _ *river.Job[StaleArgs]) error {
	now := time.Now()
	if w.Now != nil {
		now = w.Now()
	}
	var errs []error
	for _, r := range w.Reapers {
		n, err := r.Fn(ctx, now)
		if err != nil {
			errs = append(errs, fmt.Errorf("stale %s: %w", r.Name, err))
			continue
		}
		if n > 0 {
			slog.WarnContext(ctx, "stale background work given up", slog.String("kind", r.Name), slog.Int64("rows", n))
		}
	}
	return errors.Join(errs...)
}

// staleInterval is how often stale work is looked for; each reaper waits far
// longer than its job's worst case before giving up, so this only bounds how
// long a user waits after that.
const staleInterval = 10 * time.Minute
