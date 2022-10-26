package job_process

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
	logger      *log.Logger
}

// NewJobProcessor creates a new job processor
func NewJobProcessor(jobRepo repository.Job, projectRepo repository.Project, jobQueue job_queue.JobQueue, logger *log.Logger) *JobProcessor {
	return &JobProcessor{
		jobRepo:     jobRepo,
		projectRepo: projectRepo,
		jobQueue:    jobQueue,
		logger:      logger,
	}
}

// StartJobs the cron job job_process
func (jobProcessor *JobProcessor) StartJobs() {
	logPrefix := jobProcessor.logger.Prefix()
	jobProcessor.logger.SetPrefix(fmt.Sprintf("%s[job-processor] ", logPrefix))
	defer jobProcessor.logger.SetPrefix(logPrefix)

	totalProjectCount, countErr := jobProcessor.projectRepo.Count()
	if countErr != nil {
		jobProcessor.logger.Fatalln(countErr.Message)
	}

	utils.Info("Total number of projects: ", totalProjectCount)

	projectTransformers, listErr := jobProcessor.projectRepo.List(0, totalProjectCount)
	if listErr != nil {
		jobProcessor.logger.Fatalln(countErr.Message)
	}

	for _, projectTransformer := range projectTransformers {
		jobsTotalCount, err := jobProcessor.jobRepo.GetJobsTotalCountByProjectID(projectTransformer.ID)
		if err != nil {
			jobProcessor.logger.Fatalln(err.Message)
		}

		utils.Info(fmt.Sprintf("Total number of jobs for project %v is %v : ", projectTransformer.ID, jobsTotalCount))
		jobs, _, loadErr := jobProcessor.jobRepo.GetJobsPaginated(projectTransformer.ID, 0, jobsTotalCount)
		if loadErr != nil {
			jobProcessor.logger.Fatalln(loadErr.Message)
		}

		jobProcessor.jobQueue.Queue(jobs)
	}
}
