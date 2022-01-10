/*
Copyright © 2021 Edmond Cotterell

*/
package cmd

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/Daskott/kronus/colors"
	"github.com/Daskott/kronus/server"
	"github.com/Daskott/kronus/shared"
	"github.com/go-playground/validator"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var devModeConfigYaml = `
kronus:
  privateKeyPem: "-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQDSrTqLNNFUr08X\nEzy5LlYXRqgDuSe0oGZs6R7wa8B60ZdlW7GEgG8DwR17pJH2QAaI4FjKwXhHXZs9\n2M/mSESnTne/IgJ3RWdT0R7nrM7NFknp2KqApVcDQSH5d5jZKVQHopwU2Es0vWsW\nbG45T0eBLZojCnWCjauTyPYJlSMeDrXOg6ngKM3WfMQMlL3cgk90hEtMnJ0ycDle\nAMTEYPm/CBqu/XC0Tr5z1DzQi0pC9RG/d4imt/7KqG6ZmUURS6pzDBG2aw6ipmeu\nBfcpu55g9mGERnYXIj/gSDmtg+JUeDsq58MsZ7ZT8YUyzoiY3z28VvXuYHLUNSX2\n1MkgfDiNAgMBAAECggEBAK4Dj7uz4MPGGdnBdgKvF0Uag2Sv5u/3HSMQWxHSrqXD\nwP1jg3kibI/5TtT11epEcCFWzYCL1UF9O+EV2IMpZiubUKV6/fZuSS6eKJzLy/Ty\nWBLjd9HSv9BcWCeqdYHJ9TJpSeqdzWC+pFldLp3/sdwtQod2+CDhy7rB3xeDLAKC\nPFkQP5nTAjQW4hzIZN3YueShbrFzyy9+coRRDREYfs8UsMWT1TJFjJpTppBpUARB\nh1wZJDAR/KMi5NZS6IzFLAXi3Ur+YdDitG/oz5k4LLRoyrbIma2CSZQ8+QDLYJaw\nmIUktSbb1jzbV1e7kSm+zn2PmWUqbQmYVnGkBkO8zTECgYEA6eljJHC+ets9FRiR\nehrIGMVKRJDDwskCfCRLanLoJB6+513iMOUrCnNa/THDUXn19KFe8mboWTgn7A5/\naJ+rGMa8dKegKSnZxyUlQngjkRBP7ozHDbFJnDF5Dfpwx3k8ok9XwCfF5gC+YzmQ\nkVirWeTCUsvmLNjRS9Fc5F2G6YsCgYEA5pInzRAjbJR87ONaq8nCbLkfkKRXO5ck\nhZ25IS36Eu/tfi2xBMxFImGqosgKAhDall216nrNVeic9feqUIS0v9pvapkc+OJG\nyPddSy+BNd6mpHR2/Nim6WEH0gagxATTN+a3tQbIrrTqgq4l17+M2q9HMnaLXRke\n4a9NhUVkuUcCgYBGSZA2Cf7i0fBH34sPYu7PqrEHa2y3okkx3oIe6YpiGC8LPQXT\n5XkKeeFUhdiIKhrDOJ5cPpoA/UPZxf15BcmW91j3wMr6s42yLrJEh+9ADuPF7d1+\netCAs8kJb0DmX8LdjvPyVME9vOl4zXpognly2K+fy49N2JUDsFS2dngswwKBgCKB\nnRNDZwnI7ylEnT04ZLCAxAiRj7yLUhvtDte4WcSbw58ul19wcqhClZbm+Rh2DUCT\npbYBytkght0Iw6RpN+O+fQ4m+/8DXjSVUJD/+wZk2+ugwm30voYOz2zPMSAk2Ld0\n/+lHqqD60l3cUi2HrTzNHoqe0xyLteNwqNlZGUnhAoGAJDrUxiDPLVcvctgiL0/1\nfIdXdaz61ISM/MzbNOlplW4uoejnTk4JLXCPM7JH/Mf1vr7A/WCBQy7k4HK7Yxnh\n+1HcFSADL+yILhEQYwc366NI/phOMbXMxUNlp5QYKPTlbxpr0LQeamIaLi+9nXkF\nkKeZzVA0xAVgPkqQ+FBM7rA=\n-----END PRIVATE KEY-----\n"
  publicUrl: "http://127.0.0.1:3900/"
  cron:
    timeZone: "America/Toronto"
  listener:
    port: 3900

sqlite:
  passPhrase: passphrase

google:
  storage:
    bucket: "kronus"
    prefix: "kronus-dev"
    sqliteBackupSchedule: "0 * * * *"
    enableSqliteBackupAndSync: false
  applicationCredentials: 

twilio:
  accountSid: none
  authToken: none
  messagingServiceSid: none
`

var serverCongFile string

var validate = validator.New()

func init() {
	err := registerValidators(validate)
	if err != nil {
		log.Fatal(err)
	}

	rootCmd.AddCommand(createServerCmd())
}

func createServerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Start a kronus server",
		Long: `This command starts a Kronus server that runs liveliness probes 
and responds to API requests. The first user account created on the server via 
the API will have the admin role, and will be the only account allowed to register new users.

Start a server with a configuration file:
	
	$ kronus server --config=config.yaml

Run in "dev" mode:
	
	$ kronus server --dev

For a full list of API endpoints, please see the documentation.`,
		PreRun: func(cmd *cobra.Command, args []string) {
			if isDevEnv {
				return
			}

			cmd.MarkFlagRequired("config")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			config, err := serverConfig()
			if err != nil {
				return err
			}

			if isDevEnv {
				cmd.Println(colors.Yellow(`WARNING! dev mode is enabled! In this mode, Kronus server
uses predefined configs, saves data to /dev/db/, db backup is disabled,
and probe messages are printed to stdout.

You may need to set the following environment variable:

	$ export KRONUS_PORT='3900'

Development mode should NOT be used in production installations!`))
			}

			server.Start(config, isDevEnv)
			return nil
		},
	}

	cmd.Flags().StringVar(&serverCongFile, "config", "", "Config for kronus server")

	return cmd
}

func serverConfig() (*shared.ServerConfig, error) {
	config := viper.New()

	// Read in environment variables that match
	config.AutomaticEnv()
	config.BindEnv("kronus.listener.port", "KRONUS_PORT")

	config.SetDefault("kronus.cron.timeZone", "UTC")
	config.SetDefault("kronus.listener.port", 3900)
	config.SetDefault("google.storage.prefix", "kronus")
	config.SetDefault("google.storage.sqliteBackupSchedule", "*/15 * * * *")

	// if no config file provided, use dev config
	if isDevEnv && serverCongFile == "" {
		config.SetConfigType("yaml")
		if err := config.ReadConfig(bytes.NewBuffer([]byte(devModeConfigYaml))); err != nil {
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
