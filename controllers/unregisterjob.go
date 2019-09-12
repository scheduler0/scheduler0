package controllers

import (
	"cron/dto"
	"cron/misc"
	"github.com/go-pg/pg"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"strconv"
)

func UnRegisterJob(w http.ResponseWriter, r *http.Request) {
	psgc := misc.GetPostgresCredentials()

	db := pg.Connect(&pg.Options{
		Addr:     psgc.Addr,
		User:     psgc.User,
		Password: psgc.Password,
		Database: psgc.Database,
	})
	defer db.Close()

	vars := mux.Vars(r)
	jobId, err := strconv.ParseInt(vars["job_id"], 10, 64)

	if err == nil {
		panic(err)
	}

	job := dto.Job{ID: jobId}

	err = db.Delete(&job)
	if err != nil {
		panic(err)
	}

	log.Println("Unregistered ", job)
	w.WriteHeader(http.StatusOK)
}
