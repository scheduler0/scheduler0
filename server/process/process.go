package process

import (
	"context"
	"cron-server/server/db"
	"cron-server/server/managers"
	"cron-server/server/misc"
	"fmt"
	"github.com/go-pg/pg"
	"github.com/robfig/cron"
	"github.com/segmentio/ksuid"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"
)

// Start the cron job process
func Start(pool *db.Pool) {
	for {
		ctx := context.Background()
		jobsToExecute, otherJobs := getJobs(ctx, pool)
		updateMissedJobs(ctx, otherJobs, pool)
		executeJobs(ctx, jobsToExecute, pool)

		time.Sleep(1 * time.Second)
	}
}

func getJobs(ctx context.Context, pool *db.Pool) (executions []managers.JobManager, others []managers.JobManager) {
	// Infinite loops that queries the database every minute
	conn, err := pool.Acquire()
	misc.CheckErr(err)

	db := conn.(*pg.DB)
	defer pool.Release(conn)

	var jobsToExecute []managers.JobManager
	var otherJobs []managers.JobManager

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

	otherJobsQuery := fmt.Sprintf("SELECT * FROM jobs WHERE "+
		// NextTime Difference in minute
		"date_part('day', now()::timestamp at time zone 'utc' - jobs.next_time::timestamp at time zone 'utc') * 24 +"+
		"date_part('hour', now()::timestamp at time zone 'utc' - jobs.next_time::timestamp at time zone 'utc') * 60 +"+
		"date_part('minute', now()::timestamp at time zone 'utc' - jobs.next_time::timestamp at time zone 'utc') <> 0 OR "+
		// NextTime Difference in hour
		"date_part('day', now()::timestamp at time zone 'utc' - jobs.next_time::timestamp at time zone 'utc') * 24 +"+
		"date_part('hour', now()::timestamp at time zone 'utc' - jobs.next_time::timestamp at time zone 'utc') <> 0 OR "+
		// NextTime Difference in day
		"date_part('day', now()::timestamp at time zone 'utc' - jobs.next_time::timestamp at time zone 'utc') <> 0 AND jobs.state != %v AND jobs.total_execs != %v", managers.StaleJob, -1)

	_, err = db.Query(&jobsToExecute, jobsToExecuteQuery)
	misc.CheckErr(err)

	_, err = db.Query(&otherJobs, otherJobsQuery)
	misc.CheckErr(err)

	return jobsToExecute, otherJobs
}

func executeJobs(ctx context.Context, jobs []managers.JobManager, pool *db.Pool) {
	for _, jb := range jobs {
		if len(jb.CallbackUrl) > 1 {
			go func(job managers.JobManager) {
				var response string
				var statusCode int

				startSecs := time.Now()

				// TODO: Encrypt the data
				r, err := http.Post(http.MethodPost, job.CallbackUrl, strings.NewReader(job.Data))
				if err != nil {
					response = err.Error()
					statusCode = 0
				} else {
					body, err := ioutil.ReadAll(r.Body)
					if err != nil {
						response = err.Error()
					}
					response = string(body)
					statusCode = r.StatusCode
				}

				timeout := uint64(time.Now().Sub(startSecs).Milliseconds())
				execution := managers.ExecutionManager{
					ID:          ksuid.New().String(),
					JobId:       job.ID,
					Timeout:     timeout,
					Response:    response,
					StatusCode:  string(statusCode),
					DateCreated: time.Now().UTC(),
				}

				_, err = execution.CreateOne(pool, ctx)
				misc.CheckErr(err)

				schedule, err := cron.ParseStandard(job.CronSpec)
				misc.CheckErr(err)

				job.NextTime = schedule.Next(job.NextTime).UTC()
				job.TotalExecs = job.TotalExecs + 1
				job.State = managers.ActiveJob

				_, err = job.UpdateOne(pool, ctx)
				misc.CheckErr(err)

				log.Println("Executed job ", job.ID, "  next execution time is ", job.NextTime)
			}(jb)
		}
	}
}

func updateMissedJobs(ctx context.Context, jobs []managers.JobManager, pool *db.Pool) {
	for i := 0; i < len(jobs); i++ {
		go func(jb managers.JobManager) {
			schedule, err := cron.ParseStandard(jb.CronSpec)
			if err != nil {
				panic(err)
			}

			if jb.TotalExecs != -1 {
				scheduledTimeBasedOnNow := schedule.Next(jb.StartDate)

				var execCountToNow int64 = -1

				for scheduledTimeBasedOnNow.Before(time.Now().UTC()) {
					execCountToNow++
					scheduledTimeBasedOnNow = schedule.Next(scheduledTimeBasedOnNow)
				}

				execCountDiff := execCountToNow - jb.TotalExecs

				if execCountDiff > 0 {
					log.Println("Update job next exec time ", jb.ID, execCountDiff, jb.NextTime, scheduledTimeBasedOnNow)

					jb.NextTime = scheduledTimeBasedOnNow
					jb.TotalExecs = execCountToNow
					jb.State = managers.StaleJob
				}
			} else {
				scheduledNextTime := schedule.Next(time.Now())
				jb.NextTime = scheduledNextTime
				jb.State = managers.StaleJob
			}

			_, err = jb.UpdateOne(pool, ctx)
			misc.CheckErr(err)
		}(jobs[i])
	}
}
