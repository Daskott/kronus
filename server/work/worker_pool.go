package work

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/Daskott/kronus/models"
	"github.com/pkg/errors"
)

type WorkerPool struct {
	Handlers    map[string]Handler
	workers     []*worker
	concurrency int
	started     bool
}

func NewWorkerPool(concurrency int) *WorkerPool {
	wp := WorkerPool{Handlers: make(map[string]Handler), concurrency: concurrency}

	for i := 0; i < concurrency; i++ {
		wp.workers = append(wp.workers, NewWorker([]int64{0, 10, 100, 120}))
	}

	return &wp
}

// RegisterHandler binds a name to a job handler for all workers in pool
func (wp *WorkerPool) RegisterHandler(name string, handler Handler) error {
	if _, ok := wp.Handlers[name]; ok {
		return ErrDuplicateHandler
	}

	for _, worker := range wp.workers {
		err := worker.RegisterHandler(name, handler)

		// Only panic if we get an error that is unexpected i.e !ErrDuplicateHandler
		if err != nil && !errors.Is(err, ErrDuplicateHandler) {
			logg.Panic(err)
		}
	}
	return nil
}

// Enqueue adds a job to the queue(to be executed) by creating a DB record based on 'JobParams' provided
func (wp *WorkerPool) Enqueue(job JobParams) error {
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

// Start starts all workers in pool i.e the workes can start processing jobs
func (wp *WorkerPool) Start() {
	if wp.started {
		return
	}
	wp.started = true

	for _, worker := range wp.workers {
		go worker.start()
	}
}

// Stop stops all workers in pool i.e jobs will stop being processed
func (wp *WorkerPool) stop() {
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
}
