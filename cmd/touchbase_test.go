package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Daskott/kronus/googleservice"
	"github.com/spf13/cobra"
)

type TestDataProvider []struct {
	description string
	args        []string
	expectedOut string
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
			description: "Should fail when group flag is not provided",
			args:        []string{""},
			expectedOut: "\"group\" not set",
		},
		{
			description: "Should NOT create touchbase events for group that does not exist",
			args:        []string{"--group", "missing-group"},
			expectedOut: "no contacts in 'missing-group'",
		},
		{
			description: "Should create touchbase events for contacts in family group",
			args:        []string{"--group", "family"},
			expectedOut: "appointments with members of family have been created",
		},
		{
			description: "Should NOT create touchbase events with invaild count flag",
			args:        []string{"--group", "family", "--count", "m"},
			expectedOut: "invalid argument \"m\"",
		},
		{
			description: "Should create touchbase events with vaild count flag",
			args:        []string{"--group", "family", "--count", "4"}, // what if it's 0 ?
			expectedOut: "appointments with members of family have been created",
		},
		{
			description: "Should NOT create touchbase events with freq flag > 2",
			args:        []string{"--group", "family", "--freq", "3"},
			expectedOut: "should be 0, 1, or 2",
		},
		{
			description: "Should NOT create touchbase events with freq flag < 0",
			args:        []string{"--group", "family", "--freq", "-1"},
			expectedOut: "should be 0, 1, or 2",
		},
		{
			description: "Should create touchbase events with vaild freq flag",
			args:        []string{"--group", "family", "--freq", "0"},
			expectedOut: "appointments with members of family have been created",
		},
		{
			description: "Should NOT create touchbase events with invalid time-slot flag",
			args:        []string{"--group", "family", "--time-slot", "1:00-2"},
			expectedOut: "inavlid arg \"1:00-2\"",
		},
		{
			description: "Should create touchbase events with vaild time-slot flag",
			args:        []string{"--group", "family", "--time-slot", "1:00-1:30"},
			expectedOut: "appointments with members of family have been created",
		},
		// TODO: Test maxContactsToTochbaseWith
	}

	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			tbCmd = createTouchbaseCmd()

			// Clear output buffer before the next test
			buff.Reset()

			tbCmd.SetOut(buff)
			tbCmd.SetErr(buff)
			tbCmd.SetArgs(c.args)

			tbCmd.Execute()

			actualOut = buff.String()
			if !strings.Contains(actualOut, c.expectedOut) {
				t.Errorf("Expected: \n\"%s\" \nTo contain: \n\"%s\"", actualOut, c.expectedOut)
			}
		})
	}
}
