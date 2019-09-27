package controllers

import (
	"cron-server/server/misc"
	"cron-server/server/models"
	"github.com/robfig/cron"
	"io/ioutil"
	"net/http"
	"time"
)

type JobController struct{}

var basicJobController = BasicController{model: models.Job{}}

func (_ *JobController) CreateOne(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	j := models.Job{}
	j.FromJson(body)

	if len(j.ProjectId) < 1 {
		w.WriteHeader(http.StatusBadRequest)
		_, err = w.Write([]byte("Project Id is required"))
		misc.CheckErr(err)
		return
	}

	if len(j.CronSpec) < 1 {
		w.WriteHeader(http.StatusBadRequest)
		_, err = w.Write([]byte("Cron is required"))
		misc.CheckErr(err)
		return
	}

	if j.StartDate.IsZero() {
		w.WriteHeader(http.StatusBadRequest)
		_, err = w.Write([]byte("Cron is required"))
		misc.CheckErr(err)
		return
	}

	schedule, err := cron.ParseStandard(j.CronSpec)
	misc.CheckErr(err)
	j.State = models.InActiveJob
	j.NextTime = schedule.Next(j.StartDate)
	j.TotalExecs = -1
	j.SecsBetweenExecs = j.NextTime.Sub(j.StartDate).Seconds()
	id, err := j.CreateOne()
	misc.CheckErr(err)
	misc.SendJson(w, id, http.StatusCreated, nil)
}

func (_ *JobController) UpdateOne(w http.ResponseWriter, r *http.Request) {
	if id, err := misc.GetRequestParam(r, "id", 2); err != nil {
		misc.SendJson(w, err, http.StatusBadRequest, nil)
	} else {
		job := models.Job{ID: id}

		err := job.GetOne("id = ?", job.ID)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(job.ToJson()))
			return
		}

		body, err := ioutil.ReadAll(r.Body)
		misc.CheckErr(err)
		jobUpdate := models.Job{}
		jobUpdate.FromJson(body)

		if jobUpdate.State == models.ActiveJob {
			schedule, err := cron.ParseStandard(job.CronSpec)
			misc.CheckErr(err)

			jobUpdate.NextTime = schedule.Next(time.Now().UTC())
		}

		jobUpdate.ID = id
		if err = jobUpdate.UpdateOne(); err != nil {
			misc.SendJson(w, err, http.StatusBadRequest, nil)
		}

		misc.SendJson(w, jobUpdate, http.StatusOK, nil)
	}
}

func (_ *JobController) GetAll(w http.ResponseWriter, r *http.Request) {
	basicJobController.GetAll(w, r)
}

func (_ *JobController) GetOne(w http.ResponseWriter, r *http.Request) {
	basicJobController.GetOne(w, r)
}

func (_ *JobController) DeleteOne(w http.ResponseWriter, r *http.Request) {
	basicJobController.DeleteOne(w, r)
}

func (c *JobController) GetAllOrCreateOne(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		c.GetAll(w, r)
	}

	if r.Method == http.MethodPost {
		c.CreateOne(w, r)
	}
}