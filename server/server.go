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
	"github.com/Daskott/kronus/server/gstorage"
	"github.com/Daskott/kronus/server/logger"
	"github.com/Daskott/kronus/server/pbscheduler"
	"github.com/Daskott/kronus/server/work"
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
	workerPool     *work.WorkerPoolAdapter
	authKeyPair    *key.KeyPair
	storage        *gstorage.GStorage
	config         *viper.Viper
	configDir      string

	validate = validator.New()
	logg     = logger.NewLogger()
)

func Start(configArg *viper.Viper, devMode bool) {
	var err error

	config = configArg

	err = RegisterValidators(validate)
	fatalOnError(err)

	configDir = configDirectory(devMode)

	err = models.AutoMigrate(config.GetString("sqlite.passPhrase"), configDir)
	fatalOnError(err)

	if config.GetBool("google.storage.enableSQliteDbBackupAndSync") {
		storage, err = gstorage.NewGStorage(
			config.GetString("google.applicationCredentials"),
			config.GetString("google.storage.prefix"),
		)
		fatalOnError(err)
	}

	authKeyPair, err = key.NewKeyPairFromRSAPrivateKeyPem(config.GetString("kronus.privateKeyPem"))
	fatalOnError(err)

	workerPool = work.NewWorkerAdapter(config.GetString("kronus.cron.timeZone"))
	registerJobHandlers(workerPool)
	enqueueJobs(workerPool)

	probeScheduler, err = pbscheduler.NewProbeScheduler(workerPool)
	fatalOnError(err)
	probeScheduler.ScheduleProbes()

	router := mux.NewRouter()
	protectedRouter := router.NewRoute().Subrouter()
	adminRouter := router.NewRoute().Subrouter()

	server := &http.Server{
		Addr:    fmt.Sprintf(":%v", config.GetString("kronus.listener.port")),
		Handler: router,
	}

	protectedRouter.HandleFunc("/users/{uid:[0-9]+}", findUserHandler).Methods("GET")
	protectedRouter.HandleFunc("/users/{uid:[0-9]+}", updateUserHandler).Methods("PUT")
	protectedRouter.HandleFunc("/users/{uid:[0-9]+}", deleteUserHandler).Methods("DELETE")

	protectedRouter.HandleFunc("/users/{uid:[0-9]+}/probe_settings", updateProbeSettingsHandler).Methods("PUT")

	protectedRouter.HandleFunc("/users/{uid:[0-9]+}/contacts", createContactHandler).Methods("POST")
	protectedRouter.HandleFunc("/users/{uid:[0-9]+}/contacts/{id:[0-9]+}", updateContactHandler).Methods("PUT")
	protectedRouter.HandleFunc("/users/{uid:[0-9]+}/contacts/{id:[0-9]+}", deleteUserContactHandler).Methods("DELETE")
	protectedRouter.Use(protectedRouteMiddleware)

	adminRouter.HandleFunc("/users", createUserHandler).Methods("POST")
	adminRouter.Use(adminRouteMiddleware)

	router.HandleFunc("/jwks", jwksHandler)
	router.HandleFunc("/health", healthCheckHandler)
	router.HandleFunc("/login", logInHandler).Methods("POST")
	router.Use(loggingMiddleware, initialContextMiddleware)

	// Start all jobs i.e liveliness probes & regoular server jobs
	err = workerPool.Start()
	fatalOnError(err)

	// Start server
	go serve(server)

	// Wait for a signal to quit:
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	<-signalChan

	// Shutdown gracefully
	cleanup(workerPool, server)
}
