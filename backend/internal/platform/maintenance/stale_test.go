package maintenance

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/riverqueue/river"
)

// A failing reaper does not keep the others from giving up their work.
func TestStaleWorkerRunsEveryReaper(t *testing.T) {
	at := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	var seen []string
	reaper := func(name string, err error) Reaper {
		return Reaper{Name: name, Fn: func(_ context.Context, now time.Time) (int64, error) {
			if !now.Equal(at) {
				t.Errorf("%s got %s", name, now)
			}
			seen = append(seen, name)
			return 1, err
		}}
	}
	w := &StaleWorker{Now: func() time.Time { return at }, Reapers: []Reaper{
		reaper("evaluations", errors.New("database is down")), reaper("materials", nil),
	}}
	err := w.Work(context.Background(), &river.Job[StaleArgs]{})
	if err == nil || !strings.Contains(err.Error(), "stale evaluations") || len(seen) != 2 {
		t.Fatalf("%v, ran %v", err, seen)
	}
	if len(PeriodicJobs()) != 2 {
		t.Fatal("the stale-work job must be scheduled")
	}
}
