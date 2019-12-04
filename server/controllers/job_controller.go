package controllers

import (
	"cron-server/server/misc"
	"cron-server/server/models"
	"cron-server/server/repository"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

type JobController struct {
	Pool repository.Pool
}

var basicJobController = BasicController{model: models.Job{}}

func (controller *JobController) CreateOne(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	ij := models.InboundJob{}

	if len(body) < 1 {
		misc.SendJson(w, "no request body", false, http.StatusBadRequest, nil)
		return
	}

	err = ij.FromJson(body)
	if err != nil {
		misc.SendJson(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	j, err := ij.ToModel()
	if err != nil {
		misc.SendJson(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	if len(j.ProjectId) < 1 {
		misc.SendJson(w, "project Id is required", false, http.StatusBadRequest, nil)
		return
	}

	if len(j.CronSpec) < 1 {
		misc.SendJson(w, "cron spec is required", false, http.StatusBadRequest, nil)
		return
	}

	if len(j.CallbackUrl) < 1 {
		misc.SendJson(w, "callback url is required", false, http.StatusBadRequest, nil)
		return
	}

	if j.StartDate.IsZero() {
		misc.SendJson(w, "start date is required", false, http.StatusBadRequest, nil)
		return
	}

	log.Println("Start Date, Time", j.StartDate.UTC(), time.Now().UTC())

	if j.StartDate.UTC().Before(time.Now().UTC()) {
		misc.SendJson(w, "start date cannot be in the past", false, http.StatusBadRequest, nil)
		return
	}

	id, err := j.CreateOne(&controller.Pool, r.Context())
	if err != nil {
		misc.SendJson(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	misc.SendJson(w, id, true, http.StatusCreated, nil)
}

func (controller *JobController) UpdateOne(w http.ResponseWriter, r *http.Request) {
	id, err := misc.GetRequestParam(r, "id", 2)
	if err != nil {
		misc.SendJson(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	job := models.Job{ID: id}
	_, err = job.GetOne(&controller.Pool, r.Context(), "id = ?", job.ID)
	if err != nil {
		misc.SendJson(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	misc.CheckErr(err)
	inboundJob := models.InboundJob{}
	if len(body) > 1 {
		err := inboundJob.FromJson(body)
		if err != nil {
			misc.SendJson(w, err.Error(), false, http.StatusBadRequest, nil)
			return
		}
	} else {
		misc.SendJson(w, "no request body", false, http.StatusBadRequest, nil)
		return
	}

	jobUpdate, err := inboundJob.ToModel()
	if err != nil {
		misc.SendJson(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	job.Timezone = jobUpdate.Timezone
	job.CallbackUrl = jobUpdate.CallbackUrl
	job.Description = jobUpdate.Description
	job.Data = jobUpdate.Data
	job.EndDate = jobUpdate.EndDate

	if _, err = job.UpdateOne(&controller.Pool, r.Context()); err != nil {
		misc.SendJson(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	misc.SendJson(w, job, true, http.StatusOK, nil)
}

func (controller *JobController) GetAll(w http.ResponseWriter, r *http.Request) {
	basicJobController.GetAll(w, r, controller.Pool)
}

func (controller *JobController) GetOne(w http.ResponseWriter, r *http.Request) {
	basicJobController.GetOne(w, r, controller.Pool)
}

func (controller *JobController) DeleteOne(w http.ResponseWriter, r *http.Request) {
	basicJobController.DeleteOne(w, r, controller.Pool)
}

func (controller *JobController) GetAllOrCreateOne(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		controller.GetAll(w, r)
	}

	if r.Method == http.MethodPost {
		controller.CreateOne(w, r)
	}
}
