package pbscheduler

import (
	"fmt"
	"log"
	"time"

	"github.com/Daskott/kronus/colors"
	"github.com/Daskott/kronus/database"
	"github.com/go-co-op/gocron"
)

const (
	PENDING_PROBE     = "pending"
	UNAVAILABLE_PROBE = "unavailable"
	PROBE_PREFIX      = "probe"
	MAX_PROBE_RETRIES = 3
)

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
		pScheduler.EnqueProbe(user.ID)
	}
	log.Printf(colors.Blue("%v probe(s) enqued"), len(users))

	return nil
}

func (pScheduler ProbeScheduler) EnqueProbe(userID uint) {
	// TODO: Change to: scheduler.Every(1).Day().Monday().At("HH:MM:SS") > time & day from config
	pScheduler.CronScheduler.Every("5m").Tag(jobTag(userID)).Do(func() { sendInitialProbe(userID) })
}

func (pScheduler ProbeScheduler) DequeProbe(tag string) {
	pScheduler.CronScheduler.RemoveByTag(tag)
}

func (pScheduler ProbeScheduler) enqueFollowupJobsForProbes() {
	pScheduler.CronScheduler.Every("30m").Tag("follow_up_probes").Do(sendFollowUpsForProbes)
}

// ---------------------------------------------------------------------------------//
// Helper functions
// --------------------------------------------------------------------------------//

func jobTag(userID interface{}) string {
	return fmt.Sprintf("%v_%v", PROBE_PREFIX, userID)
}

func sendInitialProbe(userID uint) error {
	msg := fmt.Sprintf("Are you okay %v?", userID)
	err := sendMessage(msg)
	if err != nil {
		return fmt.Errorf("sendInitialProbe: %v", err)
	}

	// Create record of initial probe msg sent to usser in db
	err = database.CreateProbe(userID)
	if err != nil {
		return fmt.Errorf("sendInitialProbe: %v", err)
	}

	return nil
}

func sendFollowupForProbe(probe database.Probe) error {
	msg := fmt.Sprintf("Are you okay %v??", probe.UserID)
	err := sendMessage(msg)
	if err != nil {
		return fmt.Errorf("sendFollowupForProbe: %v", err)
	}

	probe.RetryCount += 1
	database.Save(&probe)
	if err != nil {
		return fmt.Errorf("sendFollowupForProbe: %v", err)
	}

	return nil
}

func sendEmergencyProbe(probe database.Probe) error {
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
	log.Println(message)

	// Set probe status to 'unavailable'
	// TODO: Retry instead of retuning error immediately
	err = database.SetProbeStatus("unavailable", &probe)
	if err != nil {
		return err
	}

	// Record emregency probe sent out
	// TODO: Retry, but in worst case scenario, it's okay to fail
	err = database.CreateEmergencyProbe(probe.ID, emergencyContact.ID)
	if err != nil {
		log.Println(err)
	}

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
			log.Println(err)
			continue
		}
		noOfFollowupsSent++
	}

	log.Printf(colors.Blue("%v pending probe(s) found"), len(probes))
	log.Printf(colors.Blue("%v probe followup messages(s) sent"), noOfFollowupsSent)
}

func sendMessage(message string) error {
	log.Println(message)
	return nil
}
