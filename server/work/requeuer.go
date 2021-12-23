package work

import (
	"errors"
	"time"

	"github.com/Daskott/kronus/colors"
	"github.com/Daskott/kronus/server/models"
	"gorm.io/gorm"
)

type requeuer struct {
	stopChan chan struct{}
}

func newRequeuer() *requeuer {
	return &requeuer{
		stopChan: make(chan struct{}),
	}
}

// Start starts the requeuer loop that pulls jobs from 'in-progress'
// that are stuck(i.e stayed too long in-progress) and requeue them
func (r *requeuer) start() {
	go r.loop()
}

func (r *requeuer) stop() {
	r.stopChan <- struct{}{}
}

func (r *requeuer) loop() {
	var stuckJob *models.Job
	var err error

	sleepBackOff := 30
	rateLimiter := time.NewTicker(DefaultTickerDuration)
	defer rateLimiter.Stop()

	logg.Infof("Starting job requeuer")
	for {
		select {
		case <-r.stopChan:
			logg.Infof("Stopping job requeuer")
			return
		case <-rateLimiter.C:
			stuckJob, err = models.LastJobLastUpdated(30, models.IN_PROGRESS_JOB)

			// If no stuck job found, sleep for 'sleepBackOff' minutes
			if errors.Is(err, gorm.ErrRecordNotFound) {
				r.logInfof("no stuck job in in-progress - sleep for %v minutes", sleepBackOff)
				rateLimiter.Reset(time.Duration(sleepBackOff) * time.Minute)
				continue
			}

			if err != nil {
				r.logError(err)
				rateLimiter.Reset(TickerDurationOnError)
				continue
			}

			r.logInfof("fetched job with id=%v, status_id=%v, job.claimed=%v",
				stuckJob.ID, stuckJob.JobStatusID, stuckJob.Claimed)

			r.requeue(stuckJob)
			rateLimiter.Reset(DefaultTickerDuration)
		}
	}
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

	err = job.Update(update)
	if err != nil {
		r.logError(err)
	}

	r.logInfof("job with id=%v requeued", job.ID)
}

func (r *requeuer) logInfof(template string, args ...interface{}) {
	prefix := colors.Yellow("[job requeuer] ")
	logg.Infof(prefix+template, args...)
}

func (r *requeuer) logError(args ...interface{}) {
	prefix := colors.Red("[job requeuer] ")
	logg.Errorf(prefix, args...)
}
