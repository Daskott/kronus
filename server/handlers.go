package server

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Daskott/kronus/server/auth"
	"github.com/Daskott/kronus/server/auth/key"
	"github.com/Daskott/kronus/server/models"
	"github.com/Daskott/kronus/server/pbscheduler"
	"github.com/Daskott/kronus/server/work"
	"github.com/gorilla/mux"

	"github.com/golang-jwt/jwt"
	"gorm.io/gorm"
)

type ResponsePayload struct {
	Errors  []string    `json:"errors,omitempty"`
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Paging  interface{} `json:"paging,omitempty"`
}

type TokenPayload struct {
	Token string `json:"token"`
}

type TwilioSmsResponse struct {
	XMLName xml.Name `xml:"Response"`
	Message string
}

func createUserHandler(rw http.ResponseWriter, r *http.Request) {
	user := models.User{}
	decoder := json.NewDecoder(r.Body)
	assignedRole := models.BASIC_USER_ROLE

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
		assignedRole = models.ADMIN_USER_ROLE
	}

	role, err := models.FindRole(assignedRole)
	if err != nil {
		writeResponse(rw, ResponsePayload{Errors: []string{err.Error()}}, http.StatusInternalServerError)
		return
	}
	user.RoleID = role.ID

	err = models.CreateUser(&user)

	if errors.Is(err, models.ErrDuplicateUserEmail) || errors.Is(err, models.ErrDuplicateUserNumber) {
		writeResponse(rw, ResponsePayload{Errors: []string{err.Error()}}, http.StatusBadRequest)
		return
	}

	if err != nil {
		writeResponse(rw, ResponsePayload{Errors: []string{err.Error()}}, http.StatusInternalServerError)
		return
	}

	writeResponse(rw, ResponsePayload{Success: true, Data: user}, http.StatusOK)
}

func fetchUsersHandler(rw http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))

	users, paging, err := models.FetchUsers(page)
	if err != nil {
		writeResponse(rw, ResponsePayload{Errors: []string{err.Error()}}, http.StatusInternalServerError)
		return
	}

	writeResponse(rw, ResponsePayload{Success: true, Data: users, Paging: paging}, http.StatusOK)
}

func findUserHandler(rw http.ResponseWriter, r *http.Request) {
	user, err := models.FindUserBy("ID", r.Context().Value(RequestContextKey("userID")))
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
	err := models.DeleteUser(r.Context().Value(RequestContextKey("userID")))
	if err != nil {
		writeResponse(rw, ResponsePayload{Errors: []string{err.Error()}}, http.StatusInternalServerError)
		return
	}

	writeResponse(rw, ResponsePayload{Success: true}, http.StatusOK)
}

func updateUserHandler(rw http.ResponseWriter, r *http.Request) {
	var errs []string

	currentUser := r.Context().Value(RequestContextKey("currentUser")).(*models.User)
	params := make(map[string]interface{})
	decoder := json.NewDecoder(r.Body)

	err := decoder.Decode(&params)
	if err != nil {
		writeResponse(rw, ResponsePayload{Errors: []string{err.Error()}}, http.StatusInternalServerError)
		return
	}

	removeUnknownFields(params, map[string]bool{
		"email":        true,
		"first_name":   true,
		"last_name":    true,
		"phone_number": true,
		"password":     true,
	})
	if len(params) <= 0 {
		writeResponse(rw,
			ResponsePayload{Errors: []string{"valid fields required"}},
			http.StatusBadRequest,
		)
		return
	}

	if params["first_name"] != nil && len(strings.TrimSpace(params["first_name"].(string))) <= 0 {
		errs = append(errs, "valid first_name is required")
	}

	if params["last_name"] != nil && len(strings.TrimSpace(params["last_name"].(string))) <= 0 {
		errs = append(errs, "valid last_name is required")
	}

	if params["password"] != nil {
		if err := validate.Var(params["password"], "password"); err != nil {
			errs = append(errs, "valid password is required")
		}
	}

	if params["phone_number"] != nil {
		if err := validate.Var(params["phone_number"], "e164"); err != nil {
			errs = append(errs, "valid phone_number is required")
		}
	}

	if params["email"] != nil {
		if err := validate.Var(params["email"], "email"); err != nil {
			errs = append(errs, "valid email is required")
		}
	}

	if len(errs) > 0 {
		writeResponse(rw, ResponsePayload{Errors: errs}, http.StatusBadRequest)
		return
	}

	err = currentUser.Update(params)

	if errors.Is(err, models.ErrDuplicateUserEmail) || errors.Is(err, models.ErrDuplicateUserNumber) {
		writeResponse(rw, ResponsePayload{Errors: []string{err.Error()}}, http.StatusBadRequest)
		return
	}

	if err != nil {
		writeResponse(rw, ResponsePayload{Errors: []string{err.Error()}}, http.StatusInternalServerError)
		return
	}

	writeResponse(rw, ResponsePayload{Success: true, Data: currentUser}, http.StatusOK)
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

	removeUnknownFields(params, map[string]bool{"active": true, "cron_expression": true})
	if len(params) <= 0 {
		writeResponse(rw,
			ResponsePayload{Errors: []string{"valid fields required"}},
			http.StatusBadRequest,
		)
		return
	}

	if _, ok := params["active"].(bool); params["active"] != nil && !ok {
		errs = append(errs, "'active' field must be a boolean e.g. true/false")
	}

	if params["cron_expression"] != nil && !isValidCronExpression(params["cron_expression"].(string)) {
		errs = append(errs, "'cron_expression' field must be valid e.g. '0 18 * * 3'")
	}

	if len(errs) > 0 {
		writeResponse(rw, ResponsePayload{Errors: errs}, http.StatusBadRequest)
		return
	}

	// Only activate liveliness probe for users with emergency contact
	if enableProbe, ok := params["active"].(bool); ok && enableProbe {
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

	err = currentUser.UpdateProbSettings(params)
	if err != nil {
		writeResponse(rw, ResponsePayload{Errors: []string{err.Error()}}, http.StatusInternalServerError)
		return
	}

	// If probe request is `Active` after update, update the probeScheduler with the user's probe settings
	if currentUser.ProbeSettings.Active {
		probeScheduler.PeriodicallyPerfomProbe(*currentUser)
	}

	// Remove user probe from probeScheduler if disabled
	if enableProbe, ok := params["active"].(bool); ok && !enableProbe {
		if err := probeScheduler.DisablePeriodicProbe(currentUser); err != nil {
			writeResponse(rw, ResponsePayload{Errors: []string{err.Error()}}, http.StatusInternalServerError)
			return
		}
	}

	writeResponse(rw, ResponsePayload{Success: true, Data: currentUser.ProbeSettings}, http.StatusOK)
}

func createContactHandler(rw http.ResponseWriter, r *http.Request) {
	currentUser := r.Context().Value(RequestContextKey("currentUser")).(*models.User)
	contact := models.Contact{}
	decoder := json.NewDecoder(r.Body)

	err := decoder.Decode(&contact)
	if err != nil {
		writeResponse(rw, ResponsePayload{Errors: []string{err.Error()}}, http.StatusInternalServerError)
		return
	}

	errs := validate.Struct(contact)
	if errs != nil {
		writeResponse(rw, ResponsePayload{Errors: strings.Split(errs.Error(), "\n")}, http.StatusBadRequest)
		return
	}

	err = currentUser.AddContact(&contact)

	if errors.Is(err, models.ErrDuplicateContactEmail) || errors.Is(err, models.ErrDuplicateContactNumber) {
		writeResponse(rw, ResponsePayload{Errors: []string{err.Error()}}, http.StatusBadRequest)
		return
	}

	if err != nil {
		writeResponse(rw, ResponsePayload{Errors: []string{err.Error()}}, http.StatusInternalServerError)
		return
	}

	writeResponse(rw, ResponsePayload{Success: true, Data: contact}, http.StatusOK)
}

func updateContactHandler(rw http.ResponseWriter, r *http.Request) {
	var errs []string

	vars := mux.Vars(r)
	currentUser := r.Context().Value(RequestContextKey("currentUser")).(*models.User)
	params := make(map[string]interface{})
	decoder := json.NewDecoder(r.Body)

	err := decoder.Decode(&params)
	if err != nil {
		writeResponse(rw, ResponsePayload{Errors: []string{err.Error()}}, http.StatusInternalServerError)
		return
	}

	removeUnknownFields(params, map[string]bool{
		"first_name":           true,
		"last_name":            true,
		"phone_number":         true,
		"email":                true,
		"is_emergency_contact": true,
	})
	if len(params) <= 0 {
		writeResponse(rw,
			ResponsePayload{Errors: []string{"valid fields required"}},
			http.StatusBadRequest,
		)
		return
	}

	if params["first_name"] != nil && len(strings.TrimSpace(params["first_name"].(string))) <= 0 {
		errs = append(errs, "valid first_name is required")
	}

	if params["last_name"] != nil && len(strings.TrimSpace(params["last_name"].(string))) <= 0 {
		errs = append(errs, "valid last_name is required")
	}

	if params["phone_number"] != nil {
		if err := validate.Var(params["phone_number"], "required,e164"); err != nil {
			errs = append(errs, "valid phone_number is required")
		}
	}

	if params["email"] != nil {
		if err := validate.Var(params["email"], "required,email"); err != nil {
			errs = append(errs, "valid email is required")
		}
	}

	if _, ok := params["is_emergency_contact"].(bool); params["is_emergency_contact"] != nil && !ok {
		errs = append(errs, "is_emergency_contact must be a oolean e.g. true/false")
	}

	if len(errs) > 0 {
		writeResponse(rw, ResponsePayload{Errors: errs}, http.StatusBadRequest)
		return
	}

	updatedContact, err := currentUser.UpdateContact(vars["id"], params)

	if errors.Is(err, models.ErrDuplicateContactEmail) || errors.Is(err, models.ErrDuplicateContactNumber) {
		writeResponse(rw, ResponsePayload{Errors: []string{err.Error()}}, http.StatusBadRequest)
		return
	}

	if err != nil {
		writeResponse(rw, ResponsePayload{Errors: []string{err.Error()}}, http.StatusInternalServerError)
		return
	}

	writeResponse(rw, ResponsePayload{Success: true, Data: updatedContact}, http.StatusOK)
}

func deleteUserContactHandler(rw http.ResponseWriter, r *http.Request) {
	currentUser := r.Context().Value(RequestContextKey("currentUser")).(*models.User)
	vars := mux.Vars(r)

	err := currentUser.DeleteContact(vars["id"])
	if err != nil {
		writeResponse(rw, ResponsePayload{Errors: []string{err.Error()}}, http.StatusInternalServerError)
		return
	}

	writeResponse(rw, ResponsePayload{Success: true}, http.StatusOK)
}

func fetchUserProbesHandler(rw http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(RequestContextKey("userID"))
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))

	probes, paging, err := models.FetchProbes(page, "user_id = ?", userID)
	if err != nil {
		writeResponse(rw, ResponsePayload{Errors: []string{err.Error()}}, http.StatusInternalServerError)
		return
	}

	writeResponse(rw, ResponsePayload{Success: true, Data: probes, Paging: paging}, http.StatusOK)
}

func fetchUserContactsHandler(rw http.ResponseWriter, r *http.Request) {
	currentUser := r.Context().Value(RequestContextKey("currentUser")).(*models.User)
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))

	contacts, paging, err := currentUser.FetchContacts(page)
	if err != nil {
		writeResponse(rw, ResponsePayload{Errors: []string{err.Error()}}, http.StatusInternalServerError)
		return
	}

	writeResponse(rw, ResponsePayload{Success: true, Data: contacts, Paging: paging}, http.StatusOK)
}

func jobsStatsHandler(rw http.ResponseWriter, r *http.Request) {
	stats, err := models.CurrentJobsStats()
	if err != nil {
		writeResponse(rw, ResponsePayload{Errors: []string{err.Error()}}, http.StatusInternalServerError)
		return
	}

	writeResponse(rw, ResponsePayload{Success: true, Data: stats}, http.StatusOK)
}

func probeStatsHandler(rw http.ResponseWriter, r *http.Request) {
	stats, err := models.CurrentProbeStats()
	if err != nil {
		writeResponse(rw, ResponsePayload{Errors: []string{err.Error()}}, http.StatusInternalServerError)
		return
	}

	writeResponse(rw, ResponsePayload{Success: true, Data: stats}, http.StatusOK)
}

func fetchJobsHandler(rw http.ResponseWriter, r *http.Request) {
	var jobs []models.Job
	var paging *models.Paging
	var err error

	status := strings.ToLower(r.URL.Query().Get("status"))
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))

	if status != "" && !models.JobStatusNameMap[status] {
		writeResponse(rw, ResponsePayload{Errors: []string{
			fmt.Sprintf("a valid 'status' param is required i.e. %v, %v, %v or %v",
				models.ENQUEUED_JOB, models.SUCCESSFUL_JOB, models.IN_PROGRESS_JOB, models.DEAD_JOB)},
		}, http.StatusBadRequest)
		return
	}

	if status == "" {
		jobs, paging, err = models.FetchJobs(page)
	} else {
		jobs, paging, err = models.FetchJobsByStatus(status, page)
	}

	if err != nil {
		writeResponse(rw, ResponsePayload{Errors: []string{err.Error()}}, http.StatusInternalServerError)
		return
	}

	writeResponse(rw, ResponsePayload{Success: true, Data: jobs, Paging: paging}, http.StatusOK)
}

func fetchProbesHandler(rw http.ResponseWriter, r *http.Request) {
	var probes []models.Probe
	var paging *models.Paging
	var err error

	status := strings.ToLower(r.URL.Query().Get("status"))
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))

	if status != "" && !models.ProbeStatusNameMap[status] {
		writeResponse(rw, ResponsePayload{Errors: []string{
			fmt.Sprintf("a valid 'status' param is required i.e. %v, %v, %v %v, or %v",
				models.PENDING_PROBE, models.GOOD_PROBE, models.BAD_PROBE, models.CANCELLED_PROBE, models.UNAVAILABLE_PROBE)},
		}, http.StatusBadRequest)
		return
	}

	if status == "" {
		probes, paging, err = models.FetchProbes(page, nil, nil)
	} else {
		probes, paging, err = models.FetchProbesByStatus(status, "desc", page)
	}

	if err != nil {
		writeResponse(rw, ResponsePayload{Errors: []string{err.Error()}}, http.StatusInternalServerError)
		return
	}

	writeResponse(rw, ResponsePayload{Success: true, Data: probes, Paging: paging}, http.StatusOK)
}

func smsWebhookHandler(rw http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	rw.Header().Set("Content-Type", "text/xml")

	message := r.PostForm.Get("Body")

	// Validate that request is coming from twilio
	if !twilioClient.ValidateRequest(r.URL.Path, r.PostForm, r.Header.Get("X-Twilio-Signature")) {
		writeSmsWebHookResponse(rw, []byte("<Response />"), http.StatusUnauthorized)
		return
	}

	user, err := models.FindUserBy("phone_number", r.PostForm.Get("From"))
	if err != nil {
		// No need to send response if user does not exist
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeSmsWebHookResponse(rw, []byte("<Response />"), http.StatusOK)
			return
		}

		writeErrMsgForSmsWebhook(rw, err)
		return
	}

	// If it's a server ping from a user on the server - return response
	if strings.TrimSpace(strings.ToLower(message)) == "ping" {
		msgBytes, _ := xml.Marshal(&TwilioSmsResponse{Message: "PONG!"})
		writeSmsWebHookResponse(rw, msgBytes, http.StatusOK)
		return
	}

	probe, err := user.LastProbe()
	if err != nil {
		// If no probe - do nothing
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeSmsWebHookResponse(rw, []byte("<Response />"), http.StatusOK)
			return
		}

		writeErrMsgForSmsWebhook(rw, err)
		return
	}

	pendingProbe, err := probe.IsPending()
	if err != nil {
		writeErrMsgForSmsWebhook(rw, err)
		return
	}

	// If no pending probe - do nothing
	if !pendingProbe {
		writeSmsWebHookResponse(rw, []byte("<Response />"), http.StatusOK)
		return
	}

	// Determine if user probe was 'good' or 'bad' from their reply i.e. message
	probe.LastResponse = message
	probeStatusName := probe.StatusFromLastResponse()

	// if unable to determine probe status from msg - save 'LastResponse' & do nothing
	if probeStatusName == "" {
		probe.Save()
		writeSmsWebHookResponse(rw, []byte("<Response />"), http.StatusOK)
		return
	}

	probeStatus, err := models.FindProbeStatus(probeStatusName)
	if err != nil {
		writeErrMsgForSmsWebhook(rw, err)
		return
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
	writeSmsWebHookResponse(rw, msgBytes, http.StatusOK)
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
