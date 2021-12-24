package server

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/Daskott/kronus/server/auth"
	"github.com/Daskott/kronus/server/auth/key"
	"github.com/Daskott/kronus/server/gstorage"
	"github.com/Daskott/kronus/server/logger"
	"github.com/Daskott/kronus/server/models"
	"github.com/Daskott/kronus/server/pbscheduler"
	"github.com/Daskott/kronus/server/twilio"
	"github.com/Daskott/kronus/server/work"
	"github.com/Daskott/kronus/shared"
	"github.com/go-playground/validator"
	"github.com/gorilla/mux"
)

var (
	probeScheduler *pbscheduler.ProbeScheduler
	workerPool     *work.WorkerPoolAdapter
	authKeyPair    *key.KeyPair
	storage        *gstorage.GStorage
	twilioClient   *twilio.ClientWrapper
	config         *shared.ServerConfig
	configDir      string

	validate = validator.New()
	logg     = logger.NewLogger()
)

type RequestContextKey string

type DecodedJWT struct {
	Claims   *auth.KronusTokenClaims
	ErrorMsg string
}

func Start(configArg *shared.ServerConfig, devMode bool) {
	var err error

	config = configArg

	err = RegisterValidators(validate)
	fatalOnError(err)

	configDir = configDirectory(devMode)

	if enabled, ok := config.Google.Storage.EnableSqliteBackupAndSync.(bool); ok && enabled {
		storage, err = gstorage.NewGStorage(
			config.Google.ApplicationCredentials,
			config.Google.Storage.Bucket,
			config.Google.Storage.Prefix,
		)
		fatalOnError(err)
	}

	err = models.InitialiazeDb(config.Sqlite.PassPhrase, configDir, storage)
	fatalOnError(err)

	authKeyPair, err = key.NewKeyPairFromRSAPrivateKeyPem(config.Kronus.PrivateKeyPem)
	fatalOnError(err)

	workerPool = work.NewWorkerAdapter(config.Kronus.Cron.TimeZone)
	registerJobHandlers(workerPool)
	enqueueJobs(workerPool)

	twilioClient = twilio.NewClient(config.Twilio, config.Kronus.PublicUrl, devMode)

	probeScheduler, err = pbscheduler.NewProbeScheduler(workerPool, twilioClient)
	fatalOnError(err)
	probeScheduler.ScheduleProbes()

	router := mux.NewRouter()
	protectedRouter := router.NewRoute().Subrouter()
	adminRouter := router.NewRoute().Subrouter()

	server := &http.Server{
		Addr:    fmt.Sprintf(":%v", config.Kronus.Listener.Port),
		Handler: router,
	}

	protectedRouter.HandleFunc("/users/{uid:[0-9]+}", findUserHandler).Methods("GET")
	protectedRouter.HandleFunc("/users/{uid:[0-9]+}", updateUserHandler).Methods("PUT")
	protectedRouter.HandleFunc("/users/{uid:[0-9]+}", deleteUserHandler).Methods("DELETE")

	protectedRouter.HandleFunc("/users/{uid:[0-9]+}/probe_settings", updateProbeSettingsHandler).Methods("PUT")

	protectedRouter.HandleFunc("/users/{uid:[0-9]+}/contacts", fetchContactsHandler).Methods("GET")
	protectedRouter.HandleFunc("/users/{uid:[0-9]+}/contacts", createContactHandler).Methods("POST")
	protectedRouter.HandleFunc("/users/{uid:[0-9]+}/contacts/{id:[0-9]+}", updateContactHandler).Methods("PUT")
	protectedRouter.HandleFunc("/users/{uid:[0-9]+}/contacts/{id:[0-9]+}", deleteUserContactHandler).Methods("DELETE")
	protectedRouter.Use(protectedRouteMiddleware)

	adminRouter.HandleFunc("/users", createUserHandler).Methods("POST")
	adminRouter.HandleFunc("/users", allUsersHandler).Methods("GET")

	adminRouter.HandleFunc("/jobs", jobsByStatusHandler).Methods("GET")
	adminRouter.HandleFunc("/jobs/stats", jobsStatsHandler).Methods("GET")
	adminRouter.HandleFunc("/probes/stats", probeStatsHandler).Methods("GET")
	adminRouter.HandleFunc("/probes", probesByStatusHandler).Methods("GET")
	adminRouter.Use(adminRouteMiddleware)

	router.HandleFunc("/webhook/sms", smsWebhookHandler).Methods("POST")

	router.HandleFunc("/jwks", jwksHandler).Methods("GET")
	router.HandleFunc("/health", healthCheckHandler).Methods("GET")
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
	cleanup(workerPool, server, storage != nil)
}
