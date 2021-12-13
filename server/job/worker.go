package job

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Daskott/kronus/database"
	"github.com/Daskott/kronus/server/logger"
	"gorm.io/gorm"
)

const (
	ENQUEUED_JOB    = "enqueued"
	IN_PROGRESS_JOB = "in-progress"
	SUCCESSFUL_JOB  = "successful"
	FAILED_JOB      = "failed"
	DEAD_JOB        = "dead"
	MAX_FAILS       = 4
	MAX_CONCURRENCY = 25
)

var logg = logger.NewLogger()

type JobParams struct {
	Name    string
	Handler string
	Unique  bool
	Args    map[string]interface{}
}

type Handler func(map[string]interface{}) error

type Worker struct {
	Handlers map[string]Handler
}

func NewWorker() *Worker {
	return &Worker{Handlers: make(map[string]Handler)}
}

// RegisterHandler binds a name to a job handler.
func (worker *Worker) RegisterHandler(name string, handler Handler) error {
	if _, ok := worker.Handlers[name]; ok {
		return fmt.Errorf("handler already mapped for '%v'", name)
	}

	worker.Handlers[name] = handler

	return nil
}

// Enqueue adds a job to the queue(to be executed) by creating a DB record based on 'JobParams' provided
func (worker *Worker) Enqueue(job JobParams) error {
	if strings.TrimSpace(job.Name) == "" || strings.TrimSpace(job.Handler) == "" {
		return fmt.Errorf("both a name & handler is required for a job")
	}

	argsAsJson, err := json.Marshal(job.Args)
	if err != nil {
		return err
	}

	if job.Unique {
		err = database.CreateUniqueJobByName(job.Name, job.Handler, string(argsAsJson))
	} else {
		err = database.CreateJob(job.Name, job.Handler, string(argsAsJson))
	}

	if err != nil {
		return err
	}

	return nil
}

// Start starts the worker loop that pulls jobs from the queue & process them
func (worker *Worker) Start() {
	worker.loop()
}

func (worker *Worker) Stop() {
	// TODO: Use channels
}

func (worker *Worker) loop() {
	for {
		job, err := database.LastJob(ENQUEUED_JOB, false)
		if err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				logg.Errorf("Worker.loop: %v", err)
			}
			continue
		}

		claimed, err := database.ClaimJob(job.ID)
		if err != nil {
			logg.Errorf("Worker.loop: %v", err)
			continue
		}

		logg.Infof("fetched job with id=%v, claimed=%v", job.ID, claimed)
		if !claimed {
			continue
		}

		worker.processJob(job)
	}
}

func (worker *Worker) processJob(job *database.Job) {
	err := database.UpdateJobStatus(job.ID, IN_PROGRESS_JOB)
	if err != nil {
		logg.Error(err)
		worker.determineFailedJobFate(job, err)
		return
	}

	// Convert job args from json string back to map[string]interface{}
	args := make(map[string]interface{})
	err = json.Unmarshal([]byte(job.Args), &args)
	if err != nil {
		logg.Error(err)
		worker.determineFailedJobFate(job, err)
		return
	}

	err = worker.Handlers[job.Handler](args)
	if err != nil {
		logg.Error(err)
		worker.determineFailedJobFate(job, err)
		return
	}
	worker.markJobAsSuccessful(job)
}

func (worker *Worker) determineFailedJobFate(job *database.Job, runError error) {
	var jobStatus *database.JobStatus
	var err error

	job.Fails++
	if job.Fails >= MAX_FAILS {
		jobStatus, err = database.FindJobStatus(DEAD_JOB)
	} else {
		jobStatus, err = database.FindJobStatus(FAILED_JOB)
	}

	if err != nil {
		logg.Error(err)
		return
	}

	// Unclaim job and update it with the necessary fail information
	err = database.UpdateJob(job.ID, map[string]interface{}{
		"claimed":       false,
		"job_status_id": jobStatus.ID,
		"fails":         job.Fails,
		"last_error":    runError.Error(),
	})
	if err != nil {
		logg.Error(err)
	}
	logg.Infof("job with id=%v completed with status=%v", jobStatus.Name)
}

func (worker *Worker) markJobAsSuccessful(job *database.Job) {
	jobStatus, err := database.FindJobStatus(SUCCESSFUL_JOB)
	if err != nil {
		logg.Error(err)
		return
	}

	update := make(map[string]interface{})
	update["claimed"] = false
	update["job_status_id"] = jobStatus.ID

	err = database.UpdateJob(job.ID, update)
	if err != nil {
		logg.Error(err)
	}
	logg.Infof("job with id=%v completed with status=%v", job.ID, jobStatus.Name)
}

// TODO:
// - Finish up worker i.e. stop func, accept queue param, max_concurrency
// - Add queue column to jobs & set default
// - Enque probe when activated via api
// - When probe is turned off
// 	* Remove 'liveliness probe' cron
//  * Set probe status to cancelled
// - Mark probe as completed as soon as user replies sms
// - Move models with their funcs & consts to separate file
// - The Reaper: A background job that just runs every 30-60mins and unclaiming & requeue jobs
// 		That are in a weird state i.e. in-progress or queued & claimed but no work is being done
