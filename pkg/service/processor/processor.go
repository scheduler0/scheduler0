package processor

import (
	"context"
	"fmt"
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

	"github.com/hashicorp/go-hclog"
)

// jobProcessor handles executions of jobs
type jobProcessor struct {
	jobRepo             job_repo.JobRepo
	singleNodeMode      bool
	nodeIsLeader        bool
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

type JobProcessorService interface {
	StartJobs()
	RecoverJobs()
	SetSingleNodeMode(singleNodeMode bool)
	SetNodeIsLeader(nodeIsLeader bool)
	GetSingleNodeMode() bool
	GetNodeIsLeader() bool
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
		singleNodeMode:      false,
		nodeIsLeader:        false,
	}
}

// StartJobs the cron job job_processor
func (jobProcessor *jobProcessor) StartJobs() {
	if !jobProcessor.singleNodeMode && jobProcessor.nodeIsLeader {
		jobProcessor.logger.Debug("multi node mode, skipping job queue allocation and increment")
		jobProcessor.jobQueue.AllocateQuotasForJobQueue()
		return
	}

	jobProcessor.jobQueue.IncrementQueueVersion()

	totalProjectCount, countErr := jobProcessor.projectRepo.CountAll()
	if countErr != nil {
		jobProcessor.logger.Error("could not get number of project count", "error", countErr.Message)
		log.Fatalln("could not get number of project count", countErr.Message)
		return
	}

	jobProcessor.logger.Debug("total number of projects: ", "count", totalProjectCount)

	projects, listErr := jobProcessor.projectRepo.ListAll(0, totalProjectCount)
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
		jobs, _, loadErr := jobProcessor.jobRepo.GetJobsPaginated(project.AccountId, project.ID, 0, jobsTotalCount, "id", "ASC")

		for i, job := range jobs {
			jobs[i].LastExecutionDate = job.DateCreated
		}

		// Filter out inactive jobs
		var activeJobs []models.Job
		for _, job := range jobs {
			if job.Status == models.JobStatusActive {
				activeJobs = append(activeJobs, job)
			} else {
				jobProcessor.logger.Debug("skipping inactive job during startup", "jobId", job.ID)
			}
		}

		if loadErr != nil {
			jobProcessor.logger.Error("could not load projects", "error", loadErr.Message)
			log.Fatalln("could not load projects", loadErr.Message)
			return
		}

		jobProcessor.jobQueue.Queue(activeJobs)
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

		schedulerTime := scheduler0time.GetSchedulerTime()
		now := schedulerTime.GetTime(time.Now())

		for _, job := range jobsFromDb {
			nowInJobTimezone, convertErr := job.ConvertTimeToJobTimezone(now)
			if convertErr != nil {
				jobProcessor.logger.Error(fmt.Sprintf("failed to convert date created time for job with id %d error=%s", job.ID, convertErr.Error()))
				return
			}

			// Skip inactive jobs
			if job.Status == models.JobStatusInactive {
				jobProcessor.logger.Debug("skipping inactive job during recovery", "jobId", job.ID)
				continue
			}

			if !job.EndDate.IsZero() && job.EndDate.Before(*nowInJobTimezone) {
				jobProcessor.logger.Debug("job has an end date in the past, skipping", "jobId", job.ID)
				continue
			}

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

			job.LastExecutionDate = lastJobState.LastExecutionDatetime
			nextExecutionDateLocal, getNextExecutionTimeErr := job.GetNextExecutionTime()
			if getNextExecutionTimeErr != nil {
				jobProcessor.logger.Error(fmt.Sprintf("failed to get next execution time for job with id %d error=%s", job.ID, getNextExecutionTimeErr.Error()))
				return
			}
			job.ExecutionId = lastJobState.UniqueId
			if nowInJobTimezone.Before(*nextExecutionDateLocal) && lastJobState.State == models.ExecutionLogScheduleState {
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

func (jobProcessor *jobProcessor) SetSingleNodeMode(singleNodeMode bool) {
	jobProcessor.singleNodeMode = singleNodeMode
}

func (jobProcessor *jobProcessor) SetNodeIsLeader(nodeIsLeader bool) {
	jobProcessor.nodeIsLeader = nodeIsLeader
}

func (jobProcessor *jobProcessor) GetSingleNodeMode() bool {
	return jobProcessor.singleNodeMode
}

func (jobProcessor *jobProcessor) GetNodeIsLeader() bool {
	return jobProcessor.nodeIsLeader
}
