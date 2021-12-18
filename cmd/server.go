/*
Copyright © 2021 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"bytes"
	"fmt"

	"github.com/Daskott/kronus/colors"
	devConfig "github.com/Daskott/kronus/dev/config"
	"github.com/Daskott/kronus/server"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const DEV_MODE_MESSAGE = `WARNING! dev mode is enabled! In this mode, Kronus server
uses predefined configs, saves data to /dev/db/
and probe messages are printed to stdout.

You may need to set the following environment variable:

    $ export KRONUS_PORT='3000'

Development mode should NOT be used in production installations!
`

// serverCmd represents the server command
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start a kronus server",
	Long:  `The kronus server houses functionality for liveliness probes(aka dead man's switch)`,
	PreRun: func(cmd *cobra.Command, args []string) {
		if isDevEnv {
			cmd.Println(colors.Yellow(DEV_MODE_MESSAGE))
			return
		}
		cmd.MarkFlagRequired("sconfig")
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		config, err := serverConfig()
		if err != nil {
			return err
		}

		server.Start(config, isDevEnv)
		return nil
	},
}

var serverCongFile string

func init() {
	rootCmd.AddCommand(serverCmd)

	serverCmd.Flags().StringVar(&serverCongFile, "sconfig", "", "Config for server")
}

func serverConfig() (*viper.Viper, error) {
	config = viper.New()

	// Read in environment variables that match
	config.AutomaticEnv()
	config.BindEnv("kronus.listener.port", "KRONUS_PORT")

	if isDevEnv {
		config.SetConfigType("yaml")
		config.ReadConfig(bytes.NewBuffer([]byte(devConfig.SERVER_YML)))
		return config, nil
	}

	config.SetConfigFile(serverCongFile)

	// If a config file is found, read it in.
	if err := config.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("unable to read server config file: %v", err)
	}

	return config, nil
}
