package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Daskott/kronus/googleservice"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

type TestDataProvider []struct {
	args        []string
	expectedOut string
	msgError    string // message to display when the test fails
}

func TestTouchbaseCmd(t *testing.T) {
	var (
		tbCmd     *cobra.Command
		buff      = new(bytes.Buffer)
		actualOut string
	)

	// Save cfgFile before stubbing it out
	// And revert to prev cfgFile after test is done
	savedCfgFile := cfgFile
	defer func() {
		cfgFile = savedCfgFile
	}()

	// Set cfgFile to point to test config.yml
	path, _ := os.Getwd()
	cfgFile = filepath.Join(path, "test-fixtures", "config.yml")

	// Save googleAPI before stubbing it out
	// And revert to prev googleAPI after test is done
	saveGoogleAPI := googleAPI
	defer func() {
		googleAPI = saveGoogleAPI
	}()

	googleAPI = &googleservice.GCalendarAPIStub{}

	cases := TestDataProvider{
		{
			args:        []string{""},
			expectedOut: "\"group\" not set",
			msgError:    "Should fail because group flag is required",
		},
		{
			args:        []string{"--group", "family"},
			expectedOut: "appointments with members of family have been created",
			msgError:    "Should create touchbase events for contacts in test group",
		},
	}

	for _, c := range cases {
		tbCmd = createTouchbaseCmd()

		// Clear output buffer before the next test
		buff.Reset()

		tbCmd.SetOut(buff)
		tbCmd.SetErr(buff)
		tbCmd.SetArgs(c.args)

		tbCmd.Execute()

		actualOut = buff.String()
		assert.True(t, strings.Contains(actualOut, c.expectedOut), c.msgError)
	}
}
