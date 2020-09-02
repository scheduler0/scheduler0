package process

import (
	"cron-server/server/src/managers"
	"cron-server/server/src/utils"
	"cron-server/server/src/models"
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
func Start(pool *utils.Pool) {
	for {
		jobsToExecute, otherJobs := getJobs(pool)
		updateMissedJobs(otherJobs, pool)
		executeJobs(jobsToExecute, pool)

		time.Sleep(1 * time.Second)
	}
}

func getJobs(pool *utils.Pool) (executions []managers.JobManager, others []managers.JobManager) {
	// Infinite loops that queries the database every minute
	conn, err := pool.Acquire()
	utils.CheckErr(err)

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
		"date_part('day', now()::timestamp at time zone 'utc' - jobs.next_time::timestamp at time zone 'utc') <> 0 AND jobs.state != %v AND jobs.total_execs != %v", models.StaleJob, -1)

	_, err = db.Query(&jobsToExecute, jobsToExecuteQuery)
	utils.CheckErr(err)

	_, err = db.Query(&otherJobs, otherJobsQuery)
	utils.CheckErr(err)

	return jobsToExecute, otherJobs
}

func executeJobs(jobs []managers.JobManager, pool *utils.Pool) {
	for _, jb := range jobs {
		if len(jb.CallbackUrl) > 1 {
			go func(job managers.JobManager) {
				var response string
				var statusCode int

				startSecs := time.Now()

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
					JobID:       job.ID,
					Timeout:     timeout,
					Response:    response,
					StatusCode:  string(statusCode),
					DateCreated: time.Now().UTC(),
				}

				_, err = execution.CreateOne(pool)
				utils.CheckErr(err)

				schedule, err := cron.ParseStandard(job.CronSpec)
				utils.CheckErr(err)

				job.NextTime = schedule.Next(job.NextTime).UTC()
				job.TotalExecs = job.TotalExecs + 1
				job.State = models.ActiveJob

				_, err = job.UpdateOne(pool)
				utils.CheckErr(err)

				log.Println("Executed job ", job.ID, "  next execution time is ", job.NextTime)
			}(jb)
		}
	}
}

func updateMissedJobs(jobs []managers.JobManager, pool *utils.Pool) {
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
					jb.State = models.StaleJob
				}
			} else {
				scheduledNextTime := schedule.Next(time.Now())
				jb.NextTime = scheduledNextTime
				jb.State = models.StaleJob
			}

			_, err = jb.UpdateOne(pool)
			utils.CheckErr(err)
		}(jobs[i])
	}
}
