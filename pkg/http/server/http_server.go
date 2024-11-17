package server

import (
	"context"
	"fmt"
	httpLogger "github.com/go-http-utils/logger"
	"github.com/gorilla/mux"
	"github.com/hashicorp/go-hclog"
	"github.com/unrolled/secure"
	"log"
	"net/http"
	"os"
	"scheduler0/pkg/config"
	"scheduler0/pkg/constants"
	"scheduler0/pkg/http/server/controllers"
	"scheduler0/pkg/http/server/middlewares"
	"scheduler0/pkg/secrets"
	"scheduler0/pkg/service"
)

// Start this will start the http server
func Start() {
	ctx := context.Background()
	logger := log.New(os.Stderr, "[http-server] ", log.LstdFlags)
	configs := config.NewScheduler0Config().GetConfigurations()
	appLogger := hclog.New(&hclog.LoggerOptions{
		Name:  "scheduler0",
		Level: hclog.LevelFromString(configs.LogLevel),
	})

	serv := service.NewService(ctx, appLogger)

	// HTTP router setup
	router := mux.NewRouter()

	// Security middleware
	secureMiddleware := secure.New(secure.Options{FrameDeny: true})

	// Initialize controllers
	jobController := controllers.NewJoBHTTPController(logger, serv.JobService, serv.ProjectService)
	projectController := controllers.NewProjectController(logger, serv.ProjectService)
	credentialController := controllers.NewCredentialController(logger, serv.CredentialService)
	healthCheckController := controllers.NewHealthCheckController(logger, serv.NodeService)
	peerController := controllers.NewPeerController(logger, configs, serv.NodeService)
	asyncTaskController := controllers.NewAsyncTaskController(logger, serv.AsyncTaskService)

	secrets := secrets.NewScheduler0Secrets().GetSecrets()
	// Mount middleware
	middleware := middlewares.NewMiddlewareHandler(logger, secrets, configs)

	router.Use(secureMiddleware.Handler)
	router.Use(mux.CORSMethodMiddleware(router))
	router.Use(middleware.ContextMiddleware)
	router.Use(middleware.AuthMiddleware(serv.CredentialService))
	router.Use(middleware.EnsureRaftLeaderMiddleware(serv.NodeService))

	// Credentials Endpoint
	router.HandleFunc(fmt.Sprintf("%s/credentials", constants.APIV1Base), credentialController.CreateOneCredential).Methods(http.MethodPost)
	router.HandleFunc(fmt.Sprintf("%s/credentials", constants.APIV1Base), credentialController.ListCredentials).Methods(http.MethodGet)
	router.HandleFunc(fmt.Sprintf("%s/credentials/{id}", constants.APIV1Base), credentialController.GetOneCredential).Methods(http.MethodGet)
	router.HandleFunc(fmt.Sprintf("%s/credentials/{id}", constants.APIV1Base), credentialController.UpdateOneCredential).Methods(http.MethodPut)
	router.HandleFunc(fmt.Sprintf("%s/credentials/{id}", constants.APIV1Base), credentialController.DeleteOneCredential).Methods(http.MethodDelete)

	// JobService Endpoint
	router.HandleFunc(fmt.Sprintf("%s/jobs", constants.APIV1Base), jobController.BatchCreateJobs).Methods(http.MethodPost)
	router.HandleFunc(fmt.Sprintf("%s/jobs", constants.APIV1Base), jobController.ListJobs).Methods(http.MethodGet)
	router.HandleFunc(fmt.Sprintf("%s/jobs/{id}", constants.APIV1Base), jobController.GetOneJob).Methods(http.MethodGet)
	router.HandleFunc(fmt.Sprintf("%s/jobs/{id}", constants.APIV1Base), jobController.UpdateOneJob).Methods(http.MethodPut)
	router.HandleFunc(fmt.Sprintf("%s/jobs/{id}", constants.APIV1Base), jobController.DeleteOneJob).Methods(http.MethodDelete)

	// Projects Endpoint
	router.HandleFunc(fmt.Sprintf("%s/projects", constants.APIV1Base), projectController.CreateOneProject).Methods(http.MethodPost)
	router.HandleFunc(fmt.Sprintf("%s/projects", constants.APIV1Base), projectController.ListProjects).Methods(http.MethodGet)
	router.HandleFunc(fmt.Sprintf("%s/projects/{id}", constants.APIV1Base), projectController.GetOneProject).Methods(http.MethodGet)
	router.HandleFunc(fmt.Sprintf("%s/projects/{id}", constants.APIV1Base), projectController.UpdateOneProject).Methods(http.MethodPut)
	router.HandleFunc(fmt.Sprintf("%s/projects/{id}", constants.APIV1Base), projectController.DeleteOneProject).Methods(http.MethodDelete)

	// Healthcheck Endpoint
	router.HandleFunc(fmt.Sprintf("%s/healthcheck", constants.APIV1Base), healthCheckController.HealthCheck).Methods(http.MethodGet)

	// NodeService Endpoints
	router.HandleFunc(fmt.Sprintf("%s/peer-handshake", constants.APIV1Base), peerController.Handshake).Methods(http.MethodGet)
	router.HandleFunc(fmt.Sprintf("%s/execution-logs", constants.APIV1Base), peerController.ExecutionLogs).Methods(http.MethodGet)
	router.HandleFunc(fmt.Sprintf("%s/start-jobs", constants.APIV1Base), peerController.StartJobs).Methods(http.MethodPost)
	router.HandleFunc(fmt.Sprintf("%s/stop-jobs", constants.APIV1Base), peerController.StopJobs).Methods(http.MethodPost)

	// AsyncTask
	router.HandleFunc(fmt.Sprintf("%s/async-tasks/{id}", constants.APIV1Base), asyncTaskController.GetTask).Methods(http.MethodGet)

	logger.Println("Server is running on port", configs.Port)

	serv.NodeService.Start()

	err := http.ListenAndServe(fmt.Sprintf(":%v", configs.Port), httpLogger.Handler(router, os.Stderr, httpLogger.CombineLoggerType))
	if err != nil {
		logger.Fatal("failed to start http-server", err)
	}
}
