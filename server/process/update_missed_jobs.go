package process

import (
	"cron-server/server/models"
	"fmt"
	"github.com/go-pg/pg"
	"github.com/robfig/cron"
	"log"
	"time"
)

func UpdateMissedJobs() {
	db := pg.Connect(&pg.Options{
		Addr:     psgc.Addr,
		User:     psgc.User,
		Password: psgc.Password,
		Database: psgc.Database,
	})
	defer db.Close()

	var jobs []models.Job
	query := fmt.Sprintf("SELECT * FROM jobs WHERE jobs.state = %v", models.ActiveJob)

	_, err := db.Query(&jobs, query)
	if err != nil {
		panic(err)
	}

	for _, jd := range jobs {
		go func(j models.Job) {
			schedule, err := cron.ParseStandard(j.CronSpec)
			if err != nil {
				panic(err)
			}

			if j.TotalExecs != -1 {
				scheduledTimeBasedOnNow := schedule.Next(j.StartDate)

				var execCountToNow int64 = -1

				for scheduledTimeBasedOnNow.Before(time.Now().UTC()) {
					execCountToNow++
					scheduledTimeBasedOnNow = schedule.Next(scheduledTimeBasedOnNow)
				}

				execCountDiff := execCountToNow - j.TotalExecs

				if execCountDiff > 0 {
					db := pg.Connect(&pg.Options{
						Addr:     psgc.Addr,
						User:     psgc.User,
						Password: psgc.Password,
						Database: psgc.Database,
					})
					defer db.Close()

					log.Println(j.ID, execCountDiff, j.NextTime, scheduledTimeBasedOnNow)

					j.NextTime = scheduledTimeBasedOnNow
					j.TotalExecs = execCountToNow
					j.MissedExecs = j.MissedExecs + execCountDiff

					_, err := db.Model(&j).
						Set("next_time = ?next_time").
						Set("total_execs = ?total_execs").
						Set("missed_execs =  ?missed_execs").
						Where("id = ?id").Update()

					if err != nil {
						panic(err)
					}

					log.Println("Fixed job ", j.ID, " to ", j.NextTime)
				}
			}
		}(jd)
	}
}
