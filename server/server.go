package server

import (
	"fmt"
	"log"
	"net/http"

	"github.com/Daskott/kronus/database"
)

func Start() {
	port := 3000

	http.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(rw, "Hello world!\n")
	})

	database.AutoMigrate()

	fmt.Printf("Kronus server is listening on port:%v...\n", port)
	err := http.ListenAndServe(fmt.Sprintf(":%v", port), nil)
	if err != nil {
		log.Fatal(err)
	}
}
