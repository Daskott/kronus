package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/Daskott/kronus/database"
	"github.com/Daskott/kronus/server/auth"
	"github.com/Daskott/kronus/server/auth/key"
	"github.com/go-playground/validator"
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

var validate *validator.Validate

func init() {
	validate = validator.New()
	err := Registervalidators(validate)
	if err != nil {
		log.Panic(err)
	}
}

func createUserHandler(rw http.ResponseWriter, r *http.Request) {
	user := database.User{}
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

	atLeastOneUserExists, err := database.AtLeastOneUserExists()
	if err != nil {
		writeResponse(rw, ResponsePayload{Errors: []string{err.Error()}}, http.StatusUnauthorized)
		return
	}

	// if no auth token and there's no user, make the 1st user an admin
	if r.Context().Value(RequestContextKey("jwt_claims")) == nil && !atLeastOneUserExists {
		assignedRole = "admin"
	}

	role, err := database.FindRole(assignedRole)
	if err != nil {
		writeResponse(rw, ResponsePayload{Errors: []string{err.Error()}}, http.StatusInternalServerError)
		return
	}
	user.RoleID = role.ID

	err = database.CreateUser(&user)
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
	user, err := database.FindUserBy("ID", r.Context().Value(RequestContextKey("requestUserID")))
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
	err := database.DeleteUser(r.Context().Value(RequestContextKey("requestUserID")))
	if err != nil {
		writeResponse(rw, ResponsePayload{Errors: []string{err.Error()}}, http.StatusInternalServerError)
		return
	}

	writeResponse(rw, ResponsePayload{Success: true}, http.StatusOK)
}

func updateUserHandler(rw http.ResponseWriter, r *http.Request) {
	var errs []string
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

	err = database.UpdateUser(r.Context().Value(RequestContextKey("requestUserID")), data)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		writeResponse(rw, ResponsePayload{Errors: []string{err.Error()}}, http.StatusInternalServerError)
		return
	}

	writeResponse(rw, ResponsePayload{Success: true}, http.StatusOK)
}

func updateProbeSettingsHandler(rw http.ResponseWriter, r *http.Request) {
	var errs []string
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

	if params["day"] != nil && database.CRON_DAY_MAPPINGS[params["day"].(string)] == "" {
		errs = append(errs, "valid 'day' field is required e.g. sun, mon, tue, wed, thu, fri or sat")
	}

	if len(errs) > 0 {
		writeResponse(rw, ResponsePayload{Errors: errs}, http.StatusBadRequest)
		return
	}

	// Only activate liveliness probe for users with emergency contact
	if _, ok := params["active"].(bool); ok && params["active"] != nil && params["active"].(bool) {
		contact, err := database.EmergencyContact(RequestContextKey("requestUserID"))
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

	err = database.UpdateProbSettings(
		r.Context().Value(RequestContextKey("requestUserID")),
		probeSettingFieldsFromParams(params),
	)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		writeResponse(rw, ResponsePayload{Errors: []string{err.Error()}}, http.StatusInternalServerError)
		return
	}

	writeResponse(rw, ResponsePayload{Success: true}, http.StatusOK)
}

func logInHandler(rw http.ResponseWriter, r *http.Request) {
	data := make(map[string]string)
	decoder := json.NewDecoder(r.Body)
	decoder.Decode(&data)

	passwordHash, err := database.FindUserPassword(data["email"])
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		writeResponse(rw, (ResponsePayload{Errors: []string{err.Error()}}), http.StatusInternalServerError)
		return
	}

	if !auth.CheckPasswordHash(data["password"], passwordHash) {
		writeResponse(rw, ResponsePayload{Errors: []string{"email/password is invalid"}}, http.StatusUnauthorized)
		return
	}

	// On success, find user record
	user, err := database.FindUserBy("email", data["email"])
	if err != nil {
		writeResponse(rw, (ResponsePayload{Errors: []string{err.Error()}}), http.StatusInternalServerError)
		return
	}

	isAdmin, err := database.IsAdmin(user)
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
	})

	if err != nil {
		writeResponse(rw, (ResponsePayload{Errors: []string{err.Error()}}), http.StatusInternalServerError)
		return
	}

	writeResponse(rw, ResponsePayload{Success: true, Data: TokenPayload{Token: token}}, http.StatusOK)
}

func jwksHandler(rw http.ResponseWriter, r *http.Request) {
	jwk, err := auth.KeyPair.JWK()
	if err != nil {
		writeResponse(rw, (ResponsePayload{Errors: []string{err.Error()}}), http.StatusInternalServerError)
		return
	}

	writeResponse(rw, key.ExportJWKAsJWKS(jwk), http.StatusOK)
}

func healthCheckHandler(rw http.ResponseWriter, r *http.Request) {
	writeResponse(rw, ResponsePayload{Success: true}, http.StatusOK)
}
