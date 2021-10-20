package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTouchbaseCmdWithMissingRequiredFlag(t *testing.T) {
	tbCmd := createTouchbaseCmd()

	err := tbCmd.Execute()
	assert.Equal(t, "required flag(s) \"group\" not set", err.Error(),
		"touchbaseCmd shoud fail if --group is not provided")
}
