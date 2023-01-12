package http_server

import (
	"fmt"
	httpLogger "github.com/go-http-utils/logger"
	"github.com/gorilla/mux"
	"github.com/unrolled/secure"
	"golang.org/x/net/context"
	"log"
	"net/http"
	"os"
	"scheduler0/config"
	"scheduler0/db"
	"scheduler0/fsm"
	"scheduler0/http_server/controllers"
	"scheduler0/http_server/middlewares"
	"scheduler0/repository"
	"scheduler0/service"
	"scheduler0/service/executor"
	"scheduler0/service/node"
	"scheduler0/service/queue"
	"scheduler0/utils"
)

// Start this will start the http server
func Start() {
	ctx := context.Background()
	logger := log.New(os.Stderr, "[http-server] ", log.LstdFlags)

	schedulerTime := utils.GetSchedulerTime()
	err := schedulerTime.SetTimezone("UTC")
	if err != nil {
		logger.Fatalln("failed to set timezone for s")
	}

	configs := config.GetConfigurations(logger)

	sqliteDb := db.GetDBConnection(logger)
	fsmStr := fsm.NewFSMStore(sqliteDb, logger)

	//repository
	credentialRepo := repository.NewCredentialRepo(logger, fsmStr)
	jobRepo := repository.NewJobRepo(logger, fsmStr)
	projectRepo := repository.NewProjectRepo(logger, fsmStr, jobRepo)
	executionsRepo := repository.NewExecutionsRepo(logger, fsmStr)
	jobQueueRepo := repository.NewJobQueuesRepo(logger, fsmStr)

	jobExecutor := executor.NewJobExecutor(ctx, logger, jobRepo, executionsRepo, jobQueueRepo)
	jobQueue := queue.NewJobQueue(ctx, logger, fsmStr, jobExecutor, jobQueueRepo)
	p := node.NewNode(logger, jobExecutor, jobQueue, jobRepo, projectRepo, executionsRepo, jobQueueRepo)

	//services
	credentialService := service.NewCredentialService(logger, credentialRepo, ctx)
	jobService := service.NewJobService(logger, jobRepo, jobQueue, projectRepo, ctx)
	projectService := service.NewProjectService(logger, projectRepo)

	// HTTP router setup
	router := mux.NewRouter()

	// Security middleware
	secureMiddleware := secure.New(secure.Options{FrameDeny: true})

	// Initialize controllers
	jobController := controllers.NewJoBHTTPController(logger, jobService, projectService)
	projectController := controllers.NewProjectController(logger, projectService)
	credentialController := controllers.NewCredentialController(logger, credentialService)
	healthCheckController := controllers.NewHealthCheckController(logger, fsmStr)
	peerController := controllers.NewPeerController(logger, fsmStr, p)

	// Mount middleware
	middleware := middlewares.NewMiddlewareHandler(logger)

	router.Use(secureMiddleware.Handler)
	router.Use(mux.CORSMethodMiddleware(router))
	router.Use(middleware.ContextMiddleware)
	router.Use(middleware.AuthMiddleware(credentialService))
	router.Use(middleware.EnsureRaftLeaderMiddleware(p))

	// Credentials Endpoint
	router.HandleFunc("/credentials", credentialController.CreateOneCredential).Methods(http.MethodPost)
	router.HandleFunc("/credentials", credentialController.ListCredentials).Methods(http.MethodGet)
	router.HandleFunc("/credentials/{id}", credentialController.GetOneCredential).Methods(http.MethodGet)
	router.HandleFunc("/credentials/{id}", credentialController.UpdateOneCredential).Methods(http.MethodPut)
	router.HandleFunc("/credentials/{id}", credentialController.DeleteOneCredential).Methods(http.MethodDelete)

	// Job Endpoint
	router.HandleFunc("/jobs", jobController.BatchCreateJobs).Methods(http.MethodPost)
	router.HandleFunc("/jobs", jobController.ListJobs).Methods(http.MethodGet)
	router.HandleFunc("/jobs/{id}", jobController.GetOneJob).Methods(http.MethodGet)
	router.HandleFunc("/jobs/{id}", jobController.UpdateOneJob).Methods(http.MethodPut)
	router.HandleFunc("/jobs/{id}", jobController.DeleteOneJob).Methods(http.MethodDelete)

	// Projects Endpoint
	router.HandleFunc("/projects", projectController.CreateOneProject).Methods(http.MethodPost)
	router.HandleFunc("/projects", projectController.ListProjects).Methods(http.MethodGet)
	router.HandleFunc("/projects/{id}", projectController.GetOneProject).Methods(http.MethodGet)
	router.HandleFunc("/projects/{id}", projectController.UpdateOneProject).Methods(http.MethodPut)
	router.HandleFunc("/projects/{id}", projectController.DeleteOneProject).Methods(http.MethodDelete)

	// Healthcheck Endpoint
	router.HandleFunc("/healthcheck", healthCheckController.HealthCheck).Methods(http.MethodGet)

	// Node Endpoints
	router.HandleFunc("/peer-handshake", peerController.Handshake).Methods(http.MethodGet)
	router.HandleFunc("/execution-logs", peerController.ExecutionLogs).Methods(http.MethodGet)

	router.PathPrefix("/api-docs/").Handler(http.StripPrefix("/api-docs/", http.FileServer(http.Dir("./server/http_server/api-docs/"))))

	logger.Println("Server is running on port", configs.Port)
	go func() {
		err := http.ListenAndServe(fmt.Sprintf(":%v", configs.Port), httpLogger.Handler(router, os.Stderr, httpLogger.CombineLoggerType))
		if err != nil {
			logger.Fatal("failed to start http-server", err)
		}
	}()

	p.Boostrap(fsmStr)
}
