package server

import (
	"fmt"
	"net/http"

	"github.com/Daskott/kronus/database"
	"github.com/Daskott/kronus/server/auth"
	"github.com/Daskott/kronus/server/cron"
	"github.com/Daskott/kronus/server/logger"
	"github.com/Daskott/kronus/server/pbscheduler"
	"github.com/go-playground/validator"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

type RequestContextKey string

type DecodedJWT struct {
	Claims   *auth.KronusTokenClaims
	ErrorMsg string
}

var validate *validator.Validate

var logg *zap.SugaredLogger

func init() {
	validate = validator.New()
	logg = logger.NewLogger()
	err := Registervalidators(validate)
	if err != nil {
		logg.Panic(err)
	}
}

func Start() {
	port := 3000
	router := mux.NewRouter()
	protectedRouter := router.NewRoute().Subrouter()
	adminRouter := router.NewRoute().Subrouter()

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

	probeScheduler := pbscheduler.NewProbeScheduler(cron.CronScheduler)
	probeScheduler.EnqueAllActiveProbes()
	probeScheduler.CronScheduler.StartAsync()

	logg.Infof("Kronus server is listening on port:%v", port)
	err := http.ListenAndServe(fmt.Sprintf(":%v", port), router)
	if err != nil {
		logg.Fatal(err)

	}
}
