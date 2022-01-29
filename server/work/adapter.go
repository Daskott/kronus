package work

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Daskott/kronus/server/cron"
	"github.com/Daskott/kronus/server/models"
	"github.com/go-co-op/gocron"
)

const MAX_CONCURRENCY = 1

var ErrJobNotFoundInCronSch = errors.New("handler with provided name already mapped")

type WorkerPoolAdapter struct {
	cronScheduler            *gocron.Scheduler
	pool                     workerPool
	useCronParserWithSeconds bool
}

func NewWorkerAdapter(timeZoneArg string, useCronParserWithSeconds bool) *WorkerPoolAdapter {
	return &WorkerPoolAdapter{
		cronScheduler:            cron.NewCronScheduler(timeZoneArg),
		pool:                     *newWorkerPool(MAX_CONCURRENCY),
		useCronParserWithSeconds: useCronParserWithSeconds,
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

// Perform sends a new job to the queue to be executed as soon as a worker is available
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

// PerformIn sends a job to the 'scheduled' queue
// to be executed as soon as 'secondsInFuture' has elapsed
func (adapter *WorkerPoolAdapter) PerformIn(secondsInFuture int, job JobParams) error {
	logg.Infof("Scheduling job: %v, to run in %v seconds", job, secondsInFuture)

	err := adapter.pool.enqueueIn(secondsInFuture, job)
	if err != nil {
		return fmt.Errorf("error scheduling job: %v, %v", job, err)
	}

	return nil
}

// PeriodicallyPerform adds a job to the queue periodically (to be executed),
// based on the 'cronExpression' expression provided.
//
// NOTE: All enqueued jobs are unique by name.
//if a duplicate is added, an error is logged when the internal cron scheduler tries to add it
// the job to the job queue.
func (adapter *WorkerPoolAdapter) PeriodicallyPerform(cronExpression string, job JobParams) error {
	var scheduler *gocron.Scheduler

	// The scheduler will use a cron parser that expects a 6th field for seconds
	if adapter.useCronParserWithSeconds {
		scheduler = adapter.cronScheduler.CronWithSeconds(cronExpression)
	} else {
		scheduler = adapter.cronScheduler.Cron(cronExpression)
	}

	_, err := scheduler.Tag(job.Name).
		Do(
			func(job JobParams) {
				err := adapter.Perform(job)
				if err != nil {
					logg.Error(err)
				}
			},
			job,
		)
	return err
}

func (adapter *WorkerPoolAdapter) RemovePeriodicJob(jobName string) {
	adapter.cronScheduler.RemoveByTag(jobName)
}

func (adapter *WorkerPoolAdapter) UpdateJobScheduleByTag(tag, cronExpression string) error {
	var job *gocron.Job

	// Find job by tag in cronScheduler
	for _, j := range adapter.cronScheduler.Jobs() {
		if strings.Contains(strings.Join(j.Tags(), ","), tag) {
			job = j
			break
		}
	}

	if job == nil {
		return ErrJobNotFoundInCronSch
	}

	_, err := adapter.cronScheduler.Job(job).Cron(cronExpression).Update()
	if err != nil {
		logg.Error(err)
	}

	return nil
}
