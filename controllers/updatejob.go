package controllers

import (
	"cron/dto"
	"cron/misc"
	"github.com/go-pg/pg"
	"github.com/gorilla/mux"
	"net/http"
	"strconv"
)

func ActivateJob(writer http.ResponseWriter, request *http.Request) {
	UpdateJobState(writer, request, dto.ACTIVE_JOB)
}

func DeactivateJob(writer http.ResponseWriter, request *http.Request) {
	UpdateJobState(writer, request, dto.INACTIVE_JOB)
}

func UpdateJobState(w http.ResponseWriter, r *http.Request, s dto.JobState) {
	psgc := misc.GetPostgresCredentials()
	params := mux.Vars(r)

	db := pg.Connect(&pg.Options{
		Addr:     psgc.Addr,
		User:     psgc.User,
		Password: psgc.Password,
		Database: psgc.Database,
	})
	defer db.Close()

	jobId, err := strconv.ParseInt(params["job_id"], 10, 64)
	if err != nil {
		panic(err)
	}

	job := dto.Job{}
	err = db.Model(&job).Where("Job.Id == ", jobId).Select()
	if err != nil {
		panic(err)
	}

	job.State = s
	err = db.Update(&job)

	if err != nil {
		panic(err)
	}

	w.WriteHeader(http.StatusOK)
}
