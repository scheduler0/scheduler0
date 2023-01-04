package job_executor

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"github.com/hashicorp/raft"
	"github.com/robfig/cron"
	"google.golang.org/protobuf/proto"
	"log"
	"math"
	"scheduler0/config"
	"scheduler0/constants"
	"scheduler0/models"
	"scheduler0/protobuffs"
	"scheduler0/repository"
	"scheduler0/service/job_executor/executors"
	"scheduler0/utils"
	"sync"
	"time"
)

type JobExecutor struct {
	context                  context.Context
	Raft                     *raft.Raft
	pendingJobInvocations    []models.JobModel
	jobRepo                  repository.Job
	jobExecutionsRepo        repository.ExecutionsRepo
	jobQueuesRepo            repository.JobQueuesRepo
	logger                   *log.Logger
	cancelReq                context.CancelFunc
	executor                 *executors.Service
	threshold                time.Time
	ticker                   *time.Ticker
	mtx                      sync.Mutex
	invocationLock           sync.Mutex
	once                     sync.Once
	SingleNodeMode           bool
	pendingJobInvocationLock sync.Mutex
	executions               map[int64]models.MemJobExecution
}

func NewJobExecutor(logger *log.Logger, jobRepository repository.Job, executionsRepo repository.ExecutionsRepo, jobQueuesRepo repository.JobQueuesRepo) *JobExecutor {
	ctx, cancel := context.WithCancel(context.Background())
	return &JobExecutor{
		pendingJobInvocations: []models.JobModel{},
		jobRepo:               jobRepository,
		jobExecutionsRepo:     executionsRepo,
		jobQueuesRepo:         jobQueuesRepo,
		logger:                logger,
		context:               ctx,
		cancelReq:             cancel,
		executor:              executors.NewService(logger),
		executions:            map[int64]models.MemJobExecution{},
	}
}

func (jobExecutor *JobExecutor) QueueExecutions(jobQueueParams []interface{}) {
	configs := config.GetConfigurations(jobExecutor.logger)

	serverAddress := jobQueueParams[0].(string)
	lowerBound := jobQueueParams[1].(int64)
	upperBound := jobQueueParams[2].(int64)

	if serverAddress != configs.RaftAddress {
		return
	}

	if upperBound-lowerBound > constants.JobMaxBatchSize {
		currentLowerBound := lowerBound
		currentUpperBound := lowerBound + constants.JobMaxBatchSize

		for currentLowerBound < upperBound {

			jobExecutor.logger.Println("fetching batching", currentLowerBound, "between", currentUpperBound)
			jobs, getErr := jobExecutor.jobRepo.BatchGetJobsWithIDRange(currentLowerBound, currentUpperBound)

			if getErr != nil {
				jobExecutor.logger.Fatalln("failed to batch get job by ranges ids ", getErr)
			}

			jobExecutor.Schedule(jobs)

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
		jobs, getErr := jobExecutor.jobRepo.BatchGetJobsWithIDRange(lowerBound, upperBound)
		if getErr != nil {
			jobExecutor.logger.Fatalln("failed to batch get job by ranges ids ", getErr)
		}
		jobExecutor.Schedule(jobs)
	}
}

func (jobExecutor *JobExecutor) Schedule(jobs []models.JobModel) {
	if len(jobs) < 1 {
		return
	}
	configs := config.GetConfigurations(jobExecutor.logger)

	jobIds := []int64{}

	for _, job := range jobs {
		jobIds = append(jobIds, job.ID)
	}

	executionLogsMap := jobExecutor.jobExecutionsRepo.GetLastExecutionLogForJobIds(jobIds)
	sha := sha1.New()

	fmt.Println("executionLogsMap", executionLogsMap)

	for i, job := range jobs {
		schedule, parseErr := cron.Parse(jobs[i].Spec)
		if parseErr != nil {
			jobExecutor.logger.Fatalln(fmt.Sprintf("failed to parse job cron spec %s", parseErr.Error()))
		}
		if _, ok := executionLogsMap[job.ID]; !ok {
			fmt.Println("aa", job.DateCreated)
			fmt.Println("bb", jobs[i].LastExecutionDate)
			jobs[i].LastExecutionDate = job.DateCreated
			fmt.Println("cc", jobs[i].LastExecutionDate)
			executionTime := schedule.Next(jobs[i].LastExecutionDate)
			uniqueId := fmt.Sprintf(
				"%v-%v-%v-%v",
				job.ProjectID,
				job.ID,
				job.LastExecutionDate.String(),
				executionTime.String(),
			)
			jobs[i].ExecutionId = fmt.Sprintf("%x", sha.Sum([]byte(uniqueId)))
			jobExecutor.AddNewProcess(jobs[i], executionTime)

			jobExecutor.executions[job.ID] = models.MemJobExecution{
				ExecutionVersion:      1,
				FailCount:             0,
				LastState:             models.ExecutionLogScheduleState,
				LastExecutionDatetime: job.DateCreated,
				NextExecutionDatetime: executionTime,
			}
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
				jobExecutor.AddNewProcess(jobs[i], executionTime)
			} else {
				jobExecutor.AddNewProcess(jobs[i], jobLastLog.NextExecutionDatetime)
			}

			jobExecutor.executions[job.ID] = models.MemJobExecution{
				ExecutionVersion:      jobLastLog.ExecutionVersion,
				FailCount:             0,
				LastState:             models.ExecutionLogScheduleState,
				LastExecutionDatetime: job.LastExecutionDate,
				NextExecutionDatetime: executionTime,
			}
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
			jobExecutor.AddNewProcess(jobs[i], executionTime)
			jobExecutor.executions[job.ID] = models.MemJobExecution{
				ExecutionVersion:      jobLastLog.ExecutionVersion,
				FailCount:             0,
				LastState:             models.ExecutionLogSuccessState,
				LastExecutionDatetime: jobLastLog.NextExecutionDatetime,
				NextExecutionDatetime: executionTime,
			}
		}

		if jobLastLog.State == uint64(models.ExecutionLogFailedState) {
			failCounts := jobExecutor.jobExecutionsRepo.CountLastFailedExecutionLogs(job.ID, configs.NodeId, jobLastLog.ExecutionVersion)
			fmt.Println("failCounts", failCounts, job.ID, configs.NodeId, jobLastLog.ExecutionVersion)
			if failCounts < uint64(configs.JobExecutionRetryMax) {
				jobs[i].LastExecutionDate = jobLastLog.LastExecutionDatetime
				jobs[i].ExecutionId = jobLastLog.UniqueId
				jobExecutor.executions[job.ID] = models.MemJobExecution{
					ExecutionVersion:      jobLastLog.ExecutionVersion,
					FailCount:             failCounts,
					LastState:             models.ExecutionLogFailedState,
					LastExecutionDatetime: jobLastLog.LastExecutionDatetime,
					NextExecutionDatetime: jobLastLog.NextExecutionDatetime,
				}
				schedulerTime := utils.GetSchedulerTime()
				now := schedulerTime.GetTime(time.Now())

				if now.Before(jobLastLog.NextExecutionDatetime) {
					fmt.Println("hereh a")
					jobExecutor.AddNewProcess(jobs[i], jobLastLog.NextExecutionDatetime)
					continue
				}
				fmt.Println("hereh 1")
			}

			fmt.Println("hereh 2")
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
			jobExecutor.AddNewProcess(jobs[i], executionTime)
			jobExecutor.executions[job.ID] = models.MemJobExecution{
				ExecutionVersion:      jobLastLog.ExecutionVersion,
				FailCount:             0,
				LastState:             models.ExecutionLogFailedState,
				LastExecutionDatetime: jobLastLog.NextExecutionDatetime,
				NextExecutionDatetime: executionTime,
			}
		}
	}

	lastVersion := jobExecutor.jobQueuesRepo.GetLastVersion()
	lastExecutionVersions := map[int64]uint64{}

	for _, job := range jobs {
		if _, ok := lastExecutionVersions[job.ID]; !ok {
			if _, ok := executionLogsMap[job.ID]; ok {
				if executionLogsMap[job.ID].State == models.ExecutionLogSuccessState ||
					(jobExecutor.executions[job.ID].FailCount == 0 &&
						jobExecutor.executions[job.ID].LastState == models.ExecutionLogFailedState) {
					lastExecutionVersions[job.ID] = executionLogsMap[job.ID].ExecutionVersion + 1
				} else {
					lastExecutionVersions[job.ID] = executionLogsMap[job.ID].ExecutionVersion
				}
			} else {
				lastExecutionVersions[job.ID] = 1
			}
		}
	}

	fmt.Println("jobs", jobs)
	jobExecutor.jobExecutionsRepo.BatchInsert(jobs, uint64(configs.NodeId), models.ExecutionLogScheduleState, lastVersion, lastExecutionVersions)

	if jobExecutor.SingleNodeMode {
		jobExecutor.logJobExecutionStateInRaft(jobs, models.ExecutionLogScheduleState, lastExecutionVersions)
	}

	//jobExecutor.logger.Println("scheduled job", jobs[0].ID, "to", jobs[len(jobs)-1].ID)
}

func (jobExecutor *JobExecutor) StopAll() {
	jobExecutor.mtx.Lock()
	defer jobExecutor.mtx.Unlock()

	jobExecutor.logger.Println("stopped all scheduled job")
	jobExecutor.cancelReq()
	ctx, cancel := context.WithCancel(context.Background())
	jobExecutor.cancelReq = cancel
	jobExecutor.context = ctx
}

func (jobExecutor *JobExecutor) AddNewProcess(job models.JobModel, executeTime time.Time) {
	go func(j models.JobModel, e time.Time) {
		schedulerTime := utils.GetSchedulerTime()
		now := schedulerTime.GetTime(time.Now())

		execTime := now.Add(e.Sub(j.LastExecutionDate))

		fmt.Println(now, j.LastExecutionDate, execTime)
		ticker := time.NewTicker(time.Duration(1) * time.Second)
		for {
			select {
			case <-ticker.C:
				currentTime := schedulerTime.GetTime(time.Now())
				if currentTime.After(execTime) {
					fmt.Println("called invoke")
					jobExecutor.invokeJob(j)
					return
				}
			case <-jobExecutor.context.Done():
				jobExecutor.logger.Println("stopping scheduled job with id", j.ID)
				return
			}
		}
	}(job, executeTime)
}

func (jobExecutor *JobExecutor) GetUncommittedLogs() []byte {
	jobExecutor.mtx.Lock()
	defer jobExecutor.mtx.Unlock()

	configs := config.GetConfigurations(jobExecutor.logger)

	executionLogs := jobExecutor.jobExecutionsRepo.GetUncommittedExecutionsLogForNode(configs.NodeId)

	logBytes, err := json.Marshal(executionLogs)
	if err != nil {
		jobExecutor.logger.Println("failed to log state to bytes", err)
		return nil
	}

	compressedByte, err := utils.GzCompress(logBytes)
	if err != nil {
		jobExecutor.logger.Println("failed to compress log file error", err)
		return nil
	}

	return compressedByte
}

func (jobExecutor *JobExecutor) reschedule(jobs []models.JobModel, newState models.JobExecutionLogState) {
	jobExecutor.mtx.Lock()
	defer jobExecutor.mtx.Unlock()

	configs := config.GetConfigurations(jobExecutor.logger)

	jobsToReschedule := []models.JobModel{}

	for i, job := range jobs {
		lastExecution := jobExecutor.executions[job.ID]

		failCounts := lastExecution.FailCount
		executionVersion := lastExecution.ExecutionVersion

		if newState == models.ExecutionLogFailedState &&
			failCounts < uint64(configs.JobExecutionRetryMax) {
			fmt.Println("failCounts", failCounts, configs.JobExecutionRetryMax)

			failCounts += 1
			jobExecutor.executions[job.ID] = models.MemJobExecution{
				ExecutionVersion:      executionVersion,
				FailCount:             failCounts,
				LastState:             newState,
				LastExecutionDatetime: lastExecution.LastExecutionDatetime,
				NextExecutionDatetime: lastExecution.NextExecutionDatetime,
			}
			continue
		} else {
			executionVersion += 1
		}

		sha := sha1.New()
		schedule, parseErr := cron.Parse(job.Spec)
		if parseErr != nil {
			jobExecutor.logger.Fatalln(fmt.Sprintf("failed to parse job cron spec %s", parseErr.Error()))
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
		jobExecutor.AddNewProcess(jobs[i], executionTime)
		jobExecutor.executions[job.ID] = models.MemJobExecution{
			ExecutionVersion:      executionVersion,
			FailCount:             0,
			LastState:             newState,
			LastExecutionDatetime: lastExecution.NextExecutionDatetime,
			NextExecutionDatetime: executionTime,
		}
		jobsToReschedule = append(jobsToReschedule, jobs[i])
	}

	lastVersion := jobExecutor.jobQueuesRepo.GetLastVersion()
	lastExecutionVersions := map[int64]uint64{}

	for _, job := range jobsToReschedule {
		if _, ok := lastExecutionVersions[job.ID]; !ok {
			lastExecutionVersions[job.ID] = jobExecutor.executions[job.ID].ExecutionVersion
		}
	}

	fmt.Println("jobsToReschedule", jobsToReschedule)
	jobExecutor.jobExecutionsRepo.BatchInsert(jobsToReschedule, uint64(configs.NodeId), models.ExecutionLogScheduleState, lastVersion, lastExecutionVersions)
	if jobExecutor.SingleNodeMode {
		jobExecutor.logJobExecutionStateInRaft(jobsToReschedule, models.ExecutionLogScheduleState, lastExecutionVersions)
	}
}

func (jobExecutor *JobExecutor) createInMemExecutionsForJobsIfNotExist(jobs []models.JobModel) {
	jobExecutor.mtx.Lock()
	defer jobExecutor.mtx.Unlock()

	jobsNotInExecution := []int64{}

	for _, job := range jobs {
		if _, ok := jobExecutor.executions[job.ID]; !ok {
			jobsNotInExecution = append(jobsNotInExecution, job.ID)
		}
	}

	lastExecutionVersionsForNewJobs := jobExecutor.jobExecutionsRepo.GetLastExecutionLogForJobIds(jobsNotInExecution)

	configs := config.GetConfigurations(jobExecutor.logger)

	for _, jobId := range jobsNotInExecution {
		failCounts := 0
		if lastExecutionVersionsForNewJobs[jobId].State == models.ExecutionLogFailedState {
			failCounts = int(jobExecutor.jobExecutionsRepo.CountLastFailedExecutionLogs(jobId, configs.NodeId, lastExecutionVersionsForNewJobs[jobId].ExecutionVersion))
		}

		jobExecutor.executions[jobId] = models.MemJobExecution{
			ExecutionVersion:      lastExecutionVersionsForNewJobs[jobId].ExecutionVersion,
			FailCount:             uint64(failCounts),
			LastState:             models.JobExecutionLogState(lastExecutionVersionsForNewJobs[jobId].State),
			LastExecutionDatetime: lastExecutionVersionsForNewJobs[jobId].LastExecutionDatetime,
			NextExecutionDatetime: lastExecutionVersionsForNewJobs[jobId].NextExecutionDatetime,
		}
	}
}

func (jobExecutor *JobExecutor) invokeJob(pendingJob models.JobModel) {
	configs := config.GetConfigurations(jobExecutor.logger)

	fmt.Println("invokeJob")

	jobExecutor.threshold = time.Now().Add(time.Duration(configs.JobInvocationDebounceDelay) * time.Millisecond)

	jobExecutor.pendingJobInvocationLock.Lock()
	jobExecutor.pendingJobInvocations = append(jobExecutor.pendingJobInvocations, pendingJob)
	jobExecutor.pendingJobInvocationLock.Unlock()

	jobExecutor.once.Do(func() {
		go func() {
			defer func() {
				jobExecutor.invocationLock.Lock()
				jobExecutor.ticker.Stop()
				jobExecutor.once = sync.Once{}
				jobExecutor.invocationLock.Unlock()
			}()

			jobExecutor.ticker = time.NewTicker(time.Duration(500) * time.Millisecond)

			for {
				select {
				case <-jobExecutor.ticker.C:
					if time.Now().After(jobExecutor.threshold) {
						jobExecutor.pendingJobInvocationLock.Lock()
						jobExecutor.invocationLock.Lock()

						jobIDs := make([]int64, 0)
						pendingJobs := jobExecutor.pendingJobInvocations

						for _, pendingJobInvocation := range pendingJobs {
							jobIDs = append(jobIDs, pendingJobInvocation.ID)
						}

						jobs, batchGetError := jobExecutor.jobRepo.BatchGetJobsByID(jobIDs)
						if batchGetError != nil {
							jobExecutor.logger.Println(fmt.Sprintf("batch query error:: %s", batchGetError.Message))
							return
						}

						getPendingJob := func(jobID int64) *models.JobModel {
							for _, pendingJobInvocation := range pendingJobs {
								if pendingJobInvocation.ID == jobID {
									return &pendingJobInvocation
								}
							}
							return nil
						}

						//jobExecutor.logger.Println(fmt.Sprintf("batched queried %v", len(jobs)))

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
								jobExecutor.executor.ExecuteHTTP(
									jobs,
									jobExecutor.context,
									jobExecutor.handleSuccessJobs,
									jobExecutor.handleFailedJobs,
								)
							default:
								jobExecutor.logger.Println(fmt.Sprintf("unrecognized execution %s", executionType))
							}
						}

						jobExecutor.pendingJobInvocations = []models.JobModel{}
						jobExecutor.invocationLock.Unlock()
						jobExecutor.pendingJobInvocationLock.Unlock()
						return
					}
				case <-jobExecutor.context.Done():
					return
				}
			}
		}()
	})
}

func (jobExecutor *JobExecutor) handleSuccessJobs(successfulJobs []models.JobModel) {
	configs := config.GetConfigurations(jobExecutor.logger)
	lastVersion := jobExecutor.jobQueuesRepo.GetLastVersion()
	jobIds := []int64{}
	for _, successfulJob := range successfulJobs {
		jobIds = append(jobIds, successfulJob.ID)
	}
	lastExecutionVersions := map[int64]uint64{}

	jobExecutor.createInMemExecutionsForJobsIfNotExist(successfulJobs)

	jobExecutor.mtx.Lock()
	for _, successfulJob := range successfulJobs {
		lastExecutionVersions[successfulJob.ID] = jobExecutor.executions[successfulJob.ID].ExecutionVersion
	}
	jobExecutor.mtx.Unlock()

	jobExecutor.jobExecutionsRepo.BatchInsert(successfulJobs, uint64(configs.NodeId), models.ExecutionLogSuccessState, lastVersion, lastExecutionVersions)
	if jobExecutor.SingleNodeMode {
		jobExecutor.logJobExecutionStateInRaft(successfulJobs, models.ExecutionLogSuccessState, lastExecutionVersions)
	}
	jobExecutor.reschedule(successfulJobs, models.ExecutionLogSuccessState)
}

func (jobExecutor *JobExecutor) handleFailedJobs(erroredJobs []models.JobModel) {
	configs := config.GetConfigurations(jobExecutor.logger)
	//for _, erroredJob := range erroredJobs {
	//	jobExecutor.logger.Println(fmt.Sprintf("failed to execute job %v", erroredJob.ID))
	//}
	lastVersion := jobExecutor.jobQueuesRepo.GetLastVersion()
	jobIds := []int64{}
	for _, erroredJob := range erroredJobs {
		jobIds = append(jobIds, erroredJob.ID)
	}
	lastExecutionVersions := map[int64]uint64{}

	jobExecutor.createInMemExecutionsForJobsIfNotExist(erroredJobs)

	jobExecutor.mtx.Lock()
	for _, erroredJob := range erroredJobs {
		lastExecutionVersions[erroredJob.ID] = jobExecutor.executions[erroredJob.ID].ExecutionVersion
	}
	jobExecutor.mtx.Unlock()

	jobExecutor.jobExecutionsRepo.BatchInsert(erroredJobs, uint64(configs.NodeId), models.ExecutionLogFailedState, lastVersion, lastExecutionVersions)
	if jobExecutor.SingleNodeMode {
		jobExecutor.logJobExecutionStateInRaft(erroredJobs, models.ExecutionLogFailedState, lastExecutionVersions)
	}
	jobExecutor.reschedule(erroredJobs, models.ExecutionLogFailedState)
}

func (jobExecutor *JobExecutor) logJobExecutionStateInRaft(jobs []models.JobModel, state models.JobExecutionLogState, executionVersions map[int64]uint64) {
	configs := config.GetConfigurations(jobExecutor.logger)
	lastVersion := jobExecutor.jobQueuesRepo.GetLastVersion()

	executionLogs := []models.JobExecutionLog{}

	for _, job := range jobs {
		schedule, parseErr := cron.Parse(job.Spec)
		if parseErr != nil {
			jobExecutor.logger.Fatalln(fmt.Sprintf("failed to parse job cron spec %s", parseErr.Error()))
		}
		executionTime := schedule.Next(job.LastExecutionDate)
		schedulerTime := utils.GetSchedulerTime()
		now := schedulerTime.GetTime(time.Now())
		executionLogs = append(executionLogs, models.JobExecutionLog{
			JobId:                 job.ID,
			UniqueId:              job.ExecutionId,
			State:                 uint64(state),
			NodeId:                uint64(configs.NodeId),
			LastExecutionDatetime: job.LastExecutionDate,
			NextExecutionDatetime: executionTime,
			JobQueueVersion:       lastVersion,
			DataCreated:           now,
			ExecutionVersion:      executionVersions[job.ID],
		})
	}

	peerAddress := utils.GetServerHTTPAddress(jobExecutor.logger)
	params := models.CommitJobStateLog{
		Address: peerAddress,
		Logs:    executionLogs,
	}

	data, err := json.Marshal(params)
	if err != nil {
		jobExecutor.logger.Fatalln("failed to marshal json")
	}

	createCommand := &protobuffs.Command{
		Type:         protobuffs.Command_Type(constants.CommandTypeJobExecutionLogs),
		Sql:          peerAddress,
		Data:         data,
		ActionTarget: peerAddress,
	}

	createCommandData, err := proto.Marshal(createCommand)
	if err != nil {
		jobExecutor.logger.Fatalln("failed to marshal json")
	}

	_ = jobExecutor.Raft.Apply(createCommandData, time.Second*time.Duration(configs.RaftApplyTimeout)).(raft.ApplyFuture)
}
