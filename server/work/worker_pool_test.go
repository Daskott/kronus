package work

import (
	"testing"
	"time"

	"github.com/Daskott/kronus/server/models"
	"github.com/stretchr/testify/assert"
)

func TestEnqueueIn(t *testing.T) {
	models.InitializeTestDb()

	workerPool, err := newWorkerPool(MAX_CONCURRENCY)
	assert.Nil(t, err)

	err = workerPool.enqueueIn(1, JobParams{
		Name:    "suits",
		Handler: "donna",
		Args: map[string]interface{}{
			"first_name": "mike",
			"last_name":  "ross",
		},
	})
	assert.Nil(t, err)

	// At some point we need to be able to
	// mock the current time, instead of stopping the
	// process. For now, keep it simple
	time.Sleep(1 * time.Second)

	// Make sure the correct job is created & scheduled to be run
	job, err := models.FirstScheduledJobToBeQueued()
	assert.Nil(t, err)
	assert.Equal(t, "suits", job.Name, "The job name should match the expected job name")
	assert.Contains(t, job.Args, "mike", "Should contain the correct arg values")
	assert.Equal(t, models.SCHEDULED_JOB, job.JobStatus.Name, "The job should be in scheduled queue")
}
