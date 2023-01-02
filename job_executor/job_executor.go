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
	"os"
	"scheduler0/config"
	"scheduler0/constants"
	"scheduler0/job_executor/executors"
	"scheduler0/models"
	"scheduler0/protobuffs"
	"scheduler0/repository"
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
	dir, err := os.Getwd()
	if err != nil {
		logger.Fatalln(fmt.Errorf("Fatal error getting working dir: %s \n", err))
	}
	dirPath := fmt.Sprintf("%v/%v", dir, constants.ExecutionLogsDir)
	mkdirErr := os.Mkdir(dirPath, os.ModePerm)
	if !os.IsExist(mkdirErr) && !os.IsNotExist(mkdirErr) && err != nil {
		logger.Fatalln(fmt.Errorf("failed to create the logs dir %s \n", err))
	}
	logFilePath := fmt.Sprintf("%v/%v", dirPath, constants.ExecutionLogsCommitFile)
	_, createErr := os.Create(logFilePath)
	if createErr != nil {
		logger.Fatalln(fmt.Errorf("failed to create the %s file %s \n", logFilePath, err))
	}
	logFilePath = fmt.Sprintf("%v/%v", dirPath, constants.ExecutionLogsUnCommitFile)
	_, createErr = os.Create(logFilePath)
	if createErr != nil {
		logger.Fatalln(fmt.Errorf("failed to create the %s file %s \n", logFilePath, err))
	}
	if err != nil {
		logger.Fatalln("logs file close error")
	}
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

			if utils.MonitorMemoryUsage(jobExecutor.logger) {
				return
			}

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

	for i, job := range jobs {
		if jobLastLog, ok := executionLogsMap[job.ID]; !ok {
			jobs[i].LastExecutionDate = job.DateCreated
			sha := sha1.New()
			schedule, parseErr := cron.Parse(jobs[i].Spec)
			if parseErr != nil {
				jobExecutor.logger.Fatalln(fmt.Sprintf("failed to parse job cron spec %s", parseErr.Error()))
			}
			executionTime := schedule.Next(jobs[i].LastExecutionDate)
			uniqueId := fmt.Sprintf(
				"%v-%v-%v-%v",
				job.ProjectID,
				job.ID,
				job.LastExecutionDate.String(),
				executionTime.UTC().String(),
			)
			jobs[i].ExecutionId = fmt.Sprintf("%x", sha.Sum([]byte(uniqueId)))
			jobExecutor.AddNewProcess(jobs[i], executionTime)
		} else {
			if jobLastLog.State == uint64(models.ExecutionLogScheduleState) {
				jobs[i].LastExecutionDate = jobLastLog.LastExecutionDatetime
				jobs[i].ExecutionId = jobLastLog.UniqueId
				schedule, parseErr := cron.Parse(job.Spec)
				if parseErr != nil {
					jobExecutor.logger.Fatalln(fmt.Sprintf("failed to parse spec %v", parseErr.Error()))
				}
				now := time.Now().UTC()
				executionTime := schedule.Next(jobLastLog.LastExecutionDatetime)
				if now.Before(executionTime) {
					jobExecutor.AddNewProcess(jobs[i], executionTime)
				} else {
					jobExecutor.AddNewProcess(jobs[i], jobLastLog.NextExecutionDatetime)
				}
			}

			if jobLastLog.State == uint64(models.ExecutionLogSuccessState) {
				jobs[i].LastExecutionDate = jobLastLog.NextExecutionDatetime

				sha := sha1.New()
				schedule, parseErr := cron.Parse(jobs[i].Spec)
				if parseErr != nil {
					jobExecutor.logger.Fatalln(fmt.Sprintf("failed to parse job cron spec %s", parseErr.Error()))
				}
				executionTime := schedule.Next(jobs[i].LastExecutionDate)
				uniqueId := fmt.Sprintf(
					"%v-%v-%v-%v",
					job.ProjectID,
					job.ID,
					jobLastLog.LastExecutionDatetime.String(),
					executionTime.UTC().String(),
				)

				jobs[i].ExecutionId = fmt.Sprintf("%x", sha.Sum([]byte(uniqueId)))
				jobExecutor.AddNewProcess(jobs[i], executionTime)
			}

			if jobLastLog.State == uint64(models.ExecutionLogFailedState) {
				failCounts := jobExecutor.jobExecutionsRepo.CountLastFailedExecutionLogs(job.ID, configs.NodeId, true)
				if failCounts < uint64(configs.JobExecutionRetryMax) {
					jobs[i].LastExecutionDate = jobLastLog.LastExecutionDatetime
					jobs[i].ExecutionId = jobLastLog.UniqueId
				} else {
					jobs[i].LastExecutionDate = jobLastLog.NextExecutionDatetime

					sha := sha1.New()
					schedule, parseErr := cron.Parse(jobs[i].Spec)
					if parseErr != nil {
						jobExecutor.logger.Fatalln(fmt.Sprintf("failed to parse job cron spec %s", parseErr.Error()))
					}
					executionTime := schedule.Next(jobs[i].LastExecutionDate)
					uniqueId := fmt.Sprintf(
						"%v-%v-%v-%v",
						job.ProjectID,
						job.ID,
						jobLastLog.LastExecutionDatetime.String(),
						executionTime.UTC().String(),
					)

					jobs[i].ExecutionId = fmt.Sprintf("%x", sha.Sum([]byte(uniqueId)))
					jobExecutor.AddNewProcess(jobs[i], executionTime)
				}
			}
		}
	}

	lastVersion := jobExecutor.jobQueuesRepo.GetLastVersion()
	lastExecutionVersions := map[int64]uint64{}

	for _, job := range jobs {
		if _, ok := lastExecutionVersions[job.ID]; !ok {
			if _, ok := executionLogsMap[job.ID]; ok {

				if executionLogsMap[job.ID].State == models.ExecutionLogSuccessState {
					lastExecutionVersions[job.ID] = executionLogsMap[job.ID].ExecutionVersion + 1
				} else {
					lastExecutionVersions[job.ID] = executionLogsMap[job.ID].ExecutionVersion
				}

			} else {
				lastExecutionVersions[job.ID] = 1
			}
		}
	}

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
		execTime := time.Now().UTC().Add(e.Sub(j.LastExecutionDate))
		ticker := time.NewTicker(time.Duration(1) * time.Second)
		for {
			select {
			case <-ticker.C:
				if time.Now().UTC().After(execTime) {
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

func (jobExecutor *JobExecutor) invokeJob(pendingJob models.JobModel) {
	configs := config.GetConfigurations(jobExecutor.logger)
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
	lastExecutionVersions := jobExecutor.jobExecutionsRepo.GetLastExecutionVersionForJobIds(jobIds, true)

	jobExecutor.jobExecutionsRepo.BatchInsert(successfulJobs, uint64(configs.NodeId), models.ExecutionLogSuccessState, lastVersion, lastExecutionVersions)
	if jobExecutor.SingleNodeMode {
		jobExecutor.logJobExecutionStateInRaft(successfulJobs, models.ExecutionLogSuccessState, lastExecutionVersions)
	}
	jobExecutor.Schedule(successfulJobs)
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
	lastExecutionVersions := jobExecutor.jobExecutionsRepo.GetLastExecutionVersionForJobIds(jobIds, true)

	jobExecutor.jobExecutionsRepo.BatchInsert(erroredJobs, uint64(configs.NodeId), models.ExecutionLogFailedState, lastVersion, lastExecutionVersions)
	if jobExecutor.SingleNodeMode {
		jobExecutor.logJobExecutionStateInRaft(erroredJobs, models.ExecutionLogFailedState, lastExecutionVersions)
	}
	jobExecutor.Schedule(erroredJobs)
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

		executionLogs = append(executionLogs, models.JobExecutionLog{
			JobId:                 job.ID,
			UniqueId:              job.ExecutionId,
			State:                 uint64(state),
			NodeId:                uint64(configs.NodeId),
			LastExecutionDatetime: job.LastExecutionDate,
			NextExecutionDatetime: executionTime,
			JobQueueVersion:       lastVersion,
			DataCreated:           time.Now().UTC(),
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
