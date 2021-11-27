package server

import (
	"fmt"
	"log"
	"net/http"

	"github.com/Daskott/kronus/database"
	"github.com/gorilla/mux"
)

func Start() {
	port := 3000
	router := mux.NewRouter()

	router.HandleFunc("/health", healthCheck)
	router.HandleFunc("/users", createUser).Methods("POST")
	router.HandleFunc("/users/{id:[0-9]+}", findUser).Methods("GET")
	router.HandleFunc("/users/{id:[0-9]+}", updateUser).Methods("PUT")
	router.HandleFunc("/users/{id:[0-9]+}", deleteUser).Methods("DELETE")
	router.HandleFunc("/login", logIn).Methods("POST")
	router.Use(loggingMiddleware)

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
