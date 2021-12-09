package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/Daskott/kronus/database"
	"github.com/Daskott/kronus/server/auth"
	"github.com/go-playground/validator"
)

// ---------------------------------------------------------------------------------//
// Handler Helper functions
// --------------------------------------------------------------------------------//

func writeResponse(rw http.ResponseWriter, payLoad interface{}, statusCode int) {
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

	cronDay := database.DEFAULT_PROBE_CRON_DAY
	cronHour := database.DEFAULT_PROBE_CRON_HOUR
	cronMinute := database.DEFAULT_PROBE_CRON_MINUTE
	cronExpression := ""

	// Extract time segments (if provided)
	if params["time"] != nil {
		timeSegments := strings.Split(params["time"].(string), ":")
		cronHour = timeSegments[0]
		cronMinute = timeSegments[1]
	}

	// Extract numeric value for day (if provided)
	if params["day"] != nil {
		cronDay = database.CRON_DAY_MAPPINGS[params["day"].(string)]
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

	tokenClaims, err := auth.DecodeJWT(authHeaderList[1])
	if err != nil {
		return DecodedJWT{ErrorMsg: "invalid token provided"}
	}

	// validate that the user account still exists
	_, err = database.FindUserBy("id", tokenClaims.Subject)
	if err != nil {
		return DecodedJWT{ErrorMsg: "invalid token provided"}
	}

	return DecodedJWT{Claims: tokenClaims}
}
