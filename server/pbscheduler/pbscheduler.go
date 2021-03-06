package pbscheduler

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/Daskott/kronus/server/logger"
	"github.com/Daskott/kronus/server/models"
	"github.com/Daskott/kronus/server/twilio"
	"github.com/Daskott/kronus/server/work"
	"gorm.io/gorm"
)

const (
	SEND_LIVELINESS_PROBE_HANDLER   = "send_liveliness_probe"
	SEND_FOLLOWUP_PROBE_HANDLER     = "send_followup_probe"
	SEND_EMERGENCY_PROBE_HANDLER    = "send_emergency_probe"
	ENQUEUE_FOLLOWUP_PROBES_HANDLER = "enqueue_followup_probes"
	SEND_DYNAMIC_PROBE_HANDLER      = "send_dynamic_probe"
)

var logg = logger.NewLogger()

type ProbeScheduler struct {
	workerPoolAdapter        *work.WorkerPoolAdapter
	messageClient            *twilio.ClientWrapper
	followProbesCronSchedule string
}

// NewProbeScheduler creates new probe scheduler
func NewProbeScheduler(
	workerPoolAdapter *work.WorkerPoolAdapter,
	msgClient *twilio.ClientWrapper,
	followProbesCronSchedule string,
) (*ProbeScheduler, error) {
	probeScheduler := ProbeScheduler{
		followProbesCronSchedule: followProbesCronSchedule,
		workerPoolAdapter:        workerPoolAdapter,
		messageClient:            msgClient,
	}

	err := probeScheduler.registerWorkerHandlers()
	if err != nil {
		return nil, err
	}

	return &probeScheduler, nil
}

// PeriodicallyPerfomProbe creates 'liveliness probe' cron jobs for user.
// And when each cron is triggered, the job is sent to a job to be executed.
func (pbs ProbeScheduler) PeriodicallyPerfomProbe(user models.User) error {
	// Try updating the user's probe job schedule if one is already running
	// Else add a new one to the the scheduler
	err := pbs.workerPoolAdapter.UpdateJobScheduleByTag(probeName(user.ID), user.ProbeSettings.CronExpression)
	if err == work.ErrJobNotFoundInCronSch {
		err = pbs.workerPoolAdapter.PeriodicallyPerform(user.ProbeSettings.CronExpression, work.JobParams{
			Name:    probeName(user.ID),
			Handler: SEND_LIVELINESS_PROBE_HANDLER,
			Args: map[string]interface{}{
				"user_id":    user.ID,
				"first_name": user.FirstName,
				"last_name":  user.LastName,
			},
		})
	}

	return err
}

// DisablePeriodicProbe removes probe from scheduler & disables probe in user settings
func (pbs ProbeScheduler) DisablePeriodicProbe(user *models.User) error {
	pbs.workerPoolAdapter.RemovePeriodicJob(probeName(user.ID))
	return user.DisableLivlinessProbe()
}

// ScheduleProbes adds probes to cron scheduler,
// as well as check-ins for possible followup probes
func (pScheduler ProbeScheduler) ScheduleProbes() {
	err := pScheduler.initUsersPeriodicProbes()
	if err != nil {
		logg.Panic(err)
	}

	err = pScheduler.initPeriodicFollowupProbesEnqeuer()
	if err != nil {
		logg.Panic(err)
	}
}

// EmergencyProbeName returns the string used as tag for an emergency probe job name
func EmergencyProbeName(userID interface{}) string {
	return fmt.Sprintf("%v-%v", SEND_EMERGENCY_PROBE_HANDLER, userID)
}

// Creates 'liveliness probe' cron jobs for users with 'active' probe_settings.
// And when each cron is triggered, the job is sent to a queue to be executed.
func (pScheduler ProbeScheduler) initUsersPeriodicProbes() error {
	users, err := models.UsersWithActiveProbe()
	if err != nil {
		return err
	}

	for _, user := range users {
		err = pScheduler.PeriodicallyPerfomProbe(user)
		if err != nil {
			return err
		}
	}
	logg.Infof("%v liveliness probe(s) cron scheduled", len(users))

	return nil
}

// Creates 'followup probe' cron jobs for users with 'pending' liveliness probes.
// And when each cron is triggered, followup jobs are sent to a queue to be executed.
func (pbs ProbeScheduler) initPeriodicFollowupProbesEnqeuer() error {
	return pbs.workerPoolAdapter.PeriodicallyPerform(pbs.followProbesCronSchedule, work.JobParams{
		Name:    ENQUEUE_FOLLOWUP_PROBES_HANDLER,
		Handler: ENQUEUE_FOLLOWUP_PROBES_HANDLER,
		Args:    map[string]interface{}{},
	})
}

func (pScheduler ProbeScheduler) enqueueFollowUpsForProbes(params map[string]interface{}) error {
	noOfEmergencyProbeJobsQueued := 0
	noOfFollowupProbeJobsQueued := 0
	probes, err := models.FetchPendingProbesWithElapsedWait()
	if err != nil {
		logg.Error(err)
		return nil
	}

	for _, probe := range probes {
		jobArgs := make(map[string]interface{})
		jobArgs["user_id"] = probe.UserID
		jobArgs["probe_id"] = probe.ID

		// if max retries is exceeded, send emergency probe
		if probe.RetryCount >= probe.MaxRetries {
			jobArgs["probe_status"] = models.UNAVAILABLE_PROBE

			err = pScheduler.workerPoolAdapter.Perform(work.JobParams{
				Name:    EmergencyProbeName(probe.UserID),
				Handler: SEND_EMERGENCY_PROBE_HANDLER,
				Args:    jobArgs,
			})

			if err != nil {
				logg.Error(err)
			}

			noOfEmergencyProbeJobsQueued++
			continue
		}

		err = pScheduler.workerPoolAdapter.Perform(work.JobParams{
			Name:    followupProbeName(probe.UserID),
			Handler: SEND_FOLLOWUP_PROBE_HANDLER,
			Args:    jobArgs,
		})

		if err != nil {
			logg.Error(err)
			continue
		}
		noOfFollowupProbeJobsQueued++
	}

	logg.Infof("%v pending probe(s), %v emergency probe job(s) queued, %v followup probe job(s) queued",
		len(probes), noOfEmergencyProbeJobsQueued, noOfFollowupProbeJobsQueued)

	return nil
}

func (pScheduler ProbeScheduler) sendMessage(to, msg string) error {
	return pScheduler.messageClient.SendMessage(to, msg)
}

// ---------------------------------------------------------------------------------//
// Tasks
// --------------------------------------------------------------------------------//

func (pScheduler ProbeScheduler) sendLivelinessProbe(params map[string]interface{}) error {
	user, err := models.FindUserBy("id", params["user_id"])
	if err != nil {
		return err
	}

	if !user.ProbeSettings.Active {
		logg.Infof("skipping liveliness probe for userID=%v, it's currently disabled", params["user_id"])
		return nil
	}

	lastProbe, err := user.LastProbe()
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		logg.Error(err)
		return err
	}

	if lastProbe != nil {
		isPendingProbe, err := lastProbe.IsPending()
		if err != nil {
			return err
		}

		if isPendingProbe {
			logg.Infof("skipping current probe for userID=%v, last liveliness probe is still pending probeID=%v",
				params["user_id"], lastProbe.ID)
			return nil
		}
	}

	msg := fmt.Sprintf("Hi %v,\n"+
		"Just your friendly check in ????. Are you good ? (Y/N)",
		strings.Title(params["first_name"].(string)))
	err = pScheduler.sendMessage(user.PhoneNumber, msg)
	if err != nil {
		logg.Error(err)
		return err
	}

	// Create record of initial probe msg sent to usser in db
	err = models.CreateProbe(user.ID, user.ProbeSettings.WaitTimeInMinutes, user.ProbeSettings.MaxRetries)
	if err != nil {
		logg.Error(err)
		return err
	}

	return nil
}

func (pScheduler ProbeScheduler) sendFollowupForProbe(params map[string]interface{}) error {
	user, err := models.FindUserBy("id", params["user_id"])
	if err != nil {
		return err
	}

	if !user.ProbeSettings.Active {
		logg.Infof("skipping followup probe for userID=%v, probe is currently disabled", params["user_id"])
		return nil
	}

	probe, err := models.FindProbe(params["probe_id"])
	if err != nil {
		return err
	}

	msg := "You good ?? (Y/N)"
	err = pScheduler.sendMessage(user.PhoneNumber, msg)
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

func (pScheduler ProbeScheduler) sendEmergencyProbe(params map[string]interface{}) error {
	user, err := models.FindUserBy("id", params["user_id"])
	if err != nil {
		return err
	}

	// Set user liveliness probe status to params["probe_status"] i.e. 'unavailable' or 'bad'
	err = models.SetProbeStatus(params["probe_id"], params["probe_status"].(string))
	if err != nil {
		return err
	}

	emergencyContact, err := user.EmergencyContact()
	if err != nil {
		return err
	}

	// Default message is the 'unavailable message'
	message := fmt.Sprintf(
		"Hi %v,\n"+
			"you're getting this message becasue you're %v's emergency contact.\n"+
			"%v missed their last routine check in, please reach out to %v and make sure they're okay.",
		strings.Title(emergencyContact.FirstName), strings.Title(user.FirstName),
		strings.Title(user.FirstName), strings.Title(user.FirstName))

	// Otherwise send 'in trouble' msg for 'bad' probe
	if params["probe_status"] == models.BAD_PROBE {
		message = fmt.Sprintf(
			"Hi %v,\n"+
				"you're getting this message becasue you're %v's emergency contact.\n"+
				"%v just indicated they're not doing okay at the moment.\n"+
				"Please reach out ASAP to make sure all's okay.",
			strings.Title(emergencyContact.FirstName), strings.Title(user.FirstName),
			strings.Title(user.FirstName))
	}

	// Send message to emergency contact
	err = pScheduler.sendMessage(emergencyContact.PhoneNumber, message)
	if err != nil {
		return err
	}

	// Record emregency probe sent out
	err = models.CreateEmergencyProbe(params["probe_id"], emergencyContact.ID)
	if err != nil {
		logg.Error(err)
	}

	err = pScheduler.DisablePeriodicProbe(user)
	if err != nil {
		logg.Error(err)
	}

	err = pScheduler.sendMessage(
		user.PhoneNumber,
		fmt.Sprintf(
			"Reached out to %v. Liveliness probe is now disabled. You can always turn this back on via your kronus API.",
			strings.Title(emergencyContact.FirstName),
		),
	)
	if err != nil {
		logg.Error(err)
	}

	return nil
}

func (pScheduler ProbeScheduler) sendDynamicProbe(params map[string]interface{}) error {
	user, err := models.FindUserBy("id", params["user_id"])
	if err != nil {
		return err
	}

	msg := fmt.Sprintf("Hi %v,\n"+
		"You asked to check on you ????. Are you good ? (Y/N)",
		strings.Title(params["first_name"].(string)))
	err = pScheduler.sendMessage(user.PhoneNumber, msg)
	if err != nil {
		logg.Error(err)
		return err
	}

	// By default use the user's probe_settings
	waitTimeInMinutes, err := strconv.Atoi(fmt.Sprint(params["wait_time_in_minutes"]))
	if err != nil {
		waitTimeInMinutes = user.ProbeSettings.WaitTimeInMinutes
		logg.Warnf("Unabe to use 'wait_time_in_minutes' params: %v", err)
	}

	maxRetries, err := strconv.Atoi(fmt.Sprint(params["max_retries"]))
	if err != nil {
		maxRetries = user.ProbeSettings.MaxRetries
		logg.Warnf("Unabe to use 'max_retries' params: %v", err)
	}

	// Create record of probe msg sent to user in db
	err = models.CreateProbe(user.ID, waitTimeInMinutes, maxRetries)
	if err != nil {
		logg.Error(err)
		return err
	}

	return nil
}

// ---------------------------------------------------------------------------------//
// Helper functions
// --------------------------------------------------------------------------------//

func (probeScheduler *ProbeScheduler) registerWorkerHandlers() error {
	err := probeScheduler.workerPoolAdapter.Register(SEND_LIVELINESS_PROBE_HANDLER, probeScheduler.sendLivelinessProbe)
	if err != nil {
		return err
	}

	err = probeScheduler.workerPoolAdapter.Register(SEND_FOLLOWUP_PROBE_HANDLER, probeScheduler.sendFollowupForProbe)
	if err != nil {
		return err
	}

	err = probeScheduler.workerPoolAdapter.Register(SEND_EMERGENCY_PROBE_HANDLER, probeScheduler.sendEmergencyProbe)
	if err != nil {
		return err
	}

	err = probeScheduler.workerPoolAdapter.Register(ENQUEUE_FOLLOWUP_PROBES_HANDLER, probeScheduler.enqueueFollowUpsForProbes)
	if err != nil {
		return err
	}

	err = probeScheduler.workerPoolAdapter.Register(SEND_DYNAMIC_PROBE_HANDLER, probeScheduler.sendDynamicProbe)
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
