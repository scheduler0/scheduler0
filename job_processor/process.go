package job_processor

import (
	"fmt"
	"log"
	"scheduler0/job_queue"
	"scheduler0/repository"
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

// StartJobs the cron job job_processor
func (jobProcessor *JobProcessor) StartJobs() {
	logPrefix := jobProcessor.logger.Prefix()
	jobProcessor.logger.SetPrefix(fmt.Sprintf("%s[job-processor] ", logPrefix))
	defer jobProcessor.logger.SetPrefix(logPrefix)

	totalProjectCount, countErr := jobProcessor.projectRepo.Count()
	if countErr != nil {
		jobProcessor.logger.Fatalln(countErr.Message)
	}

	jobProcessor.logger.Println("Total number of projects: ", totalProjectCount)

	projects, listErr := jobProcessor.projectRepo.List(0, totalProjectCount)
	if listErr != nil {
		jobProcessor.logger.Fatalln(listErr.Message)
	}

	for _, project := range projects {
		jobsTotalCount, err := jobProcessor.jobRepo.GetJobsTotalCountByProjectID(project.ID)
		if err != nil {
			jobProcessor.logger.Fatalln(err.Message)
		}

		jobProcessor.logger.Println(fmt.Sprintf("Total number of jobs for project %v is %v : ", project.ID, jobsTotalCount))
		jobs, _, loadErr := jobProcessor.jobRepo.GetJobsPaginated(project.ID, 0, jobsTotalCount)

		for i, job := range jobs {
			jobs[i].LastExecutionDate = job.DateCreated
		}

		if loadErr != nil {
			jobProcessor.logger.Fatalln(loadErr.Message)
		}

		jobProcessor.jobQueue.Queue(jobs)
	}
}
