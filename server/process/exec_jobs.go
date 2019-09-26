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

	_, err := db.Query(&jobs, query)
	misc.CheckErr(err)

	for _, jb := range jobs {
		if len(jb.CallbackUrl) > 1 {
			go func(job models.Job) {
				db := pg.Connect(&pg.Options{
					Addr:     psgc.Addr,
					User:     psgc.User,
					Password: psgc.Password,
					Database: psgc.Database,
				})
				defer db.Close()

				r, err := http.Post(http.MethodPost, job.CallbackUrl, strings.NewReader(job.Data))
				misc.CheckErr(err)
				job.LastStatusCode = r.StatusCode

				schedule, err := cron.ParseStandard(job.CronSpec)
				misc.CheckErr(err)

				job.NextTime = schedule.Next(job.NextTime)
				job.TotalExecs = job.TotalExecs + 1

				_, err = db.Model(&job).Set("next_time = ?next_time").
					Set("total_execs = ?total_execs").
					Set("state = ?state").
					Set("last_status_code = ?last_status_code").
					Where("id = ?id").
					Update()

				misc.CheckErr(err)
				log.Println("Updated job with id ", job.ID, " next time to ", job.NextTime)
			}(jb)
		}
	}
}
