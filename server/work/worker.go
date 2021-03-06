package work

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Daskott/kronus/colors"
	"github.com/Daskott/kronus/server/logger"
	"github.com/Daskott/kronus/server/models"
	"gorm.io/gorm"
)

const MAX_FAILS = 4

var (
	DefaultTickerDuration = 5 * time.Millisecond
	TickerDurationOnError = 10 * time.Millisecond

	ErrDuplicateHandler = errors.New("handler with provided name already mapped")

	logg = logger.NewLogger()
)

type JobParams struct {
	Name    string
	Handler string
	Args    map[string]interface{}
}

type Handler func(map[string]interface{}) error

type worker struct {
	id                     string
	handlers               map[string]Handler
	stopChan               chan struct{}
	sleepBackoffsInSeconds []int64
}

func newWorker(sleepBackoffsInSeconds []int64) *worker {
	return &worker{
		id:                     makeIdentifier(),
		handlers:               make(map[string]Handler),
		stopChan:               make(chan struct{}),
		sleepBackoffsInSeconds: sleepBackoffsInSeconds,
	}
}

// registerHandler binds a name to a job handler.
func (w *worker) registerHandler(name string, handler Handler) error {
	if _, ok := w.handlers[name]; ok {
		return ErrDuplicateHandler
	}

	w.handlers[name] = handler

	return nil
}

// start starts the worker loop that pulls jobs from the queue & process them
func (w *worker) start() {
	go w.loop()
}

func (w *worker) stop() {
	w.stopChan <- struct{}{}
}

func (w *worker) loop() {
	var consequtiveNoJobs int64
	var currentJob *models.Job
	var err error

	sleepBackoffs := w.sleepBackoffsInSeconds
	rateLimiter := time.NewTicker(DefaultTickerDuration)
	defer rateLimiter.Stop()

	logg.Infof("Starting worker %s", w.id)
	for {
		select {
		case <-w.stopChan:
			logg.Infof("Stopping worker %s", w.id)
			return
		case <-rateLimiter.C:
			currentJob, err = models.FirstJob(models.ENQUEUED_JOB, false)
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					// If no job found, slowly increase the wait time between each job fetch
					// using 'sleepBackoffsInSeconds'. To reduce db hit when it's not necessary.
					consequtiveNoJobs++
					idx := consequtiveNoJobs
					if idx >= int64(len(sleepBackoffs)) {
						idx = int64(len(sleepBackoffs)) - 1
					}
					w.logInfof("no job in queue - sleep for %v seconds", sleepBackoffs[idx])
					rateLimiter.Reset(time.Duration(sleepBackoffs[idx]) * time.Second)
					continue
				}

				w.logError(err)
				rateLimiter.Reset(TickerDurationOnError)
				continue
			}

			claimed, err := currentJob.MarkAsClaimed()
			if err != nil {
				w.logError(err)
				rateLimiter.Reset(TickerDurationOnError)
				continue
			}

			w.logInfof("fetched job with id=%v, status_id=%v, claimed=%v, job.claimed=%v",
				currentJob.ID, currentJob.JobStatusID, claimed, currentJob.Claimed)

			if !claimed {
				continue
			}

			w.processJob(currentJob)
			rateLimiter.Reset(DefaultTickerDuration)
			consequtiveNoJobs = 0
		}
	}
}

func (w *worker) processJob(job *models.Job) {
	args := make(map[string]interface{})
	err := json.Unmarshal([]byte(job.Args), &args)
	if err != nil {
		logg.Error(err)
		w.determineFailedJobFate(job, err)
		return
	}

	err = w.handlers[job.Handler](args)
	if err != nil {
		w.logError(err)
		w.determineFailedJobFate(job, err)
		return
	}
	w.markJobAsSuccessful(job)
}

func (w *worker) determineFailedJobFate(job *models.Job, runError error) {
	var jobStatus *models.JobStatus
	var err error

	job.Fails++

	// For job with Fails >= MAX_FAILS mark as DEAD else requeue the job to be retried
	if job.Fails >= MAX_FAILS {
		jobStatus, err = models.FindJobStatus(models.DEAD_JOB)
	} else {
		jobStatus, err = models.FindJobStatus(models.ENQUEUED_JOB)
	}

	if err != nil {
		w.logError(err)
		return
	}

	// Unclaim job and update it with the necessary fail information
	err = job.Update(map[string]interface{}{
		"claimed":       false,
		"job_status_id": jobStatus.ID,
		"fails":         job.Fails,
		"last_error":    runError.Error(),
	})
	if err != nil {
		w.logError(err)
	}
	w.logInfof("job with id=%v completed with status=%v", job.ID, jobStatus.Name)
}

func (w *worker) markJobAsSuccessful(job *models.Job) {
	jobStatus, err := models.FindJobStatus(models.SUCCESSFUL_JOB)
	if err != nil {
		logg.Error(err)
		return
	}

	update := make(map[string]interface{})
	update["claimed"] = false
	update["job_status_id"] = jobStatus.ID

	err = job.Update(update)
	if err != nil {
		w.logError(err)
	}
	w.logInfof("job with id=%v completed with status=%v", job.ID, jobStatus.Name)
}

func (w *worker) logInfof(template string, args ...interface{}) {
	prefix := colors.Yellow((fmt.Sprintf("[worker %v] ", w.id)))
	logg.Infof(prefix+template, args...)
}

func (w *worker) logError(args ...interface{}) {
	prefix := colors.Red((fmt.Sprintf("[worker %v] ", w.id)))
	logg.Errorf(prefix, args...)
}
