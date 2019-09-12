package controllers

import (
	"cron/dto"
	"cron/misc"
	"encoding/json"
	"github.com/go-pg/pg"
	"github.com/gorilla/mux"
	"net/http"
)

func FetchJob(w http.ResponseWriter, r *http.Request) {
	psgc := misc.GetPostgresCredentials()
	params := mux.Vars(r)

	db := pg.Connect(&pg.Options{
		Addr:     psgc.Addr,
		User:     psgc.User,
		Password: psgc.Password,
		Database: psgc.Database,
	})
	defer db.Close()

	var jobs []dto.Job
	err := db.Model(&jobs).Where("Job.Id == ", params["service_name"]).Select()
	if err != nil {
		panic(err)
	}

	data, err := json.Marshal(jobs)
	if err != nil {
		panic(err)
	}

	w.Write(data)
}
