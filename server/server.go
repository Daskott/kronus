package server

import (
	"fmt"
	"log"
	"net/http"
	"regexp"

	"github.com/Daskott/kronus/database"
	"github.com/gorilla/mux"
)

func Start() {
	port := 3000
	router := mux.NewRouter()

	router.Use(loggingMiddleware)

	router.HandleFunc("/health", func(rw http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(rw, "OK\n")
	})

	router.HandleFunc("/users", createUser).Methods("POST")
	router.HandleFunc("/users/{id:[0-9]+}", findUser).Methods("GET")
	router.HandleFunc("/users/{id:[0-9]+}", updateUser).Methods("PUT")
	router.HandleFunc("/users/{id:[0-9]+}", deleteUser).Methods("DELETE")
	router.HandleFunc("/login", logIn).Methods("POST")

	database.AutoMigrate()

	fmt.Printf("Kronus server is listening on port:%v...\n", port)
	err := http.ListenAndServe(fmt.Sprintf(":%v", port), router)
	if err != nil {
		log.Fatal(err)

	}
}

func isValidatePhoneNumber(phoneNumber string) bool {
	re := regexp.MustCompile(`^\+(?:[0-9] ?){6,14}[0-9]$`)
	return re.MatchString(phoneNumber)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.Method, r.RequestURI)
		next.ServeHTTP(w, r)
	})
}
