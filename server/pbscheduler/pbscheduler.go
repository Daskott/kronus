package pbscheduler

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Daskott/kronus/colors"
	"github.com/Daskott/kronus/server/logger"
	"github.com/Daskott/kronus/server/models"
	"github.com/Daskott/kronus/server/work"
	"gorm.io/gorm"
)

const (
	MAX_PROBE_RETRIES               = 3
	SEND_LIVELINESS_PROBE_HANDLER   = "send_liveliness_probe"
	SEND_FOLLOWUP_PROBE_HANDLER     = "send_followup_probe"
	SEND_EMERGENCY_PROBE_HANDLER    = "send_emergency_probe"
	ENQUEUE_FOLLOWUP_PROBES_HANDLER = "enqueue_followup_probes"
)

var logg = logger.NewLogger()

type ProbeScheduler struct {
	workerPoolAdapter *work.WorkerPoolAdapter
}

// NewProbeScheduler creates new probe scheduler
func NewProbeScheduler(workerPoolAdapter *work.WorkerPoolAdapter) (*ProbeScheduler, error) {
	probeScheduler := ProbeScheduler{
		workerPoolAdapter: workerPoolAdapter,
	}

	err := registerWorkerHandlers(&probeScheduler)
	if err != nil {
		return nil, err
	}

	return &probeScheduler, nil
}

// PeriodicallyPerfomProbe creates 'liveliness probe' cron jobs for user.
// And when each cron is triggered, the job is sent to a job to be executed.
func (pbs ProbeScheduler) PeriodicallyPerfomProbe(user models.User) {
	pbs.workerPoolAdapter.PeriodicallyPerform(user.ProbeSettings.CronExpression, work.JobParams{
		Name:    probeName(user.ID),
		Handler: SEND_LIVELINESS_PROBE_HANDLER,
		Args: map[string]interface{}{
			"user_id":    user.ID,
			"first_name": user.FirstName,
			"last_name":  user.LastName,
		},
		Unique: true,
	})
}

// RemoveCronJob removes probe cron job from scheduler & cancels any pending probe for user
func (pbs ProbeScheduler) RemovePeriodicProbe(user *models.User) {
	pbs.workerPoolAdapter.RemovePeriodicJob(probeName(user.ID))
	user.CancelAllPendingProbes()
}

// ScheduleProbes adds probes to cron scheduler,
// as well as check-ins for possible followup probes
func (pScheduler ProbeScheduler) ScheduleProbes() {
	err := pScheduler.initUsersPeriodicProbes()
	if err != nil {
		logg.Panic(err)
	}

	pScheduler.initPeriodicFollowupProbesEnqeuer()
}

// Creates 'liveliness probe' cron jobs for users with 'active' probe_settings.
// And when each cron is triggered, the job is sent to a queue to be executed.
func (pScheduler ProbeScheduler) initUsersPeriodicProbes() error {
	users, err := models.UsersWithActiveProbe()
	if err != nil {
		return err
	}

	for _, user := range users {
		pScheduler.PeriodicallyPerfomProbe(user)
	}
	logg.Infof("%v liveliness probe(s) cron scheduled", len(users))

	return nil
}

// Creates 'followup probe' cron jobs for users with 'pending' liveliness probes.
// And when each cron is triggered i.e every 30mins, followup jobs are sent to a queue to be executed.
func (pbs ProbeScheduler) initPeriodicFollowupProbesEnqeuer() {
	pbs.workerPoolAdapter.PeriodicallyPerform("*/5 * * * *", work.JobParams{
		Name:    ENQUEUE_FOLLOWUP_PROBES_HANDLER,
		Handler: ENQUEUE_FOLLOWUP_PROBES_HANDLER,
		Args:    map[string]interface{}{},
		Unique:  true,
	})
}

func (pScheduler ProbeScheduler) enqueueFollowUpsForProbes(params map[string]interface{}) error {
	noOfFollowupProbeJobsQueued := 0
	probes, err := models.ProbesByStatus(models.PENDING_PROBE)
	if err != nil {
		logg.Error(err)
		return nil
	}

	for _, probe := range probes {
		jobArgs := make(map[string]interface{})
		jobArgs["user_id"] = probe.UserID
		jobArgs["probe_id"] = probe.ID

		// if max retries is exceeded, send emergency probe
		if probe.RetryCount >= MAX_PROBE_RETRIES {
			err = pScheduler.workerPoolAdapter.Perform(work.JobParams{
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

		err = pScheduler.workerPoolAdapter.Perform(work.JobParams{
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

	return nil
}

// ---------------------------------------------------------------------------------//
// Tasks
// --------------------------------------------------------------------------------//

func sendLivelinessProbe(params map[string]interface{}) error {
	enabled, err := isProbeEnabled(params["user_id"])
	if err != nil {
		return err
	}

	if !enabled {
		logg.Infof("skipping liveliness probe for userID=%v, it's currently disabled", params["user_id"])
		return nil
	}

	lastProbe, err := models.LastProbeForUser(params["user_id"])
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		logg.Error(err)
		return err
	}

	pendingProbeStatus, err := models.FindProbeStatus(models.PENDING_PROBE)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		logg.Error(err)
		return err
	}

	if lastProbe != nil && lastProbe.ProbeStatusID == pendingProbeStatus.ID {
		logg.Infof("skipping current probe for userID=%v, last liveliness probe is still pending probeID=%v",
			params["user_id"], lastProbe.ID)
		return nil
	}

	msg := fmt.Sprintf("Are you okay %v?", strings.Title(params["first_name"].(string)))
	err = sendMessage(msg)
	if err != nil {
		logg.Error(err)
		return err
	}

	// Create record of initial probe msg sent to usser in db
	err = models.CreateProbe(params["user_id"])
	if err != nil {
		logg.Error(err)
		return err
	}

	return nil
}

func sendFollowupForProbe(params map[string]interface{}) error {
	enabled, err := isProbeEnabled(params["user_id"])
	if err != nil {
		return err
	}

	if !enabled {
		logg.Infof("skipping followup probe for userID=%v, probe is currently disabled", params["user_id"])
		return nil
	}

	user, err := models.FindUserBy("id", params["user_id"])
	if err != nil {
		return err
	}

	probe, err := models.FindProbe(params["probe_id"])
	if err != nil {
		return err
	}

	msg := fmt.Sprintf("Are you okay %v??", strings.Title(user.FirstName))
	err = sendMessage(msg)
	if err != nil {
		return err
	}

	probe.RetryCount += 1
	err = probe.Save()
	if err != nil {
		return err
	}

	return nil
}

func sendEmergencyProbe(params map[string]interface{}) error {
	enabled, err := isProbeEnabled(params["user_id"])
	if err != nil {
		return err
	}

	if !enabled {
		logg.Infof("skipping emergency probe for userID=%v, probe is currently disabled", params["user_id"])
		return nil
	}

	// Set user liveliness probe status to 'unavailable'
	err = models.SetProbeStatus(params["probe_id"], "unavailable")
	if err != nil {
		return err
	}

	user, err := models.FindUserBy("id", params["user_id"])
	if err != nil {
		return err
	}

	emergencyContact, err := user.EmergencyContact()
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
	err = models.CreateEmergencyProbe(params["probe_id"], emergencyContact.ID)
	if err != nil {
		logg.Error(err)
	}

	return nil
}

// ---------------------------------------------------------------------------------//
// Helper functions
// --------------------------------------------------------------------------------//
func registerWorkerHandlers(probeScheduler *ProbeScheduler) error {
	err := probeScheduler.workerPoolAdapter.Register(SEND_LIVELINESS_PROBE_HANDLER, sendLivelinessProbe)
	if err != nil {
		return err
	}

	err = probeScheduler.workerPoolAdapter.Register(SEND_FOLLOWUP_PROBE_HANDLER, sendFollowupForProbe)
	if err != nil {
		return err
	}

	err = probeScheduler.workerPoolAdapter.Register(SEND_EMERGENCY_PROBE_HANDLER, sendEmergencyProbe)
	if err != nil {
		return err
	}

	err = probeScheduler.workerPoolAdapter.Register(ENQUEUE_FOLLOWUP_PROBES_HANDLER, probeScheduler.enqueueFollowUpsForProbes)
	if err != nil {
		return err
	}
	return nil
}

func probeName(userID interface{}) string {
	return fmt.Sprintf("%v-%v", SEND_LIVELINESS_PROBE_HANDLER, userID)
}

func followupProbeName(userID interface{}) string {
	return fmt.Sprintf("%v-%v", SEND_FOLLOWUP_PROBE_HANDLER, userID)
}

func emergencyProbeName(userID interface{}) string {
	return fmt.Sprintf("%v-%v", SEND_EMERGENCY_PROBE_HANDLER, userID)
}

func isProbeEnabled(userID interface{}) (bool, error) {
	pbSettings, err := models.FindProbeSettings(userID)
	if err != nil {
		return false, nil
	}

	return pbSettings.Active, nil
}

func sendMessage(message string) error {
	logg.Infof(fmt.Sprintf("%v %v", colors.Green("[message]"), message))
	return nil
}
