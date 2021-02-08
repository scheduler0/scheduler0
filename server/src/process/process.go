package process

import (
	"github.com/robfig/cron"
	execution2 "github.com/victorlenerd/scheduler0/server/src/managers/execution"
	"github.com/victorlenerd/scheduler0/server/src/managers/job"
	"github.com/victorlenerd/scheduler0/server/src/managers/project"
	"github.com/victorlenerd/scheduler0/server/src/service"
	"github.com/victorlenerd/scheduler0/server/src/utils"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

// Start the cron job process
func Start(pool *utils.Pool) {
	projectManager := project.ProjectManager{}

	totalProjectCount, err := projectManager.GetTotalCount(pool)
	if err != nil {
		panic(err)
	}

	projectService := service.ProjectService{
		Pool: pool,
	}

	projectTransformers, err :=projectService.List(0, totalProjectCount)
	if err != nil {
		panic(err)
	}

	jobService := service.JobService{
		Pool: pool,
	}

	cronJobs := cron.New()

	for _, projectTransformer := range projectTransformers {
		jobManager := job.JobManager{}

		jobsTotalCount, err :=jobManager.GetJobsTotalCountByProjectID(pool, projectTransformer.UUID)
		if err != nil {
			panic(err)
		}

		jobTransformers, err := jobService.GetJobsByProjectUUID(projectTransformer.UUID, 0, jobsTotalCount, "date_created")

		for _, jobTransformer := range jobTransformers {

			err := cronJobs.AddFunc(jobTransformer.CronSpec, func() {
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
				execution := execution2.ExecutionManager{
					JobUUID:       jobTransformer.UUID,
					Timeout:     timeout,
					Response:    response,
					StatusCode:  string(statusCode),
					DateCreated: time.Now().UTC(),
				}

				_, err = execution.CreateOne(pool)
				utils.CheckErr(err)
			})

			if err != nil {
				panic(err)
			}

		}
	}

	cronJobs.Start()
}
