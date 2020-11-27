package process

import (
	"cron-server/server/src/managers"
	"cron-server/server/src/service"
	"cron-server/server/src/utils"
	"fmt"
	"github.com/robfig/cron"
)

// Start the cron job process
func Start(pool *utils.Pool) {
	projectManager := managers.ProjectManager{}

	totalProjectCount, err := projectManager.GetTotalCount(pool)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Total Project Count %v", totalProjectCount)

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
		jobManager := managers.JobManager{}

		jobsTotalCount, err :=jobManager.GetJobsTotalCountByProjectID(pool, projectTransformer.ID)
		if err != nil {
			panic(err)
		}

		fmt.Printf("Total Job Count %v", jobsTotalCount)


		jobTransformers, err := jobService.GetJobsByProjectID(projectTransformer.ID, 0, jobsTotalCount, "date_created")

		for _, jobTransformer := range jobTransformers {

			fmt.Println("Adding job to ", jobTransformer.Description)

			err := cronJobs.AddFunc(jobTransformer.CronSpec, func() {
				fmt.Println("Should send request to "+ jobTransformer.CallbackUrl)
			})

			if err != nil {
				panic(err)
			}

		}
	}

	cronJobs.Start()
}
