package pbscheduler

import (
	"fmt"
	"testing"
	"time"

	"github.com/Daskott/kronus/server/models"
	"github.com/Daskott/kronus/server/twilio"
	"github.com/Daskott/kronus/server/work"
	"github.com/Daskott/kronus/shared"
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
	if err != nil {
		t.Error(err)
	}

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

	models.CreateUser(testUser)
	if testUser.ID == 0 {
		t.Error("Unable to create 'testUser' record")
	}
	testUser.UpdateProbSettings(map[string]interface{}{"active": true, "cron_expression": everySecondCronExp})

	models.CreateUser(testUser2)
	if testUser2.ID == 0 {
		t.Error("Unable to create 'testUser2' record")
	}
	testUser2.UpdateProbSettings(map[string]interface{}{"active": true, "cron_expression": everySecondCronExp})

	pbScheduler.ScheduleProbes()
	workerPool.Start()

	time.Sleep(3 * time.Second)

	jobs, _, err := models.FetchJobs(1)
	if err != nil {
		t.Errorf("Failed to fetch jobs: %v", err)
	}

	// Check # of jobs created match what is expected
	// i.e number of users with active probes + task to schedule follow up probes
	expectedNoOfJobs := 3
	if len(jobs) < expectedNoOfJobs {
		t.Errorf("Expected >= %v jobs to be queued, got %v", expectedNoOfJobs, len(jobs))
	}

	// Check that each user has a probe created in the database
	for _, user := range []*models.User{testUser, testUser2} {
		fmt.Println("Here")
		if probes, _, err := models.FetchProbes(1, "user_id = ?", user.ID); err != nil ||
			len(probes) < 1 {
			t.Errorf("Expected user: %v to have probe, found none: %v", user.FirstName, err)
		}
	}

	workerPool.Stop()
}
