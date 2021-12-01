package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/Daskott/kronus/database"
	"github.com/Daskott/kronus/server/auth"
	"github.com/fatih/color"
	"github.com/gorilla/mux"
)

var (
	redColor    = color.New(color.FgRed).SprintFunc()
	yellowColor = color.New(color.FgYellow).SprintFunc()
	greenColor  = color.New(color.FgGreen).SprintFunc()
)

type ResponseWriterWithStatus struct {
	http.ResponseWriter
	Status int
}

func (r *ResponseWriterWithStatus) WriteHeader(status int) {
	r.Status = status
	r.ResponseWriter.WriteHeader(status)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		responseWriter := &ResponseWriterWithStatus{
			ResponseWriter: w,
			Status:         200,
		}

		defer func() {
			responseStatus := greenColor(responseWriter.Status)
			if responseWriter.Status > 400 {
				responseStatus = redColor(responseWriter.Status)
			}

			log.Println(
				r.Method,
				r.RequestURI,
				responseStatus,
				yellowColor(fmt.Sprintf("[%v]", time.Since(start))))
		}()

		next.ServeHTTP(responseWriter, r)
	})
}

func initialContextMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		w.Header().Add("Content-Type", "application/json")

		// 	Add decoded token & requestUserID to request context
		ctx := context.WithValue(r.Context(), RequestContextKey("decodedJWT"), decodeAndVerifyAuthHeader(r.Header.Get("Authorization")))
		ctx = context.WithValue(ctx, RequestContextKey("requestUserID"), vars["id"])

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func protectedRouteMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)

		decodedJWT := r.Context().Value(RequestContextKey("decodedJWT")).(DecodedJWT)
		if decodedJWT.ErrorMsg != "" {
			writeResponse(w, ResponsePayload{Errors: []string{decodedJWT.ErrorMsg}}, http.StatusUnauthorized)
			return
		}

		// client is only able to update/view their own record unless client is an admin
		if vars["id"] != "" && vars["id"] != fmt.Sprint(decodedJWT.Claims.Subject) && !decodedJWT.Claims.IsAdmin {
			writeResponse(w, ResponsePayload{Errors: []string{"action is forbidden"}}, http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func adminRouteMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The very first user is allowed to create an account without a token
		decodedJWT := r.Context().Value(RequestContextKey("decodedJWT")).(DecodedJWT)
		if strings.Contains(decodedJWT.ErrorMsg, "no token") && !database.AtLeastOneUserExists() {
			if r.Method == "POST" && strings.Contains(r.RequestURI, "/users") {
				next.ServeHTTP(w, r)
				return
			}
		}

		if decodedJWT.ErrorMsg != "" {
			writeResponse(w, ResponsePayload{Errors: []string{decodedJWT.ErrorMsg}}, http.StatusUnauthorized)
			return
		}

		if !decodedJWT.Claims.IsAdmin {
			writeResponse(w, ResponsePayload{Errors: []string{"action is forbidden"}}, http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// ---------------------------------------------------------------------------------//
// Helper functions
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
	err = database.FindUserBy(&database.User{}, "id", tokenClaims.Subject)
	if err != nil {
		return DecodedJWT{ErrorMsg: "invalid token provided"}
	}

	return DecodedJWT{Claims: tokenClaims}
}
