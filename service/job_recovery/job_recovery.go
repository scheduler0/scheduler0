package job_recovery

import (
	"fmt"
	"github.com/robfig/cron"
	"log"
	"scheduler0/config"
	models "scheduler0/models"
	"scheduler0/repository"
	"scheduler0/service/job_executor"
	"scheduler0/utils"
	"sync"
	"time"
)

type JobRecovery struct {
	jobExecutor         *job_executor.JobExecutor
	jobRepo             repository.Job
	jobExecutionLogRepo repository.ExecutionsRepo
	jobQueuesRepo       repository.JobQueuesRepo
	logger              *log.Logger
	recoveredJobs       []models.JobModel
	mtx                 sync.Mutex
}

func NewJobRecovery(
	logger *log.Logger,
	jobRepo repository.Job,
	jobExecutor *job_executor.JobExecutor,
	jobExecutionLogRepo repository.ExecutionsRepo,
	jobQueuesRepo repository.JobQueuesRepo,
) *JobRecovery {
	return &JobRecovery{
		jobExecutor:         jobExecutor,
		logger:              logger,
		jobRepo:             jobRepo,
		jobExecutionLogRepo: jobExecutionLogRepo,
		jobQueuesRepo:       jobQueuesRepo,
	}
}

func (jobRecovery *JobRecovery) Run() {
	jobRecovery.mtx.Lock()
	defer jobRecovery.mtx.Unlock()

	jobRecovery.logger.Println("recovering jobs.")

	configs := config.GetConfigurations(jobRecovery.logger)

	lastVersion := jobRecovery.jobQueuesRepo.GetLastVersion()

	lastJobQueueLogs := jobRecovery.jobQueuesRepo.GetLastJobQueueLogForNode(configs.NodeId, lastVersion)
	if len(lastJobQueueLogs) < 1 {
		jobRecovery.logger.Println("no existing job queues for node")
		return
	}

	for _, lastJobQueueLog := range lastJobQueueLogs {
		expandedJobIds := []int64{}
		for i := lastJobQueueLog.LowerBoundJobId; i <= lastJobQueueLog.UpperBoundJobId; i++ {
			expandedJobIds = append(expandedJobIds, int64(i))
		}

		jobsStates := jobRecovery.jobExecutionLogRepo.GetLastExecutionLogForJobIds(expandedJobIds)

		jobsFromDb, err := jobRecovery.jobRepo.BatchGetJobsByID(expandedJobIds)
		if err != nil {
			jobRecovery.logger.Fatalln("failed to retrieve jobs from db", err)
		}

		jobRecovery.logger.Println("recovered ", len(jobsFromDb), " jobs")

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
				jobRecovery.logger.Fatalln(fmt.Sprintf("failed to parse spec %v", parseErr.Error()))
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
				jobRecovery.logger.Println("quick recovered job", job.ID)
				jobRecovery.jobExecutor.AddNewProcess(job, executionTime)
			} else {
				jobsToSchedule = append(jobsToSchedule, job)
			}
			jobRecovery.recoveredJobs = append(jobRecovery.recoveredJobs, job)
		}

		if len(jobsToSchedule) > 0 {
			jobRecovery.jobExecutor.Schedule(jobsToSchedule)
		}
	}
}

// isRecovered Check if a job is in recovered job queues
func (jobRecovery *JobRecovery) isRecovered(jobID int64) bool {
	for _, recoveredJob := range jobRecovery.recoveredJobs {
		if recoveredJob.ID == jobID {
			return true
		}
	}

	return false
}

// getRecovery Returns recovery object
func (jobRecovery *JobRecovery) getRecovery(jobID int64) *models.JobModel {
	for _, recoveredJob := range jobRecovery.recoveredJobs {
		if recoveredJob.ID == jobID {
			return &recoveredJob
		}
	}

	return nil
}

// removeJobRecovery Removes a recovery object
func (jobRecovery *JobRecovery) removeJobRecovery(jobID int64) {
	jobIndex := -1

	for index, recoveredJob := range jobRecovery.recoveredJobs {
		if recoveredJob.ID == jobID {
			jobIndex = index
			break
		}
	}

	jobRecovery.recoveredJobs = append(jobRecovery.recoveredJobs[:jobIndex], jobRecovery.recoveredJobs[jobIndex+1:]...)
}
