package process

import (
	"cron-server/job"
	"cron-server/misc"
	"fmt"
	"github.com/go-pg/pg"
	"github.com/go-redis/redis"
	"github.com/robfig/cron"
	"log"
)

/*
	Execute jobs scheduled to be executed and
 	update the next execution time of the job
*/

var psgc = misc.GetPostgresCredentials()

func ExecuteJobs() {
	rdc := misc.GetRedisCredentials()
	client := redis.NewClient(&redis.Options{Addr: rdc.Addr})
	defer client.Close()

	db := pg.Connect(&pg.Options{
		Addr:     psgc.Addr,
		User:     psgc.User,
		Password: psgc.Password,
		Database: psgc.Database,
	})
	defer db.Close()

	var jobs []job.Job

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
		"jobs.state = %v OR jobs.state = %v", job.ActiveJob, job.StaleJob)

	r, err := db.Query(&jobs, query)

	if err != nil {
		panic(err)
	}

	log.Println("Jobs ::", r.RowsReturned())

	for _, jb := range jobs {
		go func(j job.Job) {
			db := pg.Connect(&pg.Options{
				Addr:     psgc.Addr,
				User:     psgc.User,
				Password: psgc.Password,
				Database: psgc.Database,
			})
			defer db.Close()

			log.Println("Publish message to job", j.ServiceName, j.ID)
			channel := "job:" + j.ServiceName + ":" + j.ID
			client.Publish(channel, j)

			schedule, err := cron.ParseStandard(j.CronSpec)
			if err != nil {
				panic(err)
			}

			j.NextTime = schedule.Next(j.NextTime)
			j.TotalExecs = j.TotalExecs + 1

			_, err = db.Model(&j).Set("next_time = ?next_time").Set("total_execs = ?total_execs").Set("state = ?state").Where("id = ?id").Update()

			log.Println("Updated job with id ", j.ID, " next time to ", j.NextTime)

			if err != nil {
				panic(err)
			}
		}(jb)
	}
}
