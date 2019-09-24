package controllers

import (
	"cron-server/server/models"
	"cron-server/server/repo"
	"github.com/robfig/cron"
	"io/ioutil"
	"log"
	"net/http"
)

func RegisterJob(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	j := models.FromJson(body)

	if err != nil {
		panic(err)
	}

	if len(j.ServiceName) < 1 {
		w.WriteHeader(http.StatusBadRequest)
		_, err = w.Write([]byte("Service name is required"))
		if err != nil {
			panic(err)
		}
		return
	}

	if len(j.CronSpec) < 1 {
		w.WriteHeader(http.StatusBadRequest)
		_, err = w.Write([]byte("Cron is required"))
		if err != nil {
			panic(err)
		}
		return
	}

	if j.StartDate.IsZero() {
		w.WriteHeader(http.StatusBadRequest)
		_, err = w.Write([]byte("Cron is required"))
		if err != nil {
			panic(err)
		}
		return
	}

	jd := j.ToDomain()

	schedule, err := cron.ParseStandard(jd.CronSpec)
	if err != nil {
		panic(err)
	}

	jd.State = models.InActiveJob
	jd.NextTime = schedule.Next(jd.StartDate)
	jd.TotalExecs = -1
	jd.SecsBetweenExecs = jd.NextTime.Sub(jd.StartDate).Minutes()

	newJD, err := repo.CreateOne(jd)
	if err != nil {
		panic(err)
	}

	log.Println("Registered ", jd)

	w.Header().Add("Content-Type", "application/json")
	w.Header().Add("Location", "jobs/"+newJD.ID)
	w.WriteHeader(http.StatusCreated)
}
