package processor

import (
	"context"
	"fmt"
	"github.com/hashicorp/go-hclog"
	"github.com/robfig/cron"
	"log"
	"scheduler0/pkg/config"
	"scheduler0/pkg/models"
	job_repo "scheduler0/pkg/repository/job"
	job_execution_repo "scheduler0/pkg/repository/job_execution"
	job_queue_repo "scheduler0/pkg/repository/job_queue"
	project_repo "scheduler0/pkg/repository/project"
	"scheduler0/pkg/scheduler0time"
	"scheduler0/pkg/service/executor"
	"scheduler0/pkg/service/queue"
	"sync"
	"time"
)

// jobProcessor handles executions of jobs
type jobProcessor struct {
	jobRepo             job_repo.JobRepo
	projectRepo         project_repo.ProjectRepo
	jobExecutionLogRepo job_execution_repo.JobExecutionsRepo
	jobQueuesRepo       job_queue_repo.JobQueuesRepo
	jobQueue            queue.JobQueueService
	jobExecutor         executor.JobExecutorService
	logger              hclog.Logger
	mtx                 sync.Mutex
	ctx                 context.Context
	scheduler0Config    config.Scheduler0Config
}

//go:generate mockery --name JobProcessorService --output ./ --inpackage
type JobProcessorService interface {
	StartJobs()
	RecoverJobs()
}

// NewJobProcessor creates a new job processor
func NewJobProcessor(
	ctx context.Context,
	logger hclog.Logger,
	scheduler0Config config.Scheduler0Config,
	jobRepo job_repo.JobRepo,
	projectRepo project_repo.ProjectRepo,
	jobQueue queue.JobQueueService,
	jobExecutor executor.JobExecutorService,
	jobExecutionLogRepo job_execution_repo.JobExecutionsRepo,
	jobQueuesRepo job_queue_repo.JobQueuesRepo,
) JobProcessorService {
	return &jobProcessor{
		jobRepo:             jobRepo,
		projectRepo:         projectRepo,
		jobQueue:            jobQueue,
		logger:              logger.Named("job-processor"),
		jobExecutionLogRepo: jobExecutionLogRepo,
		jobQueuesRepo:       jobQueuesRepo,
		jobExecutor:         jobExecutor,
		ctx:                 ctx,
		scheduler0Config:    scheduler0Config,
	}
}

// StartJobs the cron job job_processor
func (jobProcessor *jobProcessor) StartJobs() {
	jobProcessor.jobQueue.IncrementQueueVersion()

	totalProjectCount, countErr := jobProcessor.projectRepo.Count()
	if countErr != nil {
		jobProcessor.logger.Error("could not get number of project count", "error", countErr.Message)
		log.Fatalln("could not get number of project count", countErr.Message)
		return
	}

	jobProcessor.logger.Debug("total number of projects: ", "count", totalProjectCount)

	projects, listErr := jobProcessor.projectRepo.List(0, totalProjectCount)
	if listErr != nil {
		jobProcessor.logger.Error("could not list the number of projects", "message", listErr.Message)
		log.Fatalln("could not list the number of projects", listErr.Message)
		return
	}

	for _, project := range projects {
		jobsTotalCount, err := jobProcessor.jobRepo.GetJobsTotalCountByProjectID(project.ID)
		if err != nil {
			jobProcessor.logger.Error("could not get the number of jobs for a projects", "error", err.Message)
			log.Fatalln("could not get the number of jobs for a projects", err.Message)
			return
		}

		jobProcessor.logger.Debug(fmt.Sprintf("total number of jobs for project %v is %v : ", project.ID, jobsTotalCount))
		jobs, _, loadErr := jobProcessor.jobRepo.GetJobsPaginated(project.ID, 0, jobsTotalCount)

		for i, job := range jobs {
			jobs[i].LastExecutionDate = job.DateCreated
		}

		if loadErr != nil {
			jobProcessor.logger.Error("could not load projects", "error", loadErr.Message)
			log.Fatalln("could not load projects", loadErr.Message)
			return
		}

		jobProcessor.jobQueue.Queue(jobs)
	}
}

// RecoverJobs restarts jobs that where previous started before the node crashed
// jobs that there execution time is in the "future" will get "quick recovered"
// this means they will be scheduled to execute at the time they're supposed to execute
func (jobProcessor *jobProcessor) RecoverJobs() {
	jobProcessor.mtx.Lock()
	defer jobProcessor.mtx.Unlock()

	jobProcessor.logger.Debug("recovering jobs.")

	configs := jobProcessor.scheduler0Config.GetConfigurations()

	lastVersion := jobProcessor.jobQueuesRepo.GetLastVersion()

	lastJobQueueLogs := jobProcessor.jobQueuesRepo.GetLastJobQueueLogForNode(configs.NodeId, lastVersion)
	if len(lastJobQueueLogs) < 1 {
		jobProcessor.logger.Error("no existing job queues for node")
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
			jobProcessor.logger.Error("failed to retrieve jobs from db", "error", err.Message)
			return
		}

		jobProcessor.logger.Debug(fmt.Sprintf("recovered %d jobs", len(jobsFromDb)))

		var jobsToSchedule []models.Job

		for _, job := range jobsFromDb {
			var lastJobState models.JobExecutionLog

			if _, ok := jobsStates[job.ID]; ok {
				lastJobState = jobsStates[job.ID]
			} else {
				jobsToSchedule = append(jobsToSchedule, job)
			}

			if lastJobState.NodeId != configs.NodeId &&
				lastJobState.JobQueueVersion != lastJobQueueLog.Version {
				continue
			}

			schedule, parseErr := cron.Parse(job.Spec)
			if parseErr != nil {
				jobProcessor.logger.Error(fmt.Sprintf("failed to parse spec %v", parseErr.Error()))
				return
			}

			schedulerTime := scheduler0time.GetSchedulerTime()
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
			if now.Before(executionTime) && lastJobState.State == models.ExecutionLogScheduleState {
				jobProcessor.logger.Debug(fmt.Sprintf("quick recovered job %d", job.ID))
				jobProcessor.jobExecutor.AddJobSchedule(job)
			} else {
				jobsToSchedule = append(jobsToSchedule, job)
			}
		}

		if len(jobsToSchedule) > 0 {
			jobProcessor.jobExecutor.ScheduleJobs(jobsToSchedule)
		}
	}
}
