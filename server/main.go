package main

import (
	"cron-server/server/controllers"
	"cron-server/server/misc"
	"cron-server/server/models"
	"cron-server/server/process"
	"cron-server/server/repository"
	"github.com/gorilla/mux"
	"github.com/unrolled/secure"
	"log"
	"net/http"
)

func main() {
	pool, err := repository.NewPool(repository.CreateConnection, repository.MaxConnections)
	misc.CheckErr(err)

	// Setup logging
	log.SetFlags(0)
	log.SetOutput(new(misc.LogWriter))

	// Set time zone, create database and run migrations
	models.Setup(pool)

	// Start process to execute cron-server jobs
	go process.Start(pool)

	// HTTP router setup
	router := mux.NewRouter()

	// Security middleware
	secureMiddleware := secure.New(secure.Options{
		FrameDeny: true,
	})

	// Initialize controllers
	jobController := controllers.JobController{}
	projectController := controllers.ProjectController{}

	// Mount middleware
	router.Use(secureMiddleware.Handler)
	router.Use(mux.CORSMethodMiddleware(router))

	// Job Endpoint
	router.HandleFunc("/jobs/", jobController.GetAllOrCreateOne).Methods(http.MethodPost, http.MethodGet)
	router.HandleFunc("/jobs/{id}", jobController.GetOne).Methods(http.MethodGet)
	router.HandleFunc("/jobs/{id}", jobController.UpdateOne).Methods(http.MethodPut)
	router.HandleFunc("/jobs/{id}", jobController.DeleteOne).Methods(http.MethodDelete)

	// Projects Endpoint
	router.HandleFunc("/projects", projectController.GetAllOrCreateOne).Methods(http.MethodPost, http.MethodGet)
	router.HandleFunc("/projects/{id}", projectController.GetOne).Methods(http.MethodGet)
	router.HandleFunc("/projects/{id}", projectController.UpdateOne).Methods(http.MethodPut)
	router.HandleFunc("/projects/{id}", projectController.DeleteOne).Methods(http.MethodDelete)

	err = http.ListenAndServe(misc.GetPort(), router)
	misc.CheckErr(err)
}
