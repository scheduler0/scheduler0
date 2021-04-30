package process

import (
	"errors"
	"github.com/go-pg/pg"
	"github.com/robfig/cron"
	"net/http"
	"scheduler0/server/db"
	executionManager "scheduler0/server/managers/execution"
	"scheduler0/server/managers/job"
	"scheduler0/server/managers/project"
	"scheduler0/server/service"
	"scheduler0/server/transformers"
	"scheduler0/utils"
	"strconv"
	"strings"
	"time"
)

// JobProcessor handles executions of jobs
type JobProcessor struct {
	Cron          *cron.Cron
	RecoveredJobs []transformers.Job
	MaxMemory     int64
	MaxCPU        int64
	Pool          *utils.Pool
}

// RecoverJobExecutions find jobs that could've not been executed due to timeout
func (jobProcessor *JobProcessor) RecoverJobExecutions(jobTransformers []transformers.Job) {
	/**
	TODO: Find execution for job that does not have a TimeExecuted but TimeAdded.
			if one exists find the difference between when the job was added and when it ought to have executed
			if we've not exceed the execution window.
			Add the job to queue of recovered jobs to be executed.
	*/
	conn, err := jobProcessor.Pool.Acquire()
	utils.CheckErr(err)
	jobIDs := make([]string, len(jobTransformers))

	params := ""

	for _, jobTransformer := range jobTransformers {
		jobIDs = append(jobIDs, jobTransformer.UUID)
		params = params + "?,"
	}

	dbConn := conn.(*pg.DB)
	query := "SELECT * FROM executions " +
			"WHERE executions.job_uuid IN ("+params+") AND executions.time_executed is NULL"

	result, err := dbConn.Exec(query, jobIDs)
	utils.CheckErr(err)

	if result.RowsReturned() > 0 {

	}
}

// ExecuteHTTPJob this will execute an http job
func (jobProcessor *JobProcessor) ExecuteHTTPJob(jobTransformer transformers.Job) func() {
	return func() {
		go func() {
			pool, err := utils.NewPool(db.OpenConnection, 1)

			// TODO: Check that job still exists before executing
			var statusCode int

			startSecs := time.Now()

			r, err := http.Post(jobTransformer.CallbackUrl, "application/json", strings.NewReader(jobTransformer.Data))
			if err != nil {
				statusCode = -1
			} else {
				statusCode = r.StatusCode
			}

			timeout := uint64(time.Now().Sub(startSecs).Milliseconds())
			// TODO: Replace this with finding the existing execution for the job and modifying it
			// TODO: If the job is a recovered job add it to the pool of main executions
			execution := executionManager.Manager{
				JobUUID:       jobTransformer.UUID,
				ExecutionTime: timeout,
				StatusCode:    strconv.Itoa(statusCode),
				DateCreated:   time.Now().UTC(),
			}

			_, createOneErr := execution.CreateOne(pool)
			if createOneErr != nil {
				utils.CheckErr(errors.New(createOneErr.Message))
			}
		}()
	}
}

// StartJobs the cron job process
func (jobProcessor *JobProcessor) StartJobs() {
	projectManager := project.ProjectManager{}

	totalProjectCount, err := projectManager.Count(jobProcessor.Pool)
	if err != nil {
		panic(err)
	}

	projectService := service.ProjectService{
		Pool: jobProcessor.Pool,
	}

	projectTransformers, err := projectService.List(0, totalProjectCount)
	if err != nil {
		panic(err)
	}

	jobService := service.JobService{
		Pool: jobProcessor.Pool,
	}

	for _, projectTransformer := range projectTransformers.Data {
		jobManager := job.Manager{}

		jobsTotalCount, err := jobManager.GetJobsTotalCountByProjectUUID(jobProcessor.Pool, projectTransformer.UUID)
		if err != nil {
			panic(err)
		}

		jobTransformers, err := jobService.GetJobsByProjectUUID(projectTransformer.UUID, 0, jobsTotalCount, "date_created")

		for _, jobTransformer := range jobTransformers.Data {
			// TODO: re-Use add job function below
			// TODO: Use wait group
			err := jobProcessor.Cron.AddFunc(jobTransformer.Spec, jobProcessor.ExecuteHTTPJob(jobTransformer))
			if err != nil {
				panic(err)
			}
		}
	}

	jobProcessor.Cron.Start()
}

// AddJob adds a single job to the queue
func (jobProcessor *JobProcessor) AddJob(jobTransformer transformers.Job) {
	// TODO: Check if we're within resource usage before adding another job
	// TODO: Update executions table with newly added job
	err := jobProcessor.Cron.AddFunc(jobTransformer.Spec, jobProcessor.ExecuteHTTPJob(jobTransformer))
	utils.CheckErr(err)
}
