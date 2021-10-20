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
	"regexp"
	"strings"

	"github.com/Daskott/kronus/types"
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
		Run: func(cmd *cobra.Command, args []string) {
			syncEvents()
		},
	}

	cmd.Flags().IntVarP(&countArg, "count", "c", 4, "How many times you want to touchbase with members of a group")
	cmd.Flags().StringVarP(&groupArg, "group", "g", "", "Group to create touchbase events for")
	cmd.Flags().IntVarP(&frequencyArg, "freq", "f", 1, "How often you want to touchbase i.e. 0 - weekly, 1 - bi-weekly, or 2 - monthly")
	cmd.Flags().StringVarP(&timeSlotArg, "time-slot", "t", "18:00-18:30", "Time slot in the day allocated for touching base")

	cmd.MarkFlagRequired("group")

	return cmd
}

func syncEvents() {
	err := validateFlags()
	cobra.CheckErr(err)

	slotStartTime, slotEndTime := splitTimeSlot(timeSlotArg)

	eventRecurrence := eventRecurrence()

	selectedGroupContactIds := viper.GetStringSlice(fmt.Sprintf("groups.%s", groupArg))
	if len(selectedGroupContactIds) == 0 {
		fmt.Printf("\nNo contacts in '%s' group. Try creating '%s' and adding some contacts to it."+
			"\nUpdate app config in %s\n", groupArg, groupArg, viper.ConfigFileUsed())
		return
	}

	contacts := []types.Contact{}
	err = viper.UnmarshalKey("contacts", &contacts)
	cobra.CheckErr(err)

	groupContacts := filterContactsByIDs(contacts, selectedGroupContactIds)
	if len(groupContacts) == 0 {
		fmt.Printf("\nUnable to find any contact details for members of '%s'."+
			"\nTry updating '%s' group in app config located in %s\n", groupArg, groupArg, viper.ConfigFileUsed())
		return
	}

	// Clear any events previously created by touchbase
	err = googleAPI.ClearAllEvents(viper.GetStringSlice("events"))
	if err != nil {
		fmt.Printf("%s %v\n", warningLabel, err)
	}

	if len(groupContacts) > maxContactsToTochbaseWith {
		groupContacts = groupContacts[:maxContactsToTochbaseWith]
		fmt.Printf("%s Touchbase events are created for a Max of %v contacts."+
			"\nEvents will be created for ONLY the top 7 contacts in '%s'."+
			"\nPlease update the group accordingly, if you'd like to create events for a different set of contacts.\n",
			warningLabel, maxContactsToTochbaseWith, groupArg)
	}

	eventIds, err := googleAPI.CreateEvents(
		groupContacts,
		slotStartTime,
		slotEndTime, eventRecurrence,
	)
	cobra.CheckErr(err)

	// Save created eventIds to config file
	viper.Set("events", eventIds)
	viper.WriteConfig()

	fmt.Printf("\nAll touchbase appointments with members of %s have been created!\n", groupArg)
}

func validateFlags() error {
	// TODO: Move these validations into custom typee later: https://github.com/spf13/cobra/issues/376
	if frequencyArg < 0 || frequencyArg >= len(intervals) {
		return fmt.Errorf("--freq should be 0, 1, or 2.\nTry `kronus touchbase --help` for more information")
	}

	match, _ := regexp.MatchString("\\d{1,2}:\\d\\d-\\d{1,2}:\\d\\d", timeSlotArg)
	if !match {
		return fmt.Errorf("proper --time-slot format required e.g. 18:00-18:30")
	}
	return nil
}

func eventRecurrence() string {
	return viper.GetString("settings.touchbase-recurrence") +
		fmt.Sprintf("COUNT=%d;INTERVAL=%d;", countArg, intervals[frequencyArg])
}

func splitTimeSlot(timeSlotStr string) (string, string) {
	list := strings.Split(timeSlotStr, "-")
	return list[0], list[1]
}

func filterContactsByIDs(allContacts []types.Contact, contactIds []string) []types.Contact {
	result := []types.Contact{}
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
