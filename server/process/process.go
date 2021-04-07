package process

import (
	"errors"
	"github.com/robfig/cron"
	"io/ioutil"
	"net/http"
	"scheduler0/server/db"
	executionManager "scheduler0/server/managers/execution"
	"scheduler0/server/managers/job"
	"scheduler0/server/managers/project"
	"scheduler0/server/service"
	"scheduler0/server/transformers"
	"scheduler0/utils"
	"strings"
	"time"
)

// Cron used for cron specific func
var Cron = cron.New()

// ExecuteHTTPJob this will execute an http job
func ExecuteHTTPJob(jobTransformer transformers.Job) func() {
	return func() {
		go func() {
			pool, err := utils.NewPool(db.OpenConnection, 1)

			var response string
			var statusCode int

			startSecs := time.Now()

			r, err := http.Post(http.MethodPost, jobTransformer.CallbackUrl, strings.NewReader(jobTransformer.Data))
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
			execution := executionManager.Manager{
				JobUUID:     jobTransformer.UUID,
				Timeout:     timeout,
				Response:    response,
				StatusCode:  string(rune(statusCode)),
				DateCreated: time.Now().UTC(),
			}

			_, createOneErr := execution.CreateOne(pool)
			utils.CheckErr(errors.New(createOneErr.Message))
		}()
	}
}

// StartAllHTTPJobs the cron job process
func StartAllHTTPJobs(pool *utils.Pool) {
	projectManager := project.ProjectManager{}

	totalProjectCount, err := projectManager.Count(pool)
	if err != nil {
		panic(err)
	}

	projectService := service.ProjectService{
		Pool: pool,
	}

	projectTransformers, err := projectService.List(0, totalProjectCount)
	if err != nil {
		panic(err)
	}

	jobService := service.JobService{
		Pool: pool,
	}

	for _, projectTransformer := range projectTransformers.Data {
		jobManager := job.Manager{}

		jobsTotalCount, err := jobManager.GetJobsTotalCountByProjectUUID(pool, projectTransformer.UUID)
		if err != nil {
			panic(err)
		}

		jobTransformers, err := jobService.GetJobsByProjectUUID(projectTransformer.UUID, 0, jobsTotalCount, "date_created")

		for _, jobTransformer := range jobTransformers.Data {

			// TODO: Make sure jobs are not archived

			err := Cron.AddFunc(jobTransformer.Spec, ExecuteHTTPJob(jobTransformer))
			if err != nil {
				panic(err)
			}
		}
	}

	Cron.Start()
}

// StartASingleHTTPJob adds a single job to the queue
func StartASingleHTTPJob(jobTransformer transformers.Job) {
	err := Cron.AddFunc(jobTransformer.Spec, ExecuteHTTPJob(jobTransformer))
	utils.CheckErr(err)
}
