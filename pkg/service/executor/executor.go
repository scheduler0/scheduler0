package executor

import (
	"context"
	"fmt"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	"math"
	"scheduler0/pkg/config"
	"scheduler0/pkg/constants"
	"scheduler0/pkg/fsm"
	"scheduler0/pkg/models"
	job_repo "scheduler0/pkg/repository/job"
	job_execution_repo "scheduler0/pkg/repository/job_execution"
	job_queue_repo "scheduler0/pkg/repository/job_queue"
	"scheduler0/pkg/scheduler0time"
	"scheduler0/pkg/service/executor/executors"
	"scheduler0/pkg/utils"
	"sync"
	"time"
)

type jobExecutor struct {
	raft                  *raft.Raft
	singleNodeMode        bool
	context               context.Context
	pendingJobInvocations []models.Job
	jobRepo               job_repo.JobRepo
	jobExecutionsRepo     job_execution_repo.JobExecutionsRepo
	jobQueuesRepo         job_queue_repo.JobQueuesRepo
	logger                hclog.Logger
	cancelReq             context.CancelFunc
	httpExecutionHandler  executors.HTTPExecutor
	mtx                   sync.Mutex
	jobExecutionsCache    sync.Map
	debounce              *utils.Debounce
	dispatcher            *utils.Dispatcher
	scheduledJobs         sync.Map
	scheduler0Config      config.Scheduler0Config
	scheduler0Actions     fsm.Scheduler0RaftActions
}

//go:generate mockery --name JobExecutorService --output ./ --inpackage
type JobExecutorService interface {
	QueueExecutions(lastInsertedId, rowsAffected int64)
	ScheduleJobs(jobs []models.Job)
	StopAll()
	AddJobSchedule(job models.Job)
	ListenForJobsToInvoke()
	GetUncommittedLogs() []models.JobExecutionLog
	SetSingleNodeMode(singleNodeMode bool)
	UpdateRaft(rft *raft.Raft)
	GetSingleNodeMode() bool
	GetScheduledJobs() *sync.Map
	GetExecutionsCache() *sync.Map
	DeleteNewUncommittedExecutionLogs(lastInsertedId, rowsAffected int64)
}

func NewJobExecutor(
	ctx context.Context,
	logger hclog.Logger,
	scheduler0Config config.Scheduler0Config,
	scheduler0Actions fsm.Scheduler0RaftActions,
	jobRepository job_repo.JobRepo,
	executionsRepo job_execution_repo.JobExecutionsRepo,
	jobQueuesRepo job_queue_repo.JobQueuesRepo,
	httpExecutionHandler executors.HTTPExecutor,
	dispatcher *utils.Dispatcher) JobExecutorService {
	reCtx, cancel := context.WithCancel(ctx)
	return &jobExecutor{
		pendingJobInvocations: []models.Job{},
		scheduledJobs:         sync.Map{},
		jobRepo:               jobRepository,
		jobExecutionsRepo:     executionsRepo,
		jobQueuesRepo:         jobQueuesRepo,
		logger:                logger.Named("job-executor-service"),
		context:               reCtx,
		cancelReq:             cancel,
		httpExecutionHandler:  httpExecutionHandler,
		jobExecutionsCache:    sync.Map{},
		debounce:              utils.NewDebounce(),
		dispatcher:            dispatcher,
		scheduler0Config:      scheduler0Config,
		scheduler0Actions:     scheduler0Actions,
	}
}

func (jobExecutor *jobExecutor) QueueExecutions(lastInsertedId, rowsAffected int64) {
	newJobQueueLogs := jobExecutor.
		jobQueuesRepo.
		GetJobQueueByLastInsertedAndRowsAffected(lastInsertedId, rowsAffected)

	lowerBound := math.MaxInt64
	upperBound := math.MinInt64

	for _, newJobQueueLog := range newJobQueueLogs {
		if uint64(lowerBound) > newJobQueueLog.LowerBoundJobId {
			lowerBound = int(int64(newJobQueueLog.LowerBoundJobId))
		}
		if uint64(upperBound) > newJobQueueLog.UpperBoundJobId {
			upperBound = int(int64(newJobQueueLog.UpperBoundJobId))
		}
	}

	jobExecutor.logger.Debug("Queueing jobs", "from", lowerBound, "to", upperBound)
	if upperBound-lowerBound > constants.JobMaxBatchSize {
		currentLowerBound := lowerBound
		currentUpperBound := lowerBound + constants.JobMaxBatchSize

		for currentLowerBound < upperBound {

			jobExecutor.logger.Debug("fetching batching", "from", currentLowerBound, "to", currentUpperBound)
			jobs, getErr := jobExecutor.jobRepo.BatchGetJobsWithIDRange(int64(uint64(currentLowerBound)), int64(uint64(currentUpperBound)))

			if getErr != nil {
				jobExecutor.logger.Error("failed to batch get job by ranges ids", "error", getErr)
				return
			}

			jobExecutor.ScheduleJobs(jobs)

			if upperBound-currentUpperBound < constants.JobMaxBatchSize {
				currentLowerBound = currentUpperBound + 1
				currentUpperBound = upperBound
			} else {
				currentLowerBound = currentUpperBound + 1
				currentUpperBound = int(math.Min(
					float64(currentLowerBound+constants.JobMaxBatchSize),
					float64(upperBound),
				))
			}
		}
	} else {
		jobs, getErr := jobExecutor.jobRepo.BatchGetJobsWithIDRange(int64(lowerBound), int64(upperBound))
		if getErr != nil {
			jobExecutor.logger.Error("failed to batch get job by ranges ids ", "error", getErr)
		}
		jobExecutor.ScheduleJobs(jobs)
	}
}

func (jobExecutor *jobExecutor) ScheduleJobs(jobs []models.Job) {
	if len(jobs) < 1 {
		return
	}
	configs := jobExecutor.scheduler0Config.GetConfigurations()

	jobIds := make([]uint64, 0, len(jobs))
	for _, job := range jobs {
		jobIds = append(jobIds, job.ID)
	}

	executionLogsMap := jobExecutor.jobExecutionsRepo.GetLastExecutionLogForJobIds(jobIds)

	for i, job := range jobs {
		// First execution of the job
		if _, ok := executionLogsMap[job.ID]; !ok {
			dateCreatedInLocal, err := jobs[i].ConvertTimeToJobTimezone(job.DateCreated)
			if err != nil {
				jobExecutor.logger.Error(fmt.Sprintf("failed to convert date created time for job with id %d error=%s", job.ID, err.Error()))
				continue
			}
			jobs[i].LastExecutionDate = *dateCreatedInLocal
			nextExecutionTime, err := jobs[i].GetNextExecutionTime()
			if err != nil {
				jobExecutor.logger.Error(fmt.Sprintf("failed to get next execution time for job with id %d error=%s", job.ID, err.Error()))
				continue
			}
			executionId, err := jobs[i].GetNextExecutionId()
			if err != nil {
				jobExecutor.logger.Error(fmt.Sprintf("failed to get next execution id for job with id %d error=%s", job.ID, err.Error()))
				continue
			}
			jobs[i].ExecutionId = executionId
			jobExecutor.AddJobSchedule(jobs[i])
			jobExecutor.jobExecutionsCache.Store(job.ID, models.MemJobExecution{
				ExecutionVersion:      1,
				FailCount:             0,
				LastState:             models.ExecutionLogScheduleState,
				LastExecutionDatetime: job.DateCreated,
				NextExecutionDatetime: *nextExecutionTime,
			})
			continue
		}

		jobLastLog := executionLogsMap[job.ID]

		// Upon a recovery when the job never executed; it's last state would be models.ExecutionLogScheduleState
		// We simply re-schedule the job
		if jobLastLog.State == models.ExecutionLogScheduleState {
			jobs[i].LastExecutionDate = jobLastLog.LastExecutionDatetime
			nextExecutionTime, err := jobs[i].GetNextExecutionTime()
			if nextExecutionTime.Sub(jobLastLog.NextExecutionDatetime).Round(time.Duration(1)*time.Minute) < 1 {
				jobs[i].ExecutionId = jobLastLog.UniqueId
			} else {
				uniqueId, err := jobs[i].GetNextExecutionId()
				if err != nil {
					jobExecutor.logger.Error(fmt.Sprintf("failed to get next execution id for job with id %d error=%s", job.ID, err.Error()))
					continue
				}
				jobs[i].ExecutionId = uniqueId
			}
			if err != nil {
				jobExecutor.logger.Error(fmt.Sprintf("failed to get next execution time for job with id %d error=%s", job.ID, err.Error()))
				continue
			}
			jobExecutor.AddJobSchedule(jobs[i])
			jobExecutor.jobExecutionsCache.Store(job.ID, models.MemJobExecution{
				ExecutionVersion:      jobLastLog.ExecutionVersion,
				FailCount:             0,
				LastState:             models.ExecutionLogScheduleState,
				LastExecutionDatetime: job.LastExecutionDate,
				NextExecutionDatetime: *nextExecutionTime,
			})
		}

		// The job executed success the last time, so now we reschedule it
		if jobLastLog.State == models.ExecutionLogSuccessState {
			jobs[i].LastExecutionDate = jobLastLog.NextExecutionDatetime
			nextExecutionTime, err := jobs[i].GetNextExecutionTime()
			if err != nil {
				jobExecutor.logger.Error(fmt.Sprintf("failed to get next execution time for job with id %d error=%s", job.ID, err.Error()))
				continue
			}
			uniqueId, err := jobs[i].GetNextExecutionId()
			if err != nil {
				jobExecutor.logger.Error(fmt.Sprintf("failed to get next execution id for job with id %d error=%s", job.ID, err.Error()))
				continue
			}
			jobs[i].ExecutionId = uniqueId
			jobExecutor.AddJobSchedule(jobs[i])
			jobExecutor.jobExecutionsCache.Store(job.ID, models.MemJobExecution{
				ExecutionVersion:      jobLastLog.ExecutionVersion,
				FailCount:             0,
				LastState:             models.ExecutionLogSuccessState,
				LastExecutionDatetime: jobLastLog.NextExecutionDatetime,
				NextExecutionDatetime: *nextExecutionTime,
			})
		}

		// The job failed the last time it executed, so we are trying it
		if jobLastLog.State == models.ExecutionLogFailedState {
			failCounts := jobExecutor.jobExecutionsRepo.CountLastFailedExecutionLogs(job.ID, configs.NodeId, jobLastLog.ExecutionVersion)
			if failCounts < uint64(configs.JobExecutionRetryMax) {
				jobs[i].LastExecutionDate = jobLastLog.LastExecutionDatetime
				uniqueId, err := jobs[i].GetNextExecutionId()
				if err != nil {
					jobExecutor.logger.Error(fmt.Sprintf("failed to get next execution id for job with id %d error=%s", job.ID, err.Error()))
					continue
				}
				jobs[i].ExecutionId = uniqueId
				jobExecutor.jobExecutionsCache.Store(job.ID, models.MemJobExecution{
					ExecutionVersion:      jobLastLog.ExecutionVersion,
					FailCount:             failCounts,
					LastState:             models.ExecutionLogFailedState,
					LastExecutionDatetime: jobLastLog.LastExecutionDatetime,
					NextExecutionDatetime: jobLastLog.NextExecutionDatetime,
				})
				jobExecutor.AddJobSchedule(jobs[i])
			}

			// After all retry attempts for the failed job
			jobs[i].LastExecutionDate = jobLastLog.NextExecutionDatetime
			nextExecutionTime, err := jobs[i].GetNextExecutionTime()
			if err != nil {
				jobExecutor.logger.Error(fmt.Sprintf("failed to get next execution time for job with id %d error=%s", job.ID, err.Error()))
				continue
			}
			uniqueId, err := jobs[i].GetNextExecutionId()
			if err != nil {
				jobExecutor.logger.Error(fmt.Sprintf("failed to get next execution id for job with id %d error=%s", job.ID, err.Error()))
				continue
			}
			jobs[i].ExecutionId = uniqueId
			jobExecutor.AddJobSchedule(jobs[i])
			jobExecutor.jobExecutionsCache.Store(job.ID, models.MemJobExecution{
				ExecutionVersion:      jobLastLog.ExecutionVersion,
				FailCount:             0,
				LastState:             models.ExecutionLogFailedState,
				LastExecutionDatetime: jobLastLog.NextExecutionDatetime,
				NextExecutionDatetime: *nextExecutionTime,
			})
		}
	}

	lastVersion := jobExecutor.jobQueuesRepo.GetLastVersion()
	lastExecutionVersions := make(map[uint64]uint64)

	for _, job := range jobs {
		if _, ok := lastExecutionVersions[job.ID]; !ok {
			if _, ok := executionLogsMap[job.ID]; ok {
				cachedJobExecutionsLog, _ := jobExecutor.jobExecutionsCache.Load(job.ID)
				cachedJobExecutionLog := (cachedJobExecutionsLog).(models.MemJobExecution)
				if executionLogsMap[job.ID].State == models.ExecutionLogSuccessState ||
					(cachedJobExecutionLog.FailCount == 0 &&
						cachedJobExecutionLog.LastState == models.ExecutionLogFailedState) {
					lastExecutionVersions[job.ID] = executionLogsMap[job.ID].ExecutionVersion + 1
				} else {
					lastExecutionVersions[job.ID] = executionLogsMap[job.ID].ExecutionVersion
				}
			} else {
				lastExecutionVersions[job.ID] = 1
			}
		}
	}

	jobExecutor.jobExecutionsRepo.BatchInsert(jobs, configs.NodeId, models.ExecutionLogScheduleState, lastVersion, lastExecutionVersions)

	if jobExecutor.singleNodeMode {
		jobExecutor.jobExecutionsRepo.LogJobExecutionStateInRaft(jobs, models.ExecutionLogScheduleState, lastExecutionVersions, lastVersion, configs.NodeId)
	}

	jobExecutor.logger.Debug("scheduled jobs", "from", jobs[0].ID, "to", jobs[len(jobs)-1].ID)
}

func (jobExecutor *jobExecutor) StopAll() {
	jobExecutor.mtx.Lock()
	defer jobExecutor.mtx.Unlock()
	jobExecutor.scheduledJobs.Range(func(key, value any) bool {
		jobExecutor.scheduledJobs.Delete(key)
		return true
	})
	jobExecutor.logger.Info("stopped all scheduled job")
}

func (jobExecutor *jobExecutor) AddJobSchedule(job models.Job) {
	schedulerTime := scheduler0time.GetSchedulerTime()
	nextExecutionDateLocal, err := job.GetNextExecutionTime()
	if err != nil {
		jobExecutor.logger.Error(fmt.Sprintf("failed to get next execution time for job with id %d error=%s", job.ID, err.Error()))
		return
	}
	jobExecutor.scheduledJobs.Store(job.ID, models.JobSchedule{
		Job:           job,
		ExecutionTime: schedulerTime.GetTime(*nextExecutionDateLocal),
	})
}

func (jobExecutor *jobExecutor) ListenForJobsToInvoke() {
	ticker := time.NewTicker(time.Duration(1) * time.Second)
	schedulerTime := scheduler0time.GetSchedulerTime()

	go func() {
		for {
			select {
			case <-ticker.C:
				currentTime := schedulerTime.GetTime(time.Now())
				jobExecutor.scheduledJobs.Range(func(key, value any) bool {
					jobSchedule := value.(models.JobSchedule)
					if currentTime.After(jobSchedule.ExecutionTime) {
						jobExecutor.scheduledJobs.Delete(jobSchedule.Job.ID)
						jobExecutor.invokeJob(jobSchedule.Job)
					}
					return true
				})
			case <-jobExecutor.context.Done():
				return
			}
		}
	}()
}

func (jobExecutor *jobExecutor) GetUncommittedLogs() []models.JobExecutionLog {
	jobExecutor.mtx.Lock()
	defer jobExecutor.mtx.Unlock()

	configs := jobExecutor.scheduler0Config.GetConfigurations()
	executionLogs := jobExecutor.jobExecutionsRepo.GetUncommittedExecutionsLogForNode(configs.NodeId)

	return executionLogs
}

func (jobExecutor *jobExecutor) SetSingleNodeMode(singleNodeMode bool) {
	jobExecutor.singleNodeMode = singleNodeMode
}

func (jobExecutor *jobExecutor) UpdateRaft(rft *raft.Raft) {
	jobExecutor.raft = rft
}

func (jobExecutor *jobExecutor) GetSingleNodeMode() bool {
	return jobExecutor.singleNodeMode
}

func (jobExecutor *jobExecutor) GetScheduledJobs() *sync.Map {
	return &jobExecutor.scheduledJobs
}

func (jobExecutor *jobExecutor) GetExecutionsCache() *sync.Map {
	return &jobExecutor.jobExecutionsCache
}

func (jobExecutor *jobExecutor) DeleteNewUncommittedExecutionLogs(lastInsertedId, rowsAffected int64) {
}

func (jobExecutor *jobExecutor) reschedule(jobs []models.Job, newState models.JobExecutionLogState) {
	jobExecutor.mtx.Lock()
	defer jobExecutor.mtx.Unlock()

	configs := jobExecutor.scheduler0Config.GetConfigurations()

	jobsToReschedule := make([]models.Job, 0, len(jobs))

	for i, job := range jobs {
		cachedJobExecutionsLog, _ := jobExecutor.jobExecutionsCache.Load(job.ID)
		lastExecution := (cachedJobExecutionsLog).(models.MemJobExecution)

		failCounts := lastExecution.FailCount
		executionVersion := lastExecution.ExecutionVersion

		if newState == models.ExecutionLogFailedState &&
			failCounts < configs.JobExecutionRetryMax {
			failCounts += 1
			jobExecutor.jobExecutionsCache.Store(job.ID, models.MemJobExecution{
				ExecutionVersion:      executionVersion,
				FailCount:             failCounts,
				LastState:             newState,
				LastExecutionDatetime: lastExecution.LastExecutionDatetime,
				NextExecutionDatetime: lastExecution.NextExecutionDatetime,
			})
			continue
		} else {
			executionVersion += 1
		}
		jobs[i].LastExecutionDate = lastExecution.NextExecutionDatetime
		executionId, err := jobs[i].GetNextExecutionId()
		if err != nil {
			jobExecutor.logger.Error(fmt.Sprintf("failed to get next execution id for job with id %d error=%s", job.ID, err.Error()))
			continue
		}
		executionTime, err := jobs[i].GetNextExecutionTime()
		if err != nil {
			jobExecutor.logger.Error(fmt.Sprintf("failed to parse job cron spec %s", err.Error()))
			continue
		}

		jobs[i].ExecutionId = executionId
		jobExecutor.AddJobSchedule(jobs[i])
		jobExecutor.jobExecutionsCache.Store(job.ID, models.MemJobExecution{
			ExecutionVersion:      executionVersion,
			FailCount:             0,
			LastState:             newState,
			LastExecutionDatetime: lastExecution.NextExecutionDatetime,
			NextExecutionDatetime: *executionTime,
		})
		jobsToReschedule = append(jobsToReschedule, jobs[i])
	}

	lastVersion := jobExecutor.jobQueuesRepo.GetLastVersion()
	lastExecutionVersions := make(map[uint64]uint64)

	for _, job := range jobsToReschedule {
		if _, ok := lastExecutionVersions[job.ID]; !ok {
			cachedJobExecutionsLog, _ := jobExecutor.jobExecutionsCache.Load(job.ID)
			lastExecution := (cachedJobExecutionsLog).(models.MemJobExecution)
			lastExecutionVersions[job.ID] = lastExecution.ExecutionVersion
		}
	}

	jobExecutor.jobExecutionsRepo.BatchInsert(jobsToReschedule, configs.NodeId, models.ExecutionLogScheduleState, lastVersion, lastExecutionVersions)
	if jobExecutor.singleNodeMode {
		jobExecutor.jobExecutionsRepo.LogJobExecutionStateInRaft(jobsToReschedule, models.ExecutionLogScheduleState, lastExecutionVersions, lastVersion, configs.NodeId)
	}
}

func (jobExecutor *jobExecutor) createInMemExecutionsForJobsIfNotExist(jobs []models.Job) {
	jobExecutor.mtx.Lock()
	defer jobExecutor.mtx.Unlock()

	jobsNotInExecution := make([]uint64, 0, len(jobs))

	for _, job := range jobs {
		_, ok := jobExecutor.jobExecutionsCache.Load(job.ID)
		if !ok {
			jobsNotInExecution = append(jobsNotInExecution, job.ID)
		}
	}

	lastExecutionVersionsForNewJobs := jobExecutor.jobExecutionsRepo.GetLastExecutionLogForJobIds(jobsNotInExecution)

	configs := jobExecutor.scheduler0Config.GetConfigurations()

	for _, jobId := range jobsNotInExecution {
		failCounts := 0
		if lastExecutionVersionsForNewJobs[jobId].State == models.ExecutionLogFailedState {
			failCounts = int(jobExecutor.jobExecutionsRepo.CountLastFailedExecutionLogs(jobId, configs.NodeId, lastExecutionVersionsForNewJobs[jobId].ExecutionVersion))
		}

		jobExecutor.jobExecutionsCache.Store(jobId, models.MemJobExecution{
			ExecutionVersion:      lastExecutionVersionsForNewJobs[jobId].ExecutionVersion,
			FailCount:             uint64(failCounts),
			LastState:             lastExecutionVersionsForNewJobs[jobId].State,
			LastExecutionDatetime: lastExecutionVersionsForNewJobs[jobId].LastExecutionDatetime,
			NextExecutionDatetime: lastExecutionVersionsForNewJobs[jobId].NextExecutionDatetime,
		})
	}
}

func (jobExecutor *jobExecutor) invokeJob(pendingJob models.Job) {
	jobExecutor.mtx.Lock()
	defer jobExecutor.mtx.Unlock()

	jobExecutor.pendingJobInvocations = append(jobExecutor.pendingJobInvocations, pendingJob)

	jobExecutor.debounce.Debounce(jobExecutor.context, 500, func() {
		jobExecutor.mtx.Lock()
		defer jobExecutor.mtx.Unlock()

		jobIDs := make([]uint64, 0)
		pendingJobs := jobExecutor.pendingJobInvocations

		for _, pendingJobInvocation := range pendingJobs {
			jobIDs = append(jobIDs, pendingJobInvocation.ID)
		}

		jobs, batchGetError := jobExecutor.jobRepo.BatchGetJobsByID(jobIDs)
		if batchGetError != nil {
			jobExecutor.logger.Error(fmt.Sprintf("batch query error:: %s", batchGetError.Message))
			return
		}

		getPendingJob := func(jobID uint64) *models.Job {
			for _, pendingJobInvocation := range pendingJobs {
				if pendingJobInvocation.ID == jobID {
					return &pendingJobInvocation
				}
			}
			return nil
		}

		jobExecutor.logger.Debug(fmt.Sprintf("batched queried %v", len(jobs)))

		jobsToExecute := make([]models.Job, 0)

		for _, job := range jobs {
			pendingJobInvocation := getPendingJob(job.ID)
			if pendingJobInvocation != nil {
				jobsToExecute = append(jobsToExecute, *pendingJobInvocation)
			}
		}

		jobsByType := make(map[string][]models.Job)

		for _, job := range jobsToExecute {
			jobsByType[job.ExecutionType] = append(jobsByType[job.ExecutionType], job)
		}

		for executionType, jobs := range jobsByType {
			switch executionType {
			case string(models.ExecutionTypeHTTP):
				jobExecutor.httpExecutionHandler.ExecuteHTTPJob(jobs, jobExecutor.handleSuccessJobs, jobExecutor.handleFailedJobs)
			default:
				jobExecutor.logger.Error(fmt.Sprintf("unrecognized execution %s", executionType))
			}
		}

		jobExecutor.pendingJobInvocations = []models.Job{}
	})
}

func (jobExecutor *jobExecutor) handleSuccessJobs(successfulJobs []models.Job) {
	configs := jobExecutor.scheduler0Config.GetConfigurations()
	lastVersion := jobExecutor.jobQueuesRepo.GetLastVersion()
	jobIds := make([]uint64, 0, len(successfulJobs))
	for _, successfulJob := range successfulJobs {
		jobIds = append(jobIds, successfulJob.ID)
	}
	lastExecutionVersions := make(map[uint64]uint64)

	jobExecutor.createInMemExecutionsForJobsIfNotExist(successfulJobs)

	jobExecutor.mtx.Lock()
	for _, successfulJob := range successfulJobs {
		cachedJobExecutionsLog, _ := jobExecutor.jobExecutionsCache.Load(successfulJob.ID)
		cachedJobExecutionLog := (cachedJobExecutionsLog).(models.MemJobExecution)

		lastExecutionVersions[successfulJob.ID] = cachedJobExecutionLog.ExecutionVersion
	}
	jobExecutor.mtx.Unlock()

	jobExecutor.jobExecutionsRepo.BatchInsert(successfulJobs, configs.NodeId, models.ExecutionLogSuccessState, lastVersion, lastExecutionVersions)
	if jobExecutor.singleNodeMode {
		jobExecutor.jobExecutionsRepo.LogJobExecutionStateInRaft(successfulJobs, models.ExecutionLogSuccessState, lastExecutionVersions, lastVersion, configs.NodeId)
	}
	jobExecutor.reschedule(successfulJobs, models.ExecutionLogSuccessState)
}

func (jobExecutor *jobExecutor) handleFailedJobs(erroredJobs []models.Job) {
	configs := jobExecutor.scheduler0Config.GetConfigurations()
	for _, erroredJob := range erroredJobs {
		jobExecutor.logger.Error(fmt.Sprintf("failed to execute job %v", erroredJob.ID))
	}
	lastVersion := jobExecutor.jobQueuesRepo.GetLastVersion()

	jobIds := make([]uint64, 0, len(erroredJobs))

	for _, erroredJob := range erroredJobs {
		jobIds = append(jobIds, erroredJob.ID)
	}
	lastExecutionVersions := map[uint64]uint64{}

	jobExecutor.createInMemExecutionsForJobsIfNotExist(erroredJobs)

	jobExecutor.mtx.Lock()
	for _, erroredJob := range erroredJobs {
		cachedJobExecutionsLog, _ := jobExecutor.jobExecutionsCache.Load(erroredJob.ID)
		cachedJobExecutionLog := (cachedJobExecutionsLog).(models.MemJobExecution)
		lastExecutionVersions[erroredJob.ID] = cachedJobExecutionLog.ExecutionVersion
	}
	jobExecutor.mtx.Unlock()

	jobExecutor.jobExecutionsRepo.BatchInsert(erroredJobs, configs.NodeId, models.ExecutionLogFailedState, lastVersion, lastExecutionVersions)
	if jobExecutor.singleNodeMode {
		jobExecutor.jobExecutionsRepo.LogJobExecutionStateInRaft(erroredJobs, models.ExecutionLogFailedState, lastExecutionVersions, lastVersion, configs.NodeId)
	}
	jobExecutor.reschedule(erroredJobs, models.ExecutionLogFailedState)
}
