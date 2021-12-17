package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Daskott/kronus/database"
	"github.com/Daskott/kronus/server/auth"
	"github.com/Daskott/kronus/server/logger"
	"github.com/Daskott/kronus/server/pbscheduler"
	"github.com/go-playground/validator"
	"github.com/gorilla/mux"
)

type RequestContextKey string

type DecodedJWT struct {
	Claims   *auth.KronusTokenClaims
	ErrorMsg string
}

var (
	probeScheduler *pbscheduler.ProbeScheduler

	validate = validator.New()
	logg     = logger.NewLogger()
)

func init() {
	var err error

	err = Registervalidators(validate)
	if err != nil {
		logg.Panic(err)
	}

	probeScheduler, err = pbscheduler.NewProbeScheduler()
	if err != nil {
		logg.Panic(err)
	}
}

func Start() {
	port := 3000
	router := mux.NewRouter()
	protectedRouter := router.NewRoute().Subrouter()
	adminRouter := router.NewRoute().Subrouter()

	server := &http.Server{
		Addr:    fmt.Sprintf(":%v", port),
		Handler: router,
	}

	protectedRouter.HandleFunc("/users/{uid:[0-9]+}", findUserHandler).Methods("GET")
	protectedRouter.HandleFunc("/users/{uid:[0-9]+}", updateUserHandler).Methods("PUT")
	protectedRouter.HandleFunc("/users/{uid:[0-9]+}", deleteUserHandler).Methods("DELETE")
	protectedRouter.HandleFunc("/users/{uid:[0-9]+}/probe_settings", updateProbeSettingsHandler).Methods("PUT")
	protectedRouter.Use(protectedRouteMiddleware)

	adminRouter.HandleFunc("/users", createUserHandler).Methods("POST")
	adminRouter.Use(adminRouteMiddleware)

	router.HandleFunc("/jwks", jwksHandler)
	router.HandleFunc("/health", healthCheckHandler)
	router.HandleFunc("/login", logInHandler).Methods("POST")
	router.Use(loggingMiddleware, initialContextMiddleware)

	database.AutoMigrate()

	// Start liveliness probe job workers
	probeScheduler.StartWorkers()

	// Start server
	go serve(server)

	// Wait for a signal to quit:
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	<-signalChan

	// Shutdown gracefully
	cleanup(probeScheduler, server)
}

func serve(server *http.Server) {
	logg.Infof("Kronus server is listening on port:%v", server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logg.Fatal(err)

	}
}

func cleanup(probeScheduler *pbscheduler.ProbeScheduler, server *http.Server) {
	// Stop liveliness probe job workers
	probeScheduler.StopWorkers()

	// Shutdown server gracefully
	ctxShutDown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctxShutDown); err != nil {
		logg.Fatalf("Kronus server shutdown failed:%+s", err)
	}

	logg.Infof("Kronus server stopped properly")
}
