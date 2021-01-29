package main

import (
	"github.com/go-http-utils/logger"
	"github.com/victorlenerd/scheduler0/server/src/controllers"
	"github.com/victorlenerd/scheduler0/server/src/db"
	"github.com/victorlenerd/scheduler0/server/src/middlewares"
	"github.com/victorlenerd/scheduler0/server/src/process"
	"github.com/victorlenerd/scheduler0/server/src/utils"
	"github.com/gorilla/mux"
	"github.com/unrolled/secure"
	"io"
	"log"
	"net/http"
	"os"
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
	executionController := controllers.ExecutionController{Pool: pool}
	jobController := controllers.JobController{Pool: pool}
	projectController := controllers.ProjectController{Pool: pool}
	credentialController := controllers.CredentialController{Pool: pool}

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
	router.HandleFunc("/credentials/{id}", credentialController.GetOne).Methods(http.MethodGet)
	router.HandleFunc("/credentials/{id}", credentialController.UpdateOne).Methods(http.MethodPut)
	router.HandleFunc("/credentials/{id}", credentialController.DeleteOne).Methods(http.MethodDelete)

	// Job Endpoint
	router.HandleFunc("/jobs", jobController.CreateJob).Methods(http.MethodPost)
	router.HandleFunc("/jobs", jobController.ListJobs).Methods(http.MethodGet)
	router.HandleFunc("/jobs/{id}", jobController.GetAJob).Methods(http.MethodGet)
	router.HandleFunc("/jobs/{id}", jobController.UpdateJob).Methods(http.MethodPut)
	router.HandleFunc("/jobs/{id}", jobController.DeleteJob).Methods(http.MethodDelete)

	// Projects Endpoint
	router.HandleFunc("/projects", projectController.CreateOne).Methods(http.MethodPost)
	router.HandleFunc("/projects", projectController.GetAll).Methods(http.MethodGet)
	router.HandleFunc("/projects/{id}", projectController.GetOne).Methods(http.MethodGet)
	router.HandleFunc("/projects/{id}", projectController.UpdateOne).Methods(http.MethodPut)
	router.HandleFunc("/projects/{id}", projectController.DeleteOne).Methods(http.MethodDelete)

	log.Println("Server is running on port", utils.GetPort(), utils.GetClientHost())
	err = http.ListenAndServe(utils.GetPort(), logger.Handler(router, os.Stdout, logger.DevLoggerType))
	utils.CheckErr(err)
}
