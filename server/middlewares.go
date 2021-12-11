package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Daskott/kronus/colors"
	"github.com/Daskott/kronus/database"
	"github.com/gorilla/mux"
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
			Status:         500, // default to 500 status
		}

		defer func() {
			responseStatus := colors.Green(responseWriter.Status)
			if responseWriter.Status >= 500 {
				responseStatus = colors.Red(responseWriter.Status)
			}

			logg.Infof("%s  %s  %s  %s",
				colors.Yellow(fmt.Sprintf("[%v]", time.Since(start))),
				colors.Blue(r.Method),
				r.RequestURI,
				responseStatus)
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
		ctx = context.WithValue(ctx, RequestContextKey("requestUserID"), vars["uid"])

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
		if vars["uid"] != "" && vars["uid"] != fmt.Sprint(decodedJWT.Claims.Subject) && !decodedJWT.Claims.IsAdmin {
			writeResponse(w, ResponsePayload{Errors: []string{"action is forbidden"}}, http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func adminRouteMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		decodedJWT := r.Context().Value(RequestContextKey("decodedJWT")).(DecodedJWT)

		atLeastOneUserExists, err := database.AtLeastOneUserExists()
		if err != nil {
			writeResponse(w, ResponsePayload{Errors: []string{err.Error()}}, http.StatusUnauthorized)
			return
		}

		// The very first user is allowed to create an account without a token
		if strings.Contains(decodedJWT.ErrorMsg, "no token") && !atLeastOneUserExists {
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
