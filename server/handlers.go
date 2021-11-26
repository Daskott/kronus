package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/Daskott/kronus/database"
	"github.com/Daskott/kronus/server/auth"
	"github.com/go-playground/validator"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

type ResponsePayload struct {
	Errors  []string    `json:"errors"`
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
}

var validate *validator.Validate

func init() {
	validate = validator.New()
	_ = validate.RegisterValidation("phone_number", func(fl validator.FieldLevel) bool {
		return isValidatePhoneNumber(fl.Field().String())
	})
}

func createUser(rw http.ResponseWriter, r *http.Request) {
	data := database.User{}
	decoder := json.NewDecoder(r.Body)

	err := decoder.Decode(&data)
	if err != nil {
		writeResponse(rw, ResponsePayload{Errors: []string{err.Error()}}, http.StatusInternalServerError)
		return
	}

	errs := validate.Struct(data)
	if errs != nil {
		writeResponse(rw, ResponsePayload{Errors: strings.Split(errs.Error(), "\n")}, http.StatusBadRequest)
		return
	}

	err = database.CreateUser(&data)
	if err != nil {
		writeResponse(rw, ResponsePayload{Errors: []string{err.Error()}}, http.StatusInternalServerError)
		return
	}

	json.NewEncoder(rw).Encode(ResponsePayload{Success: true})
}

func findUser(rw http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	user := database.User{}

	err := database.FindUserBy(&user, "ID", vars["id"])
	if errors.Is(err, gorm.ErrRecordNotFound) {
		writeResponse(rw, ResponsePayload{Errors: []string{err.Error()}}, http.StatusNotFound)
		return
	}

	if err != nil {
		writeResponse(rw, ResponsePayload{Errors: []string{err.Error()}}, http.StatusInternalServerError)
		return
	}

	json.NewEncoder(rw).Encode(ResponsePayload{Data: user})
}

func deleteUser(rw http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	err := database.DeleteUser(vars["id"])
	if err != nil {
		writeResponse(rw, ResponsePayload{Errors: []string{err.Error()}}, http.StatusInternalServerError)
		return
	}

	json.NewEncoder(rw).Encode(ResponsePayload{Success: true})
}

func updateUser(rw http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
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

	if data["password"] != nil && strings.TrimSpace(fmt.Sprintf("%v", data["password"])) == "" {
		errs = append(errs, "password cannot be empty")
	}

	if data["phone_number"] != nil && isValidatePhoneNumber(fmt.Sprintf("%v", data["phone_number"])) {
		errs = append(errs, "password cannot be empty")
	}

	if len(errs) > 0 {
		writeResponse(rw, ResponsePayload{Errors: errs}, http.StatusBadRequest)
		return
	}

	err = database.UpdateUser(vars["id"], data)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		writeResponse(rw, ResponsePayload{Errors: []string{err.Error()}}, http.StatusInternalServerError)
		return
	}

	json.NewEncoder(rw).Encode(ResponsePayload{Success: true})
}

func logIn(rw http.ResponseWriter, r *http.Request) {
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

	// TODO: return JWT includes IP address
	json.NewEncoder(rw).Encode(ResponsePayload{Success: true})
}

func writeResponse(rw http.ResponseWriter, payLoad ResponsePayload, statusCode int) {
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
