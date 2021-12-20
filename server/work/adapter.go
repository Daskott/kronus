package work

import (
	"errors"
	"fmt"

	"github.com/Daskott/kronus/models"
	"github.com/Daskott/kronus/server/cron"
	"github.com/go-co-op/gocron"
)

const MAX_CONCURRENCY = 1

type WorkerPoolAdapter struct {
	cronScheduler *gocron.Scheduler
	pool          WorkerPool
}

func NewWorkerAdapter(timeZoneArg string) *WorkerPoolAdapter {
	return &WorkerPoolAdapter{
		cronScheduler: cron.NewCronScheduler(timeZoneArg),
		pool:          *NewWorkerPool(MAX_CONCURRENCY),
	}
}

// Start starts the cron scheduler & worker pool
func (adapter *WorkerPoolAdapter) Start() error {
	logg.Info("Starting cron scheduler & worker pool")
	adapter.cronScheduler.StartAsync()
	adapter.pool.start()

	return nil
}

// Stop stops the cron scheduler & worker pool
func (adapter *WorkerPoolAdapter) Stop() error {
	logg.Info("Stopping cron scheduler & worker pool")
	adapter.cronScheduler.Stop()
	adapter.pool.stop()

	return nil
}

// Register binds a name to a handler.
func (adapter *WorkerPoolAdapter) Register(name string, handler Handler) error {
	return adapter.pool.registerHandler(name, handler)
}

// Perform sends a new job to the queue, now - to be executed as soon as a worker is available
func (adapter *WorkerPoolAdapter) Perform(job JobParams) error {
	logg.Infof("Enqueuing job: %v", job)

	err := adapter.pool.enqueue(job)
	if errors.Is(err, models.ErrDuplicateJob) {
		logg.Warnf("Duplicate job already in queue for: %v", job)
		return nil
	}

	if err != nil {
		return fmt.Errorf("error enqueuing job: %v, %v", job, err)
	}

	return nil
}

// PeriodicallyPerform adds a job to the queue (to be executed)
// periodically, based on the 'cronExpression' expression provided
func (adapter *WorkerPoolAdapter) PeriodicallyPerform(cronExpression string, job JobParams) error {
	adapter.cronScheduler.Cron(cronExpression).Tag(job.Name).
		Do(
			func(job JobParams) {
				err := adapter.Perform(job)
				if err != nil {
					logg.Error(err)
				}
			},
			job,
		)
	return nil
}

func (adapter *WorkerPoolAdapter) RemovePeriodicJob(jobName string) {
	adapter.cronScheduler.RemoveByTag(jobName)
}
