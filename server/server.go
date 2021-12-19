package server

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/Daskott/kronus/models"
	"github.com/Daskott/kronus/server/auth"
	"github.com/Daskott/kronus/server/auth/key"
	"github.com/Daskott/kronus/server/logger"
	"github.com/Daskott/kronus/server/pbscheduler"
	"github.com/go-playground/validator"
	"github.com/gorilla/mux"
	"github.com/spf13/viper"
)

type RequestContextKey string

type DecodedJWT struct {
	Claims   *auth.KronusTokenClaims
	ErrorMsg string
}

var (
	probeScheduler *pbscheduler.ProbeScheduler
	authKeyPair    *key.KeyPair

	validate = validator.New()
	logg     = logger.NewLogger()
)

func init() {
	var err error

	err = Registervalidators(validate)
	fatalOnError(err)

	probeScheduler, err = pbscheduler.NewProbeScheduler()
	fatalOnError(err)
}

func Start(config *viper.Viper, devMode bool) {
	var configDir string
	var err error

	router := mux.NewRouter()
	protectedRouter := router.NewRoute().Subrouter()
	adminRouter := router.NewRoute().Subrouter()

	authKeyPair, err = key.NewKeyPairFromRSAPrivateKeyPem(config.GetString("kronus.privateKeyPem"))
	fatalOnError(err)

	configDir = configDirectory(devMode)

	err = models.AutoMigrate(config.GetString("sqlite.passPhrase"), configDir)
	fatalOnError(err)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%v", config.GetString("kronus.listener.port")),
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
