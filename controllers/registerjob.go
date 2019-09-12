package controllers

import (
	"cron/dto"
	"cron/misc"
	"fmt"
	"github.com/go-pg/pg"
	"github.com/robfig/cron"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

func RegisterJob(w http.ResponseWriter, r *http.Request) {
	psgc := misc.GetPostgresCredentials()

	body, err := ioutil.ReadAll(r.Body)
	job := dto.JobFromJson(body)

	if err != nil {
		panic(err)
	}

	if len(job.ServiceName) < 1 {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Service name is required"))
		return
	}

	if len(job.Cron) < 1 {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Cron is required"))
		return
	}

	if job.StartDate.IsZero() {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Cron is required"))
		return
	}

	db := pg.Connect(&pg.Options{
		Addr:     psgc.Addr,
		User:     psgc.User,
		Password: psgc.Password,
		Database: psgc.Database,
	})
	defer db.Close()

	job.State = dto.INACTIVE_JOB
	schedule, err := cron.ParseStandard(job.Cron)

	fmt.Println("Current time", time.Now())

	job.NextTime = schedule.Next(job.StartDate)

	fmt.Println("Next time", job.NextTime)

	if err != nil {
		panic(err)
	}

	err = db.Insert(&job)
	if err != nil {
		panic(err)
	}

	log.Println("Registered ", job)

	r.Header.Add("Content-Type", "application/json")
	r.Header.Add("Location", "jobs"+string(job.ID))
	w.WriteHeader(http.StatusCreated)
}
