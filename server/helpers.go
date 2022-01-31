package server

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Daskott/kronus/server/auth"
	"github.com/Daskott/kronus/server/models"
	"github.com/Daskott/kronus/server/pbscheduler"
	"github.com/Daskott/kronus/server/work"
	"github.com/Daskott/kronus/utils"
	"github.com/go-co-op/gocron"
	"github.com/go-playground/validator"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------------//
// Handler Helper functions
// --------------------------------------------------------------------------------//

func writeResponse(rw http.ResponseWriter, payLoad ResponsePayload, statusCode int) {
	rw.WriteHeader(statusCode)
	json.NewEncoder(rw).Encode(payLoad)

	if statusCode >= http.StatusInternalServerError {
		logg.Error(payLoad.Errors)
		return
	}

	if statusCode >= http.StatusBadRequest {
		logg.Info(payLoad.Errors)
	}
}

func writeErrMsgForSmsWebhook(rw http.ResponseWriter, err error) {
	logg.Error(err)

	errMsg := "Sorry an application error has occured.\nPlease try again later"
	msgBytes, err := xml.Marshal(&TwilioSmsResponse{Message: errMsg})
	if err != nil {
		logg.Errorf("writeErrMsgForSmsWebhook: %v", err)
	}

	writeSmsWebHookResponse(rw, msgBytes, http.StatusOK)
}

func writeSmsWebHookResponse(rw http.ResponseWriter, body []byte, status int) {
	rw.WriteHeader(status)
	rw.Write(body)
}

func removeUnknownFields(args map[string]interface{}, validFields map[string]bool) {
	for key := range args {
		if !validFields[key] {
			delete(args, key)
		}
	}
}

func RegisterValidators(validate *validator.Validate) error {
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

	if !userClaims.IsAdmin {
		return false
	}

	if !allowedMethodsForAdmins[r.Method] {
		return false
	}

	for _, deniedPath := range deniedPathsForAdmin {
		if strings.Contains(r.URL.Path, deniedPath) {
			return false
		}
	}

	return true
}

func isValidCronExpression(expression string) bool {
	_, err := gocron.NewScheduler(time.UTC).Cron(expression).Do(func() {})
	return err == nil
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

func cleanup(workerPool *work.WorkerPoolAdapter, server *http.Server, backupDb bool) {
	// Stop all jobs i.e. liveliness probes & regular server jobs
	workerPool.Stop()

	if backupDb {
		backupSqliteDb(nil)
	}

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
	// Use 'kronus' folder in home directory for prod
	configFolderName := "kronus"
	rootDir, err := os.UserHomeDir()
	fatalOnError(err)

	// Use 'dev' folder in current directory for dev mode
	if devMode {
		configFolderName = "dev"
		rootDir, err = os.Getwd()
		fatalOnError(err)
	}

	configDir := filepath.Join(rootDir, configFolderName)

	err = utils.CreateDirIfNotExist(configDir)
	fatalOnError(err)

	return configDir
}

func fatalOnError(err error) {
	if err != nil {
		logg.Fatal(err)
	}
}

// ---------------------------------------------------------------------------------//
// Sms Helper functions
// --------------------------------------------------------------------------------//

func handleProbeMsgReply(user models.User, message string) ([]byte, error) {
	probe, err := user.LastProbe()
	if err != nil {
		// If no probe - do nothing
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return []byte("<Response />"), nil
		}

		return []byte{}, err
	}

	pendingProbe, err := probe.IsPending()
	if err != nil {
		return []byte{}, err
	}

	// If no pending probe - do nothing
	if !pendingProbe {
		return []byte("<Response />"), nil
	}

	// Determine if user probe was 'good' or 'bad' from their reply i.e. message
	probe.LastResponse = message
	probeStatusName := probe.StatusFromLastResponse()

	// if unable to determine probe status from msg - save 'LastResponse' & do nothing
	if probeStatusName == "" {
		probe.Save()
		return []byte("<Response />"), nil
	}

	probeStatus, err := models.FindProbeStatus(probeStatusName)
	if err != nil {
		return []byte{}, err
	}

	probe.ProbeStatusID = probeStatus.ID
	probe.Save()

	msg := "👍"
	if probeStatusName == models.BAD_PROBE {
		msg = "Hang in there! Reaching out to your emergency contact ASAP."

		// Equeue job to send out message to emergency contact
		workerPool.Perform(work.JobParams{
			Name:    pbscheduler.EmergencyProbeName(probe.UserID),
			Handler: pbscheduler.SEND_EMERGENCY_PROBE_HANDLER,
			Args: map[string]interface{}{
				"user_id":      probe.UserID,
				"probe_id":     probe.ID,
				"probe_status": models.BAD_PROBE,
			},
		})
	}

	msgBytes, _ := xml.Marshal(&TwilioSmsResponse{Message: msg})
	return msgBytes, nil
}

func handlePingCmd(input string) ([]byte, error) {
	outputBuffer := new(bytes.Buffer)
	pingCmd := flag.NewFlagSet("ping", flag.ContinueOnError)
	pingCmd.SetOutput(outputBuffer)

	err := pingCmd.Parse(strings.Split(input, " ")[1:])
	if err != nil {
		return xml.Marshal(&TwilioSmsResponse{Message: outputBuffer.String()})
	}

	return xml.Marshal(&TwilioSmsResponse{Message: "PONG!"})
}

func handleDynamicProbeCmd(user *models.User, input string) ([]byte, error) {
	var err error
	outputBuffer := new(bytes.Buffer)

	probeCmd := flag.NewFlagSet("probe", flag.ContinueOnError)
	probeCmd.SetOutput(outputBuffer)

	inPtr := probeCmd.Int("in", 5, "Minutes from now when probe should be sent, default(5)")
	retriesPtr := probeCmd.Int("retries", 3, "Number of retries after no response is received, default(3")
	waitPtr := probeCmd.Int("wait", 10, "The amount of minutes to wait for a response to a probe, default(10)")

	// Parse Arguments without the name of command
	err = probeCmd.Parse(strings.Split(input, " ")[1:])
	if err != nil {
		return xml.Marshal(&TwilioSmsResponse{Message: outputBuffer.String()})
	}

	if _, err := user.EmergencyContact(); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return xml.Marshal(&TwilioSmsResponse{Message: "An emergency contact is required to use the 'probe' cmd"})
		}
		return []byte{}, err
	}

	err = workerPool.PerformIn(*inPtr*60, work.JobParams{
		Name:    "send_liveliness_probe",
		Handler: "send_liveliness_probe",
		Args: map[string]interface{}{
			"first_name":           user.FirstName,
			"last_name":            user.LastName,
			"user_id":              user.ID,
			"max_retries":          *retriesPtr,
			"wait_time_in_minutes": *waitPtr,
		},
	})
	if err != nil {
		return []byte{}, err
	}

	// TODO: Include retries
	return xml.Marshal(&TwilioSmsResponse{Message: fmt.Sprintf("Probe will be sent in %v minutes.", *inPtr)})
}

func handleHelpCmd(input string) ([]byte, error) {
	outputBuffer := new(bytes.Buffer)
	helpCmd := flag.NewFlagSet("help", flag.ContinueOnError)
	helpCmd.SetOutput(outputBuffer)

	err := helpCmd.Parse(strings.Split(input, " ")[1:])
	if err != nil {
		return xml.Marshal(&TwilioSmsResponse{Message: outputBuffer.String()})
	}

	res := `
Available Commands:
	help        Help about any command
	probe       Ask kronus to check on you in a couple minutes
	ping    	Health check for the server`
	return xml.Marshal(&TwilioSmsResponse{Message: res})
}
