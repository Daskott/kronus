package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Daskott/kronus/models"
	"github.com/Daskott/kronus/server/auth"
	"github.com/Daskott/kronus/server/pbscheduler"
	"github.com/Daskott/kronus/utils"
	"github.com/go-playground/validator"
	"github.com/gorilla/mux"
)

// ---------------------------------------------------------------------------------//
// Handler Helper functions
// --------------------------------------------------------------------------------//

func writeResponse(rw http.ResponseWriter, payLoad ResponsePayload, statusCode int) {
	if statusCode >= http.StatusInternalServerError {
		logg.Error(payLoad.Errors)
	}

	if statusCode >= http.StatusBadRequest {
		logg.Info(payLoad.Errors)
	}

	rw.WriteHeader(statusCode)
	json.NewEncoder(rw).Encode(payLoad)
}

func removeUnknownFields(args map[string]interface{}, validFields map[string]bool) {
	for key := range args {
		if !validFields[key] {
			delete(args, key)
		}
	}
}

func probeSettingFieldsFromParams(params map[string]interface{}) map[string]interface{} {
	newParams := make(map[string]interface{})

	cronDay := models.DEFAULT_PROBE_CRON_DAY
	cronHour := models.DEFAULT_PROBE_CRON_HOUR
	cronMinute := models.DEFAULT_PROBE_CRON_MINUTE
	cronExpression := ""

	// Extract time segments (if provided)
	if params["time"] != nil {
		timeSegments := strings.Split(params["time"].(string), ":")
		cronHour = timeSegments[0]
		cronMinute = timeSegments[1]
	}

	// Extract numeric value for day (if provided)
	if params["day"] != nil {
		cronDay = models.CRON_DAY_MAPPINGS[params["day"].(string)]
	}

	// Set value of cron expression to be stored (if time or day is provided)
	if params["time"] != nil || params["day"] != nil {
		cronExpression = fmt.Sprintf("%v */%v * * %v", cronMinute, cronHour, cronDay)
	}

	if cronExpression != "" {
		newParams["cron_expression"] = cronExpression
	}

	if params["active"] != nil {
		newParams["active"] = params["active"]
	}

	return newParams
}

func Registervalidators(validate *validator.Validate) error {
	err := validate.RegisterValidation("password", func(fl validator.FieldLevel) bool {
		// if whitespace in password return false
		err := validate.Var(fl.Field().String(), "contains= ")
		if err == nil {
			return false
		}
		return len(fl.Field().String()) > 0
	})
	if err != nil {
		return err
	}

	err = validate.RegisterValidation("time_stamp", func(fl validator.FieldLevel) bool {
		timeSegments := strings.Split(fl.Field().String(), ":")
		if len(timeSegments) < 2 {
			return false
		}

		hour, err := strconv.Atoi(timeSegments[0])
		if err != nil {
			return false
		}

		minute, err := strconv.Atoi(timeSegments[1])
		if err != nil {
			return false
		}

		err = validate.Var(hour, "min=0,max=23")
		if err != nil {
			return false
		}

		err = validate.Var(minute, "min=0,max=59")
		return err == nil
	})
	if err != nil {
		return err
	}

	return nil
}

// ---------------------------------------------------------------------------------//
// Middleware Helper functions
// --------------------------------------------------------------------------------//

func decodeAndVerifyAuthHeader(authHeaderValue string) DecodedJWT {
	authHeaderList := strings.Split(authHeaderValue, "Bearer ")
	if len(authHeaderList) < 2 {
		return DecodedJWT{ErrorMsg: "no token provided"}
	}

	tokenClaims, err := auth.DecodeJWT(authHeaderList[1], authKeyPair)
	if err != nil {
		return DecodedJWT{ErrorMsg: "invalid token provided"}
	}

	// validate that the user account still exists
	_, err = models.FindUserBy("id", tokenClaims.Subject)
	if err != nil {
		return DecodedJWT{ErrorMsg: "invalid token provided"}
	}

	return DecodedJWT{Claims: tokenClaims}
}

// client is only able to update/view their own record unless client is an admin
// who can GET/DELETE certain user resources
func canAccessUserResource(r *http.Request, userClaims *auth.KronusTokenClaims) bool {
	allowedMethodsForAdmins := map[string]bool{"GET": true, "DELETE": true}
	deniedPathsForAdmin := []string{"/contacts"}

	if mux.Vars(r)["uid"] == userClaims.Subject {
		return true
	}

	if userClaims.IsAdmin {
		if !allowedMethodsForAdmins[r.Method] {
			return false
		}

		for _, deniedPath := range deniedPathsForAdmin {
			if strings.Contains(r.URL.Path, deniedPath) {
				return false
			}
		}
	}

	return true
}

// ---------------------------------------------------------------------------------//
// Server Helper functions
// --------------------------------------------------------------------------------//

func serve(server *http.Server) {
	logg.Infof("Kronus server is listening on port:%v", server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logg.Fatal(err)

	}
}

func cleanup(probeScheduler *pbscheduler.ProbeScheduler, server *http.Server) {
	// Stop liveliness probe job workers
	probeScheduler.StopWorkers()

	// Shutdown server gracefully
	ctxShutDown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctxShutDown); err != nil {
		logg.Fatalf("Kronus server shutdown failed:%+s", err)
	}

	logg.Infof("Kronus server stopped properly")
}

// configDirectory retrieves the directory to store kronus configs
// Or logs an error message and then calls os.Exit if it's unable to.
func configDirectory(devMode bool) string {
	var configFolderName string

	// Use dev folder in project directory for dev
	if devMode {
		configFolderName = "dev"
		projectDir, err := os.Getwd()
		fatalOnError(err)

		return filepath.Join(projectDir, configFolderName)
	}

	// Use kronus folder in home directory for prod
	configFolderName = "kronus"
	homeDir, err := os.UserHomeDir()
	fatalOnError(err)

	configDir := filepath.Join(homeDir, configFolderName)

	err = utils.CreateDirIfNotExist(configDir)
	fatalOnError(err)

	return configDir
}

func fatalOnError(err error) {
	if err != nil {
		logg.Fatal(err)
	}
}
