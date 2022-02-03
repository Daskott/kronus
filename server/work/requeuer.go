package work

import (
	"errors"
	"fmt"
	"time"

	"github.com/Daskott/kronus/colors"
	"github.com/Daskott/kronus/server/models"
	"gorm.io/gorm"
)

type requeuer struct {
	fromQueue string
	stopChan  chan struct{}
}

var supportedQueues = map[string]bool{models.IN_PROGRESS_JOB: true, models.SCHEDULED_JOB: true}

func newRequeuer(fromQueue string) (*requeuer, error) {
	if !supportedQueues[fromQueue] {
		return nil, fmt.Errorf("%v is not a supported queue, must be in %v", fromQueue, supportedQueues)
	}

	return &requeuer{
		fromQueue: fromQueue,
		stopChan:  make(chan struct{}),
	}, nil
}

// start starts the requeuer loop that pulls jobs from 'in-progress'
// that are stuck(i.e stayed too long in-progress) and requeue them
func (r *requeuer) start() {
	go r.loop()
}

func (r *requeuer) stop() {
	r.stopChan <- struct{}{}
}

func (r *requeuer) loop() {
	var job *models.Job
	var err error

	// At some point we may need an expnential back-off,
	// but for now keep it simple
	sleepBackOff := 5
	rateLimiter := time.NewTicker(DefaultTickerDuration)
	defer rateLimiter.Stop()

	logg.Infof("Starting %s job requeuer", r.fromQueue)
	for {
		select {
		case <-r.stopChan:
			logg.Infof("Stopping %s job requeuer", r.fromQueue)
			return
		case <-rateLimiter.C:
			job, err = r.nextJob()

			// If no job found, sleep for 'sleepBackOff' seconds
			if errors.Is(err, gorm.ErrRecordNotFound) {
				rateLimiter.Reset(time.Duration(sleepBackOff) * time.Second)
				continue
			}

			if err != nil {
				r.logError(err)
				rateLimiter.Reset(TickerDurationOnError)
				continue
			}

			r.logInfof("fetched job with id=%v, status_id=%v, job.claimed=%v",
				job.ID, job.JobStatusID, job.Claimed)

			r.requeue(job)
			rateLimiter.Reset(DefaultTickerDuration)
		}
	}
}

func (r *requeuer) nextJob() (*models.Job, error) {
	if r.fromQueue == models.IN_PROGRESS_JOB {
		return models.LastJobLastUpdated(10, models.IN_PROGRESS_JOB)
	}
	return models.FirstScheduledJobToBeQueued()
}

func (r *requeuer) requeue(job *models.Job) {
	jobStatus, err := models.FindJobStatus(models.ENQUEUED_JOB)
	if err != nil {
		logg.Error(err)
		return
	}

	update := make(map[string]interface{})
	update["claimed"] = false
	update["job_status_id"] = jobStatus.ID
	update["enqueued_at"] = time.Now()

	err = job.Update(update)
	if err != nil {
		r.logError(err)
	}

	r.logInfof("job with id=%v requeued", job.ID)
}

func (r *requeuer) logInfof(template string, args ...interface{}) {
	prefix := colors.Yellow(fmt.Sprintf("[%s job requeuer] ", r.fromQueue))
	logg.Infof(prefix+template, args...)
}

func (r *requeuer) logError(args ...interface{}) {
	prefix := colors.Red(fmt.Sprintf("[%s job requeuer] ", r.fromQueue))
	logg.Errorf(prefix, args...)
}
