package pbscheduler

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/Daskott/kronus/database"
	"github.com/Daskott/kronus/server/logger"
	"github.com/go-co-op/gocron"
	"gorm.io/gorm"
)

const (
	PENDING_PROBE     = "pending"
	UNAVAILABLE_PROBE = "unavailable"
	PROBE_PREFIX      = "probe"
	MAX_PROBE_RETRIES = 3
)

var logg = logger.NewLogger()

type ProbeScheduler struct {
	CronScheduler *gocron.Scheduler
}

func NewProbeScheduler(cronScheduler *gocron.Scheduler) *ProbeScheduler {
	probeScheduler := ProbeScheduler{CronScheduler: cronScheduler}
	probeScheduler.enqueFollowupJobsForProbes()

	return &probeScheduler
}

func (pScheduler ProbeScheduler) EnqueAllActiveProbes() error {
	users, err := database.UsersWithActiveProbe()
	if err != nil {
		return err
	}

	for _, user := range users {
		pScheduler.EnqueProbe(user)
	}
	logg.Infof("%v probe(s) enqued", len(users))

	return nil
}

func (pScheduler ProbeScheduler) EnqueProbe(user database.User) {
	pScheduler.CronScheduler.Cron(user.ProbeSettings.CronExpression).
		Tag(jobTag(user.ID)).Do(func() { sendLivelinessProbe(user) })
}

func (pScheduler ProbeScheduler) DequeProbe(tag string) {
	pScheduler.CronScheduler.RemoveByTag(tag)
}

func (pScheduler ProbeScheduler) enqueFollowupJobsForProbes() {
	pScheduler.CronScheduler.Every("30m").Do(sendFollowUpsForProbes)
}

// ---------------------------------------------------------------------------------//
// Helper functions
// --------------------------------------------------------------------------------//

func jobTag(userID interface{}) string {
	return fmt.Sprintf("%v_%v", PROBE_PREFIX, userID)
}

func sendLivelinessProbe(user database.User) error {
	lastProbe, err := database.LastProbe(user.ID)
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
		logg.Infof("skipping current probe for userID=%v, last liveliness probe is still pending", user.ID)
		return nil
	}

	msg := fmt.Sprintf("Are you okay %v?", user.FirstName)
	err = sendMessage(msg)
	if err != nil {
		logg.Error(err)
		return err
	}

	// Create record of initial probe msg sent to usser in db
	err = database.CreateProbe(user.ID)
	if err != nil {
		logg.Error(err)
		return err
	}

	return nil
}

func sendFollowupForProbe(probe database.Probe) error {
	msg := fmt.Sprintf("Are you okay %v??", probe.UserID)
	err := sendMessage(msg)
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

func sendEmergencyProbe(probe database.Probe) error {
	// Set user liveliness probe status to 'unavailable'
	err := database.SetProbeStatus("unavailable", &probe)
	if err != nil {
		return err
	}

	emergencyContact, err := database.EmergencyContact(probe.UserID)
	if err != nil {
		return err
	}

	user, err := database.FindUserBy("ID", probe.UserID)
	if err != nil {
		return err
	}

	message := fmt.Sprintf(
		"Hi %v,\n"+
			"you're getting this message becasue you're %v's"+
			"emergency contact. %v missed their check in, can you please reach out to make sure their okay?\n"+
			"Thanks",
		emergencyContact.FirstName, user.FirstName, user.FirstName)

	// Send message to emergency contact
	sendMessage(message)

	// Record emregency probe sent out
	// TODO: Retry, but in worst case scenario, it's okay to fail
	err = database.CreateEmergencyProbe(probe.ID, emergencyContact.ID)
	if err != nil {
		logg.Error(err)
	}

	// TODO: Turn off liveliness probe for user

	return nil
}

func sendFollowUpsForProbes() {
	noOfFollowupsSent := 0
	probes, err := database.ProbesByStatus(PENDING_PROBE)
	if err != nil {
		log.Printf("Error: followUpProbes:%v\n", err)
		return
	}

	for _, probe := range probes {
		if probe.RetryCount >= MAX_PROBE_RETRIES {
			go sendEmergencyProbe(probe)
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

		err = sendFollowupForProbe(probe)
		if err != nil {
			logg.Error(err)
			continue
		}
		noOfFollowupsSent++
	}

	logg.Infof("followups: %v pending probe(s) found", len(probes))
	logg.Infof("followups: %v probe followup messages(s) sent", noOfFollowupsSent)
}

func sendMessage(message string) error {
	logg.Infof(message)
	return nil
}
