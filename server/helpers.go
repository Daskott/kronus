package server

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Daskott/kronus/server/auth"
	"github.com/Daskott/kronus/server/models"
	"github.com/Daskott/kronus/server/work"
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
