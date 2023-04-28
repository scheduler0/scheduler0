package executor

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	"github.com/robfig/cron"
	"google.golang.org/protobuf/proto"
	"math"
	"scheduler0/config"
	"scheduler0/constants"
	"scheduler0/models"
	"scheduler0/protobuffs"
	"scheduler0/repository"
	"scheduler0/service/executor/executors"
	"scheduler0/utils"
	"scheduler0/utils/workers"
	"sync"
	"time"
)

type JobExecutor struct {
	Raft                  *raft.Raft
	SingleNodeMode        bool
	context               context.Context
	pendingJobInvocations []models.JobModel
	jobRepo               repository.Job
	jobExecutionsRepo     repository.ExecutionsRepo
	jobQueuesRepo         repository.JobQueuesRepo
	logger                hclog.Logger
	cancelReq             context.CancelFunc
	httpExecutionHandler  *executors.HTTPExecutionHandler
	mtx                   sync.Mutex
	executions            sync.Map
	debounce              *utils.Debounce
	dispatcher            *workers.Dispatcher
	scheduledJobs         sync.Map
}

func NewJobExecutor(ctx context.Context, logger hclog.Logger, jobRepository repository.Job, executionsRepo repository.ExecutionsRepo, jobQueuesRepo repository.JobQueuesRepo, dispatcher *workers.Dispatcher) *JobExecutor {
	reCtx, cancel := context.WithCancel(ctx)
	return &JobExecutor{
		pendingJobInvocations: []models.JobModel{},
		scheduledJobs:         sync.Map{},
		jobRepo:               jobRepository,
		jobExecutionsRepo:     executionsRepo,
		jobQueuesRepo:         jobQueuesRepo,
		logger:                logger.Named("job-executor-service"),
		context:               reCtx,
		cancelReq:             cancel,
		httpExecutionHandler:  executors.NewHTTTPExecutor(logger),
		executions:            sync.Map{},
		debounce:              utils.NewDebounce(),
		dispatcher:            dispatcher,
	}
}

func (jobExecutor *JobExecutor) QueueExecutions(jobQueueParams []interface{}) {
	configs := config.GetConfigurations()

	serverId := jobQueueParams[0].(uint64)
	lowerBound := jobQueueParams[1].(int64)
	upperBound := jobQueueParams[2].(int64)

	if serverId != configs.NodeId {
		return
	}

	jobExecutor.logger.Debug("Queueing jobs", "from", lowerBound, "to", upperBound)

	if upperBound-lowerBound > constants.JobMaxBatchSize {
		currentLowerBound := lowerBound
		currentUpperBound := lowerBound + constants.JobMaxBatchSize

		for currentLowerBound < upperBound {

			jobExecutor.logger.Debug("fetching batching", currentLowerBound, "between", currentUpperBound)
			jobs, getErr := jobExecutor.jobRepo.BatchGetJobsWithIDRange(uint64(currentLowerBound), uint64(currentUpperBound))

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
				currentUpperBound = int64(math.Min(
					float64(currentLowerBound+constants.JobMaxBatchSize),
					float64(upperBound),
				))
			}
		}
	} else {
		jobs, getErr := jobExecutor.jobRepo.BatchGetJobsWithIDRange(uint64(lowerBound), uint64(upperBound))
		if getErr != nil {
			jobExecutor.logger.Error("failed to batch get job by ranges ids ", "error", getErr)
		}
		jobExecutor.ScheduleJobs(jobs)
	}
}

func (jobExecutor *JobExecutor) ScheduleJobs(jobs []models.JobModel) {
	if len(jobs) < 1 {
		return
	}
	configs := config.GetConfigurations()

	jobIds := make([]uint64, 0, len(jobs))

	for _, job := range jobs {
		jobIds = append(jobIds, job.ID)
	}

	executionLogsMap := jobExecutor.jobExecutionsRepo.GetLastExecutionLogForJobIds(jobIds)
	sha := sha1.New()

	for i, job := range jobs {
		schedule, parseErr := cron.Parse(jobs[i].Spec)
		if parseErr != nil {
			jobExecutor.logger.Error(fmt.Sprintf("failed to parse job cron spec %s", parseErr.Error()))
			return
		}
		if _, ok := executionLogsMap[job.ID]; !ok {
			jobs[i].LastExecutionDate = job.DateCreated
			executionTime := schedule.Next(jobs[i].LastExecutionDate)
			uniqueId := fmt.Sprintf(
				"%v-%v-%v-%v",
				job.ProjectID,
				job.ID,
				job.LastExecutionDate.String(),
				executionTime.String(),
			)
			jobs[i].ExecutionId = fmt.Sprintf("%x", sha.Sum([]byte(uniqueId)))
			jobExecutor.ScheduleProcess(jobs[i], executionTime)

			jobExecutor.executions.Store(job.ID, models.MemJobExecution{
				ExecutionVersion:      1,
				FailCount:             0,
				LastState:             models.ExecutionLogScheduleState,
				LastExecutionDatetime: job.DateCreated,
				NextExecutionDatetime: executionTime,
			})
			continue
		}

		jobLastLog := executionLogsMap[job.ID]

		if jobLastLog.State == uint64(models.ExecutionLogScheduleState) {
			jobs[i].LastExecutionDate = jobLastLog.LastExecutionDatetime
			jobs[i].ExecutionId = jobLastLog.UniqueId

			schedulerTime := utils.GetSchedulerTime()
			now := schedulerTime.GetTime(time.Now())

			executionTime := schedule.Next(jobLastLog.LastExecutionDatetime)
			if now.Before(executionTime) {
				jobExecutor.ScheduleProcess(jobs[i], executionTime)
			} else {
				jobExecutor.ScheduleProcess(jobs[i], jobLastLog.NextExecutionDatetime)
			}

			jobExecutor.executions.Store(job.ID, models.MemJobExecution{
				ExecutionVersion:      jobLastLog.ExecutionVersion,
				FailCount:             0,
				LastState:             models.ExecutionLogScheduleState,
				LastExecutionDatetime: job.LastExecutionDate,
				NextExecutionDatetime: executionTime,
			})
		}

		if jobLastLog.State == uint64(models.ExecutionLogSuccessState) {
			jobs[i].LastExecutionDate = jobLastLog.NextExecutionDatetime
			executionTime := schedule.Next(jobLastLog.NextExecutionDatetime)
			uniqueId := fmt.Sprintf(
				"%v-%v-%v-%v",
				job.ProjectID,
				job.ID,
				jobLastLog.NextExecutionDatetime.String(),
				executionTime.String(),
			)
			jobs[i].ExecutionId = fmt.Sprintf("%x", sha.Sum([]byte(uniqueId)))
			jobExecutor.ScheduleProcess(jobs[i], executionTime)
			jobExecutor.executions.Store(job.ID, models.MemJobExecution{
				ExecutionVersion:      jobLastLog.ExecutionVersion,
				FailCount:             0,
				LastState:             models.ExecutionLogSuccessState,
				LastExecutionDatetime: jobLastLog.NextExecutionDatetime,
				NextExecutionDatetime: executionTime,
			})
		}

		if jobLastLog.State == uint64(models.ExecutionLogFailedState) {
			failCounts := jobExecutor.jobExecutionsRepo.CountLastFailedExecutionLogs(job.ID, configs.NodeId, jobLastLog.ExecutionVersion)
			if failCounts < uint64(configs.JobExecutionRetryMax) {
				jobs[i].LastExecutionDate = jobLastLog.LastExecutionDatetime
				jobs[i].ExecutionId = jobLastLog.UniqueId
				jobExecutor.executions.Store(job.ID, models.MemJobExecution{
					ExecutionVersion:      jobLastLog.ExecutionVersion,
					FailCount:             failCounts,
					LastState:             models.ExecutionLogFailedState,
					LastExecutionDatetime: jobLastLog.LastExecutionDatetime,
					NextExecutionDatetime: jobLastLog.NextExecutionDatetime,
				})
				schedulerTime := utils.GetSchedulerTime()
				now := schedulerTime.GetTime(time.Now())

				if now.Before(jobLastLog.NextExecutionDatetime) {
					jobExecutor.ScheduleProcess(jobs[i], jobLastLog.NextExecutionDatetime)
					continue
				}
			}

			jobs[i].LastExecutionDate = jobLastLog.NextExecutionDatetime
			executionTime := schedule.Next(jobLastLog.NextExecutionDatetime)
			uniqueId := fmt.Sprintf(
				"%v-%v-%v-%v",
				job.ProjectID,
				job.ID,
				jobLastLog.LastExecutionDatetime.String(),
				executionTime.String(),
			)
			jobs[i].ExecutionId = fmt.Sprintf("%x", sha.Sum([]byte(uniqueId)))
			jobExecutor.ScheduleProcess(jobs[i], executionTime)
			jobExecutor.executions.Store(job.ID, models.MemJobExecution{
				ExecutionVersion:      jobLastLog.ExecutionVersion,
				FailCount:             0,
				LastState:             models.ExecutionLogFailedState,
				LastExecutionDatetime: jobLastLog.NextExecutionDatetime,
				NextExecutionDatetime: executionTime,
			})
		}
	}

	lastVersion := jobExecutor.jobQueuesRepo.GetLastVersion()
	lastExecutionVersions := make(map[uint64]uint64)

	for _, job := range jobs {
		if _, ok := lastExecutionVersions[job.ID]; !ok {
			if _, ok := executionLogsMap[job.ID]; ok {
				cachedJobExecutionsLog, _ := jobExecutor.executions.Load(job.ID)
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

	if jobExecutor.SingleNodeMode {
		jobExecutor.logJobExecutionStateInRaft(jobs, models.ExecutionLogScheduleState, lastExecutionVersions)
	}

	jobExecutor.logger.Debug("scheduled jobs", "from", jobs[0].ID, "to", jobs[len(jobs)-1].ID)
}

func (jobExecutor *JobExecutor) StopAll() {
	jobExecutor.mtx.Lock()
	defer jobExecutor.mtx.Unlock()

	jobExecutor.logger.Info("stopped all scheduled job")
	jobExecutor.cancelReq()
	ctx, cancel := context.WithCancel(context.Background())
	jobExecutor.cancelReq = cancel
	jobExecutor.context = ctx
	jobExecutor.ListenForJobsToInvoke()
}

func (jobExecutor *JobExecutor) ScheduleProcess(job models.JobModel, executeTime time.Time) {
	schedulerTime := utils.GetSchedulerTime()
	now := schedulerTime.GetTime(time.Now())
	execTime := now.Add(executeTime.Sub(job.LastExecutionDate))
	jobExecutor.scheduledJobs.Store(job.ID, models.JobSchedule{
		Job:           job,
		ExecutionTime: execTime,
	})
}

func (jobExecutor *JobExecutor) ListenForJobsToInvoke() {
	ticker := time.NewTicker(time.Duration(1) * time.Second)
	schedulerTime := utils.GetSchedulerTime()

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

func (jobExecutor *JobExecutor) GetUncommittedLogs() []models.JobExecutionLog {
	jobExecutor.mtx.Lock()
	defer jobExecutor.mtx.Unlock()

	configs := config.GetConfigurations()
	executionLogs := jobExecutor.jobExecutionsRepo.GetUncommittedExecutionsLogForNode(configs.NodeId)

	return executionLogs
}

func (jobExecutor *JobExecutor) ExecuteHTTP(jobs []models.JobModel, ctx context.Context, onSuccess func(pj []models.JobModel), onFailure func(pj []models.JobModel)) {
	jobExecutor.httpExecutionHandler.ExecuteHTTPJob(ctx, jobExecutor.dispatcher, jobs, onSuccess, onFailure)
}

func (jobExecutor *JobExecutor) reschedule(jobs []models.JobModel, newState models.JobExecutionLogState) {
	jobExecutor.mtx.Lock()
	defer jobExecutor.mtx.Unlock()

	configs := config.GetConfigurations()

	jobsToReschedule := make([]models.JobModel, 0, len(jobs))

	for i, job := range jobs {

		cachedJobExecutionsLog, _ := jobExecutor.executions.Load(job.ID)
		lastExecution := (cachedJobExecutionsLog).(models.MemJobExecution)

		failCounts := lastExecution.FailCount
		executionVersion := lastExecution.ExecutionVersion

		if newState == models.ExecutionLogFailedState &&
			failCounts < configs.JobExecutionRetryMax {
			failCounts += 1
			jobExecutor.executions.Store(job.ID, models.MemJobExecution{
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

		sha := sha1.New()
		schedule, parseErr := cron.Parse(job.Spec)
		if parseErr != nil {
			jobExecutor.logger.Error(fmt.Sprintf("failed to parse job cron spec %s", parseErr.Error()))
			return
		}
		executionTime := schedule.Next(lastExecution.NextExecutionDatetime)
		uniqueId := fmt.Sprintf(
			"%v-%v-%v-%v",
			job.ProjectID,
			job.ID,
			lastExecution.NextExecutionDatetime.String(),
			executionTime.String(),
		)
		jobs[i].LastExecutionDate = lastExecution.NextExecutionDatetime
		jobs[i].ExecutionId = fmt.Sprintf("%x", sha.Sum([]byte(uniqueId)))
		jobExecutor.ScheduleProcess(jobs[i], executionTime)
		jobExecutor.executions.Store(job.ID, models.MemJobExecution{
			ExecutionVersion:      executionVersion,
			FailCount:             0,
			LastState:             newState,
			LastExecutionDatetime: lastExecution.NextExecutionDatetime,
			NextExecutionDatetime: executionTime,
		})
		jobsToReschedule = append(jobsToReschedule, jobs[i])
	}

	lastVersion := jobExecutor.jobQueuesRepo.GetLastVersion()
	lastExecutionVersions := make(map[uint64]uint64)

	for _, job := range jobsToReschedule {
		if _, ok := lastExecutionVersions[job.ID]; !ok {
			cachedJobExecutionsLog, _ := jobExecutor.executions.Load(job.ID)
			lastExecution := (cachedJobExecutionsLog).(models.MemJobExecution)

			lastExecutionVersions[job.ID] = lastExecution.ExecutionVersion
		}
	}

	jobExecutor.jobExecutionsRepo.BatchInsert(jobsToReschedule, configs.NodeId, models.ExecutionLogScheduleState, lastVersion, lastExecutionVersions)
	if jobExecutor.SingleNodeMode {
		jobExecutor.logJobExecutionStateInRaft(jobsToReschedule, models.ExecutionLogScheduleState, lastExecutionVersions)
	}
}

func (jobExecutor *JobExecutor) createInMemExecutionsForJobsIfNotExist(jobs []models.JobModel) {
	jobExecutor.mtx.Lock()
	defer jobExecutor.mtx.Unlock()

	jobsNotInExecution := make([]uint64, 0, len(jobs))

	for _, job := range jobs {
		_, ok := jobExecutor.executions.Load(job.ID)
		if !ok {
			jobsNotInExecution = append(jobsNotInExecution, job.ID)
		}
	}

	lastExecutionVersionsForNewJobs := jobExecutor.jobExecutionsRepo.GetLastExecutionLogForJobIds(jobsNotInExecution)

	configs := config.GetConfigurations()

	for _, jobId := range jobsNotInExecution {
		failCounts := 0
		if lastExecutionVersionsForNewJobs[jobId].State == models.ExecutionLogFailedState {
			failCounts = int(jobExecutor.jobExecutionsRepo.CountLastFailedExecutionLogs(jobId, configs.NodeId, lastExecutionVersionsForNewJobs[jobId].ExecutionVersion))
		}

		jobExecutor.executions.Store(jobId, models.MemJobExecution{
			ExecutionVersion:      lastExecutionVersionsForNewJobs[jobId].ExecutionVersion,
			FailCount:             uint64(failCounts),
			LastState:             models.JobExecutionLogState(lastExecutionVersionsForNewJobs[jobId].State),
			LastExecutionDatetime: lastExecutionVersionsForNewJobs[jobId].LastExecutionDatetime,
			NextExecutionDatetime: lastExecutionVersionsForNewJobs[jobId].NextExecutionDatetime,
		})
	}
}

func (jobExecutor *JobExecutor) invokeJob(pendingJob models.JobModel) {
	jobExecutor.mtx.Lock()
	defer jobExecutor.mtx.Unlock()

	configs := config.GetConfigurations()

	jobExecutor.pendingJobInvocations = append(jobExecutor.pendingJobInvocations, pendingJob)

	jobExecutor.debounce.Debounce(jobExecutor.context, configs.JobInvocationDebounceDelay, func() {
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

		getPendingJob := func(jobID uint64) *models.JobModel {
			for _, pendingJobInvocation := range pendingJobs {
				if pendingJobInvocation.ID == jobID {
					return &pendingJobInvocation
				}
			}
			return nil
		}

		jobExecutor.logger.Debug(fmt.Sprintf("batched queried %v", len(jobs)))

		jobsToExecute := make([]models.JobModel, 0)

		for _, job := range jobs {
			pendingJobInvocation := getPendingJob(job.ID)
			if pendingJobInvocation != nil {
				jobsToExecute = append(jobsToExecute, *pendingJobInvocation)
			}
		}

		jobsByType := make(map[string][]models.JobModel)

		for _, job := range jobsToExecute {
			jobsByType[job.ExecutionType] = append(jobsByType[job.ExecutionType], job)
		}

		for executionType, jobs := range jobsByType {
			switch executionType {
			case string(models.ExecutionTypeHTTP):
				jobExecutor.ExecuteHTTP(
					jobs,
					jobExecutor.context,
					jobExecutor.handleSuccessJobs,
					jobExecutor.handleFailedJobs,
				)
			default:
				jobExecutor.logger.Error(fmt.Sprintf("unrecognized execution %s", executionType))
			}
		}

		jobExecutor.pendingJobInvocations = []models.JobModel{}
	})
}

func (jobExecutor *JobExecutor) handleSuccessJobs(successfulJobs []models.JobModel) {
	configs := config.GetConfigurations()
	lastVersion := jobExecutor.jobQueuesRepo.GetLastVersion()
	jobIds := make([]uint64, 0, len(successfulJobs))
	for _, successfulJob := range successfulJobs {
		jobIds = append(jobIds, successfulJob.ID)
	}
	lastExecutionVersions := make(map[uint64]uint64)

	jobExecutor.createInMemExecutionsForJobsIfNotExist(successfulJobs)

	jobExecutor.mtx.Lock()
	for _, successfulJob := range successfulJobs {
		cachedJobExecutionsLog, _ := jobExecutor.executions.Load(successfulJob.ID)
		cachedJobExecutionLog := (cachedJobExecutionsLog).(models.MemJobExecution)

		lastExecutionVersions[successfulJob.ID] = cachedJobExecutionLog.ExecutionVersion
	}
	jobExecutor.mtx.Unlock()

	jobExecutor.jobExecutionsRepo.BatchInsert(successfulJobs, uint64(configs.NodeId), models.ExecutionLogSuccessState, lastVersion, lastExecutionVersions)
	if jobExecutor.SingleNodeMode {
		jobExecutor.logJobExecutionStateInRaft(successfulJobs, models.ExecutionLogSuccessState, lastExecutionVersions)
	}
	jobExecutor.reschedule(successfulJobs, models.ExecutionLogSuccessState)
}

func (jobExecutor *JobExecutor) handleFailedJobs(erroredJobs []models.JobModel) {
	configs := config.GetConfigurations()
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
		cachedJobExecutionsLog, _ := jobExecutor.executions.Load(erroredJob.ID)
		cachedJobExecutionLog := (cachedJobExecutionsLog).(models.MemJobExecution)

		lastExecutionVersions[erroredJob.ID] = cachedJobExecutionLog.ExecutionVersion
	}
	jobExecutor.mtx.Unlock()

	jobExecutor.jobExecutionsRepo.BatchInsert(erroredJobs, configs.NodeId, models.ExecutionLogFailedState, lastVersion, lastExecutionVersions)
	if jobExecutor.SingleNodeMode {
		jobExecutor.logJobExecutionStateInRaft(erroredJobs, models.ExecutionLogFailedState, lastExecutionVersions)
	}
	jobExecutor.reschedule(erroredJobs, models.ExecutionLogFailedState)
}

func (jobExecutor *JobExecutor) logJobExecutionStateInRaft(jobs []models.JobModel, state models.JobExecutionLogState, executionVersions map[uint64]uint64) {
	configs := config.GetConfigurations()
	lastVersion := jobExecutor.jobQueuesRepo.GetLastVersion()

	executionLogs := make([]models.JobExecutionLog, 0, len(jobs))

	for _, job := range jobs {
		schedule, parseErr := cron.Parse(job.Spec)
		if parseErr != nil {
			jobExecutor.logger.Error(fmt.Sprintf("failed to parse job cron spec %s", parseErr.Error()))
			return
		}
		executionTime := schedule.Next(job.LastExecutionDate)
		schedulerTime := utils.GetSchedulerTime()
		now := schedulerTime.GetTime(time.Now())
		executionLogs = append(executionLogs, models.JobExecutionLog{
			JobId:                 job.ID,
			UniqueId:              job.ExecutionId,
			State:                 uint64(state),
			NodeId:                configs.NodeId,
			LastExecutionDatetime: job.LastExecutionDate,
			NextExecutionDatetime: executionTime,
			JobQueueVersion:       lastVersion,
			DataCreated:           now,
			ExecutionVersion:      executionVersions[job.ID],
		})
	}

	peerAddress := utils.GetServerHTTPAddress()
	params := models.LocalData{
		ExecutionLogs: executionLogs,
	}

	data, err := json.Marshal(params)
	if err != nil {
		jobExecutor.logger.Error("failed to marshal json")
		return
	}

	createCommand := &protobuffs.Command{
		Type:       protobuffs.Command_Type(constants.CommandTypeLocalData),
		Sql:        peerAddress,
		Data:       data,
		TargetNode: configs.NodeId,
	}

	createCommandData, err := proto.Marshal(createCommand)
	if err != nil {
		jobExecutor.logger.Error("failed to marshal json")
		return
	}

	_ = jobExecutor.Raft.Apply(createCommandData, time.Second*time.Duration(configs.RaftApplyTimeout)).(raft.ApplyFuture)
}
