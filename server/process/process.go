package process

import (
	"context"
	"cron-server/server/misc"
	"cron-server/server/models"
	"cron-server/server/repository"
	"fmt"
	"github.com/go-pg/pg"
	"github.com/robfig/cron"
	"log"
	"net/http"
	"strings"
	"time"
)

var psgc = misc.GetPostgresCredentials()

func Start(pool *repository.Pool) {
	for {
		ctx := context.Background()

		jobsToExecute, otherJobs := getJobs(pool, ctx)
		updateMissedJobs(otherJobs, pool, ctx)
		executeJobs(jobsToExecute, pool, ctx)

		time.Sleep(60 * time.Second)
	}
}

func getJobs(pool *repository.Pool, ctx context.Context) (executions []models.Job, others []models.Job) {
	// Infinite loops that queries the database every minute
	conn, err := pool.Acquire()
	misc.CheckErr(err)

	db := conn.(*pg.DB)
	defer pool.Release(conn)

	var jobsToExecute []models.Job
	var otherJobs []models.Job

	jobsToExecuteQuery := fmt.Sprintf("SELECT * FROM jobs WHERE " +
		// NextTime Difference in minute
		"date_part('day', now()::timestamp at time zone 'utc' - jobs.next_time::timestamp at time zone 'utc') * 24 +" +
		"date_part('hour', now()::timestamp at time zone 'utc' - jobs.next_time::timestamp at time zone 'utc') * 60 +" +
		"date_part('minute', now()::timestamp at time zone 'utc' - jobs.next_time::timestamp at time zone 'utc') = 0 AND " +
		// NextTime Difference in hour
		"date_part('day', now()::timestamp at time zone 'utc' - jobs.next_time::timestamp at time zone 'utc') * 24 +" +
		"date_part('hour', now()::timestamp at time zone 'utc' - jobs.next_time::timestamp at time zone 'utc') = 0 AND " +
		// NextTime Difference in day
		"date_part('day', now()::timestamp at time zone 'utc' - jobs.next_time::timestamp at time zone 'utc') = 0")

	otherJobsQuery := fmt.Sprintf("SELECT * FROM jobs WHERE " +
		// NextTime Difference in minute
		"date_part('day', now()::timestamp at time zone 'utc' - jobs.next_time::timestamp at time zone 'utc') * 24 +" +
		"date_part('hour', now()::timestamp at time zone 'utc' - jobs.next_time::timestamp at time zone 'utc') * 60 +" +
		"date_part('minute', now()::timestamp at time zone 'utc' - jobs.next_time::timestamp at time zone 'utc') <> 0 AND " +
		// NextTime Difference in hour
		"date_part('day', now()::timestamp at time zone 'utc' - jobs.next_time::timestamp at time zone 'utc') * 24 +" +
		"date_part('hour', now()::timestamp at time zone 'utc' - jobs.next_time::timestamp at time zone 'utc') <> 0 AND " +
		// NextTime Difference in day
		"date_part('day', now()::timestamp at time zone 'utc' - jobs.next_time::timestamp at time zone 'utc') <> 0")

	_, err = db.Query(&jobsToExecute, jobsToExecuteQuery)
	misc.CheckErr(err)

	_, err = db.Query(&otherJobs, otherJobsQuery)
	misc.CheckErr(err)

	return jobsToExecute, otherJobs
}

func executeJobs(jobs []models.Job, pool *repository.Pool, ctx context.Context) {
	for _, jb := range jobs {
		if len(jb.CallbackUrl) > 1 {
			go func(job models.Job) {
				r, err := http.Post(http.MethodPost, job.CallbackUrl, strings.NewReader(job.Data))
				misc.CheckErr(err)
				job.LastStatusCode = r.StatusCode

				schedule, err := cron.ParseStandard(job.CronSpec)
				misc.CheckErr(err)

				job.NextTime = schedule.Next(job.NextTime)
				job.TotalExecs = job.TotalExecs + 1
				job.State = models.ActiveJob

				err = jb.UpdateOne(pool, ctx)
				misc.CheckErr(err)

				log.Println("Updated job with id ", job.ID, " next time to ", job.NextTime)
			}(jb)
		}
	}
}

func updateMissedJobs(jobs []models.Job, pool *repository.Pool, ctx context.Context) {
	for i := 0; i < len(jobs); i++ {
		go func(jb models.Job) {
			schedule, err := cron.ParseStandard(jb.CronSpec)
			if err != nil {
				panic(err)
			}

			if jobs[i].TotalExecs != -1 {
				scheduledTimeBasedOnNow := schedule.Next(jb.StartDate)

				var execCountToNow int64 = -1

				for scheduledTimeBasedOnNow.Before(time.Now().UTC()) {
					execCountToNow++
					scheduledTimeBasedOnNow = schedule.Next(scheduledTimeBasedOnNow)
				}

				execCountDiff := execCountToNow - jb.TotalExecs

				if execCountDiff > 0 {
					log.Println(jobs[i].ID, execCountDiff, jb.NextTime, scheduledTimeBasedOnNow)

					jb.NextTime = scheduledTimeBasedOnNow
					jb.TotalExecs = execCountToNow
					jb.MissedExecs = jb.MissedExecs + execCountDiff
				}

				err = jb.UpdateOne(pool, ctx)
				misc.CheckErr(err)
			}
		}(jobs[i])
	}
}
