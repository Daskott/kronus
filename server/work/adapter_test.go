package work

import (
	"bytes"
	"testing"
	"time"

	"github.com/Daskott/kronus/server/models"
	"github.com/stretchr/testify/assert"
)

func TestPerformIn(t *testing.T) {
	models.InitializeTestDb()

	workerPool := NewWorkerAdapter("UTC", true)
	outputBuffer := new(bytes.Buffer)
	outStr := outputBuffer.String()

	// Register job function
	writeToBuffer := func(m map[string]interface{}) error {
		_, err := outputBuffer.WriteString("Hello")
		return err
	}
	workerPool.Register("write_to_buffer", writeToBuffer)

	err := workerPool.PerformIn(2, JobParams{
		Name:    "write_to_buffer",
		Handler: "write_to_buffer",
		Args:    map[string]interface{}{},
	})
	assert.Nil(t, err)
	assert.Empty(t, outStr, "Expected outputBuffer to be empty")

	// Wait until time to perform job has elapsed
	time.Sleep(3 * time.Second)

	workerPool.Start()

	// Wait for job to be processed
	time.Sleep(2 * time.Second)

	workerPool.Stop()

	outStr = outputBuffer.String()
	assert.Equal(t, "Hello", outStr, "Expected job to write to outputBuffer")
}
