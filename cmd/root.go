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
	"github.com/Daskott/kronus/types"
	"github.com/Daskott/kronus/version"
	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/spf13/viper"
)

var (
	cfgFile   string
	config    *viper.Viper
	googleAPI googleservice.GCalendarAPIInterface

	yellow       = color.New(color.FgYellow).SprintFunc()
	warningLabel = yellow("Warning:")

	credentials = types.GoogleAppCredentials{
		Installed: types.InstalledType{
			ClientId:                "984074116152-2mj5vshqb06c1gdlajlelfp9bdi6906e.apps.googleusercontent.com",
			ProjectId:               "keep-up-326712",
			AuthURI:                 "https://accounts.google.com/o/oauth2/auth",
			TokenURI:                "https://oauth2.googleapis.com/token",
			AuthProviderx509CertURL: "https://www.googleapis.com/oauth2/v1/certs",
			ClientSecret:            "WHLhwFpDEv-60vpH2TSPlsVB",
			RedirectUris:            []string{"urn:ietf:wg:oauth:2.0:oob", "http://localhost"},
		},
	}
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use: "kronus",
	Short: `kronus is a CLI library for Go that allows you to create
coffee chat appointments with your contacts.

The application is a tool to generate recurring google calender events for each of your contacts,
to remind you to reach out and see how they are doing :)`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	cobra.OnInitialize(initConfig)
	googleAPI = googleservice.NewGoogleCalendarAPI(credentials)
	rootCmd.Version = fmt.Sprintf("v%s", version.Version)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.kronus.yaml)")
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	config = viper.New()

	if cfgFile != "" {
		// Use config file from the flag.
		config.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// If no default config file is found, create one using defaultConfigFileContent
		configFilePath := filepath.Join(home, ".kronus.yaml")
		if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
			err = ioutil.WriteFile(configFilePath, []byte(defaultConfigValue()), 0600)
			cobra.CheckErr(err)
		}

		// Search config in home directory with name ".kronus" (without extension).
		config.AddConfigPath(home)
		config.SetConfigType("yaml")
		config.SetConfigName(".kronus")
	}

	config.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := config.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", config.ConfigFileUsed())
	}
}

// defaultConfigValue returns the default content for .kronus.yaml
func defaultConfigValue() string {
	return `env: production
settings:
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
`
}
