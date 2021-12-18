/*
Copyright © 2021 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/Daskott/kronus/server"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// serverCmd represents the server command
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start a kronus server",
	Long:  `The kronus server houses functionality for liveliness probes(aka dead man's switch)`,
	Run: func(cmd *cobra.Command, args []string) {

		server.Start(serverConfig(), isDevEnv)
	},
}

var serverCongFile string

func init() {
	rootCmd.AddCommand(serverCmd)

	// TODO: Make this required, when not in dev mode
	serverCmd.Flags().StringVar(&serverCongFile, "sconfig", "", "Config for server")
}

func serverConfig() *viper.Viper {
	config = viper.New()

	if isDevEnv {
		serverCongFile = devConfigFilePath()
	}

	config.SetConfigFile(serverCongFile)
	config.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := config.ReadInConfig(); err != nil {
		log.Panic(fmt.Sprintf("error reading server config file: %v", err))
	}

	return config
}

func devConfigFilePath() string {
	configDir, err := os.Getwd()
	if err != nil {
		log.Panic(err)
	}

	return filepath.Join(configDir, "dev", "config", "server.yml")
}
