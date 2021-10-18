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
	"github.com/spf13/cobra"

	"github.com/spf13/viper"
)

var cfgFile string

var credentials = types.GoogleAppCredentials{
	Installed: types.InstalledType{
		ClientId:                "984074116152-2mj5vshqb06c1gdlajlelfp9bdi6906e.apps.googleusercontent.com",
		ProjectId:               "kronus-326712",
		AuthURI:                 "https://accounts.google.com/o/oauth2/auth",
		TokenURI:                "https://oauth2.googleapis.com/token",
		AuthProviderx509CertURL: "https://www.googleapis.com/oauth2/v1/certs",
		ClientSecret:            "WHLhwFpDEv-60vpH2TSPlsVB",
		RedirectUris:            []string{"urn:ietf:wg:oauth:2.0:oob", "http://localhost"},
	},
}

var googleAPI = googleservice.NewGoogleCalendarAPI(credentials)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use: "kronus",
	Short: `kronus is a CLI library for Go that allows you to create
coffee chat appointments with select contacts.

The application is a tool to generate recurring google calender events for each of your contacts,
to remind you to reach out and see how they are doing :)`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.kronus.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// If no default config file is found, create one using config.template.yml
		configFilePath := filepath.Join(home, ".kronus.yaml")
		if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
			data, err := ioutil.ReadFile("config.template.yml")
			cobra.CheckErr(err)

			err = ioutil.WriteFile(configFilePath, data, 0600)
			cobra.CheckErr(err)
		}

		// Search config in home directory with name ".kronus" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".kronus")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}
