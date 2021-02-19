package main

import (
	"github.com/go-http-utils/logger"
	"github.com/gorilla/mux"
	"github.com/unrolled/secure"
	"io"
	"log"
	"net/http"
	"os"
	"scheduler0/server/src/controllers/credential"
	"scheduler0/server/src/controllers/execution"
	"scheduler0/server/src/controllers/job"
	"scheduler0/server/src/controllers/project"
	"scheduler0/server/src/db"
	"scheduler0/server/src/middlewares"
	"scheduler0/server/src/process"
	"scheduler0/server/src/utils"
)

func getDatabaseConnectionForEnvironment() (closer io.Closer, err error) {
	env := os.Getenv("ENV")
	return db.CreateConnectionEnv(env)
}

func main() {
	pool, err := utils.NewPool(getDatabaseConnectionForEnvironment, db.MaxConnections)
	if err != nil {
		panic(err)
	}

	// SetupDB logging
	log.SetFlags(0)
	log.SetOutput(new(utils.LogWriter))

	// Set time zone, create database and run db
	db.CreateModelTables(pool)
	db.SeedDatabase(pool)
	//db.RunSQLMigratiqons(pool)

	// Start process to execute cron-server jobs
	go process.Start(pool)

	// HTTP router setup
	router := mux.NewRouter()

	// Security middleware
	secureMiddleware := secure.New(secure.Options{FrameDeny: true})

	// Initialize controllers
	executionController := execution.Controller{Pool: pool}
	jobController := job.JobController{Pool: pool}
	projectController := project.ProjectController{Pool: pool}
	credentialController := credential.CredentialController{Pool: pool}

	// Mount middleware
	middleware := middlewares.MiddlewareType{}

	router.Use(secureMiddleware.Handler)
	router.Use(mux.CORSMethodMiddleware(router))
	router.Use(middleware.ContextMiddleware)
	router.Use(middleware.AuthMiddleware(pool))

	// Executions Endpoint
	router.HandleFunc("/executions", executionController.List).Methods(http.MethodGet)

	// Credentials Endpoint
	router.HandleFunc("/credentials", credentialController.CreateOne).Methods(http.MethodPost)
	router.HandleFunc("/credentials", credentialController.List).Methods(http.MethodGet)
	router.HandleFunc("/credentials/{uuid}", credentialController.GetOne).Methods(http.MethodGet)
	router.HandleFunc("/credentials/{uuid}", credentialController.UpdateOne).Methods(http.MethodPut)
	router.HandleFunc("/credentials/{uuid}", credentialController.DeleteOne).Methods(http.MethodDelete)

	// Job Endpoint
	router.HandleFunc("/jobs", jobController.CreateOne).Methods(http.MethodPost)
	router.HandleFunc("/jobs", jobController.List).Methods(http.MethodGet)
	router.HandleFunc("/jobs/{uuid}", jobController.GetOne).Methods(http.MethodGet)
	router.HandleFunc("/jobs/{uuid}", jobController.UpdateOne).Methods(http.MethodPut)
	router.HandleFunc("/jobs/{uuid}", jobController.DeleteOne).Methods(http.MethodDelete)

	// Projects Endpoint
	router.HandleFunc("/projects", projectController.CreateOne).Methods(http.MethodPost)
	router.HandleFunc("/projects", projectController.List).Methods(http.MethodGet)
	router.HandleFunc("/projects/{uuid}", projectController.GetOne).Methods(http.MethodGet)
	router.HandleFunc("/projects/{uuid}", projectController.UpdateOne).Methods(http.MethodPut)
	router.HandleFunc("/projects/{uuid}", projectController.DeleteOne).Methods(http.MethodDelete)

	log.Println("Server is running on port", utils.GetPort(), utils.GetClientHost())
	err = http.ListenAndServe(utils.GetPort(), logger.Handler(router, os.Stdout, logger.DevLoggerType))
	utils.CheckErr(err)
}
