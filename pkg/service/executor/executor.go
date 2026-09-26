package executor

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"scheduler0/pkg/alerts"
	"scheduler0/pkg/config"
	"scheduler0/pkg/constants"
	"scheduler0/pkg/fsm"
	"scheduler0/pkg/models"
	account_repo "scheduler0/pkg/repository/account"
	account_job_executions_count_repo "scheduler0/pkg/repository/account_job_executions_count"
	job_executors_repo "scheduler0/pkg/repository/executor"
	job_repo "scheduler0/pkg/repository/job"
	job_execution_repo "scheduler0/pkg/repository/job_execution"
	job_queue_repo "scheduler0/pkg/repository/job_queue"
	"scheduler0/pkg/scheduler0time"
	aws_lambda "scheduler0/pkg/service/executor/executors/aws"
	azure_function "scheduler0/pkg/service/executor/executors/azure"
	gcp_function "scheduler0/pkg/service/executor/executors/gcp"
	webhook_executor "scheduler0/pkg/service/executor/executors/webhook"
	"scheduler0/pkg/utils"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	queue "scheduler0/pkg/service/queue"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
)

type jobExecutor struct {
	raft                          *raft.Raft
	singleNodeMode                bool
	nodeIsLeader                  bool
	context                       context.Context
	pendingJobInvocations         []models.Job
	jobRepo                       job_repo.JobRepo
	jobExecutionsRepo             job_execution_repo.JobExecutionsRepo
	jobExecutorRepo               job_executors_repo.JobExecutorRepo
	accountRepo                   account_repo.AccountRepository
	accountJobExecutionsCountRepo account_job_executions_count_repo.AccountJobExecutionsCountRepo
	jobQueuesRepo                 job_queue_repo.JobQueuesRepo
	jobQueueService               queue.JobQueueService
	logger                        hclog.Logger
	cancelReq                     context.CancelFunc
	awsLambdaExecutionHandler     aws_lambda.LambdaExecutor
	webhookExecutionHandler       webhook_executor.WebhookExecutor
	gcpFunctionExecutionHandler   gcp_function.FunctionsExecutor
	azureFunctionExecutionHandler azure_function.FunctionsExecutor
	mtx                           sync.Mutex
	jobExecutionsCache            sync.Map
	debounce                      *utils.Debounce
	dispatcher                    *utils.Dispatcher
	scheduler0Config              config.Scheduler0Config
	scheduler0Actions             fsm.Scheduler0RaftActions

	scheduleQueueMtx        sync.Mutex
	scheduleQueue           models.ScheduleQueue
	jobAddedChan            chan struct{}
	localQuotaAllocations   sync.Map
	quotaMtx                sync.RWMutex
	notifyAccountExhaustion func(accountId uint64) error
	updateJobOnLeader       func(job models.Job) error
	notifyJobFailure        func(job models.Job, failCount uint64, executionVersion uint64) error
	// alertPublisher receives operator-facing alerts (SNS). Nil means disabled.
	alertPublisher alerts.Publisher
}

type JobExecutorService interface {
	AddJobSchedule(job models.Job)
	QueueExecutions(lastInsertedId, rowsAffected int64)
	ScheduleJobs(jobs []models.Job)
	StopAll()
	ListenForJobsToInvokeV1()
	GetUncommittedLogs() []models.JobExecutionLog
	SetSingleNodeMode(singleNodeMode bool)
	SetNodeIsLeader(nodeIsLeader bool)
	GetNodeIsLeader() bool
	UpdateRaft(rft *raft.Raft)
	GetSingleNodeMode() bool
	GetExecutionsCache() *sync.Map
	GetScheduleQueue() models.ScheduleQueue
	DeleteNewUncommittedExecutionLogs(lastInsertedId, rowsAffected int64)
	ListJobExecutors(offset uint64, limit uint64, orderByColumn string, orderByDirection string, accountId uint64) (*models.PaginatedJobExecutor, *utils.GenericError)
	CreateExecutor(newJobExecutor models.JobExecutor) (uint64, *utils.GenericError)
	TestInvokeExecutor(accountId uint64, executorId uint64, req models.TestInvocationRequest) (*models.TestInvocationResult, *utils.GenericError)
	GetOneByID(id uint64, accountId uint64) (*models.JobExecutor, *utils.GenericError)
	UpdateOneBy(executor models.JobExecutor) (uint64, *utils.GenericError)
	DeleteOneByID(id uint64, accountId uint64, deletedBy string) (uint64, *utils.GenericError)
	RegisterLocalExecutor(name, command, workingDir, createdBy string, accountId uint64) (uint64, *utils.GenericError)
	PullExecutorJobs(executorId uint64, accountId uint64) ([]models.Job, *utils.GenericError)
	ReportExecutions(executorId uint64, accountId uint64, reports []models.LocalExecutionReport) (int, *utils.GenericError)
	UpdateLocalQuotaAllocations(accountAllocations map[uint64]uint64)
	ResetLocalQuotaAllocations()
	GetLocalQuotaRemaining(accountId uint64) (uint64, bool)
	DecrementLocalQuota(accountId uint64) bool
	GetAllLocalQuotaAllocations() map[uint64]uint64
	SetNotifyAccountExhaustionCallback(callback func(accountId uint64) error)
	SetUpdateJobOnLeaderCallback(callback func(job models.Job) error)
	SetNotifyJobFailureCallback(callback func(job models.Job, failCount uint64, executionVersion uint64) error)
	SetAlertPublisher(publisher alerts.Publisher)
}

func NewJobExecutor(
	ctx context.Context,
	logger hclog.Logger,
	scheduler0Config config.Scheduler0Config,
	scheduler0Actions fsm.Scheduler0RaftActions,
	jobRepository job_repo.JobRepo,
	executionsRepo job_execution_repo.JobExecutionsRepo,
	executorsRepo job_executors_repo.JobExecutorRepo,
	jobQueuesRepo job_queue_repo.JobQueuesRepo,
	awsLambdaExecutionHandler aws_lambda.LambdaExecutor,
	webhookExecutionHandler webhook_executor.WebhookExecutor,
	gcpFunctionExecutionHandler gcp_function.FunctionsExecutor,
	azureFunctionExecutionHandler azure_function.FunctionsExecutor,
	dispatcher *utils.Dispatcher,
	accountRepo account_repo.AccountRepository,
	accountJobExecutionsCountRepo account_job_executions_count_repo.AccountJobExecutionsCountRepo,
	jobQueueService queue.JobQueueService) JobExecutorService {
	reCtx, cancel := context.WithCancel(ctx)
	executor := &jobExecutor{
		jobRepo:                       jobRepository,
		jobExecutionsRepo:             executionsRepo,
		jobExecutorRepo:               executorsRepo,
		jobQueuesRepo:                 jobQueuesRepo,
		jobQueueService:               jobQueueService,
		accountJobExecutionsCountRepo: accountJobExecutionsCountRepo,
		logger:                        logger.Named("job-executor-service"),
		context:                       reCtx,
		cancelReq:                     cancel,
		awsLambdaExecutionHandler:     awsLambdaExecutionHandler,
		webhookExecutionHandler:       webhookExecutionHandler,
		gcpFunctionExecutionHandler:   gcpFunctionExecutionHandler,
		azureFunctionExecutionHandler: azureFunctionExecutionHandler,
		jobExecutionsCache:            sync.Map{},
		debounce:                      utils.NewDebounce(),
		dispatcher:                    dispatcher,
		scheduler0Config:              scheduler0Config,
		scheduler0Actions:             scheduler0Actions,
		scheduleQueue:                 models.NewScheduleQueue(),
		jobAddedChan:                  make(chan struct{}, 1),
		accountRepo:                   accountRepo,
	}

	return executor
}

func (jobExecutor *jobExecutor) QueueExecutions(lastInsertedId, rowsAffected int64) {
	jobExecutor.logger.Debug("Fetching jobs to queue", "from", lastInsertedId, "to", lastInsertedId+(rowsAffected-1))

	configs := jobExecutor.scheduler0Config.GetConfigurations()
	currentNodeId := configs.NodeId

	newJobQueueLogs := jobExecutor.
		jobQueuesRepo.
		GetJobQueueByLastInsertedAndRowsAffected(lastInsertedId, rowsAffected)

	jobExecutor.logger.Debug("newJobQueueLogs", "newJobQueueLogs", newJobQueueLogs)

	if len(newJobQueueLogs) == 0 {
		jobExecutor.logger.Debug("No job queue logs for this node", "from", lastInsertedId, "to", lastInsertedId+(rowsAffected-1), "currentNodeId", currentNodeId)
		return
	}

	allNodeIds := make(map[uint64]bool)
	var leaderNodeId uint64
	workerNodes := make([]uint64, 0, len(allNodeIds))
	var numberOfWorkerNodes uint64

	if !jobExecutor.singleNodeMode {
		for _, log := range newJobQueueLogs {
			allNodeIds[log.NodeId] = true
		}

		if jobExecutor.raft != nil {
			_, leaderServerID := jobExecutor.raft.LeaderWithID()
			if leaderServerID != "" {
				parsedLeaderId, err := strconv.ParseUint(string(leaderServerID), 10, 64)
				if err == nil {
					leaderNodeId = parsedLeaderId
				}
			}
		}

		for nodeId := range allNodeIds {
			if nodeId != leaderNodeId {
				workerNodes = append(workerNodes, nodeId)
			}
		}

		if len(workerNodes) == 0 {
			if currentNodeId != leaderNodeId {
				workerNodes = []uint64{currentNodeId}
			} else {
				jobExecutor.logger.Debug("No worker nodes available, leader node cannot execute jobs")
				return
			}
		}

		sort.Slice(workerNodes, func(i, j int) bool {
			return workerNodes[i] < workerNodes[j]
		})

		numberOfWorkerNodes = uint64(len(workerNodes))
		jobExecutor.logger.Debug("determined node count for account hashing", "nodeCount", numberOfWorkerNodes, "workerNodes", workerNodes, "currentNodeId", currentNodeId, "leaderNodeId", leaderNodeId)
	}

	lowerBound := math.MaxInt64
	upperBound := math.MinInt64

	for _, newJobQueueLog := range newJobQueueLogs {
		if int64(newJobQueueLog.LowerBoundJobId) < int64(lowerBound) {
			lowerBound = int(int64(newJobQueueLog.LowerBoundJobId))
		}
		if int64(newJobQueueLog.UpperBoundJobId) > int64(upperBound) {
			upperBound = int(int64(newJobQueueLog.UpperBoundJobId))
		}
	}

	if lowerBound == math.MaxInt64 || upperBound == math.MinInt64 {
		jobExecutor.logger.Debug("No jobs to queue", "from", lastInsertedId, "to", lastInsertedId+(rowsAffected-1))
		return
	}

	jobExecutor.logger.Debug("Queueing jobs", "from", lowerBound, "to", upperBound, "currentNodeId", currentNodeId)

	jobBelongsToThisNode := func(job models.Job) bool {
		nodeIndex := job.AccountId % numberOfWorkerNodes
		assignedNodeId := workerNodes[nodeIndex]
		belongs := assignedNodeId == currentNodeId
		if !belongs {
			jobExecutor.logger.Debug("job filtered out - belongs to different node",
				"jobId", job.ID,
				"accountId", job.AccountId,
				"assignedNodeId", assignedNodeId,
				"currentNodeId", currentNodeId,
				"nodeIndex", nodeIndex)
		}
		return belongs
	}

	if upperBound-lowerBound > constants.JobMaxBatchSize {
		currentLowerBound := lowerBound
		currentUpperBound := lowerBound + constants.JobMaxBatchSize

		for currentLowerBound <= upperBound {
			jobExecutor.logger.Debug("fetching batching", "from", currentLowerBound, "to", currentUpperBound)
			allJobs, getErr := jobExecutor.jobRepo.BatchGetJobsWithIDRange(int64(currentLowerBound), int64(currentUpperBound))

			if getErr != nil {
				jobExecutor.logger.Error("failed to batch get job by ranges ids", "error", getErr)
				return
			}

			if jobExecutor.singleNodeMode {
				jobExecutor.ScheduleJobs(allJobs)
			} else {
				filteredJobs := make([]models.Job, 0, len(allJobs))
				for _, job := range allJobs {
					if jobBelongsToThisNode(job) {
						filteredJobs = append(filteredJobs, job)
					}
				}

				jobExecutor.logger.Debug("scheduling jobs", "totalJobs", len(allJobs), "filteredJobs", len(filteredJobs), "currentNodeId", currentNodeId)
				if len(filteredJobs) > 0 {
					jobExecutor.ScheduleJobs(filteredJobs)
				}
			}

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
		allJobs, getErr := jobExecutor.jobRepo.BatchGetJobsWithIDRange(int64(lowerBound), int64(upperBound))
		if getErr != nil {
			jobExecutor.logger.Error("failed to batch get job by ranges ids ", "error", getErr)
			return
		}

		if jobExecutor.singleNodeMode {
			jobExecutor.ScheduleJobs(allJobs)
		} else {
			filteredJobs := make([]models.Job, 0, len(allJobs))
			for _, job := range allJobs {
				if jobBelongsToThisNode(job) {
					filteredJobs = append(filteredJobs, job)
				}
			}
			jobExecutor.logger.Debug("scheduling jobs", "totalJobs", len(allJobs), "filteredJobs", len(filteredJobs), "currentNodeId", currentNodeId)
			if len(filteredJobs) > 0 {
				jobExecutor.ScheduleJobs(filteredJobs)
			}
		}
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

	accountIds := make([]uint64, 0, len(jobs))
	for _, job := range jobs {
		if !utils.Contains(accountIds, job.AccountId) {
			accountIds = append(accountIds, job.AccountId)
		}
	}

	accountToScheduleJobs := make(map[uint64]bool)

	if !jobExecutor.singleNodeMode {
		for _, job := range jobs {
			if job.AccountId == 1 {
				accountToScheduleJobs[job.AccountId] = true
				continue
			}

			quotaRemaining, hasQuota := jobExecutor.GetLocalQuotaRemaining(job.AccountId)
			if !hasQuota {
				jobExecutor.logger.Debug("no local quota allocation for account, skipping schedule", "accountId", job.AccountId)
				continue
			}

<<<<<<< HEAD
			if job.AccountId == 1 || quotaRemaining > 0 {
=======
			if quotaRemaining > 0 {
>>>>>>> 00eb8f2 (Fix 9 critical bugs: variable shadowing, mutex leaks, quota logic, nil callbacks, panic handling, recovery state, CLI flags, secret exposure, and workflow triggers)
				accountToScheduleJobs[job.AccountId] = true
				jobExecutor.logger.Debug("account has local quota available", "accountId", job.AccountId, "quotaRemaining", quotaRemaining)
			} else {
				jobExecutor.logger.Debug("local quota exhausted, notifying leader of account exhaustion", "accountId", job.AccountId)
				if jobExecutor.notifyAccountExhaustion != nil {
					jobExecutor.notifyAccountExhaustion(job.AccountId)
				}
			}
		}
	} else {
		for _, job := range jobs {
			accountToScheduleJobs[job.AccountId] = true
		}
	}

	jobExecutor.logger.Debug("account to schedule jobs", "accountToScheduleJobs", accountToScheduleJobs)

	scheduledJobIds := make(map[uint64]bool)

	for i, job := range jobs {
		if _, ok := accountToScheduleJobs[job.AccountId]; !ok {
			jobExecutor.logger.Debug("skipping job because account is not scheduled", "jobId", job.ID)
			continue
		}

		if job.Status == models.JobStatusInactive {
			jobExecutor.logger.Debug("skipping inactive job", "jobId", job.ID)
			continue
		}

		hasEnded, err := job.HasJobEnded()
		if err != nil {
			jobExecutor.logger.Error("failed to check if job has ended", "jobId", job.ID, "error", err)
			continue
		}
		if !job.EndDate.IsZero() && hasEnded {
			delete(executionLogsMap, job.ID)
			jobExecutor.logger.Debug("job has an end date in the past, skipping", "jobId", job.ID)
			continue
		}

		scheduledJobIds[job.ID] = true

		if _, ok := executionLogsMap[job.ID]; !ok {
			nextExecutionTime, err := jobs[i].GetNextExecutionTime()
			jobExecutor.logger.Debug("nextExecutionTime", "nextExecutionTime", nextExecutionTime, "jobId", job.ID)
			if err != nil {
				jobExecutor.logger.Error(fmt.Sprintf("failed to get next execution time for job with id %d error=%s", job.ID, err.Error()))
				continue
			}
			executionId, err := jobs[i].GetNextExecutionId()
			jobExecutor.logger.Debug("executionId", "executionId", executionId, "jobId", job.ID)
			if err != nil {
				jobExecutor.logger.Error(fmt.Sprintf("failed to get next execution id for job with id %d error=%s", job.ID, err.Error()))
				continue
			}
			jobs[i].ExecutionId = executionId

			jobScheduleKey := models.JobScheduleKey{
				JobId:         job.ID,
				ExecutionTime: *nextExecutionTime,
			}
			jobExecutor.logger.Debug("jobScheduleKey", "jobScheduleKey", jobScheduleKey, "jobId", job.ID)
			jobSchedule := models.JobSchedule{
				Job: jobs[i],
				MemExecution: models.MemJobExecution{
					ExecutionVersion:      1,
					FailCount:             0,
					LastState:             models.ExecutionLogScheduleState,
					LastExecutionDatetime: time.Time{},
					NextExecutionDatetime: *nextExecutionTime,
				},
			}
			jobExecutor.jobExecutionsCache.Store(job.ID, &jobSchedule)
			jobExecutor.addJobToScheduleQueue(jobScheduleKey)
			jobExecutor.logger.Debug("Added job with no prior executions logs to schedule queue", "jobId", job.ID, "nextExecutionTime", nextExecutionTime)

			continue
		}

		jobLastLog := executionLogsMap[job.ID]

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
			jobScheduleKey := models.JobScheduleKey{
				JobId:         job.ID,
				ExecutionTime: *nextExecutionTime,
			}

			jobExecutor.jobExecutionsCache.Store(job.ID, &models.JobSchedule{
				Job: jobs[i],
				MemExecution: models.MemJobExecution{
					ExecutionVersion:      jobLastLog.ExecutionVersion,
					FailCount:             0,
					LastState:             models.ExecutionLogScheduleState,
					LastExecutionDatetime: jobLastLog.LastExecutionDatetime,
					NextExecutionDatetime: *nextExecutionTime,
				},
			})
			jobExecutor.addJobToScheduleQueue(jobScheduleKey)
			jobExecutor.logger.Debug("Added job with last a schedule state last executions log to schedule queue", "jobId", job.ID, "nextExecutionTime", nextExecutionTime)
		}

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
			jobScheduleKey := models.JobScheduleKey{
				JobId:         job.ID,
				ExecutionTime: *nextExecutionTime,
			}
			jobExecutor.jobExecutionsCache.Store(job.ID, &models.JobSchedule{
				Job: jobs[i],
				MemExecution: models.MemJobExecution{
					ExecutionVersion:      jobLastLog.ExecutionVersion,
					FailCount:             0,
					LastState:             models.ExecutionLogSuccessState,
					LastExecutionDatetime: jobLastLog.LastExecutionDatetime,
					NextExecutionDatetime: *nextExecutionTime,
				},
			})
			jobExecutor.addJobToScheduleQueue(jobScheduleKey)
			jobExecutor.logger.Debug("Added job with last a success state last executions log to schedule queue", "jobId", job.ID, "nextExecutionTime", nextExecutionTime)
		}

		if jobLastLog.State == models.ExecutionLogFailedState {
			failCounts := jobExecutor.jobExecutionsRepo.CountLastFailedExecutionLogs(job.ID, configs.NodeId, jobLastLog.ExecutionVersion)
			if failCounts < uint64(jobs[i].RetryMax) {
				jobs[i].LastExecutionDate = jobLastLog.LastExecutionDatetime
				uniqueId, err := jobs[i].GetNextExecutionId()
				if err != nil {
					jobExecutor.logger.Error(fmt.Sprintf("failed to get next execution id for job with id %d error=%s", job.ID, err.Error()))
					continue
				}
				jobs[i].ExecutionId = uniqueId
				jobScheduleKey := models.JobScheduleKey{
					JobId:         job.ID,
					ExecutionTime: jobLastLog.NextExecutionDatetime,
				}
				jobExecutor.addJobToScheduleQueue(jobScheduleKey)
				jobExecutor.jobExecutionsCache.Store(job.ID, &models.JobSchedule{
					Job: jobs[i],
					MemExecution: models.MemJobExecution{
						ExecutionVersion:      jobLastLog.ExecutionVersion,
						FailCount:             failCounts,
						LastState:             models.ExecutionLogFailedState,
						LastExecutionDatetime: jobLastLog.LastExecutionDatetime,
						NextExecutionDatetime: jobLastLog.NextExecutionDatetime,
					},
				})
				jobExecutor.logger.Debug("Added job with a retry failed state last executions log to schedule queue", "jobId", job.ID, "nextExecutionTime", jobLastLog.NextExecutionDatetime)
				continue
			}

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
			jobScheduleKey := models.JobScheduleKey{
				JobId:         job.ID,
				ExecutionTime: *nextExecutionTime,
			}
			jobExecutor.jobExecutionsCache.Store(job.ID, &models.JobSchedule{
				Job: jobs[i],
				MemExecution: models.MemJobExecution{
					ExecutionVersion:      jobLastLog.ExecutionVersion,
					FailCount:             failCounts,
					LastState:             models.ExecutionLogFailedState,
					LastExecutionDatetime: jobLastLog.LastExecutionDatetime,
					NextExecutionDatetime: *nextExecutionTime,
				},
			})
			jobExecutor.addJobToScheduleQueue(jobScheduleKey)
			jobExecutor.logger.Debug("Added job with no retry failed state last executions log to schedule queue", "jobId", job.ID, "nextExecutionTime", nextExecutionTime)
		}
	}

	lastVersion := jobExecutor.jobQueuesRepo.GetLastVersion()
	lastExecutionVersions := make(map[uint64]uint64)

	jobsToLog := make([]models.Job, 0, len(jobs))
	for _, job := range jobs {
		if _, wasScheduled := scheduledJobIds[job.ID]; !wasScheduled {
			continue
		}
		jobsToLog = append(jobsToLog, job)
		if _, ok := lastExecutionVersions[job.ID]; !ok {
			if _, ok := executionLogsMap[job.ID]; ok {
				cachedJobExecutionsLog, exists := jobExecutor.jobExecutionsCache.Load(job.ID)
				if exists && cachedJobExecutionsLog != nil {
					cachedJobExecutionLog := (cachedJobExecutionsLog).(*models.JobSchedule)
					if executionLogsMap[job.ID].State == models.ExecutionLogSuccessState ||
						(cachedJobExecutionLog.MemExecution.FailCount == 0 &&
							cachedJobExecutionLog.MemExecution.LastState == models.ExecutionLogFailedState) {
						lastExecutionVersions[job.ID] = executionLogsMap[job.ID].ExecutionVersion + 1
					} else {
						lastExecutionVersions[job.ID] = executionLogsMap[job.ID].ExecutionVersion
					}
				} else {
					lastExecutionVersions[job.ID] = executionLogsMap[job.ID].ExecutionVersion
				}
			} else {
				lastExecutionVersions[job.ID] = 1
			}
		}
	}

	if len(jobsToLog) > 0 {
		jobExecutor.logger.Debug("batch inserting jobs", "jobs", jobsToLog, "lastExecutionVersions", lastExecutionVersions, "lastVersion", lastVersion, "nodeId", configs.NodeId)
		jobExecutor.jobExecutionsRepo.BatchInsert(jobsToLog, configs.NodeId, models.ExecutionLogScheduleState, lastVersion, lastExecutionVersions)

		if jobExecutor.singleNodeMode {
			jobExecutor.logger.Debug("logging job execution state in raft", "jobs", jobsToLog, "lastExecutionVersions", lastExecutionVersions, "lastVersion", lastVersion, "nodeId", configs.NodeId)
			jobExecutor.jobExecutionsRepo.LogJobExecutionStateInRaft(jobsToLog, models.ExecutionLogScheduleState, lastExecutionVersions, lastVersion, configs.NodeId)
		}

		jobExecutor.logger.Debug("scheduled jobs", "from", jobsToLog[0].ID, "to", jobsToLog[len(jobsToLog)-1].ID)
	}
}

func (jobExecutor *jobExecutor) addJobToScheduleQueue(jobScheduleKey models.JobScheduleKey) {
	jobExecutor.scheduleQueueMtx.Lock()
	jobExecutor.scheduleQueue.AddJob(jobScheduleKey)
	jobExecutor.scheduleQueueMtx.Unlock()
	select {
	case jobExecutor.jobAddedChan <- struct{}{}:
	default:
	}
}

func (jobExecutor *jobExecutor) StopAll() {
	jobExecutor.mtx.Lock()
	defer jobExecutor.mtx.Unlock()

	jobExecutor.scheduleQueueMtx.Lock()
	defer jobExecutor.scheduleQueueMtx.Unlock()

	jobExecutor.scheduleQueue.Clear()
	jobExecutor.jobExecutionsCache.Clear()
	jobExecutor.localQuotaAllocations.Clear()

	jobExecutor.logger.Info("stopped all scheduled job")
}

func (jobExecutor *jobExecutor) ListenForJobsToInvokeV1() {
	scheduler0time := scheduler0time.GetSchedulerTime()
	for {
		select {
		case <-jobExecutor.context.Done():
			jobExecutor.logger.Debug("ListenForJobsToInvokeV1-context cancelled, stopping loop")
			return
		default:
		}

		jobExecutor.scheduleQueueMtx.Lock()
		job := jobExecutor.scheduleQueue.Peek()
		jobExecutor.scheduleQueueMtx.Unlock()
		now := scheduler0time.GetTime(time.Now())

		var sleepDuration time.Duration
		if job.JobId == 0 {
			sleepDuration = time.Duration(1) * time.Second
		} else {
			sleepDuration = job.ExecutionTime.Sub(now)
			if sleepDuration <= 0 {
				sleepDuration = 0
			}
		}

		select {
		case <-jobExecutor.context.Done():
			jobExecutor.logger.Debug("ListenForJobsToInvokeV1-context cancelled during sleep, stopping loop")
			return
		case <-jobExecutor.jobAddedChan:
			continue
		case <-time.After(sleepDuration):
		}

		now = scheduler0time.GetTime(time.Now())

		dueJobs := make([]models.Job, 0)
		jobExecutor.scheduleQueueMtx.Lock()
		for {
			next := jobExecutor.scheduleQueue.Peek()
			if next.JobId == 0 || next.ExecutionTime.After(now) {
				break
			}
			jobExecutor.scheduleQueue.Pop()

			jobSchedule, exists := jobExecutor.jobExecutionsCache.Load(next.JobId)
			if !exists || jobSchedule == nil {
				jobExecutor.logger.Debug("ListenForJobsToInvokeV1-Job schedule not found in cache, dropping due job", "jobId", next.JobId)
				continue
			}
			dueJobs = append(dueJobs, jobSchedule.(*models.JobSchedule).Job)
		}
		jobExecutor.scheduleQueueMtx.Unlock()

		if len(dueJobs) == 0 {
			continue
		}

		jobExecutor.logger.Debug("ListenForJobsToInvokeV1-Invoking due jobs", "count", len(dueJobs))
		jobExecutor.invokeJobsV1(dueJobs)
	}
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

func (jobExecutor *jobExecutor) SetNodeIsLeader(nodeIsLeader bool) {
	jobExecutor.nodeIsLeader = nodeIsLeader
}

func (jobExecutor *jobExecutor) GetNodeIsLeader() bool {
	return jobExecutor.nodeIsLeader
}

func (jobExecutor *jobExecutor) UpdateRaft(rft *raft.Raft) {
	jobExecutor.raft = rft
}

func (jobExecutor *jobExecutor) GetSingleNodeMode() bool {
	return jobExecutor.singleNodeMode
}

func (jobExecutor *jobExecutor) GetExecutionsCache() *sync.Map {
	return &jobExecutor.jobExecutionsCache
}

func (jobExecutor *jobExecutor) GetScheduleQueue() models.ScheduleQueue {
	return jobExecutor.scheduleQueue
}

func (jobExecutor *jobExecutor) DeleteNewUncommittedExecutionLogs(lastInsertedId, rowsAffected int64) {
	jobExecutor.mtx.Lock()
	defer jobExecutor.mtx.Unlock()

	if rowsAffected == 0 {
		jobExecutor.logger.Debug("no rows affected, skipping deletion of uncommitted execution logs")
		return
	}

	configs := jobExecutor.scheduler0Config.GetConfigurations()

	minId := lastInsertedId - rowsAffected + 1
	maxId := lastInsertedId

	jobExecutor.logger.Debug("deleting uncommitted execution logs",
		"minId", minId,
		"maxId", maxId,
		"rowsAffected", rowsAffected,
		"nodeId", configs.NodeId)

	err := jobExecutor.jobExecutionsRepo.DeleteUncommittedExecutionLogsByIdRange(minId, maxId, configs.NodeId)
	if err != nil {
		jobExecutor.logger.Error("failed to delete uncommitted execution logs",
			"error", err,
			"minId", minId,
			"maxId", maxId,
			"nodeId", configs.NodeId)
	}
}

func (jobExecutor *jobExecutor) AddJobSchedule(job models.Job) {
	schedulerTime := scheduler0time.GetSchedulerTime()
	nextExecutionDateLocal, err := job.GetNextExecutionTime()
	if err != nil {
		jobExecutor.logger.Error(fmt.Sprintf("failed to get next execution time for job with id %d error=%s", job.ID, err.Error()))
		return
	}
	if job.DateCreated.IsZero() {
		panic("date created is zero")
	}
	jobExecutor.jobExecutionsCache.Store(job.ID, &models.JobSchedule{
		Job: job,
		MemExecution: models.MemJobExecution{
			ExecutionVersion:      1,
			FailCount:             0,
			LastState:             models.ExecutionLogScheduleState,
			LastExecutionDatetime: job.DateCreated,
			NextExecutionDatetime: schedulerTime.GetTime(*nextExecutionDateLocal),
		},
	})
	jobExecutor.addJobToScheduleQueue(models.JobScheduleKey{
		JobId:         job.ID,
		ExecutionTime: schedulerTime.GetTime(*nextExecutionDateLocal),
	})
}

func (jobExecutor *jobExecutor) UpdateLocalQuotaAllocations(accountAllocations map[uint64]uint64) {
	jobExecutor.quotaMtx.Lock()
	defer jobExecutor.quotaMtx.Unlock()

	jobExecutor.logger.Info("updating local quota allocations", "accountCount", len(accountAllocations))

	for accountId, allocatedCount := range accountAllocations {
		jobExecutor.localQuotaAllocations.Store(accountId, allocatedCount)
		jobExecutor.logger.Debug("updated local quota allocation", "accountId", accountId, "allocatedCount", allocatedCount)
	}

	jobExecutor.logger.Info("local quota allocations updated", "accountCount", len(accountAllocations))
}

func (jobExecutor *jobExecutor) ResetLocalQuotaAllocations() {
	jobExecutor.quotaMtx.Lock()
	defer jobExecutor.quotaMtx.Unlock()

	jobExecutor.logger.Info("resetting local quota allocations")

	jobExecutor.localQuotaAllocations.Clear()

	jobExecutor.logger.Info("local quota allocations reset")
}

func (jobExecutor *jobExecutor) GetLocalQuotaRemaining(accountId uint64) (uint64, bool) {
	jobExecutor.quotaMtx.RLock()
	defer jobExecutor.quotaMtx.RUnlock()

	value, ok := jobExecutor.localQuotaAllocations.Load(accountId)
	if !ok {
		return 0, false
	}

	count, ok := value.(uint64)
	if !ok {
		jobExecutor.logger.Warn("invalid quota allocation type", "accountId", accountId, "type", fmt.Sprintf("%T", value))
		return 0, false
	}

	return count, true
}

func (jobExecutor *jobExecutor) DecrementLocalQuota(accountId uint64) bool {
	jobExecutor.quotaMtx.Lock()
	defer jobExecutor.quotaMtx.Unlock()

	value, ok := jobExecutor.localQuotaAllocations.Load(accountId)
	if !ok {
		return false
	}

	count, ok := value.(uint64)
	if !ok {
		jobExecutor.logger.Warn("invalid quota allocation type", "accountId", accountId, "type", fmt.Sprintf("%T", value))
		return false
	}

	if count == 0 {
		return false
	}

	newCount := count - 1
	jobExecutor.localQuotaAllocations.Store(accountId, newCount)
	jobExecutor.logger.Debug("decremented local quota", "accountId", accountId, "oldCount", count, "newCount", newCount)

	return true
}

func (jobExecutor *jobExecutor) GetAllLocalQuotaAllocations() map[uint64]uint64 {
	jobExecutor.quotaMtx.RLock()
	defer jobExecutor.quotaMtx.RUnlock()

	result := make(map[uint64]uint64)
	jobExecutor.localQuotaAllocations.Range(func(key, value interface{}) bool {
		accountId, ok := key.(uint64)
		if !ok {
			return true
		}
		count, ok := value.(uint64)
		if !ok {
			return true
		}
		result[accountId] = count
		return true
	})

	return result
}

func (jobExecutor *jobExecutor) SetNotifyAccountExhaustionCallback(callback func(accountId uint64) error) {
	if callback == nil {
		jobExecutor.logger.Warn("attempted to set nil account exhaustion notification callback")
		return
	}
	jobExecutor.quotaMtx.Lock()
	defer jobExecutor.quotaMtx.Unlock()

	jobExecutor.notifyAccountExhaustion = callback

	jobExecutor.logger.Info("set account exhaustion notification callback")
}

func (jobExecutor *jobExecutor) SetUpdateJobOnLeaderCallback(callback func(job models.Job) error) {
	if callback == nil {
		jobExecutor.logger.Warn("attempted to set nil update job on leader callback")
		return
	}
	jobExecutor.quotaMtx.Lock()
	defer jobExecutor.quotaMtx.Unlock()
	jobExecutor.updateJobOnLeader = callback
	jobExecutor.logger.Info("set update job on leader callback")
}

func (jobExecutor *jobExecutor) SetNotifyJobFailureCallback(callback func(job models.Job, failCount uint64, executionVersion uint64) error) {
	if callback == nil {
		jobExecutor.logger.Warn("attempted to set nil job failure notification callback")
		return
	}
	jobExecutor.quotaMtx.Lock()
	defer jobExecutor.quotaMtx.Unlock()
	jobExecutor.notifyJobFailure = callback
	jobExecutor.logger.Info("set job failure notification callback")
}

// SetAlertPublisher wires the operator-alert sink (SNS). Safe to leave unset.
func (jobExecutor *jobExecutor) SetAlertPublisher(publisher alerts.Publisher) {
	jobExecutor.quotaMtx.Lock()
	defer jobExecutor.quotaMtx.Unlock()
	jobExecutor.alertPublisher = publisher
	jobExecutor.logger.Info("set ops alert publisher")
}

// notifyJobFailureAsync runs after a job has exhausted its retries. It does two
// independent things in one fire-and-forget goroutine, in this order:
//
//  1. Customer path: the platform webhook callback (app.scheduler.com emails the
//     account owner).
//  2. Operator path: publish job_execution_failed to the ops alert topic, and
//     platform_notify_failed if step 1 errored. SNS runs after the webhook so a
//     slow SNS never delays the customer notification.
//
// Neither path blocks the scheduler.
func (jobExecutor *jobExecutor) notifyJobFailureAsync(job models.Job, failCount uint64, executionVersion uint64) {
	if job.AccountId == 0 {
		return
	}
	jobExecutor.quotaMtx.RLock()
	callback := jobExecutor.notifyJobFailure
	publisher := jobExecutor.alertPublisher
	jobExecutor.quotaMtx.RUnlock()
	if callback == nil && publisher == nil {
		return
	}

	go func(j models.Job, fc, ev uint64) {
		var notifyErr error
		if callback != nil {
			if notifyErr = callback(j, fc, ev); notifyErr != nil {
				jobExecutor.logger.Error("job failure notification failed",
					"error", notifyErr,
					"jobId", j.ID,
					"accountId", j.AccountId,
					"failCount", fc,
					"executionVersion", ev,
				)
			}
		}

		if publisher == nil {
			return
		}
		ctx := context.Background()
		details := map[string]any{
			"jobId":            j.ID,
			"accountId":        j.AccountId,
			"projectId":        j.ProjectID,
			"failCount":        fc,
			"executionVersion": ev,
			"retryMax":         j.RetryMax,
			"spec":             j.Spec,
		}
		if j.ExecutorId != nil {
			details["executorId"] = *j.ExecutorId
		}
		if err := publisher.Publish(ctx, alerts.Alert{
			Event:       alerts.EventJobExecutionFailed,
			Severity:    alerts.SeverityWarn,
			Summary:     fmt.Sprintf("job %d (account %d) failed after %d attempt(s)", j.ID, j.AccountId, fc),
			Details:     details,
			ThrottleKey: fmt.Sprintf("%s:%d", alerts.EventJobExecutionFailed, j.ID),
		}); err != nil {
			jobExecutor.logger.Error("ops alert publish failed", "event", alerts.EventJobExecutionFailed, "jobId", j.ID, "error", err)
		}

		if notifyErr != nil {
			if err := publisher.Publish(ctx, alerts.Alert{
				Event:    alerts.EventPlatformNotifyFailed,
				Severity: alerts.SeverityError,
				Summary:  fmt.Sprintf("job failure notification to the platform failed: %v", notifyErr),
				Details: map[string]any{
					"jobId":            j.ID,
					"accountId":        j.AccountId,
					"failCount":        fc,
					"executionVersion": ev,
					"error":            notifyErr,
				},
			}); err != nil {
				jobExecutor.logger.Error("ops alert publish failed", "event", alerts.EventPlatformNotifyFailed, "jobId", j.ID, "error", err)
			}
		}
	}(job, failCount, executionVersion)
}

func (jobExecutor *jobExecutor) reschedule(jobs []models.Job, newState models.JobExecutionLogState) {
	jobExecutor.mtx.Lock()
	defer jobExecutor.mtx.Unlock()

	jobExecutor.logger.Debug("rescheduling jobs", "jobs", jobs, "newState", newState)

	configs := jobExecutor.scheduler0Config.GetConfigurations()

	jobsToReschedule := make([]models.Job, 0, len(jobs))

	accountIds := make([]uint64, 0, len(jobs))
	for _, job := range jobs {
		if !utils.Contains(accountIds, job.AccountId) {
			accountIds = append(accountIds, job.AccountId)
		}
	}

	accounts, getAccountsErr := jobExecutor.accountRepo.GetAccounts(accountIds)
	if getAccountsErr != nil {
		jobExecutor.logger.Error("failed to get accounts", "error", getAccountsErr)
	}
	jobExecutor.logger.Debug("accounts retrieved for rescheduling", "accounts", accounts)

	jobExecutor.logger.Debug("getting account job executions count", "accountIds", accountIds)
	accountJobExecutionsCount, getErr := jobExecutor.accountJobExecutionsCountRepo.GetExecutionCountsByAccountIds(accountIds)
	if getErr != nil {
		jobExecutor.logger.Error("failed to get account job executions count", "error", getErr)
		return
	}

<<<<<<< HEAD
	accountsExhausted := make(map[uint64]bool)
=======
	accountToRescheduleJobs := make(map[uint64]bool)

	for _, accountId := range accountIds {
		if jobExecutor.singleNodeMode {
			count, ok := accountJobExecutionsCount[accountId]
			if !ok {
				jobExecutor.logger.Error("account job executions count not found, skipping reschedule", "accountId", accountId)
				continue
			}

			if count > 0 {
				accountToRescheduleJobs[accountId] = true
				jobExecutor.accountJobExecutionsCountRepo.UpdateExecutionCount(accountId, count-1)
			} else {
				jobExecutor.logger.Debug("account job executions count is 0, setting jobs to inactive", "accountId", accountId)
				if accountId != 1 {
					updateErr := jobExecutor.jobRepo.UpdateJobsStatusByAccountId(accountId, models.JobStatusInactive)
					if updateErr != nil {
						jobExecutor.logger.Error("failed to update jobs status to inactive", "accountId", accountId, "error", updateErr)
					}
				} else {
					jobExecutor.logger.Debug("account id is 1, skipping update jobs status to inactive", "accountId", accountId)
				}
			}
		} else {
			if accountId == 1 {
				accountToRescheduleJobs[accountId] = true
				continue
			}

			quotaRemaining, hasQuota := jobExecutor.GetLocalQuotaRemaining(accountId)
			if !hasQuota {
				jobExecutor.logger.Debug("no local quota allocation for account, skipping reschedule", "accountId", accountId)
				continue
			}

			if quotaRemaining > 0 {
				if jobExecutor.DecrementLocalQuota(accountId) {
					accountToRescheduleJobs[accountId] = true
					jobExecutor.logger.Debug("decremented local quota for reschedule", "accountId", accountId, "remainingQuota", quotaRemaining-1)
				} else {
					jobExecutor.logger.Debug("failed to decrement local quota, quota may be exhausted", "accountId", accountId)
				}
			} else {
				jobExecutor.logger.Debug("local quota exhausted, skipping reschedule for account on this worker", "accountId", accountId)

				if jobExecutor.notifyAccountExhaustion != nil {
					if err := jobExecutor.notifyAccountExhaustion(accountId); err != nil {
						jobExecutor.logger.Warn("failed to notify leader of account exhaustion", "accountId", accountId, "error", err)
					} else {
						jobExecutor.logger.Debug("notified leader of account exhaustion", "accountId", accountId)
					}
				}
			}
		}
	}

	jobExecutor.logger.Debug("account to reschedule jobs", "accountToRescheduleJobs", accountToRescheduleJobs)
>>>>>>> 00eb8f2 (Fix 9 critical bugs: variable shadowing, mutex leaks, quota logic, nil callbacks, panic handling, recovery state, CLI flags, secret exposure, and workflow triggers)

	jobsToInactivate := make([]models.Job, 0)

	for i, job := range jobs {
		jobExecutor.logger.Debug("rescheduling job", "job", job)

		canReschedule := false
		if jobExecutor.singleNodeMode {
			if count, ok := accountJobExecutionsCount[job.AccountId]; !ok {
				jobExecutor.logger.Error("account job executions count not found", "accountId", job.AccountId)
			} else if count > 0 {
				canReschedule = true
				accountJobExecutionsCount[job.AccountId] = count - 1
				jobExecutor.accountJobExecutionsCountRepo.UpdateExecutionCount(job.AccountId, count-1)
			} else if !accountsExhausted[job.AccountId] {
				accountsExhausted[job.AccountId] = true
				jobExecutor.logger.Debug("account job executions count is 0, setting jobs to inactive", "accountId", job.AccountId)
				if job.AccountId != 1 {
					updateErr := jobExecutor.jobRepo.UpdateJobsStatusByAccountId(job.AccountId, models.JobStatusInactive)
					if updateErr != nil {
						jobExecutor.logger.Error("failed to update jobs status to inactive", "accountId", job.AccountId, "error", updateErr)
					}
				} else {
					jobExecutor.logger.Debug("account id is 1, skipping update jobs status to inactive", "accountId", job.AccountId)
				}
			}
		} else {
			if job.AccountId == 1 {
				canReschedule = true
			} else {
				quotaRemaining, hasQuota := jobExecutor.GetLocalQuotaRemaining(job.AccountId)
				if !hasQuota {
					jobExecutor.logger.Debug("no local quota allocation for account, skipping reschedule", "accountId", job.AccountId)
				} else if quotaRemaining > 0 {
					if jobExecutor.DecrementLocalQuota(job.AccountId) {
						canReschedule = true
						jobExecutor.logger.Debug("decremented local quota for reschedule", "accountId", job.AccountId, "jobId", job.ID, "remainingQuota", quotaRemaining-1)
					} else {
						jobExecutor.logger.Debug("failed to decrement local quota, quota may be exhausted", "accountId", job.AccountId)
					}
				} else if !accountsExhausted[job.AccountId] {
					accountsExhausted[job.AccountId] = true
					jobExecutor.logger.Debug("local quota exhausted, skipping reschedule for account on this worker", "accountId", job.AccountId)
					if jobExecutor.notifyAccountExhaustion != nil {
						if err := jobExecutor.notifyAccountExhaustion(job.AccountId); err != nil {
							jobExecutor.logger.Warn("failed to notify leader of account exhaustion", "accountId", job.AccountId, "error", err)
						} else {
							jobExecutor.logger.Debug("notified leader of account exhaustion", "accountId", job.AccountId)
						}
					}
				}
			}
		}

		if !canReschedule {
			jobExecutor.jobExecutionsCache.Delete(job.ID)
			continue
		}

		if job.Status == models.JobStatusInactive {
			jobExecutor.logger.Debug("skipping inactive job during reschedule", "jobId", job.ID)
			jobExecutor.jobExecutionsCache.Delete(job.ID)
			continue
		}

		startDateInPast, err := job.IsStartDateInPast()
		if err != nil {
			jobExecutor.logger.Error("failed to check if job start date is in past", "jobId", job.ID, "error", err)
			jobExecutor.jobExecutionsCache.Delete(job.ID)
			jobsToInactivate = append(jobsToInactivate, job)
			continue
		}

		if startDateInPast && job.Spec == "" {
			jobExecutor.logger.Debug("skipping job because it has a start date in the past", "jobId", job.ID)
			jobExecutor.jobExecutionsCache.Delete(job.ID)
			jobsToInactivate = append(jobsToInactivate, job)
			continue
		}

		hasEnded, err := job.HasJobEnded()
		if err != nil {
			jobExecutor.logger.Error("failed to check if job has ended", "jobId", job.ID, "error", err)
			jobExecutor.jobExecutionsCache.Delete(job.ID)
			jobsToInactivate = append(jobsToInactivate, job)
			continue
		}

		if hasEnded {
			jobExecutor.logger.Debug("skipping job because it has ended", "jobId", job.ID)
			jobExecutor.jobExecutionsCache.Delete(job.ID)
			jobsToInactivate = append(jobsToInactivate, job)
			continue
		}

		cachedJobExecutionsLog, exists := jobExecutor.jobExecutionsCache.Load(job.ID)
		if !exists || cachedJobExecutionsLog == nil {
			jobExecutor.logger.Error(fmt.Sprintf("job execution log not found in cache for job ID %v", job.ID))
			continue
		}
		lastExecution := (cachedJobExecutionsLog).(*models.JobSchedule)
		failCounts := lastExecution.MemExecution.FailCount
		executionVersion := lastExecution.MemExecution.ExecutionVersion

		if newState == models.ExecutionLogFailedState &&
			failCounts < uint64(jobs[i].RetryMax) {
			failCounts += 1
			jobExecutor.logger.Debug("incrementing fail count", "jobId", job.ID, "failCounts", failCounts)
			jobExecutor.jobExecutionsCache.Store(job.ID, &models.JobSchedule{
				Job: jobs[i],
				MemExecution: models.MemJobExecution{
					ExecutionVersion:      executionVersion,
					FailCount:             failCounts,
					LastState:             newState,
					LastExecutionDatetime: lastExecution.MemExecution.LastExecutionDatetime,
					NextExecutionDatetime: lastExecution.MemExecution.NextExecutionDatetime,
				},
			})
			if lastExecution.MemExecution.NextExecutionDatetime.IsZero() {
				jobExecutor.logger.Error("next execution datetime is zero", "jobId", job.ID)
				jobsToInactivate = append(jobsToInactivate, job)
				continue
			}
			jobExecutor.logger.Debug("Adding rescheduled failed state job to schedule queue", "jobId", job.ID, "executionTime", lastExecution.MemExecution.NextExecutionDatetime)
			jobExecutor.addJobToScheduleQueue(models.JobScheduleKey{
				JobId:         job.ID,
				ExecutionTime: lastExecution.MemExecution.NextExecutionDatetime,
			})
			continue
		}

		if newState == models.ExecutionLogFailedState {
			notifiedFailCount := failCounts
			if notifiedFailCount == 0 {
				notifiedFailCount = 1
			}
			jobExecutor.notifyJobFailureAsync(jobs[i], notifiedFailCount, executionVersion)
		}

		executionVersion += 1

		jobs[i].LastExecutionDate = lastExecution.MemExecution.NextExecutionDatetime
		executionId, err := jobs[i].GetNextExecutionId()
		if err != nil {
			jobExecutor.logger.Error(fmt.Sprintf("failed to get next execution id for job with id %d error=%s", job.ID, err.Error()))
			continue
		}
		executionTime, err := jobs[i].GetNextExecutionTime()
		if err != nil {
			jobExecutor.logger.Error(fmt.Sprintf("failed to get next execution time %s", err.Error()))
			continue
		}
		if executionTime.IsZero() {
			jobExecutor.logger.Debug("one-time job has completed, marking as inactive", "jobId", job.ID)
			jobExecutor.jobExecutionsCache.Delete(job.ID)
			jobsToInactivate = append(jobsToInactivate, jobs[i])
			continue
		}

		jobs[i].ExecutionId = executionId

		jobScheduleKey := models.JobScheduleKey{
			JobId:         jobs[i].ID,
			ExecutionTime: *executionTime,
		}
		jobExecutor.jobExecutionsCache.Store(job.ID, &models.JobSchedule{
			Job: jobs[i],
			MemExecution: models.MemJobExecution{
				ExecutionVersion:      executionVersion,
				FailCount:             0,
				LastState:             newState,
				LastExecutionDatetime: lastExecution.MemExecution.NextExecutionDatetime,
				NextExecutionDatetime: *executionTime,
			},
		})
		if jobScheduleKey.ExecutionTime.IsZero() {
			panic("next execution datetime is zero")
		}
		jobExecutor.logger.Debug("Adding rescheduled a state job to schedule queue", "jobId", job.ID, "executionTime", jobScheduleKey.ExecutionTime)
		jobExecutor.addJobToScheduleQueue(jobScheduleKey)
		jobsToReschedule = append(jobsToReschedule, jobs[i])
	}

	lastVersion := jobExecutor.jobQueuesRepo.GetLastVersion()
	lastExecutionVersions := make(map[uint64]uint64)

	for _, job := range jobsToReschedule {
		if _, ok := lastExecutionVersions[job.ID]; !ok {
			cachedJobExecutionsLog, exists := jobExecutor.jobExecutionsCache.Load(job.ID)
			if !exists || cachedJobExecutionsLog == nil {
				jobExecutor.logger.Error(fmt.Sprintf("job execution log not found in cache for job ID %v", job.ID))
				continue
			}
			lastExecution := (cachedJobExecutionsLog).(*models.JobSchedule)
			lastExecutionVersions[job.ID] = lastExecution.MemExecution.ExecutionVersion
		}
	}

	for i := range jobsToInactivate {
		jobExecutor.logger.Debug("marking job as inactive during reschedule", "jobId", jobsToInactivate[i].ID)
		jobsToInactivate[i].Status = models.JobStatusInactive
		if !jobExecutor.singleNodeMode && !jobExecutor.nodeIsLeader {
			if jobExecutor.updateJobOnLeader != nil {
				if err := jobExecutor.updateJobOnLeader(jobsToInactivate[i]); err != nil {
					jobExecutor.logger.Warn("failed to update job on leader", "jobId", jobsToInactivate[i].ID, "error", err)
				} else {
					jobExecutor.logger.Debug("updated job on leader", "jobId", jobsToInactivate[i].ID)
				}
			} else {
				jobExecutor.logger.Warn("update job on leader callback not set, cannot update job", "jobId", jobsToInactivate[i].ID)
			}
		} else {
			_, updateErr := jobExecutor.jobRepo.UpdateOneByID(jobsToInactivate[i])
			if updateErr != nil {
				jobExecutor.logger.Error("failed to mark job as inactive", "jobId", jobsToInactivate[i].ID, "error", updateErr)
			}
		}
	}

	jobExecutor.logger.Debug("batch inserting jobs to reschedule", "jobsToReschedule", jobsToReschedule, "lastExecutionVersions", lastExecutionVersions, "lastVersion", lastVersion, "nodeId", configs.NodeId)
	jobExecutor.jobExecutionsRepo.BatchInsert(jobsToReschedule, configs.NodeId, models.ExecutionLogScheduleState, lastVersion, lastExecutionVersions)
	if jobExecutor.singleNodeMode {
		jobExecutor.logger.Debug("logging job execution state in raft", "jobsToReschedule", jobsToReschedule, "lastExecutionVersions", lastExecutionVersions, "lastVersion", lastVersion, "nodeId", configs.NodeId)
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

	if len(jobsNotInExecution) == 0 {
		return
	}

	lastExecutionVersionsForNewJobs := jobExecutor.jobExecutionsRepo.GetLastExecutionLogForJobIds(jobsNotInExecution)

	configs := jobExecutor.scheduler0Config.GetConfigurations()

	for _, job := range jobsNotInExecution {
		failCounts := 0
		if lastExecutionVersionsForNewJobs[job].State == models.ExecutionLogFailedState {
			failCounts = int(jobExecutor.jobExecutionsRepo.CountLastFailedExecutionLogs(job, configs.NodeId, lastExecutionVersionsForNewJobs[job].ExecutionVersion))
		}

		if lastExecutionVersionsForNewJobs[job].LastExecutionDatetime.IsZero() {
			jobExecutor.logger.Error("last execution datetime is zero", "jobId", job, "lastExecutionVersionsForNewJobs", lastExecutionVersionsForNewJobs[job].Id)
			panic("lc/last execution datetime is zero")
		}
		if lastExecutionVersionsForNewJobs[job].NextExecutionDatetime.IsZero() {
			jobExecutor.logger.Error("next execution datetime is zero", "jobId", job, "lastExecutionVersionsForNewJobs", lastExecutionVersionsForNewJobs[job].Id)
			panic("lc/next execution datetime is zero")
		}

		_, ok := jobExecutor.jobExecutionsCache.Load(job)
		if ok {
			panic("job in cache")
		}

		actualJob := models.Job{}

		for _, j := range jobs {
			if j.ID == job {
				actualJob = j
				break
			}
		}

		jobExecutor.jobExecutionsCache.Store(job, &models.JobSchedule{
			Job: actualJob,
			MemExecution: models.MemJobExecution{
				ExecutionVersion:      lastExecutionVersionsForNewJobs[job].ExecutionVersion,
				FailCount:             uint64(failCounts),
				LastState:             lastExecutionVersionsForNewJobs[job].State,
				LastExecutionDatetime: lastExecutionVersionsForNewJobs[job].LastExecutionDatetime,
				NextExecutionDatetime: lastExecutionVersionsForNewJobs[job].NextExecutionDatetime,
			},
		})
	}
}

func (jobExecutor *jobExecutor) invokeJobsV1(inputJobs []models.Job) {
	jobExecutor.dispatcher.NoBlockQueue(func(successChannel chan any, errorChannel chan any) {
		defer func() {
			close(successChannel)
			close(errorChannel)
		}()

		if len(inputJobs) == 0 {
			return
		}

		jobIds := make([]uint64, 0, len(inputJobs))
		for _, job := range inputJobs {
			jobIds = append(jobIds, job.ID)
		}

		jobExecutor.logger.Info("Invoking jobs v1", "count", len(jobIds), "jobIds", jobIds)

		jobs, batchGetError := jobExecutor.jobRepo.BatchGetJobsByID(jobIds)
		if batchGetError != nil {
			jobExecutor.logger.Error(fmt.Sprintf("batch query error:: %s", batchGetError.Message))
			return
		}
		if len(jobs) == 0 {
			jobExecutor.logger.Error(fmt.Sprintf("jobs %v not found", jobIds))
			return
		}

		executorJobs := make(map[uint64][]models.Job)
		executorIdSet := make(map[uint64]struct{})
		for _, job := range jobs {
			if job.Status == models.JobStatusInactive {
				jobExecutor.logger.Debug("skipping inactive job", "jobId", job.ID)
				continue
			}

			if job.ExecutorId == nil {
				jobExecutor.logger.Error(fmt.Sprintf("job %v has no executor ID", job.ID))
				continue
			}
			executorJobs[*job.ExecutorId] = append(executorJobs[*job.ExecutorId], job)
			executorIdSet[*job.ExecutorId] = struct{}{}
		}

		if len(executorJobs) == 0 {
			return
		}

		executorIds := make([]uint64, 0, len(executorIdSet))
		for id := range executorIdSet {
			executorIds = append(executorIds, id)
		}

		jobExecutors, batchGetError := jobExecutor.jobExecutorRepo.BatchGetByIds(executorIds)
		if batchGetError != nil {
			jobExecutor.logger.Error(fmt.Sprintf("batch query error:: %s", batchGetError.Message))
			return
		}

		jobExecutorMap := make(map[uint64]models.JobExecutor)
		for _, executor := range jobExecutors {
			jobExecutorMap[executor.ID] = executor
		}

		if len(jobExecutorMap) == 0 {
			panic("no job executor found")
		}

		for executorId, groupedJobs := range executorJobs {
			executor, ok := jobExecutorMap[executorId]
			if !ok {
				jobExecutor.logger.Error(fmt.Sprintf("job executor not found for jobs with executor ID %v", executorId))
				continue
			}

			if executor.PayloadAggregation && len(groupedJobs) > 1 {
				for _, subgroup := range jobExecutor.groupJobsByFireTime(groupedJobs) {
					if len(subgroup) == 1 {
						jobExecutor.invokeSingleJob(executor, subgroup[0])
						continue
					}
					jobExecutor.invokeAggregatedJobs(executor, subgroup)
				}
				continue
			}

			for _, job := range groupedJobs {
				jobExecutor.invokeSingleJob(executor, job)
			}
		}
	})
}

func (jobExecutor *jobExecutor) buildInvocationPayload(job models.Job) models.JobInvocationPayload {
	var lastExecutionDateTime *time.Time
	lastExecutionStatus := models.ExecutionStateScheduled

	if cached, ok := jobExecutor.jobExecutionsCache.Load(job.ID); ok && cached != nil {
		schedule := cached.(*models.JobSchedule)
		lastExecutionDateTime = &schedule.MemExecution.LastExecutionDatetime

		switch schedule.MemExecution.LastState {
		case models.ExecutionLogFailedState:
			lastExecutionStatus = models.ExecutionStateFailed
		case models.ExecutionLogSuccessState:
			lastExecutionStatus = models.ExecutionStateSuccess
		case models.ExecutionLogScheduleState:
			lastExecutionStatus = models.ExecutionStateScheduled
		}
	}

	return models.JobInvocationPayload{
		Job:                   job,
		LastExecutionDateTime: lastExecutionDateTime,
		LastExecutionStatus:   lastExecutionStatus,
	}
}

func (jobExecutor *jobExecutor) groupJobsByFireTime(jobs []models.Job) [][]models.Job {
	groups := make(map[int64][]models.Job)
	order := make([]int64, 0, len(jobs))
	var uncached [][]models.Job

	for _, job := range jobs {
		cached, ok := jobExecutor.jobExecutionsCache.Load(job.ID)
		if !ok || cached == nil {
			uncached = append(uncached, []models.Job{job})
			continue
		}
		schedule := cached.(*models.JobSchedule)
		key := schedule.MemExecution.NextExecutionDatetime.UTC().Truncate(time.Second).Unix()
		if _, exists := groups[key]; !exists {
			order = append(order, key)
		}
		groups[key] = append(groups[key], job)
	}

	result := make([][]models.Job, 0, len(order)+len(uncached))
	for _, key := range order {
		result = append(result, groups[key])
	}
	result = append(result, uncached...)
	return result
}

func (jobExecutor *jobExecutor) invokeSingleJob(executor models.JobExecutor, job models.Job) {
	payload := jobExecutor.buildInvocationPayload(job)

	switch models.ExecutorType(executor.Type) {
	case models.ExecutorTypeWebhookUrl:
		jobExecutor.webhookExecutionHandler.ExecuteWebhookJob(
			executor,
			payload,
			jobExecutor.handleSuccessJobs,
			jobExecutor.handleFailedJobs,
		)
	case models.ExecutorTypeCloudFunction:
		switch executor.CloudProvider {
		case "aws":
			jobExecutor.awsLambdaExecutionHandler.ExecuteLambdaJob(
				executor.Region,
				executor.CloudResourceUrl,
				executor.CloudApiKey,
				executor.CloudApiSecret,
				payload,
				jobExecutor.handleSuccessJobs,
				jobExecutor.handleFailedJobs,
			)
		case "azure":
			jobExecutor.azureFunctionExecutionHandler.ExecuteFunctionJob(
				executor.CloudResourceUrl,
				executor.CloudApiKey,
				payload,
				jobExecutor.handleSuccessJobs,
				jobExecutor.handleFailedJobs,
			)
		case "gcp":
			jobExecutor.gcpFunctionExecutionHandler.ExecuteFunctionJob(
				executor.CloudResourceUrl,
				executor.CloudApiKey,
				payload,
				jobExecutor.handleSuccessJobs,
				jobExecutor.handleFailedJobs,
			)
		default:
			jobExecutor.logger.Error(fmt.Sprintf("unrecognized cloud provider %s for executor %v", executor.CloudProvider, executor.ID))
		}
	case models.ExecutorTypeLocal:
		jobExecutor.logger.Debug("skipping push invocation for local executor", "executorId", executor.ID, "jobId", job.ID)
	default:
		jobExecutor.logger.Error(fmt.Sprintf("unrecognized execution type %s for executor %v", executor.Type, executor.ID))
	}
}

func (jobExecutor *jobExecutor) invokeAggregatedJobs(executor models.JobExecutor, jobs []models.Job) {
	payload := models.AggregatedJobInvocationPayload{Aggregated: true}
	for _, job := range jobs {
		payload.Jobs = append(payload.Jobs, jobExecutor.buildInvocationPayload(job))
	}

	jobExecutor.logger.Info("invoking aggregated jobs", "executorId", executor.ID, "count", len(jobs))

	switch models.ExecutorType(executor.Type) {
	case models.ExecutorTypeWebhookUrl:
		jobExecutor.webhookExecutionHandler.ExecuteWebhookJobBatch(
			executor,
			payload,
			jobExecutor.handleSuccessJobsBatch,
			jobExecutor.handleFailedJobsBatch,
		)
	case models.ExecutorTypeCloudFunction:
		switch executor.CloudProvider {
		case "aws":
			jobExecutor.awsLambdaExecutionHandler.ExecuteLambdaJobBatch(
				executor.Region,
				executor.CloudResourceUrl,
				executor.CloudApiKey,
				executor.CloudApiSecret,
				payload,
				jobExecutor.handleSuccessJobsBatch,
				jobExecutor.handleFailedJobsBatch,
			)
		case "azure":
			jobExecutor.azureFunctionExecutionHandler.ExecuteFunctionJobBatch(
				executor.CloudResourceUrl,
				executor.CloudApiKey,
				payload,
				jobExecutor.handleSuccessJobsBatch,
				jobExecutor.handleFailedJobsBatch,
			)
		case "gcp":
			jobExecutor.gcpFunctionExecutionHandler.ExecuteFunctionJobBatch(
				executor.CloudResourceUrl,
				executor.CloudApiKey,
				payload,
				jobExecutor.handleSuccessJobsBatch,
				jobExecutor.handleFailedJobsBatch,
			)
		default:
			jobExecutor.logger.Error(fmt.Sprintf("unrecognized cloud provider %s for executor %v", executor.CloudProvider, executor.ID))
		}
	case models.ExecutorTypeLocal:
		jobExecutor.logger.Debug("skipping aggregated push invocation for local executor", "executorId", executor.ID)
	default:
		jobExecutor.logger.Error(fmt.Sprintf("unrecognized execution type %s for executor %v", executor.Type, executor.ID))
	}
}

func (jobExecutor *jobExecutor) handleSuccessJobs(successfulJob models.Job) {
	configs := jobExecutor.scheduler0Config.GetConfigurations()
	lastVersion := jobExecutor.jobQueuesRepo.GetLastVersion()
	lastExecutionVersions := make(map[uint64]uint64)

	jobExecutor.createInMemExecutionsForJobsIfNotExist([]models.Job{successfulJob})

	jobExecutor.mtx.Lock()
	cachedJobExecutionsLog, exists := jobExecutor.jobExecutionsCache.Load(successfulJob.ID)
	if !exists || cachedJobExecutionsLog == nil {
		jobExecutor.mtx.Unlock()
		jobExecutor.logger.Error(fmt.Sprintf("job execution log not found in cache for successful job ID %v", successfulJob.ID))
		jobExecutor.mtx.Unlock()
		return
	}
	cachedJobExecutionLog := (cachedJobExecutionsLog).(*models.JobSchedule)

	lastExecutionVersions[successfulJob.ID] = cachedJobExecutionLog.MemExecution.ExecutionVersion
	jobExecutor.mtx.Unlock()

	if jobExecutor.singleNodeMode {
		jobExecutor.jobExecutionsRepo.LogJobExecutionStateInRaft([]models.Job{successfulJob}, models.ExecutionLogSuccessState, lastExecutionVersions, lastVersion, configs.NodeId)
	} else {
		jobExecutor.jobExecutionsRepo.BatchInsert([]models.Job{successfulJob}, configs.NodeId, models.ExecutionLogSuccessState, lastVersion, lastExecutionVersions)
	}
	jobExecutor.reschedule([]models.Job{successfulJob}, models.ExecutionLogSuccessState)
}

func (jobExecutor *jobExecutor) handleFailedJobs(erroredJob models.Job) {
	configs := jobExecutor.scheduler0Config.GetConfigurations()
	jobExecutor.logger.Error(fmt.Sprintf("failed to execute job %v", erroredJob.ID))
	lastVersion := jobExecutor.jobQueuesRepo.GetLastVersion()
	lastExecutionVersions := map[uint64]uint64{}

	jobExecutor.createInMemExecutionsForJobsIfNotExist([]models.Job{erroredJob})

	jobExecutor.mtx.Lock()
	cachedJobExecutionsLog, exists := jobExecutor.jobExecutionsCache.Load(erroredJob.ID)
	if !exists || cachedJobExecutionsLog == nil {
		jobExecutor.mtx.Unlock()
		jobExecutor.logger.Error(fmt.Sprintf("job execution log not found in cache for errored job ID %v", erroredJob.ID))
		jobExecutor.mtx.Unlock()
		return
	}
	cachedJobExecutionLog := (cachedJobExecutionsLog).(*models.JobSchedule)
	lastExecutionVersions[erroredJob.ID] = cachedJobExecutionLog.MemExecution.ExecutionVersion
	jobExecutor.mtx.Unlock()

	if jobExecutor.singleNodeMode {
		jobExecutor.jobExecutionsRepo.LogJobExecutionStateInRaft([]models.Job{erroredJob}, models.ExecutionLogFailedState, lastExecutionVersions, lastVersion, configs.NodeId)
	} else {
		jobExecutor.jobExecutionsRepo.BatchInsert([]models.Job{erroredJob}, configs.NodeId, models.ExecutionLogFailedState, lastVersion, lastExecutionVersions)
	}
	jobExecutor.reschedule([]models.Job{erroredJob}, models.ExecutionLogFailedState)
}

func (jobExecutor *jobExecutor) handleSuccessJobsBatch(successfulJobs []models.Job) {
	jobExecutor.handleJobsBatch(successfulJobs, models.ExecutionLogSuccessState)
}

func (jobExecutor *jobExecutor) handleFailedJobsBatch(erroredJobs []models.Job) {
	for _, job := range erroredJobs {
		jobExecutor.logger.Error(fmt.Sprintf("failed to execute job %v (aggregated)", job.ID))
	}
	jobExecutor.handleJobsBatch(erroredJobs, models.ExecutionLogFailedState)
}

func (jobExecutor *jobExecutor) handleJobsBatch(jobs []models.Job, state models.JobExecutionLogState) {
	if len(jobs) == 0 {
		return
	}

	configs := jobExecutor.scheduler0Config.GetConfigurations()
	lastVersion := jobExecutor.jobQueuesRepo.GetLastVersion()
	lastExecutionVersions := make(map[uint64]uint64)

	jobExecutor.createInMemExecutionsForJobsIfNotExist(jobs)

	jobExecutor.mtx.Lock()
	resolved := make([]models.Job, 0, len(jobs))
	for _, job := range jobs {
		cached, exists := jobExecutor.jobExecutionsCache.Load(job.ID)
		if !exists || cached == nil {
			jobExecutor.logger.Error(fmt.Sprintf("job execution log not found in cache for job ID %v", job.ID))
			continue
		}
		schedule := cached.(*models.JobSchedule)
		lastExecutionVersions[job.ID] = schedule.MemExecution.ExecutionVersion
		resolved = append(resolved, job)
	}
	jobExecutor.mtx.Unlock()

	if len(resolved) == 0 {
		return
	}

	if jobExecutor.singleNodeMode {
		jobExecutor.jobExecutionsRepo.LogJobExecutionStateInRaft(resolved, state, lastExecutionVersions, lastVersion, configs.NodeId)
	} else {
		jobExecutor.jobExecutionsRepo.BatchInsert(resolved, configs.NodeId, state, lastVersion, lastExecutionVersions)
	}
	jobExecutor.reschedule(resolved, state)
}

func (jobExecutor *jobExecutor) ListJobExecutors(offset uint64, limit uint64, orderByColumn string, orderByDirection string, accountId uint64) (*models.PaginatedJobExecutor, *utils.GenericError) {
	total, err := jobExecutor.jobExecutorRepo.Count(accountId)
	if err != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Message)
	}

	if total < 1 {
		return &models.PaginatedJobExecutor{
			Data:   []models.JobExecutor{},
			Total:  0,
			Limit:  limit,
			Offset: offset,
		}, nil
	}

	if limit > constants.MaxListLimit {
		return nil, utils.HTTPGenericError(http.StatusTooManyRequests, fmt.Sprintf("too many executors. limit should be less than %d", constants.MaxListLimit))
	}

	if limit < 1 {
		return nil, utils.HTTPGenericError(http.StatusBadRequest, "limit should be greater than 0")
	}

	if offset < 0 {
		return nil, utils.HTTPGenericError(http.StatusBadRequest, "offset should be greater than 0")
	}

	data, err := jobExecutor.jobExecutorRepo.List(offset, limit, orderByColumn, orderByDirection, accountId)
	if err != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Message)
	}

	return &models.PaginatedJobExecutor{
		Data:   data,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}, nil
}

func (jobExecutor *jobExecutor) CreateExecutor(newJobExecutor models.JobExecutor) (uint64, *utils.GenericError) {
	return jobExecutor.jobExecutorRepo.CreateOne(newJobExecutor)
}

func (jobExecutor *jobExecutor) TestInvokeExecutor(accountId uint64, executorId uint64, req models.TestInvocationRequest) (*models.TestInvocationResult, *utils.GenericError) {
	if accountId == 0 {
		return nil, utils.HTTPGenericError(http.StatusBadRequest, "account id is required")
	}

	executor, getErr := jobExecutor.jobExecutorRepo.GetOneByID(executorId, accountId)
	if getErr != nil {
		return nil, getErr
	}
	if executor == nil || executor.ID == 0 {
		return nil, utils.HTTPGenericError(http.StatusNotFound, fmt.Sprintf("executor %d not found", executorId))
	}

	if models.ExecutorType(executor.Type) == models.ExecutorTypeLocal {
		return nil, utils.HTTPGenericError(http.StatusBadRequest, "local executors are pull-based and cannot be test-invoked; poll jobs via the local-executor API instead")
	}

	schedulerTime := scheduler0time.GetSchedulerTime()
	now := schedulerTime.GetTime(time.Now())

	testJob := req.Job
	testJob.AccountId = accountId
	testJob.ExecutorId = &executor.ID
	if testJob.Timezone == "" {
		testJob.Timezone = "UTC"
	}
	if testJob.Status == "" {
		testJob.Status = models.JobStatusActive
	}

	if strings.TrimSpace(req.Age) != "" {
		age, parseErr := time.ParseDuration(strings.TrimSpace(req.Age))
		if parseErr != nil {
			return nil, utils.HTTPGenericError(http.StatusBadRequest, fmt.Sprintf("invalid age %q: %s", req.Age, parseErr.Error()))
		}
		if age < 0 {
			return nil, utils.HTTPGenericError(http.StatusBadRequest, "age must be a positive duration")
		}
		aged := now.Add(-age)
		testJob.DateCreated = aged
		testJob.LastExecutionDate = aged
	} else if testJob.DateCreated.IsZero() {
		testJob.DateCreated = now
	}

	executionTime := now
	if req.ExecutionTime != nil {
		executionTime = *req.ExecutionTime
	}

	payload := models.JobInvocationPayload{
		Job:                   testJob,
		LastExecutionDateTime: &executionTime,
		LastExecutionStatus:   models.ExecutionStateScheduled,
	}

	result := &models.TestInvocationResult{
		Test:         true,
		ExecutorId:   executor.ID,
		ExecutorType: executor.Type,
		Payload:      payload,
		StartedAt:    now,
	}

	done := make(chan bool, 1)
	onSuccess := func(job models.Job) {
		select {
		case done <- true:
		default:
		}
	}
	onError := func(job models.Job) {
		select {
		case done <- false:
		default:
		}
	}

	jobExecutor.dispatchTestInvocation(*executor, payload, onSuccess, onError)

	configs := jobExecutor.scheduler0Config.GetConfigurations()
	timeout := time.Duration(configs.JobExecutionTimeout) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	timeout += 5 * time.Second

	select {
	case ok := <-done:
		result.Success = ok
		if !ok {
			result.Error = "executor reported the invocation as failed; inspect your endpoint and the server logs for details"
		}
	case <-time.After(timeout):
		result.Success = false
		result.Error = fmt.Sprintf("test invocation timed out after %s", timeout)
	}

	finished := schedulerTime.GetTime(time.Now())
	result.FinishedAt = finished
	result.DurationMs = finished.Sub(result.StartedAt).Milliseconds()

	return result, nil
}

func (jobExecutor *jobExecutor) dispatchTestInvocation(
	executor models.JobExecutor,
	payload models.JobInvocationPayload,
	onSuccess func(job models.Job),
	onError func(job models.Job),
) {
	switch models.ExecutorType(executor.Type) {
	case models.ExecutorTypeWebhookUrl:
		jobExecutor.webhookExecutionHandler.ExecuteWebhookJob(executor, payload, onSuccess, onError)
	case models.ExecutorTypeCloudFunction:
		switch executor.CloudProvider {
		case "aws":
			jobExecutor.awsLambdaExecutionHandler.ExecuteLambdaJob(
				executor.Region,
				executor.CloudResourceUrl,
				executor.CloudApiKey,
				executor.CloudApiSecret,
				payload,
				onSuccess,
				onError,
			)
		case "azure":
			jobExecutor.azureFunctionExecutionHandler.ExecuteFunctionJob(
				executor.CloudResourceUrl,
				executor.CloudApiKey,
				payload,
				onSuccess,
				onError,
			)
		case "gcp":
			jobExecutor.gcpFunctionExecutionHandler.ExecuteFunctionJob(
				executor.CloudResourceUrl,
				executor.CloudApiKey,
				payload,
				onSuccess,
				onError,
			)
		default:
			jobExecutor.logger.Error(fmt.Sprintf("test invocation: unrecognized cloud provider %s for executor %v", executor.CloudProvider, executor.ID))
			onError(payload.Job)
		}
	default:
		jobExecutor.logger.Error(fmt.Sprintf("test invocation: unrecognized execution type %s for executor %v", executor.Type, executor.ID))
		onError(payload.Job)
	}
}

func (jobExecutor *jobExecutor) GetOneByID(id uint64, accountId uint64) (*models.JobExecutor, *utils.GenericError) {
	return jobExecutor.jobExecutorRepo.GetOneByID(id, accountId)
}

func (jobExecutor *jobExecutor) UpdateOneBy(executor models.JobExecutor) (uint64, *utils.GenericError) {
	existing, err := jobExecutor.jobExecutorRepo.GetOneByID(executor.ID, executor.AccountId)
	if err != nil {
		return 0, err
	}

	if executor.CloudProvider == "" {
		executor.CloudProvider = existing.CloudProvider
	}
	if executor.Region == "" {
		executor.Region = existing.Region
	}
	if executor.CloudResourceUrl == "" {
		executor.CloudResourceUrl = existing.CloudResourceUrl
	}
	if executor.WebhookUrl == "" {
		executor.WebhookUrl = existing.WebhookUrl
	}
	if executor.WebhookMethod == "" {
		executor.WebhookMethod = existing.WebhookMethod
	}

	return jobExecutor.jobExecutorRepo.UpdateOneByID(executor)
}

func (jobExecutor *jobExecutor) DeleteOneByID(id uint64, accountId uint64, deletedBy string) (uint64, *utils.GenericError) {
	if deletedBy == "" {
		return 0, utils.HTTPGenericError(http.StatusBadRequest, "deletedBy is required")
	}
	executor, err := jobExecutor.jobExecutorRepo.GetOneByID(id, accountId)
	if err != nil {
		return 0, err
	}
	executor.DeletedBy = &deletedBy
	return jobExecutor.jobExecutorRepo.DeleteOneByID(*executor)
}

func (jobExecutor *jobExecutor) RegisterLocalExecutor(name, command, workingDir, createdBy string, accountId uint64) (uint64, *utils.GenericError) {
	newExecutor := models.JobExecutor{
		Name:       name,
		Type:       string(models.ExecutorTypeLocal),
		Command:    command,
		WorkingDir: workingDir,
		CreatedBy:  createdBy,
		AccountId:  accountId,
	}
	return jobExecutor.jobExecutorRepo.CreateOne(newExecutor)
}

func (jobExecutor *jobExecutor) getLocalExecutor(executorId uint64, accountId uint64) (*models.JobExecutor, *utils.GenericError) {
	executor, err := jobExecutor.jobExecutorRepo.GetOneByID(executorId, accountId)
	if err != nil {
		return nil, err
	}
	if executor == nil || executor.ID == 0 {
		return nil, utils.HTTPGenericError(http.StatusNotFound, "local executor not found")
	}
	if models.ExecutorType(executor.Type) != models.ExecutorTypeLocal {
		return nil, utils.HTTPGenericError(http.StatusBadRequest, "executor is not a local executor")
	}
	return executor, nil
}

func (jobExecutor *jobExecutor) PullExecutorJobs(executorId uint64, accountId uint64) ([]models.Job, *utils.GenericError) {
	if _, err := jobExecutor.getLocalExecutor(executorId, accountId); err != nil {
		return nil, err
	}

	jobs, err := jobExecutor.jobRepo.GetActiveJobsByExecutorID(executorId, accountId)
	if err != nil {
		return nil, err
	}
	if jobs == nil {
		jobs = []models.Job{}
	}
	return jobs, nil
}

func (jobExecutor *jobExecutor) ReportExecutions(executorId uint64, accountId uint64, reports []models.LocalExecutionReport) (int, *utils.GenericError) {
	if _, err := jobExecutor.getLocalExecutor(executorId, accountId); err != nil {
		return 0, err
	}

	if len(reports) == 0 {
		return 0, nil
	}

	nodeId := jobExecutor.scheduler0Config.GetConfigurations().NodeId
	schedulerTime := scheduler0time.GetSchedulerTime()
	now := schedulerTime.GetTime(time.Now())

	executionLogs := make([]models.JobExecutionLog, 0, len(reports))
	var billableExecutions uint64
	for _, report := range reports {
		if report.JobID == 0 || report.UniqueID == "" {
			jobExecutor.logger.Warn("skipping malformed local execution report", "executorId", executorId, "jobId", report.JobID, "uniqueId", report.UniqueID)
			continue
		}

		lastExecTime := parseReportTime(report.LastExecutionTime)
		nextExecTime := parseReportTime(report.NextExecutionTime)

		executionLogs = append(executionLogs, models.JobExecutionLog{
			UniqueId:              report.UniqueID,
			State:                 models.JobExecutionLogState(report.State),
			NodeId:                nodeId,
			LastExecutionDatetime: lastExecTime,
			NextExecutionDatetime: nextExecTime,
			JobId:                 report.JobID,
			JobQueueVersion:       report.JobQueueVersion,
			ExecutionVersion:      report.ExecutionVersion,
			DateCreated:           now,
			AccountId:             accountId,
		})

		if models.JobExecutionLogState(report.State) != models.ExecutionLogScheduleState {
			billableExecutions++
		}
	}

	if len(executionLogs) == 0 {
		return 0, nil
	}

	jobExecutor.jobExecutionsRepo.RaftInsertExecutionLogs(executionLogs, nodeId)

	jobExecutor.decrementAccountQuota(accountId, billableExecutions)

	return len(executionLogs), nil
}

func (jobExecutor *jobExecutor) decrementAccountQuota(accountId uint64, delta uint64) {
	if delta == 0 || accountId == 0 || accountId == 1 {
		return
	}
	counts, err := jobExecutor.accountJobExecutionsCountRepo.GetExecutionCountsByAccountIds([]uint64{accountId})
	if err != nil {
		jobExecutor.logger.Error("ReportExecutions: failed to load account execution count for quota update", "accountId", accountId, "error", err)
		return
	}
	current, ok := counts[accountId]
	if !ok {
		return
	}
	newCount := uint64(0)
	if current > delta {
		newCount = current - delta
	}
	if updateErr := jobExecutor.accountJobExecutionsCountRepo.UpdateExecutionCount(accountId, newCount); updateErr != nil {
		jobExecutor.logger.Error("ReportExecutions: failed to update account execution count", "accountId", accountId, "error", updateErr)
	}
}

func parseReportTime(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}
	}
	return t
}
