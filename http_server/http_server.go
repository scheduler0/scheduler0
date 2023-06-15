package http_server

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
	"scheduler0/config"
	"scheduler0/http_server/controllers"
	"scheduler0/http_server/middlewares"
	"scheduler0/secrets"
	"scheduler0/service"
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
	healthCheckController := controllers.NewHealthCheckController(logger, serv.NodeService.FsmStore)
	peerController := controllers.NewPeerController(logger, configs, serv.NodeService.FsmStore, serv.NodeService)
	asyncTaskController := controllers.NewAsyncTaskController(logger, serv.NodeService.FsmStore, serv.AsyncTaskManager)

	secrets := secrets.NewScheduler0Secrets().GetSecrets()
	// Mount middleware
	middleware := middlewares.NewMiddlewareHandler(logger, secrets, configs)

	router.Use(secureMiddleware.Handler)
	router.Use(mux.CORSMethodMiddleware(router))
	router.Use(middleware.ContextMiddleware)
	router.Use(middleware.AuthMiddleware(serv.CredentialService))
	router.Use(middleware.EnsureRaftLeaderMiddleware(serv.NodeService))

	// Credentials Endpoint
	router.HandleFunc("/v1/credentials", credentialController.CreateOneCredential).Methods(http.MethodPost)
	router.HandleFunc("/v1/credentials", credentialController.ListCredentials).Methods(http.MethodGet)
	router.HandleFunc("/v1/credentials/{id}", credentialController.GetOneCredential).Methods(http.MethodGet)
	router.HandleFunc("/v1/credentials/{id}", credentialController.UpdateOneCredential).Methods(http.MethodPut)
	router.HandleFunc("/v1/credentials/{id}", credentialController.DeleteOneCredential).Methods(http.MethodDelete)

	// JobService Endpoint
	router.HandleFunc("/v1/jobs", jobController.BatchCreateJobs).Methods(http.MethodPost)
	router.HandleFunc("/v1/jobs", jobController.ListJobs).Methods(http.MethodGet)
	router.HandleFunc("/v1/jobs/{id}", jobController.GetOneJob).Methods(http.MethodGet)
	router.HandleFunc("/v1/jobs/{id}", jobController.UpdateOneJob).Methods(http.MethodPut)
	router.HandleFunc("/v1/jobs/{id}", jobController.DeleteOneJob).Methods(http.MethodDelete)

	// Projects Endpoint
	router.HandleFunc("/v1/projects", projectController.CreateOneProject).Methods(http.MethodPost)
	router.HandleFunc("/v1/projects", projectController.ListProjects).Methods(http.MethodGet)
	router.HandleFunc("/v1/projects/{id}", projectController.GetOneProject).Methods(http.MethodGet)
	router.HandleFunc("/v1/projects/{id}", projectController.UpdateOneProject).Methods(http.MethodPut)
	router.HandleFunc("/v1/projects/{id}", projectController.DeleteOneProject).Methods(http.MethodDelete)

	// Healthcheck Endpoint
	router.HandleFunc("/v1/healthcheck", healthCheckController.HealthCheck).Methods(http.MethodGet)

	// Node Endpoints
	router.HandleFunc("/v1/peer-handshake", peerController.Handshake).Methods(http.MethodGet)
	router.HandleFunc("/v1/execution-logs", peerController.ExecutionLogs).Methods(http.MethodGet)

	// AsyncTask
	router.HandleFunc("/v1/async-tasks/{id}", asyncTaskController.GetTask).Methods(http.MethodGet)

	router.PathPrefix("/v1/api-docs/").Handler(http.StripPrefix("/v1/api-docs/", http.FileServer(http.Dir("./server/http_server/api-docs/"))))

	logger.Println("Server is running on port", configs.Port)
	go func() {
		err := http.ListenAndServe(fmt.Sprintf(":%v", configs.Port), httpLogger.Handler(router, os.Stderr, httpLogger.CombineLoggerType))
		if err != nil {
			logger.Fatal("failed to start http-server", err)
		}
	}()

	serv.NodeService.Boostrap()
}
