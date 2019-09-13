package controllers

import (
	"cron-server/job"
	"cron-server/repo"
	"encoding/json"
	"github.com/gorilla/mux"
	"net/http"
)

func FetchJob(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	serviceName := params["service_name"]

	jds, err := repo.GetAll(serviceName)
	if err != nil {
		panic(err)
	}

	var jobs []job.Dto
	for _, jd := range jds {
		jobs = append(jobs, jd.ToDto())
	}

	data, err := json.Marshal(jobs)
	if err != nil {
		panic(err)
	}

	w.Write(data)
}
