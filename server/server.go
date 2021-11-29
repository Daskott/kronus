package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/Daskott/kronus/database"
	"github.com/Daskott/kronus/server/auth"
	"github.com/golang-jwt/jwt"
	"github.com/gorilla/mux"
)

type RequestContextKey string

type DecodedJWT struct {
	Claims   jwt.MapClaims
	ErrorMsg string
}

func Start() {
	port := 3000
	router := mux.NewRouter()
	protectedRouter := router.PathPrefix("").Subrouter()
	adminRouter := router.PathPrefix("").Subrouter()

	protectedRouter.HandleFunc("/users/{id:[0-9]+}", findUserHandler).Methods("GET")
	protectedRouter.HandleFunc("/users/{id:[0-9]+}", updateUserHandler).Methods("PUT")
	protectedRouter.HandleFunc("/users/{id:[0-9]+}", deleteUserHandler).Methods("DELETE")
	protectedRouter.Use(protectedRouteMiddleware)

	adminRouter.HandleFunc("/users", createUserHandler).Methods("POST")
	adminRouter.Use(adminRouteMiddleware)

	router.HandleFunc("/health", healthCheckHandler)
	router.HandleFunc("/login", logInHandler).Methods("POST")
	router.Use(loggingMiddleware, initialContextMiddleware)

	database.AutoMigrate()

	fmt.Printf("Kronus server is listening on port:%v...\n", port)
	err := http.ListenAndServe(fmt.Sprintf(":%v", port), router)
	if err != nil {
		log.Fatal(err)

	}
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.Method, r.RequestURI)
		next.ServeHTTP(w, r)
	})
}

func initialContextMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		w.Header().Add("Content-Type", "application/json")

		// 	Add decoded token to request context
		ctx := context.WithValue(r.Context(), RequestContextKey("decodedJWT"), decodeAuthHeaderJWT(r.Header.Get("Authorization")))
		ctx = context.WithValue(ctx, RequestContextKey("requestUserID"), decodeAuthHeaderJWT(vars["id"]))

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
		if vars["id"] != "" && vars["id"] != fmt.Sprint(decodedJWT.Claims["sub"]) && !decodedJWT.Claims["is_admin"].(bool) {
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

		if !decodedJWT.Claims["is_admin"].(bool) {
			writeResponse(w, ResponsePayload{Errors: []string{"action is forbidden"}}, http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func decodeAuthHeaderJWT(authHeaderValue string) DecodedJWT {
	authHeaderList := strings.Split(authHeaderValue, "Bearer ")
	if len(authHeaderList) < 2 {
		return DecodedJWT{ErrorMsg: "no token provided"}
	}

	claims, err := auth.DecodeJWT(authHeaderList[1])
	if err != nil {
		return DecodedJWT{ErrorMsg: "invalid token provided"}
	}

	return DecodedJWT{Claims: claims}
}
