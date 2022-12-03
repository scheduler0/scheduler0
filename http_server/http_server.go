package http_server

import (
	"database/sql"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/unrolled/secure"
	"golang.org/x/net/context"
	"log"
	"net/http"
	"os"
	"scheduler0/config"
	"scheduler0/constants"
	"scheduler0/db"
	"scheduler0/fsm"
	"scheduler0/http_server/controllers"
	"scheduler0/http_server/middlewares"
	"scheduler0/job_executor"
	"scheduler0/job_queue"
	"scheduler0/node"
	"scheduler0/repository"
	"scheduler0/service"
)

func getDBConnection(logger *log.Logger) (*sql.DB, db.DataStore) {
	dir, err := os.Getwd()
	if err != nil {
		logger.Fatalln(fmt.Errorf("Fatal error getting working dir: %s \n", err))
	}
	dbFilePath := fmt.Sprintf("%v/%v", dir, constants.SqliteDbFileName)

	sqliteDb := db.NewSqliteDbConnection(dbFilePath)
	conn, err := sqliteDb.OpenConnection()
	if err != nil {
		logger.Fatal("Failed to open connection", err)
	}

	dbConnection := conn.(*sql.DB)
	err = dbConnection.Ping()
	if err != nil {
		logger.Fatalln(fmt.Errorf("ping error: restore failed to create db: %v", err))
	}

	return dbConnection, sqliteDb
}

// Start this will start the http server
func Start() {
	ctx := context.Background()
	logger := log.New(os.Stderr, "[http-server] ", log.LstdFlags)

	configs := config.GetScheduler0Configurations(logger)

	dbConnection, sqliteDb := getDBConnection(logger)
	fsmStr := fsm.NewFSMStore(sqliteDb, dbConnection, logger)

	//repository
	credentialRepo := repository.NewCredentialRepo(logger, fsmStr)
	jobRepo := repository.NewJobRepo(logger, fsmStr)
	projectRepo := repository.NewProjectRepo(logger, fsmStr, jobRepo)

	jobExecutor := job_executor.NewJobExecutor(logger, jobRepo)
	jobQueue := job_queue.NewJobQueue(logger, fsmStr, jobExecutor)
	p := node.NewNode(logger, jobExecutor, jobQueue, jobRepo, projectRepo)

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
	healthCheckController := controllers.NewHealthCheckController(logger, fsmStr.Raft)
	peerController := controllers.NewPeerController(logger, fsmStr.Raft, p)

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
	router.HandleFunc("/execution-logs", peerController.ExecutionLogs).Methods(http.MethodPost)

	router.PathPrefix("/api-docs/").Handler(http.StripPrefix("/api-docs/", http.FileServer(http.Dir("./server/http_server/api-docs/"))))

	logger.Println("Server is running on port", configs.Port)
	go func() {
		err := http.ListenAndServe(fmt.Sprintf(":%v", configs.Port), router)
		if err != nil {
			logger.Fatal("failed to start http-server", err)
		}
	}()

	p.Boostrap(fsmStr)
}
