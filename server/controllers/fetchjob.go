package controllers

import (
	"cron-server/server/models"
	"cron-server/server/repo"
	"encoding/json"
	"github.com/gorilla/mux"
	"net/http"
)

func FetchJob(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	serviceName := params["service_name"]

	if len(serviceName) < 1 {
		w.WriteHeader(http.StatusBadRequest)
	}

	jds, err := repo.GetAll(serviceName)
	if err != nil {
		panic(err)
	}

	var jobs []models.Dto
	for _, jd := range jds {
		jobs = append(jobs, jd.ToDto())
	}

	data, err := json.Marshal(jobs)
	if err != nil {
		panic(err)
	}

	_, err = w.Write(data)
	if err != nil {
		panic(err)
	}
}
