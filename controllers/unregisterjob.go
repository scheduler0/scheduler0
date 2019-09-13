package controllers

import (
	"cron-server/repo"
	"github.com/gorilla/mux"
	"log"
	"net/http"
)

func UnRegisterJob(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	jobId := vars["job_id"]

	_, err := repo.DeleteOne(jobId)

	if err != nil {
		panic(err)
	}

	log.Println("Unregistered job with id :", jobId)
	w.WriteHeader(http.StatusOK)
}
