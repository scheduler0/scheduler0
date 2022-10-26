package process

import (
	"fmt"
	"log"
	"scheduler0/job_queue"
	"scheduler0/repository"
	"scheduler0/utils"
)

// JobProcessor handles executions of jobs
type JobProcessor struct {
	jobRepo     repository.Job
	projectRepo repository.Project
	jobQueue    job_queue.JobQueue
}

// NewJobProcessor creates a new job processor
func NewJobProcessor(jobRepo repository.Job, projectRepo repository.Project, jobQueue job_queue.JobQueue) *JobProcessor {
	return &JobProcessor{
		jobRepo:     jobRepo,
		projectRepo: projectRepo,
		jobQueue:    jobQueue,
	}
}

// RecoverJobExecutions find jobs that could've not been executed due to timeout
//func (jobProcessor *JobProcessor) RecoverJobExecutions(jobTransformers []models.JobModel) {
//	manager := repository.ExecutionRepo{}
//
//	//dir, err := os.Getwd()
//	//if err != nil {
//	//	log.Fatalln(fmt.Errorf("Fatal error getting working dir: %s \n", err))
//	//}
//	//dbFilePath := fmt.Sprintf("%v/.scheduler0.execs", dir)
//	//execsLogFile, err := os.ReadFile(dbFilePath)
//	//
//	//buf := &bytes.Buffer{}
//	//encodeErr := gob.NewEncoder(buf).Encode(execsLogFile)
//	//if encodeErr != nil {
//	//	log.Fatalln(fmt.Errorf("Fatal encode err: %s \n", encodeErr))
//	//}
//	//bs := buf.String()
//	//
//	//fmt.Println("bs", bs)
//
//	for _, jobTransformer := range jobTransformers {
//		if jobProcessor.IsRecovered(jobTransformer.ID) {
//			continue
//		}
//		count, err, executionManagers := manager.FindJobExecutionPlaceholderByID(jobProcessor.DBConnection, jobTransformer.ID)
//		if err != nil {
//			utils.Error(fmt.Sprintf("Error occurred while fetching execution mangers for jobs to be recovered %s", err.Message))
//			continue
//		}
//
//		if count < 1 {
//			continue
//		}
//
//		executionManager := executionManagers[0]
//		schedule, parseErr := cron.Parse(jobTransformer.Spec)
//
//		if parseErr != nil {
//			utils.Error(fmt.Sprintf("Failed to create schedule%s", parseErr.Error()))
//			continue
//		}
//		now := time.Now().UTC()
//		executionTime := schedule.Next(executionManager.TimeAdded).UTC()
//
//		if now.Before(executionTime) {
//			executionTransformer := transformers.Execution{}
//			executionTransformer.FromRepo(executionManager)
//			recovery := RecoveredJob{
//				Execution: &executionTransformer,
//				Job:       &jobTransformer,
//			}
//			jobProcessor.RecoveredJobs = append(jobProcessor.RecoveredJobs, recovery)
//		}
//	}
//}

// StartJobs the cron job process
func (jobProcessor *JobProcessor) StartJobs() {
	totalProjectCount, countErr := jobProcessor.projectRepo.Count()
	if countErr != nil {
		log.Fatalln(countErr.Message)
	}

	utils.Info("Total number of projects: ", totalProjectCount)

	projectTransformers, listErr := jobProcessor.projectRepo.List(0, totalProjectCount)
	if listErr != nil {
		log.Fatalln(countErr.Message)
	}

	for _, projectTransformer := range projectTransformers {
		jobsTotalCount, err := jobProcessor.jobRepo.GetJobsTotalCountByProjectID(projectTransformer.ID)
		if err != nil {
			log.Fatalln(err.Message)
		}

		utils.Info(fmt.Sprintf("Total number of jobs for project %v is %v : ", projectTransformer.ID, jobsTotalCount))
		jobs, _, loadErr := jobProcessor.jobRepo.GetJobsPaginated(projectTransformer.ID, 0, jobsTotalCount)
		if loadErr != nil {
			log.Fatalln(loadErr.Message)
		}

		jobProcessor.jobQueue.Queue(jobs)
	}
}
