package pbscheduler

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Daskott/kronus/database"
	"github.com/Daskott/kronus/server/cron"
	"github.com/Daskott/kronus/server/logger"
	"github.com/Daskott/kronus/server/work"
	"github.com/go-co-op/gocron"
	"gorm.io/gorm"
)

const (
	PENDING_PROBE     = "pending"
	UNAVAILABLE_PROBE = "unavailable"
	MAX_PROBE_RETRIES = 3

	SEND_LIVELINESS_PROBE_HANDLER = "send_liveliness_probe"
	SEND_FOLLOWUP_PROBE_HANDLER   = "send_followup_probe"
	SEND_EMERGENCY_PROBE_HANDLER  = "send_emergency_probe"

	FOLLOWUP_PROBE_RECCURRING_CRON_TIME = "5m"
)

var logg = logger.NewLogger()

type ProbeScheduler struct {
	WorkerAdapter *work.WorkerAdapter
	cronScheduler *gocron.Scheduler
}

// NewProbeScheduler creates new probe scheduler
func NewProbeScheduler() *ProbeScheduler {
	probeScheduler := ProbeScheduler{
		cronScheduler: cron.CronScheduler,
		WorkerAdapter: work.NewWorkerAdapter(),
	}

	// Register worker handlers
	probeScheduler.WorkerAdapter.Register(SEND_LIVELINESS_PROBE_HANDLER, sendLivelinessProbe)
	probeScheduler.WorkerAdapter.Register(SEND_FOLLOWUP_PROBE_HANDLER, sendFollowupForProbe)
	probeScheduler.WorkerAdapter.Register(SEND_EMERGENCY_PROBE_HANDLER, sendEmergencyProbe)

	// Create cron jobs
	probeScheduler.setCronJobsForInitialProbes()
	probeScheduler.setCronJobsForFollowupProbes()

	return &probeScheduler
}

// AddCronJobForProbe creates 'liveliness probe' cron jobs for user.
// And when each cron is triggered, the job is sent to a job to be executed.
func (pScheduler ProbeScheduler) AddCronJobForProbe(user database.User) {
	pScheduler.cronScheduler.Cron(user.ProbeSettings.CronExpression).
		Tag(probeName(user.ID)).
		Do(
			func() {
				// Enqueue liveliness probe job for user when cron job is triggered
				err := pScheduler.WorkerAdapter.Perform(work.JobParams{
					Name:    probeName(user.ID),
					Handler: SEND_LIVELINESS_PROBE_HANDLER,
					Args: map[string]interface{}{
						"user_id":    user.ID,
						"first_name": user.FirstName,
						"last_name":  user.LastName,
					},
					Unique: true,
				})

				if err != nil {
					logg.Error(err)
				}
			},
		)
}

// RemoveCronJob removes cron job from scheduler
func (pScheduler ProbeScheduler) RemoveCronJob(tag string) {
	pScheduler.cronScheduler.RemoveByTag(tag)
}

// StartWorkers starts cron jobs and job workers
func (pScheduler ProbeScheduler) StartWorkers() {
	// Start cron jobs that handle enqueuing of 'probe' jobs
	pScheduler.cronScheduler.StartAsync()

	// Start worker that executes queued jobs
	pScheduler.WorkerAdapter.Start(context.TODO())
}

// StopWorkers safely stops all cron jobs and job workers
func (pScheduler ProbeScheduler) StopWorkers() {
	// Start cron jobs that handle enqueuing of 'probe' jobs
	pScheduler.cronScheduler.Stop()

	// Start worker that executes queued jobs
	pScheduler.WorkerAdapter.Stop()
}

// Creates 'liveliness probe' cron jobs for users with 'active' probe_settings.
// And when each cron is triggered, the job is sent to a queue to be executed.
func (pScheduler ProbeScheduler) setCronJobsForInitialProbes() error {
	users, err := database.UsersWithActiveProbe()
	if err != nil {
		return err
	}

	for _, user := range users {
		pScheduler.AddCronJobForProbe(user)
	}
	logg.Infof("%v liveliness probe(s) cron scheduled", len(users))

	return nil
}

// Creates 'followup probe' cron jobs for users with 'pending' liveliness probes.
// And when each cron is triggered i.e every 30mins, followup jobs are sent to a queue to be executed.
func (pScheduler ProbeScheduler) setCronJobsForFollowupProbes() {
	pScheduler.cronScheduler.Every(FOLLOWUP_PROBE_RECCURRING_CRON_TIME).
		Do(pScheduler.sendFollowUpsForProbes)
}

func (pScheduler ProbeScheduler) sendFollowUpsForProbes() {
	noOfFollowupProbeJobsQueued := 0
	probes, err := database.ProbesByStatus(PENDING_PROBE)
	if err != nil {
		logg.Error(err)
		return
	}

	for _, probe := range probes {
		jobArgs := make(map[string]interface{})
		jobArgs["user_id"] = probe.UserID
		jobArgs["probe_id"] = probe.ID

		// if max retries is exceeded, send emergency probe
		if probe.RetryCount >= MAX_PROBE_RETRIES {
			err = pScheduler.WorkerAdapter.Perform(work.JobParams{
				Name:    emergencyProbeName(probe.UserID),
				Handler: SEND_EMERGENCY_PROBE_HANDLER,
				Args:    jobArgs,
				Unique:  true,
			})

			if err != nil {
				logg.Error(err)
			}

			continue
		}

		// Only send out followup probes at least 1 hour after the last probe was sent
		// With each followup taking (1+ no. of retires) hours longer, so user doesn't get spammed
		// up until max-retries. So the user has enough time to respond
		//
		// E.g sendInitialProbe @ 5:OOpm
		// Follow up 1 will be @ ~6:00pm i.e (retryCount+1)hours where retryCount = 0
		// Follow up 2 will be @ ~8:00pm i.e (retryCount+1)hours where retryCount = 1
		// Follow up 3 will be @ ~11:00pm i.e (retryCount+1)hours where retryCount = 2
		if time.Since(probe.UpdatedAt) < time.Duration(probe.RetryCount+1)*time.Hour {
			continue
		}

		err = pScheduler.WorkerAdapter.Perform(work.JobParams{
			Name:    followupProbeName(probe.UserID),
			Handler: SEND_FOLLOWUP_PROBE_HANDLER,
			Args:    jobArgs,
			Unique:  true,
		})

		if err != nil {
			logg.Error(err)
			continue
		}
		noOfFollowupProbeJobsQueued++
	}

	logg.Infof("%v pending liveliness probe(s) found", len(probes))
	logg.Infof("%v followup probe job(s) queued", noOfFollowupProbeJobsQueued)
}

// ---------------------------------------------------------------------------------//
// Helper functions
// --------------------------------------------------------------------------------//

func probeName(userID interface{}) string {
	return fmt.Sprintf("%v-%v", SEND_LIVELINESS_PROBE_HANDLER, userID)
}

func followupProbeName(userID interface{}) string {
	return fmt.Sprintf("%v-%v", SEND_FOLLOWUP_PROBE_HANDLER, userID)
}

func emergencyProbeName(userID interface{}) string {
	return fmt.Sprintf("%v-%v", SEND_EMERGENCY_PROBE_HANDLER, userID)
}

func sendLivelinessProbe(params map[string]interface{}) error {
	lastProbe, err := database.LastProbeForUser(params["user_id"])
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		logg.Error(err)
		return err
	}

	pendingProbeStatus, err := database.FindProbeStatus(PENDING_PROBE)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		logg.Error(err)
		return err
	}

	if lastProbe != nil && lastProbe.ProbeStatusID == pendingProbeStatus.ID {
		logg.Infof("skipping current probe for userID=%v, last liveliness probe is still pending", params["user_id"])
		return nil
	}

	msg := fmt.Sprintf("Are you okay %v?", strings.Title(params["first_name"].(string)))
	err = sendMessage(msg)
	if err != nil {
		logg.Error(err)
		return err
	}

	// Create record of initial probe msg sent to usser in db
	err = database.CreateProbe(params["user_id"])
	if err != nil {
		logg.Error(err)
		return err
	}

	return nil
}

func sendFollowupForProbe(params map[string]interface{}) error {
	user, err := database.FindUserBy("id", params["user_id"])
	if err != nil {
		return err
	}

	probe, err := database.FindProbe(params["probe_id"])
	if err != nil {
		return err
	}

	msg := fmt.Sprintf("Are you okay %v??", strings.Title(user.FirstName))
	err = sendMessage(msg)
	if err != nil {
		return err
	}

	probe.RetryCount += 1
	database.Save(&probe)
	if err != nil {
		return err
	}

	return nil
}

func sendEmergencyProbe(params map[string]interface{}) error {
	// Set user liveliness probe status to 'unavailable'
	err := database.SetProbeStatus(params["probe_id"], "unavailable")
	if err != nil {
		return err
	}

	emergencyContact, err := database.EmergencyContact(params["user_id"])
	if err != nil {
		return err
	}

	user, err := database.FindUserBy("id", params["user_id"])
	if err != nil {
		return err
	}

	message := fmt.Sprintf(
		"Hi %v,\n"+
			"you're getting this message becasue you're %v's emergency contact.\n"+
			"%v missed their last routine check in, can you please reach out to %v\n"+
			"and make sure they're okay?\n"+
			"Thanks",
		strings.Title(emergencyContact.FirstName), strings.Title(user.FirstName),
		strings.Title(user.FirstName), strings.Title(user.FirstName))

	// Send message to emergency contact
	sendMessage(message)

	// Record emregency probe sent out
	// TODO: Retry, but in worst case scenario, it's okay to fail
	err = database.CreateEmergencyProbe(params["probe_id"], emergencyContact.ID)
	if err != nil {
		logg.Error(err)
	}

	return nil
}

func sendMessage(message string) error {
	logg.Infof(message)
	return nil
}
