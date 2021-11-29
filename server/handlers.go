package server

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/Daskott/kronus/database"
	"github.com/Daskott/kronus/server/auth"
	"github.com/go-playground/validator"
	"github.com/golang-jwt/jwt"
	"gorm.io/gorm"
)

type ResponsePayload struct {
	Errors  []string    `json:"errors"`
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
}

type TokenPayload struct {
	Token string `json:"token"`
}

var validate *validator.Validate

func init() {
	validate = validator.New()
	err := validate.RegisterValidation("password", func(fl validator.FieldLevel) bool {
		// if whitesapece in password return false
		err := validate.Var(fl.Field().String(), "contains= ")
		if err == nil {
			return false
		}
		return len(fl.Field().String()) > 0
	})
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

	// if no auth token and there's no user, make the 1st user an admin
	if r.Context().Value(RequestContextKey("jwt_claims")) == nil && !database.AtLeastOneUserExists() {
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
		writeResponse(rw, ResponsePayload{Errors: []string{err.Error()}}, http.StatusInternalServerError)
		return
	}

	json.NewEncoder(rw).Encode(ResponsePayload{Success: true})
}

func findUserHandler(rw http.ResponseWriter, r *http.Request) {
	user := database.User{}

	err := database.FindUserBy(&user, "ID", r.Context().Value(RequestContextKey("requestUserID")))
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

func deleteUserHandler(rw http.ResponseWriter, r *http.Request) {
	err := database.DeleteUser(r.Context().Value(RequestContextKey("requestUserID")))
	if err != nil {
		writeResponse(rw, ResponsePayload{Errors: []string{err.Error()}}, http.StatusInternalServerError)
		return
	}

	json.NewEncoder(rw).Encode(ResponsePayload{Success: true})
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

	json.NewEncoder(rw).Encode(ResponsePayload{Success: true})
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
	user := database.User{}
	err = database.FindUserBy(&user, "email", data["email"])
	if err != nil {
		writeResponse(rw, (ResponsePayload{Errors: []string{err.Error()}}), http.StatusInternalServerError)
		return
	}

	isAdmin, err := database.IsAdmin(user)
	if err != nil {
		writeResponse(rw, (ResponsePayload{Errors: []string{err.Error()}}), http.StatusInternalServerError)
		return
	}

	token, err := auth.EncodeJWT(jwt.MapClaims{
		"sub":        user.ID,
		"first_name": user.FirstName,
		"last_name":  user.LastName,
		"is_admin":   isAdmin,
		"iss":        "kronus",
		"iat":        time.Now().UTC().Unix(),
		"exp":        time.Now().UTC().Add(24 * time.Hour).Unix(),
	})

	if err != nil {
		writeResponse(rw, (ResponsePayload{Errors: []string{err.Error()}}), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(rw).Encode(ResponsePayload{Success: true, Data: TokenPayload{Token: token}})
}

func healthCheckHandler(rw http.ResponseWriter, r *http.Request) {
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
