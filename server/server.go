package server

import (
	"fmt"
	"log"
	"net/http"

	"github.com/Daskott/kronus/database"
	"github.com/Daskott/kronus/server/auth"
	"github.com/gorilla/mux"
)

type RequestContextKey string

type DecodedJWT struct {
	Claims   *auth.KronusTokenClaims
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

	router.HandleFunc("/jwks", jwksHandler)
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
