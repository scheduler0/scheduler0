package process

import (
	"cron-server/server/misc"
	"cron-server/server/models"
	"fmt"
	"github.com/go-pg/pg"
	"github.com/robfig/cron"
	"log"
	"net/http"
	"strings"
)

/*
	Execute jobs scheduled to be executed and
 	update the next execution time of the job
*/

var psgc = misc.GetPostgresCredentials()

func ExecuteJobs() {
	db := pg.Connect(&pg.Options{
		Addr:     psgc.Addr,
		User:     psgc.User,
		Password: psgc.Password,
		Database: psgc.Database,
	})
	defer db.Close()

	var jobs []models.Job

	query := fmt.Sprintf("SELECT * FROM jobs WHERE "+
		// Difference in minute
		"date_part('day', now()::timestamp at time zone 'utc' - jobs.next_time::timestamp at time zone 'utc') * 24 +"+
		"date_part('hour', now()::timestamp at time zone 'utc' - jobs.next_time::timestamp at time zone 'utc') * 60 +"+
		"date_part('minute', now()::timestamp at time zone 'utc' - jobs.next_time::timestamp at time zone 'utc') = 0 AND "+
		// Difference in hour
		"date_part('day', now()::timestamp at time zone 'utc' - jobs.next_time::timestamp at time zone 'utc') * 24 +"+
		"date_part('hour', now()::timestamp at time zone 'utc' - jobs.next_time::timestamp at time zone 'utc') = 0 AND "+
		// Difference in day
		"date_part('day', now()::timestamp at time zone 'utc' - jobs.next_time::timestamp at time zone 'utc') = 0 AND "+
		// Should be active
		"jobs.state = %v OR jobs.state = %v", models.ActiveJob, models.StaleJob)

	r, err := db.Query(&jobs, query)
	misc.CheckErr(err)

	log.Println("Jobs ::", r.RowsReturned())

	for _, jb := range jobs {
		go func(j models.Job) {
			db := pg.Connect(&pg.Options{
				Addr:     psgc.Addr,
				User:     psgc.User,
				Password: psgc.Password,
				Database: psgc.Database,
			})
			defer db.Close()

			if len(j.CallbackUrl) > 1 {
				r, err := http.Post(http.MethodPost, j.CallbackUrl, strings.NewReader(j.Data))
				misc.CheckErr(err)
				j.LastStatusCode = r.StatusCode
			}

			schedule, err := cron.ParseStandard(j.CronSpec)
			misc.CheckErr(err)

			j.NextTime = schedule.Next(j.NextTime)
			j.TotalExecs = j.TotalExecs + 1

			_, err = db.Model(&j).Set("next_time = ?next_time").Set("total_execs = ?total_execs").Set("state = ?state").Where("id = ?id").Update()
			misc.CheckErr(err)
			log.Println("Updated job with id ", j.ID, " next time to ", j.NextTime)
		}(jb)
	}
}
