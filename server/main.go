package main

import (
	"cron-server/server/controllers"
	"cron-server/server/misc"
	"cron-server/server/process"
	"cron-server/server/repo"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"time"
)

func main() {

	// Setup logging
	log.SetFlags(0)
	log.SetOutput(new(misc.LogWriter))

	// Create database and set time zone
	repo.Setup()

	// Start process to execute cron-server jobs
	go func() {
		for {
			go process.ExecuteJobs()
			time.Sleep(time.Second * 1)
		}
	}()

	// Start process to update missed jobs
	go func() {
		for {
			go process.FixStaleJobs()
			time.Sleep(time.Second * 1)
		}
	}()

	// HTTP router setup
	router := mux.NewRouter()

	router.HandleFunc("/job/{service_name}", controllers.FetchJob).Methods("GET")
	router.HandleFunc("/register", controllers.RegisterJob).Methods("POST")
	router.HandleFunc("/activate/{job_id}", controllers.ActivateJob).Methods("PUT")
	router.HandleFunc("/deactivate/{job_id}", controllers.DeactivateJob).Methods("PUT")
	router.HandleFunc("/unregister/{job_id}", controllers.UnRegisterJob).Methods("POST")

	err := http.ListenAndServe(misc.GetPort(), router)
	if err != nil {
		panic(err)
	}
}
