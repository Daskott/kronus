package pbscheduler

import (
	"fmt"
	"testing"
	"time"

	"github.com/Daskott/kronus/server/models"
	"github.com/Daskott/kronus/server/twilio"
	"github.com/Daskott/kronus/server/work"
	"github.com/Daskott/kronus/shared"
	"github.com/stretchr/testify/assert"
)

func TestScheduleProbes(t *testing.T) {
	models.InitializeTestDb()

	everySecondCronExp := "*/1 * * * * *"
	workerPool := work.NewWorkerAdapter("UTC", true)

	pbScheduler, err := NewProbeScheduler(
		workerPool,
		twilio.NewClient(shared.TwilioConfig{}, "", true),
		everySecondCronExp,
	)
	assert.Nil(t, err)

	testUser := &models.User{
		FirstName:   "tony",
		LastName:    "stark",
		Email:       "stark@avengers.com",
		Password:    "very-secure",
		PhoneNumber: "+12345678900",
	}

	testUser2 := &models.User{
		FirstName:   "spider",
		LastName:    "man",
		Email:       "web@avengers.com",
		Password:    "secure???",
		PhoneNumber: "+22345678900",
	}

	testUser2Contact := &models.Contact{
		FirstName:          "doctor",
		LastName:           "strange",
		PhoneNumber:        "+32345678900",
		IsEmergencyContact: true,
		Email:              "supreme@avengers.com",
	}

	err = models.CreateUser(testUser)
	assert.Nil(t, err, "Should create 'testUser' record")

	err = models.CreateUser(testUser2)
	assert.Nil(t, err, "Should create 'testUser2' record")

	err = testUser.UpdateProbSettings(map[string]interface{}{"active": true, "cron_expression": everySecondCronExp})
	assert.Nil(t, err)

	err = testUser2.UpdateProbSettings(map[string]interface{}{"active": true, "cron_expression": everySecondCronExp})
	assert.Nil(t, err)

	err = testUser2.AddContact(testUser2Contact)
	assert.Nil(t, err, "Should create 'testUser2' Contact")

	// ScheduleProbes & start job worker to process probes
	pbScheduler.ScheduleProbes()
	workerPool.Start()

	time.Sleep(4 * time.Second)

	// ---------------------------------------------------------------------------------//
	// Test initial probe(s) are sent
	// --------------------------------------------------------------------------------//

	testCases := []struct {
		user                 models.User
		expectedProbeCount   int
		expectedProbeRetries int
		respondToProbe       bool
	}{
		{*testUser, 1, 0, true},
		{*testUser2, 1, 0, false},
	}

	for _, tcase := range testCases {
		desc := fmt.Sprintf("User %v shoud have 1 probe recorded in db", tcase.user.FirstName)

		t.Run(desc, func(t *testing.T) {
			probes, _, err := models.FetchProbes(1, "user_id = ?", tcase.user.ID)
			if err != nil {
				t.Fatalf("could not fetch probes: %v", err)
			}

			if len(probes) != tcase.expectedProbeCount {
				t.Errorf("Expected to have %v probe, found none: %v", tcase.expectedProbeCount, len(probes))
			}

			probe := probes[0]
			if probe.RetryCount != tcase.expectedProbeRetries {
				t.Errorf("Expected user to have %v probe retries, got %v",
					tcase.expectedProbeRetries, probe.RetryCount)
			}

			// Setup for next test
			if tcase.respondToProbe {
				probe.LastResponse = "Yeah"
				probeStatusName := probe.StatusFromLastResponse()
				probeStatus, err := models.FindProbeStatus(probeStatusName)
				assert.Nil(t, err, "Should fetch probe status")

				probe.ProbeStatusID = probeStatus.ID
				probe.Save()
			} else {
				// Set probe 'update_at' time to 1hr behind, to simulate 1hr of
				// waiting with no respons fro user
				probe.Update(map[string]interface{}{"updated_at": probe.UpdatedAt.Add(-time.Hour)})
			}
		})
	}

	time.Sleep(2 * time.Second)

	// ---------------------------------------------------------------------------------//
	// Test followup probe(s) are sent
	// --------------------------------------------------------------------------------//

	testCases2 := []struct {
		user                 models.User
		expectedProbeRetries int
	}{
		{*testUser, 0},
		{*testUser2, 1},
	}

	for _, tcase := range testCases2 {
		desc := fmt.Sprintf("User %v shoud have %v followup probe recorded in db",
			tcase.user.FirstName, tcase.expectedProbeRetries)

		t.Run(desc, func(t *testing.T) {
			probes, _, err := models.FetchProbes(1, "user_id = ?", tcase.user.ID)
			assert.Nil(t, err)

			probe := probes[0]
			assert.Equal(t, tcase.expectedProbeRetries, probe.RetryCount)

			// Set probe retries to max_retries to simulate
			// no reply from user with > 1 followup [setup for next test]
			if tcase.expectedProbeRetries > 0 {
				err = probe.Update(map[string]interface{}{
					"retry_count": probe.MaxRetries,
					"updated_at":  probe.UpdatedAt.Add(-time.Hour)})
				assert.Nil(t, err)

			}
		})
	}

	time.Sleep(2 * time.Second)

	// ---------------------------------------------------------------------------------//
	// Test emergency probe(s) are sent
	// --------------------------------------------------------------------------------//

	testCases3 := []struct {
		user                         models.User
		expectedToHaveEmergencyProbe bool
		expectedEmergencyContact     *models.Contact
	}{
		{*testUser, false, nil},
		{*testUser2, true, testUser2Contact},
	}

	for _, tcase := range testCases3 {
		msgPatch := ""
		if !tcase.expectedToHaveEmergencyProbe {
			msgPatch = " NOT"
		}

		desc := fmt.Sprintf("User %v shoud%v have emergency probe recorded in db",
			tcase.user.FirstName, msgPatch)

		t.Run(desc, func(t *testing.T) {
			probes, _, err := models.FetchProbes(1, "user_id = ?", tcase.user.ID)
			assert.Nil(t, err)

			probe := probes[0]
			if !tcase.expectedToHaveEmergencyProbe && probe.EmergencyProbe != nil {
				t.Fatalf("Expected user to have emergency probe recorded, found: %v", probe.EmergencyProbe)
			}

			if tcase.expectedToHaveEmergencyProbe {
				if probe.EmergencyProbe == nil {
					t.Fatalf("Expected user to have emergency probe recorded, found none. Probe: %v", probe)
				}

				if probe.EmergencyProbe.ContactID != tcase.expectedEmergencyContact.ID {
					t.Fatalf("Expected emergency probe to be sent to contact with ID=%v, got sent to contact with ID=%v",
						tcase.expectedEmergencyContact.ID, probe.EmergencyProbe.ContactID)
				}
			}
		})
	}

	workerPool.Stop()
}
