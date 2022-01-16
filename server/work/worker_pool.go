package work

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/Daskott/kronus/server/models"
	"github.com/pkg/errors"
)

type workerPool struct {
	handlers    map[string]Handler
	workers     []*worker
	requeuer    *requeuer
	concurrency int
	started     bool
}

func newWorkerPool(concurrency int) *workerPool {
	wp := workerPool{
		handlers:    make(map[string]Handler),
		concurrency: concurrency,
		requeuer:    newRequeuer(),
	}

	for i := 0; i < concurrency; i++ {
		wp.workers = append(wp.workers, newWorker([]int64{0, 1, 2, 5, 15, 30}))
	}

	return &wp
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
	err = models.CreateUniqueJobByName(job.Name, job.Handler, string(argsAsJson))

	if err != nil {
		return err
	}

	return nil
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

	wp.requeuer.start()
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

	wp.requeuer.stop()
}
