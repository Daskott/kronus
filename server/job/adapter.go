package job

import "context"

type WorkerAdapter struct {
	Worker Worker
}

func NewWorkerAdapter() *WorkerAdapter {
	return &WorkerAdapter{
		Worker: *NewWorker(),
	}
}

// Start starts the adapter event loop.
func (adapter *WorkerAdapter) Start(ctx context.Context) error {
	logg.Info("Starting Worker")
	adapter.Worker.Start()
	return nil
}

// Stop stops the adapter event loop.
func (adapter *WorkerAdapter) Stop() error {
	logg.Info("Stopping Worker")
	adapter.Worker.Stop()
	return nil
}

// Register binds a name to a handler.
func (adapter *WorkerAdapter) Register(name string, handler Handler) error {
	return adapter.Worker.RegisterHandler(name, handler)
}

// Perform sends a new job to the queue, now.
func (adapter *WorkerAdapter) Perform(job JobParams) error {
	logg.Infof("Enqueuing job: %v", job)
	err := adapter.Worker.Enqueue(job)
	if err != nil {
		logg.Errorf("error enqueuing job: %v, %v", job, err)
	}
	return nil
}
