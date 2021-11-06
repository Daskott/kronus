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

	"github.com/Daskott/kronus/googleservice"
	"github.com/Daskott/kronus/version"
	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/spf13/viper"
)

var (
	cfgFile   string
	config    *viper.Viper
	googleAPI googleservice.GCalendarAPIInterface

	isDevEnv  bool
	isTestEnv bool

	yellow       = color.New(color.FgYellow).SprintFunc()
	red          = color.New(color.FgRed).SprintFunc()
	warningLabel = yellow("Warning:")
)

// rootCmd represents the base command when called without any subcommands
var rootCmd *cobra.Command

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	cobra.OnInitialize(initConfig, initGCalendarAPI)

	rootCmd = createRootCmd()
	rootCmd.Version = fmt.Sprintf("v%s", version.Version)
}

func createRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "kronus",
		Short: `kronus is a CLI library for Go that allows you to create
coffee chat appointments with your contacts.

The application is a tool to generate recurring google calender events for each of your contacts,
to remind you to reach out and see how they are doing :)`,
	}

	cmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.kronus.yaml)")
	cmd.PersistentFlags().BoolVarP(&isDevEnv, "dev", "", false, "run in development mode")
	cmd.PersistentFlags().BoolVarP(&isTestEnv, "test", "", false, "run in test mode")

	cmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	return cmd
}

func initGCalendarAPI() {
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

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	config = viper.New()

	if cfgFile != "" {
		// Use config file from the flag.
		config.SetConfigFile(cfgFile)
	} else {
		configName, configDir, err := defaulatCFgNameAndDir()
		cobra.CheckErr(err)

		// If config file is not found, create one using defaultConfigFileContent
		configFilePath := filepath.Join(configDir, configName)
		if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
			err = ioutil.WriteFile(configFilePath, []byte(defaultConfigValue()), 0600)
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

// defaultConfigValue returns the default content for .kronus.yaml
func defaultConfigValue() string {
	return `settings:
 timezone: "America/Toronto"
 touchbase-recurrence: "RRULE:FREQ=WEEKLY;"

# Here you update your contact list with their names.
# e.g.
# contacts:
# - name: Smally
# - name: Dad
#
contacts:

# Here you add the different groups you'd like to have for your
# contacts. And populate each group with 
# each contact's id(i.e. index of their record in contacts)
# e.g. 
# groups:
#   friends:
#     - 0
#     - 1
#   family:
#     - 2
#
groups:


# This section is automatically updated by the CLI App to manage
# events created by kronus
events:

owner:
 email: <The email associated with your google calendar>
secrets:
  GOOGLE_APPLICATION_CREDENTIALS: <Path to the JSON file that contains your service account key>
`
}

func formattedError(format string, a ...interface{}) error {
	return fmt.Errorf(red(format), a...)
}
