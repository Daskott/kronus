/*
Copyright © 2021 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/Daskott/kronus/colors"
	devConfig "github.com/Daskott/kronus/dev/config"
	"github.com/Daskott/kronus/server"
	"github.com/Daskott/kronus/shared"
	"github.com/go-playground/validator"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const DEV_MODE_MESSAGE = `WARNING! dev mode is enabled! In this mode, Kronus server
uses predefined configs, saves data to /dev/db/, db backup is disabled,
and probe messages are printed to stdout.

You may need to set the following environment variable:

    $ export KRONUS_PORT='3000'

Development mode should NOT be used in production installations!
`

var validate = validator.New()

func init() {
	err := registerValidators(validate)
	if err != nil {
		log.Fatal(err)
	}
}

// serverCmd represents the server command
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start a kronus server",
	Long:  `The kronus server houses functionality for liveliness probes(aka dead man's switch)`,
	PreRun: func(cmd *cobra.Command, args []string) {
		if isDevEnv {
			return
		}

		cmd.MarkFlagRequired("sconfig")
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		config, err := serverConfig()
		if err != nil {
			return err
		}

		if isDevEnv {
			cmd.Println(colors.Yellow(DEV_MODE_MESSAGE))
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

func serverConfig() (*shared.ServerConfig, error) {
	config := viper.New()

	// Read in environment variables that match
	config.AutomaticEnv()
	config.BindEnv("kronus.listener.port", "KRONUS_PORT")

	config.SetDefault("kronus.cron.timeZone", "UTC")
	config.SetDefault("kronus.listener.port", 3000)
	config.SetDefault("google.storage.prefix", "kronus")
	config.SetDefault("google.storage.sqliteBackupSchedule", "*/15 * * * *")

	if isDevEnv {
		config.SetConfigType("yaml")
		if err := config.ReadConfig(bytes.NewBuffer([]byte(devConfig.SERVER_YML))); err != nil {
			return nil, fmt.Errorf("unable to read server config file: %v", err)
		}
		return validatedConfigStruct(config)
	}

	config.SetConfigFile(serverCongFile)
	if err := config.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("unable to read server config file: %v", err)
	}

	return validatedConfigStruct(config)
}

func validatedConfigStruct(config *viper.Viper) (*shared.ServerConfig, error) {
	serverConfig := shared.ServerConfig{}

	// Map viper configurations to struct
	err := config.Unmarshal(&serverConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to decode config into struct, %v", err)
	}

	// This is done to help with chained validation
	// Since there's no good way to do chained validation with 'bool'
	// 'EnableSqliteBackupAndSync' is set to an interface
	// And this set's it to 'nil' if false is provided.
	//
	// This is so validation rules like "required_with=EnableSqliteBackupAndSync" will
	// work as expected i.e. a field will only be required if 'EnableSqliteBackupAndSync' is
	// provided i.e. 'true' since 'false' will be set to 'nil'
	setEnableSqliteBackupFieldToNilIfFalse(&serverConfig)

	// Exit on validation errors - as error format is better than the default
	if errs := validate.Struct(serverConfig); errs != nil {
		for _, msg := range strings.Split(errs.Error(), "\n") {
			fmt.Printf("%v %v\n", colors.Red("[Error]"), msg)
		}
		os.Exit(1)
	}

	return &serverConfig, nil
}

func registerValidators(validate *validator.Validate) error {
	err := validate.RegisterValidation("bool", func(fl validator.FieldLevel) bool {
		_, ok := fl.Field().Interface().(bool)
		return ok
	})
	if err != nil {
		return err
	}

	return nil
}

func setEnableSqliteBackupFieldToNilIfFalse(config *shared.ServerConfig) {
	if enabled, ok := config.Google.Storage.EnableSqliteBackupAndSync.(bool); ok && !enabled {
		config.Google.Storage.EnableSqliteBackupAndSync = nil
	}
}
