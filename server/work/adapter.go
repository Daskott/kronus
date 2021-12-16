package work

import (
	"context"
	"errors"
	"fmt"

	"github.com/Daskott/kronus/database"
)

const MAX_CONCURRENCY = 1

type WorkerAdapter struct {
	Pool WorkerPool
}

func NewWorkerAdapter() *WorkerAdapter {
	return &WorkerAdapter{
		Pool: *NewWorkerPool(MAX_CONCURRENCY),
	}
}

// Start starts the adapter event loop.
func (adapter *WorkerAdapter) Start(ctx context.Context) error {
	logg.Info("Starting worker pool")
	adapter.Pool.Start()
	return nil
}

// Stop stops the adapter event loop.
func (adapter *WorkerAdapter) Stop() error {
	logg.Info("Stopping worker pool")
	adapter.Pool.stop()
	return nil
}

// Register binds a name to a handler.
func (adapter *WorkerAdapter) Register(name string, handler Handler) error {
	return adapter.Pool.RegisterHandler(name, handler)
}

// Perform sends a new job to the queue, now - to be executed as soon as a worker is available
func (adapter *WorkerAdapter) Perform(job JobParams) error {
	logg.Infof("Enqueuing job: %v", job)

	err := adapter.Pool.Enqueue(job)
	if err != nil {
		if errors.Is(err, database.ErrDuplicateJob) {
			logg.Warnf("Duplicate job already in queue for: %v", job)
			return nil
		}
		return fmt.Errorf("error enqueuing job: %v, %v", job, err)
	}

	return nil
}
