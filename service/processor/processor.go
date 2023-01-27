package processor

import (
	"context"
	"fmt"
	"github.com/robfig/cron"
	"log"
	"scheduler0/config"
	"scheduler0/models"
	"scheduler0/repository"
	"scheduler0/service/executor"
	"scheduler0/service/queue"
	"scheduler0/utils"
	"sync"
	"time"
)

// JobProcessor handles executions of jobs
type JobProcessor struct {
	jobRepo             repository.Job
	projectRepo         repository.Project
	jobExecutionLogRepo repository.ExecutionsRepo
	jobQueuesRepo       repository.JobQueuesRepo
	jobQueue            *queue.JobQueue
	jobExecutor         *executor.JobExecutor
	logger              *log.Logger
	mtx                 sync.Mutex
	ctx                 context.Context
}

// NewJobProcessor creates a new job processor
func NewJobProcessor(ctx context.Context, logger *log.Logger, jobRepo repository.Job, projectRepo repository.Project, jobQueue *queue.JobQueue, jobExecutor *executor.JobExecutor, jobExecutionLogRepo repository.ExecutionsRepo, jobQueuesRepo repository.JobQueuesRepo) *JobProcessor {
	return &JobProcessor{
		jobRepo:             jobRepo,
		projectRepo:         projectRepo,
		jobQueue:            jobQueue,
		logger:              logger,
		jobExecutionLogRepo: jobExecutionLogRepo,
		jobQueuesRepo:       jobQueuesRepo,
		jobExecutor:         jobExecutor,
		ctx:                 ctx,
	}
}

// StartJobs the cron job job_processor
func (jobProcessor *JobProcessor) StartJobs() {
	logPrefix := jobProcessor.logger.Prefix()
	jobProcessor.logger.SetPrefix(fmt.Sprintf("%s[job-processor] ", logPrefix))
	defer jobProcessor.logger.SetPrefix(logPrefix)

	jobProcessor.jobQueue.IncrementQueueVersion()

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

// RecoverJobs restarts jobs that where previous started before the node crashed
// jobs that there execution time is in the "future" will get "quick recovered"
// this means they will be scheduled to execute at the time they're supposed to execute
func (jobProcessor *JobProcessor) RecoverJobs() {
	jobProcessor.mtx.Lock()
	defer jobProcessor.mtx.Unlock()

	jobProcessor.logger.Println("recovering jobs.")

	configs := config.GetConfigurations(jobProcessor.logger)

	lastVersion := jobProcessor.jobQueuesRepo.GetLastVersion()

	lastJobQueueLogs := jobProcessor.jobQueuesRepo.GetLastJobQueueLogForNode(configs.NodeId, lastVersion)
	if len(lastJobQueueLogs) < 1 {
		jobProcessor.logger.Println("no existing job queues for node")
		return
	}

	for _, lastJobQueueLog := range lastJobQueueLogs {
		expandedJobIds := []uint64{}
		for i := lastJobQueueLog.LowerBoundJobId; i <= lastJobQueueLog.UpperBoundJobId; i++ {
			expandedJobIds = append(expandedJobIds, i)
		}

		jobsStates := jobProcessor.jobExecutionLogRepo.GetLastExecutionLogForJobIds(expandedJobIds)

		jobsFromDb, err := jobProcessor.jobRepo.BatchGetJobsByID(expandedJobIds)
		if err != nil {
			jobProcessor.logger.Fatalln("failed to retrieve jobs from db", err)
		}

		jobProcessor.logger.Println("recovered ", len(jobsFromDb), " jobs")

		jobsToSchedule := []models.JobModel{}

		for _, job := range jobsFromDb {
			var lastJobState models.JobExecutionLog

			if _, ok := jobsStates[job.ID]; ok {
				lastJobState = jobsStates[job.ID]
			} else {
				jobsToSchedule = append(jobsToSchedule, job)
			}

			if lastJobState.NodeId != uint64(configs.NodeId) &&
				lastJobState.JobQueueVersion != lastJobQueueLog.Version {
				continue
			}

			schedule, parseErr := cron.Parse(job.Spec)
			if parseErr != nil {
				jobProcessor.logger.Fatalln(fmt.Sprintf("failed to parse spec %v", parseErr.Error()))
			}

			schedulerTime := utils.GetSchedulerTime()
			now := schedulerTime.GetTime(time.Now())

			executionTime := schedule.Next(lastJobState.LastExecutionDatetime)
			job.ExecutionId = lastJobState.UniqueId
			job.LastExecutionDate = lastJobState.LastExecutionDatetime

			// This is a comparison with the absolute time of the node
			// which may be false compared to time on other nodes.
			// To prevent this bug leaders can veto scheduled by
			// comparing the execution time on the schedule by recalculating it against it's time.
			// Time clocks are sources of distributed systems errors and a monotonic clock should always be preferred.
			// While 60 minutes is quite an unlike delay in a close it's not impossible
			if now.Before(executionTime) && lastJobState.State == uint64(models.ExecutionLogScheduleState) {
				//jobProcessor.logger.Println("quick recovered job", job.ID)
				jobProcessor.jobExecutor.ScheduleProcess(job, executionTime)
			} else {
				jobsToSchedule = append(jobsToSchedule, job)
			}
		}

		if len(jobsToSchedule) > 0 {
			jobProcessor.jobExecutor.ScheduleJobs(jobsToSchedule)
		}
	}
}
