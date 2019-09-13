package controllers

import (
	"cron-server/job"
	"cron-server/repo"
	"github.com/gorilla/mux"
	"github.com/robfig/cron"
	"net/http"
	"strings"
	"time"
)

func ActivateJob(writer http.ResponseWriter, request *http.Request) {
	UpdateJobState(writer, request, job.ActiveJob)
}

func DeactivateJob(writer http.ResponseWriter, request *http.Request) {
	UpdateJobState(writer, request, job.InActiveJob)
}

func UpdateJobState(w http.ResponseWriter, r *http.Request, s job.State) {
	params := mux.Vars(r)
	jobId := params["job_id"]

	if len(jobId) < 1 {
		path := strings.Split(r.URL.Path, "/")
		jobId = path[2]
	}

	j, err := repo.GetOne(jobId)
	if err != nil {
		panic(err)
	}

	j.State = s

	if s == job.ActiveJob {
		schedule, err := cron.ParseStandard(j.CronSpec)
		if err != nil {
			panic(err)
		}

		j.NextTime = schedule.Next(time.Now().UTC())
	}

	jd, err := repo.UpdateOne(j)
	if err != nil {
		panic(err)
	}

	job := jd.ToDto()

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(job.ToJson()))
}
