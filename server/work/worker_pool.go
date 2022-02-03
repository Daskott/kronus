package work

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Daskott/kronus/server/models"
	"github.com/pkg/errors"
)

type workerPool struct {
	handlers    map[string]Handler
	workers     []*worker
	retrier     *requeuer
	scheduler   *requeuer
	concurrency int
	started     bool
}

func newWorkerPool(concurrency int) (*workerPool, error) {
	retrier, err := newRequeuer(models.IN_PROGRESS_JOB)
	if err != nil {
		return nil, err
	}

	scheduler, err := newRequeuer(models.SCHEDULED_JOB)
	if err != nil {
		return nil, err
	}

	wp := workerPool{
		handlers:    make(map[string]Handler),
		concurrency: concurrency,
		retrier:     retrier,
		scheduler:   scheduler,
	}

	for i := 0; i < concurrency; i++ {
		wp.workers = append(wp.workers, newWorker([]int64{0, 1, 2, 5, 15, 30}))
	}

	return &wp, nil
}

// registerHandler binds a name to a job handler for all workers in pool
func (wp *workerPool) registerHandler(name string, handler Handler) error {
	if _, ok := wp.handlers[name]; ok {
		return ErrDuplicateHandler
	}

	for _, worker := range wp.workers {
		err := worker.registerHandler(name, handler)

		// Only panic if we get an error that is unexpected i.e !ErrDuplicateHandler
		if err != nil && !errors.Is(err, ErrDuplicateHandler) {
			logg.Panic(err)
		}
	}
	return nil
}

// enqueue adds a job to the queue(to be executed) by creating a DB record based on 'JobParams' provided.
// Each job is unique by name. Can't have more than one job with the same name 'enqueued' or 'in-progress'
// at the same time.
func (wp *workerPool) enqueue(job JobParams) error {
	if strings.TrimSpace(job.Name) == "" || strings.TrimSpace(job.Handler) == "" {
		return fmt.Errorf("both a name & handler is required for a job")
	}

	argsAsJson, err := json.Marshal(job.Args)
	if err != nil {
		return err
	}

	// This ensures that all jobs currently in the queue or in-progress are unique
	return models.CreateUniqueJobByName(job.Name, job.Handler, string(argsAsJson))
}

func (wp *workerPool) enqueueIn(secondsInFuture int, job JobParams) error {
	if strings.TrimSpace(job.Name) == "" || strings.TrimSpace(job.Handler) == "" {
		return fmt.Errorf("both a name & handler is required for a job")
	}

	argsAsJson, err := json.Marshal(job.Args)
	if err != nil {
		return err
	}

	return models.CreateScheduledJob(
		job.Name, job.Handler,
		string(argsAsJson),
		time.Now().Add(time.Duration(secondsInFuture)*time.Second),
	)
}

// start starts all workers in pool & job reaper i.e the workers can start processing jobs
func (wp *workerPool) start() {
	if wp.started {
		return
	}
	wp.started = true

	for _, worker := range wp.workers {
		go worker.start()
	}

	wp.retrier.start()
	wp.scheduler.start()
}

// stop stops all workers in pool & job reaper i.e jobs will stop being processed
func (wp *workerPool) stop() {
	if !wp.started {
		return
	}

	wg := sync.WaitGroup{}
	for _, w := range wp.workers {
		wg.Add(1)
		go func(w *worker) {
			w.stop()
			wg.Done()
		}(w)
	}
	wg.Wait()
	wp.started = false

	wp.retrier.stop()
	wp.scheduler.stop()
}
