// Package jobs enqueues background work in the River queue. Jobs inserted
// inside db.Tx are committed atomically with the business change: a letter is
// sent only if the account really was created.
package jobs

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"github.com/yelnurq/startify/backend/internal/platform/db"
)

// Enqueuer inserts jobs.
type Enqueuer interface {
	Enqueue(ctx context.Context, args river.JobArgs, opts *river.InsertOpts) error
}

// Queue is the insert-only River client of the API process.
type Queue struct {
	db     *db.DB
	client *river.Client[pgx.Tx]
}

// NewQueue builds an insert-only client: the API never works jobs itself.
func NewQueue(database *db.DB) (*Queue, error) {
	client, err := river.NewClient(riverpgxv5.New(database.Pool()), &river.Config{})
	if err != nil {
		return nil, fmt.Errorf("jobs: client: %w", err)
	}
	return &Queue{db: database, client: client}, nil
}

// Enqueue inserts the job in the transaction of ctx when there is one.
func (q *Queue) Enqueue(ctx context.Context, args river.JobArgs, opts *river.InsertOpts) error {
	if tx, ok := q.db.Conn(ctx).(pgx.Tx); ok {
		if _, err := q.client.InsertTx(ctx, tx, args, opts); err != nil {
			return fmt.Errorf("jobs: insert %s: %w", args.Kind(), err)
		}
		return nil
	}
	if _, err := q.client.Insert(ctx, args, opts); err != nil {
		return fmt.Errorf("jobs: insert %s: %w", args.Kind(), err)
	}
	return nil
}
