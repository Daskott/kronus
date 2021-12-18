package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Daskott/kronus/models"
	"github.com/Daskott/kronus/server/auth"
	"github.com/Daskott/kronus/server/auth/key"

	"github.com/golang-jwt/jwt"
	"gorm.io/gorm"
)

type ResponsePayload struct {
	Errors  []string    `json:"errors,omitempty"`
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
}

type TokenPayload struct {
	Token string `json:"token"`
}

func createUserHandler(rw http.ResponseWriter, r *http.Request) {
	user := models.User{}
	decoder := json.NewDecoder(r.Body)
	assignedRole := "basic"

	err := decoder.Decode(&user)
	if err != nil {
		writeResponse(rw, ResponsePayload{Errors: []string{err.Error()}}, http.StatusInternalServerError)
		return
	}

	errs := validate.Struct(user)
	if errs != nil {
		writeResponse(rw, ResponsePayload{Errors: strings.Split(errs.Error(), "\n")}, http.StatusBadRequest)
		return
	}

	atLeastOneUserExists, err := models.AtLeastOneUserExists()
	if err != nil {
		writeResponse(rw, ResponsePayload{Errors: []string{err.Error()}}, http.StatusInternalServerError)
		return
	}

	// if no user in db, make this user an admin
	if !atLeastOneUserExists {
		assignedRole = "admin"
	}

	role, err := models.FindRole(assignedRole)
	if err != nil {
		writeResponse(rw, ResponsePayload{Errors: []string{err.Error()}}, http.StatusInternalServerError)
		return
	}
	user.RoleID = role.ID

	err = models.CreateUser(&user)
	if err != nil {
		if strings.Contains(err.Error(), "Duplicate") {
			writeResponse(rw, ResponsePayload{Errors: []string{err.Error()}}, http.StatusBadRequest)
			return
		}

		writeResponse(rw, ResponsePayload{Errors: []string{err.Error()}}, http.StatusInternalServerError)
		return
	}

	writeResponse(rw, ResponsePayload{Success: true}, http.StatusOK)
}

func findUserHandler(rw http.ResponseWriter, r *http.Request) {
	user, err := models.FindUserBy("ID", r.Context().Value(RequestContextKey("requestUserID")))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		writeResponse(rw, ResponsePayload{Errors: []string{err.Error()}}, http.StatusNotFound)
		return
	}

	if err != nil {
		writeResponse(rw, ResponsePayload{Errors: []string{err.Error()}}, http.StatusInternalServerError)
		return
	}

	writeResponse(rw, ResponsePayload{Success: true, Data: user}, http.StatusOK)
}

func deleteUserHandler(rw http.ResponseWriter, r *http.Request) {
	err := models.DeleteUser(r.Context().Value(RequestContextKey("requestUserID")))
	if err != nil {
		writeResponse(rw, ResponsePayload{Errors: []string{err.Error()}}, http.StatusInternalServerError)
		return
	}

	writeResponse(rw, ResponsePayload{Success: true}, http.StatusOK)
}

func updateUserHandler(rw http.ResponseWriter, r *http.Request) {
	var errs []string

	currentUser := r.Context().Value(RequestContextKey("currentUser")).(*models.User)
	data := make(map[string]interface{})
	decoder := json.NewDecoder(r.Body)

	err := decoder.Decode(&data)
	if err != nil {
		writeResponse(rw, ResponsePayload{Errors: []string{err.Error()}}, http.StatusInternalServerError)
		return
	}

	removeUnknownFields(data, map[string]bool{"first_name": true, "last_name": true, "phone_number": true, "password": true})
	if len(data) <= 0 {
		writeResponse(rw,
			ResponsePayload{Errors: []string{"valid fields required"}},
			http.StatusBadRequest,
		)
		return
	}

	if data["password"] != nil {
		if err := validate.Var(data["password"], "password"); err != nil {
			errs = append(errs, "valid password is required")
		}
	}

	if data["phone_number"] != nil {
		if err := validate.Var(data["phone_number"], "e164"); err != nil {
			errs = append(errs, "valid phone_number is required")
		}
	}

	if len(errs) > 0 {
		writeResponse(rw, ResponsePayload{Errors: errs}, http.StatusBadRequest)
		return
	}

	err = models.UpdateUser(currentUser.ID, data)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		writeResponse(rw, ResponsePayload{Errors: []string{err.Error()}}, http.StatusInternalServerError)
		return
	}

	writeResponse(rw, ResponsePayload{Success: true}, http.StatusOK)
}

func updateProbeSettingsHandler(rw http.ResponseWriter, r *http.Request) {
	var errs []string

	currentUser := r.Context().Value(RequestContextKey("currentUser")).(*models.User)
	params := make(map[string]interface{})
	decoder := json.NewDecoder(r.Body)

	err := decoder.Decode(&params)
	if err != nil {
		writeResponse(rw, ResponsePayload{Errors: []string{err.Error()}}, http.StatusInternalServerError)
		return
	}

	removeUnknownFields(params, map[string]bool{"day": true, "time": true, "active": true})
	if len(params) <= 0 {
		writeResponse(rw,
			ResponsePayload{Errors: []string{"valid fields required"}},
			http.StatusBadRequest,
		)
		return
	}

	if _, ok := params["active"].(bool); params["active"] != nil && !ok {
		errs = append(errs, "active must be boolean e.g. true/false")
	}

	if params["time"] != nil {
		if err := validate.Var(params["time"], "time_stamp"); err != nil {
			errs = append(errs, "valid 'time' field is required e.g. 18:30")
		}
	}

	if params["day"] != nil && models.CRON_DAY_MAPPINGS[params["day"].(string)] == "" {
		errs = append(errs, "valid 'day' field is required e.g. sun, mon, tue, wed, thu, fri or sat")
	}

	if len(errs) > 0 {
		writeResponse(rw, ResponsePayload{Errors: errs}, http.StatusBadRequest)
		return
	}

	// Only activate liveliness probe for users with emergency contact
	if _, ok := params["active"].(bool); ok && params["active"] != nil && params["active"].(bool) {
		contact, err := currentUser.EmergencyContact()
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			writeResponse(rw, ResponsePayload{Errors: []string{err.Error()}}, http.StatusInternalServerError)
			return
		}

		if contact == nil {
			writeResponse(rw, ResponsePayload{Errors: []string{
				"an emergency contact is required to enable liveliness probe i.e 'active = true'"}}, http.StatusForbidden)
			return
		}
	}

	err = currentUser.UpdateProbSettings(probeSettingFieldsFromParams(params))
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		writeResponse(rw, ResponsePayload{Errors: []string{err.Error()}}, http.StatusInternalServerError)
		return
	}

	if activateProbe, ok := params["active"].(bool); ok {
		if activateProbe {
			probeScheduler.AddCronJobForProbe(*currentUser)
		} else {
			probeScheduler.RemoveCronJobForProbe(currentUser)
		}
	}

	writeResponse(rw, ResponsePayload{Success: true}, http.StatusOK)
}

func logInHandler(rw http.ResponseWriter, r *http.Request) {
	data := make(map[string]string)
	decoder := json.NewDecoder(r.Body)
	decoder.Decode(&data)

	passwordHash, err := models.FindUserPassword(data["email"])
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		writeResponse(rw, (ResponsePayload{Errors: []string{err.Error()}}), http.StatusInternalServerError)
		return
	}

	if !auth.CheckPasswordHash(data["password"], passwordHash) {
		writeResponse(rw, ResponsePayload{Errors: []string{"email/password is invalid"}}, http.StatusUnauthorized)
		return
	}

	// On success, find user record
	user, err := models.FindUserBy("email", data["email"])
	if err != nil {
		writeResponse(rw, (ResponsePayload{Errors: []string{err.Error()}}), http.StatusInternalServerError)
		return
	}

	isAdmin, err := user.IsAdmin()
	if err != nil {
		writeResponse(rw, (ResponsePayload{Errors: []string{err.Error()}}), http.StatusInternalServerError)
		return
	}

	token, err := auth.EncodeJWT(auth.KronusTokenClaims{
		FirstName: user.FirstName,
		LastName:  user.LastName,
		IsAdmin:   isAdmin,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: time.Now().UTC().Add(24 * time.Hour).Unix(),
			IssuedAt:  time.Now().UTC().Unix(),
			Issuer:    "kronus",
			Subject:   fmt.Sprint(user.ID),
		},
	}, authKeyPair)

	if err != nil {
		writeResponse(rw, (ResponsePayload{Errors: []string{err.Error()}}), http.StatusInternalServerError)
		return
	}

	writeResponse(rw, ResponsePayload{Success: true, Data: TokenPayload{Token: token}}, http.StatusOK)
}

func jwksHandler(rw http.ResponseWriter, r *http.Request) {
	jwk, err := authKeyPair.JWK()
	if err != nil {
		writeResponse(rw, (ResponsePayload{Errors: []string{err.Error()}}), http.StatusInternalServerError)
		return
	}

	writeResponse(rw, ResponsePayload{Success: true, Data: key.ExportJWKAsJWKS(jwk)}, http.StatusOK)
}

func healthCheckHandler(rw http.ResponseWriter, r *http.Request) {
	writeResponse(rw, ResponsePayload{Success: true}, http.StatusOK)
}
