package main

import (
	"cron/controllers"
	"cron/dto"
	"cron/misc"
	"github.com/go-pg/pg/v9"
	"github.com/go-pg/pg/v9/orm"
	"github.com/go-redis/redis"
	"github.com/gorilla/mux"
	"github.com/robfig/cron"
	"log"
	"net/http"
	"time"
)

func main() {
	psgc := misc.GetPostgresCredentials()

	db := pg.Connect(&pg.Options{
		Addr:     psgc.Addr,
		User:     psgc.User,
		Password: psgc.Password,
		Database: psgc.Database,
	})

	defer db.Close()

	err := createSchema(db)

	if err != nil {
		panic(err)
	}

	go func() {
		for {
			go startCronManager()
			time.Sleep(time.Second * 1)
		}
	}()

	router := mux.NewRouter()

	router.HandleFunc("/job/{service_name}", controllers.FetchJob).Methods("GET")
	router.HandleFunc("/register", controllers.RegisterJob).Methods("POST")
	router.HandleFunc("/activate/{job_id}", controllers.ActivateJob).Methods("PUT")
	router.HandleFunc("/deactivate/{job_id}", controllers.DeactivateJob).Methods("PUT")
	router.HandleFunc("/unregister/{job_id}", controllers.UnRegisterJob).Methods("POST")

	http.ListenAndServe(misc.GetPort(), router)
}

func startCronManager() {
	psgc := misc.GetPostgresCredentials()
	rdc := misc.GetRedisCredentials()

	client := redis.NewClient(&redis.Options{Addr: rdc.Addr})
	log.Println("Connected to redis")

	log.Println("Connected to postgres")

	db := pg.Connect(&pg.Options{
		Addr:     psgc.Addr,
		User:     psgc.User,
		Password: psgc.Password,
		Database: psgc.Database,
	})
	defer db.Close()

	var jobs []dto.Job

	r, err := db.Query(&jobs,
		""+
			"SELECT * FROM jobs WHERE "+
			// Difference in minute
			"date_part('day', now()::timestamp at time zone 'utc' - jobs.next_time::timestamp at time zone 'utc') * 24 +"+
			"date_part('hour', now()::timestamp at time zone 'utc' - jobs.next_time::timestamp at time zone 'utc') * 60 +"+
			"date_part('minute', now()::timestamp at time zone 'utc' - jobs.next_time::timestamp at time zone 'utc') = 0 AND "+
			// Difference in hour
			"date_part('day', now()::timestamp at time zone 'utc' - jobs.next_time::timestamp at time zone 'utc') * 24 +"+
			"date_part('hour', now()::timestamp at time zone 'utc' - jobs.next_time::timestamp at time zone 'utc') = 0 AND "+
			// Difference in day
			"date_part('day', now()::timestamp at time zone 'utc' - jobs.next_time::timestamp at time zone 'utc') = 0"+
			"")

	if err != nil {
		panic(err)
	}

	log.Println("Jobs ::", r.RowsReturned())

	for _, job := range jobs {
		go func(j dto.Job) {
			log.Println("Publish message to job", j.ServiceName, j.ID)
			channel := "job:" + j.ServiceName + ":" + string(j.ID)
			client.Publish(channel, j)

			db := pg.Connect(&pg.Options{
				Addr:     psgc.Addr,
				User:     psgc.User,
				Password: psgc.Password,
				Database: psgc.Database,
			})

			schedule, err := cron.ParseStandard(j.Cron)
			if err != nil {
				panic(err)
			}

			j.NextTime = schedule.Next(j.NextTime)

			res, err := db.Model(&j).Set("next_time = ?next_time").Where("id = ?id").Update()
			log.Println("Updated row ", res.RowsAffected(), " next_time:: ", j.NextTime, j.ID)

			if err != nil {
				panic(err)
			}

			db.Close()
		}(job)
	}
}

func createSchema(db *pg.DB) error {
	db.Exec("SET TIME ZONE 'UTC';")

	for _, model := range []interface{}{(*dto.Job)(nil)} {
		err := db.CreateTable(model, &orm.CreateTableOptions{IfNotExists: true})
		if err != nil {
			return err
		}
	}

	return nil
}
