/*
Copyright © 2021 Edmond Cotterell

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	devConfig "github.com/Daskott/kronus/dev/config"
	"github.com/Daskott/kronus/googleservice"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const maxContactsToTochbaseWith = 7

var (
	countArg     int
	frequencyArg int
	groupArg     string
	timeSlotArg  string
	intervals    = []int{
		1, // weekly
		2, // bi-weekly
		4, // monthly
	}

	cfgFile   string
	googleAPI googleservice.GCalendarAPIInterface

	yellow       = color.New(color.FgYellow).SprintFunc()
	red          = color.New(color.FgRed).SprintFunc()
	warningLabel = yellow("Warning:")
)

func init() {
	rootCmd.AddCommand(createTouchbaseCmd())
}

func createTouchbaseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "touchbase",
		Short: "Deletes previous touchbase events and creates new ones based on configs",
		Long: `Deletes previous touchbase google calender events created by kronus
and creates new ones(up to a max of 7 contacts for a group) to match the values set in .kronus.yaml`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runtTouchbase(cmd, touchbaseConfig())
		},
	}

	cmd.Flags().IntVarP(&countArg, "count", "c", 4, "how many times you want to touchbase with members of a group")
	cmd.Flags().StringVarP(&groupArg, "group", "g", "", "group to create touchbase events for")
	cmd.Flags().IntVarP(&frequencyArg, "freq", "f", 1, "how often you want to touchbase i.e. 0 - weekly, 1 - bi-weekly, or 2 - monthly")
	cmd.Flags().StringVarP(&timeSlotArg, "time-slot", "t", "18:00-18:30", "time slot in the day allocated for touching base")
	cmd.Flags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.kronus.yaml)")

	cmd.MarkFlagRequired("group")

	return cmd
}

func runtTouchbase(cmd *cobra.Command, config *viper.Viper) error {
	initGCalendarAPI(config)

	err := validateFlags()
	if err != nil {
		return err
	}

	slotStartTime, slotEndTime := splitTimeSlot(timeSlotArg)

	eventRecurrence := eventRecurrence(config.GetString("settings.touchbase-recurrence"))

	selectedGroupContactIds := config.GetStringSlice(fmt.Sprintf("groups.%s", groupArg))
	if len(selectedGroupContactIds) == 0 {
		return fmt.Errorf("no contacts in '%s' group. Try creating '%s' and adding some contacts to it."+
			"\nUpdate app config in %s", groupArg, groupArg, config.ConfigFileUsed())
	}

	contacts := []googleservice.Contact{}
	err = config.UnmarshalKey("contacts", &contacts)
	cobra.CheckErr(err)

	groupContacts := filterContactsByIDs(contacts, selectedGroupContactIds)
	if len(groupContacts) == 0 {
		return fmt.Errorf("unable to find any contact details for members of '%s'"+
			"\nTry updating '%s' group in app config located in %s", groupArg, groupArg, config.ConfigFileUsed())
	}

	// Clear any events previously created by touchbase
	err = googleAPI.ClearAllEvents(config.GetStringSlice("events"))
	if err != nil {
		cmd.Printf("%s %v\n", warningLabel, err)
	}

	if len(groupContacts) > maxContactsToTochbaseWith {
		groupContacts = groupContacts[:maxContactsToTochbaseWith]
		cmd.Printf("%s Touchbase events are created for a Max of %v contacts."+
			"\nEvents will be created for ONLY the top %v contacts in '%s'."+
			"\nPlease update the group accordingly, if you'd like to create events for a different set of contacts.\n",
			warningLabel, maxContactsToTochbaseWith, len(groupContacts), groupArg)
	}

	eventIds, err := googleAPI.CreateEvents(
		groupContacts,
		slotStartTime,
		slotEndTime, eventRecurrence,
	)
	if err != nil {
		return err
	}

	// Save created eventIds to config file
	config.Set("events", eventIds)
	config.WriteConfig()

	cmd.Printf("\nAll touchbase appointments with members of %s have been created!\n", groupArg)

	return nil
}

func validateFlags() error {
	// TODO: Move these validations into custom typee later: https://github.com/spf13/cobra/issues/376
	if countArg <= 0 {
		return fmt.Errorf("inavlid argument \"%v\", --count must be > 0", countArg)
	}

	if frequencyArg < 0 || frequencyArg >= len(intervals) {
		return fmt.Errorf("inavlid argument \"%v\", --freq should be 0, 1, or 2", frequencyArg)
	}

	match, _ := regexp.MatchString("\\d{1,2}:\\d\\d-\\d{1,2}:\\d\\d", timeSlotArg)
	if !match {
		return fmt.Errorf("inavlid argument \"%v\", valid --time-slot format required e.g. 18:00-18:30", timeSlotArg)
	}
	return nil
}

func eventRecurrence(recurrence string) string {
	return recurrence +
		fmt.Sprintf("COUNT=%d;INTERVAL=%d;", countArg, intervals[frequencyArg])
}

func splitTimeSlot(timeSlotStr string) (string, string) {
	list := strings.Split(timeSlotStr, "-")
	return list[0], list[1]
}

func filterContactsByIDs(allContacts []googleservice.Contact, contactIds []string) []googleservice.Contact {
	result := []googleservice.Contact{}
	for index, contact := range allContacts {
		if inList(contactIds, fmt.Sprintf("%v", index)) {
			result = append(result, contact)
		}
	}
	return result
}

func inList(list []string, item string) bool {
	for _, value := range list {
		if value == item {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------------//
// Config Helpers
// --------------------------------------------------------------------------------//

// touchbaseConfig reads in config file and ENV variables & returns a single
// '*viper.Viper' config object
func touchbaseConfig() *viper.Viper {
	config := viper.New()

	if cfgFile != "" {
		// Use config file from the flag.
		config.SetConfigFile(cfgFile)
	} else {
		configName, configDir, err := defaulatCFgNameAndDir()
		cobra.CheckErr(err)

		// If config file is not found, create one using defaultConfigFileContent
		configFilePath := filepath.Join(configDir, configName)
		if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
			err = ioutil.WriteFile(configFilePath, []byte(devConfig.DEFAULT_TOUCHBASE_YML), 0600)
			cobra.CheckErr(err)
		}

		// Search config in home directory with name ".kronus" (without extension).
		config.AddConfigPath(configDir)
		config.SetConfigType("yaml")
		config.SetConfigName(configName)
	}

	// BIND secrets.GOOGLE... to GOOGLE_APPLICATION_CREDENTIALS env, so the value doesn't need to be
	// stored in the .kronus.yaml config, but can be read from the system ENV var.
	// FYI: The env var overrides whatever is in the config file
	config.BindEnv("secrets.GOOGLE_APPLICATION_CREDENTIALS", "GOOGLE_APPLICATION_CREDENTIALS")

	config.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := config.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", config.ConfigFileUsed())
	}

	return config
}

// TODO: Refactor this to return gcalAPI
func initGCalendarAPI(config *viper.Viper) {
	googleCredentials := config.GetString("secrets.GOOGLE_APPLICATION_CREDENTIALS")
	ownerEmail := config.GetString("owner.email")

	if googleCredentials == "" {
		cobra.CheckErr(formattedError(
			"must set the env var 'GOOGLE_APPLICATION_CREDENTIALS' or add it to 'secrets' in %s", config.ConfigFileUsed()))

	}

	if ownerEmail == "" {
		cobra.CheckErr(formattedError("must set 'owner.email' in %s", config.ConfigFileUsed()))
	}

	// No need to use real googleAPI in tests
	if isTestEnv {
		return
	}

	var err error
	googleAPI, err = googleservice.NewGoogleCalendarAPI(googleCredentials, ownerEmail)
	cobra.CheckErr(err)
}

func defaulatCFgNameAndDir() (configName string, configDir string, err error) {
	configName = ".kronus.yaml"

	// Use home directory for production
	configDir, err = os.UserHomeDir()
	if err != nil {
		return "", "", err
	}

	if isDevEnv || isTestEnv {
		configName = ".kronus.dev.yaml"
		configDir, err = os.Getwd()
		if err != nil {
			return "", "", err
		}

		if isTestEnv {
			configName = ".kronus.yaml"
			configDir = filepath.Join(configDir, "test-fixtures")
		}
	}

	return configName, configDir, err
}

func formattedError(format string, a ...interface{}) error {
	return fmt.Errorf(red(format), a...)
}
