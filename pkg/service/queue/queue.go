package queue

import (
	"context"
	"fmt"
	"math"
	"scheduler0/pkg/config"
	"scheduler0/pkg/constants"
	"scheduler0/pkg/fsm"
	"scheduler0/pkg/models"
	account_repo "scheduler0/pkg/repository/account"
	account_job_executions_count_repo "scheduler0/pkg/repository/account_job_executions_count"
	job_repo "scheduler0/pkg/repository/job"
	job_execution_repo "scheduler0/pkg/repository/job_execution"
	"scheduler0/pkg/repository/job_queue"
	etcd_service "scheduler0/pkg/service/etcd"
	"scheduler0/pkg/utils"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
)

// QuotaAllocationSender is an interface for sending quota allocations to peer nodes.
// This avoids import cycles by not importing the node package directly.
type QuotaAllocationSender interface {
	SendQuotaAllocation(ctx context.Context, peer config.RaftNode, accountAllocations map[uint64]uint64) error
}

type JobQueueCommand struct {
	job      models.Job
	serverId string
}

type jobQueue struct {
	singleNodeMode                bool
	jobsQueueRepo                 job_queue.JobQueuesRepo
	fsm                           fsm.Scheduler0RaftStore
	logger                        hclog.Logger
	minId                         int64
	maxId                         int64
	numberOfActiveNodes           uint64
	nodeIsLeader                  bool
	mtx                           sync.Mutex
	once                          sync.Once
	debounce                      *utils.Debounce
	context                       context.Context
	schedulerOConfig              config.Scheduler0Config
	scheduler0RaftActions         fsm.Scheduler0RaftActions
	jobRepo                       job_repo.JobRepo
	executionsRepo                job_execution_repo.JobExecutionsRepo
	accountRepo                   account_repo.AccountRepository
	accountJobExecutionsCountRepo account_job_executions_count_repo.AccountJobExecutionsCountRepo
	quotaAllocationSender         QuotaAllocationSender
	etcdService                   etcd_service.EtcdService
}

type JobQueueService interface {
	Queue(jobs []models.Job)
	IncrementQueueVersion()
	SetSingleNodeMode(singleNodeMode bool)
	GetSingleNodeMode() bool
	SetNumberOfActiveNodes(numberOfActiveNodes uint64)
	SetNodeIsLeader(nodeIsLeader bool)
	AllocateQuotasForJobQueue() error
	GetNodeIsLeader() bool
}

func NewJobQueue(
	ctx context.Context,
	logger hclog.Logger,
	scheduler0Config config.Scheduler0Config,
	scheduler0RaftActions fsm.Scheduler0RaftActions,
	fsm fsm.Scheduler0RaftStore,
	jobsQueueRepo job_queue.JobQueuesRepo,
	jobRepo job_repo.JobRepo,
	executionsRepo job_execution_repo.JobExecutionsRepo,
	accountRepo account_repo.AccountRepository,
	accountJobExecutionsCountRepo account_job_executions_count_repo.AccountJobExecutionsCountRepo,
	quotaAllocationSender QuotaAllocationSender,
	etcdService etcd_service.EtcdService,
) JobQueueService {
	return &jobQueue{
		jobsQueueRepo:                 jobsQueueRepo,
		context:                       ctx,
		fsm:                           fsm,
		logger:                        logger.Named("job-queue-service"),
		minId:                         math.MaxInt64,
		maxId:                         math.MinInt64,
		debounce:                      utils.NewDebounce(),
		schedulerOConfig:              scheduler0Config,
		scheduler0RaftActions:         scheduler0RaftActions,
		jobRepo:                       jobRepo,
		executionsRepo:                executionsRepo,
		accountRepo:                   accountRepo,
		accountJobExecutionsCountRepo: accountJobExecutionsCountRepo,
		quotaAllocationSender:         quotaAllocationSender,
		etcdService:                   etcdService,
	}
}

func (jobQ *jobQueue) Queue(jobs []models.Job) {
	jobQ.mtx.Lock()
	defer jobQ.mtx.Unlock()

	if len(jobs) < 1 {
		return
	}

	for _, job := range jobs {
		if jobQ.maxId < int64(job.ID) {
			jobQ.maxId = int64(job.ID)
		}
		if jobQ.minId > int64(job.ID) {
			jobQ.minId = int64(job.ID)
		}
	}

	jobQ.logger.Debug("Begin queueing", "jobs", len(jobs), "minId", jobQ.minId, "maxId", jobQ.maxId)

	jobQ.queue(jobQ.minId, jobQ.maxId, jobs)
	jobQ.minId = math.MaxInt64
	jobQ.maxId = math.MinInt16
}

func (jobQ *jobQueue) SetSingleNodeMode(singleNodeMode bool) {
	jobQ.singleNodeMode = singleNodeMode
}

func (jobQ *jobQueue) GetSingleNodeMode() bool {
	return jobQ.singleNodeMode
}

func (jobQ *jobQueue) SetNodeIsLeader(nodeIsLeader bool) {
	jobQ.nodeIsLeader = nodeIsLeader
}

func (jobQ *jobQueue) SetNumberOfActiveNodes(numberOfActiveNodes uint64) {
	jobQ.numberOfActiveNodes = numberOfActiveNodes
}

func (jobQ *jobQueue) GetNodeIsLeader() bool {
	return jobQ.nodeIsLeader
}

func (jobQ *jobQueue) IncrementQueueVersion() {
	jobQ.jobsQueueRepo.IncrementQueueVersion(int(jobQ.numberOfActiveNodes))
}

func (jobQ *jobQueue) queue(minId, maxId int64, jobs []models.Job) {
	if !jobQ.nodeIsLeader {
		jobQ.logger.Error("skipping job queueing as node is not the leader")
		return
	}

	lastVersion := jobQ.jobsQueueRepo.GetLastVersion()
	var jobQueueLogs []models.JobQueueLog

	if jobQ.singleNodeMode {
		jobQ.logger.Debug("single node mode: assigning all jobs to current node", "minId", minId, "maxId", maxId)
		configs := jobQ.schedulerOConfig.GetConfigurations()
		jobQueueLogs = append(jobQueueLogs, models.JobQueueLog{
			NodeId:          configs.NodeId,
			LowerBoundJobId: uint64(minId),
			UpperBoundJobId: uint64(maxId),
			Version:         lastVersion,
		})
	} else {
		jobQ.logger.Debug("multi node mode: assigning jobs by account to servers", "jobCount", len(jobs))
		jobQueueLogs = jobQ.assignJobsByAccountToServers(jobs, lastVersion)
		if len(jobQueueLogs) == 0 {
			jobQ.logger.Warn("no job queue logs created from account-based assignment", "jobCount", len(jobs))
			return
		}
	}

	jobQ.jobsQueueRepo.InsertJobQueueLogs(jobQueueLogs)
}

// assignJobsByAccountToServers groups jobs by accountId and assigns all jobs for each account
// to the same node using consistent hashing (accountId % numberOfNodes).
// Returns JobQueueLog entries grouped by account and node.
func (jobQ *jobQueue) assignJobsByAccountToServers(jobs []models.Job, version uint64) []models.JobQueueLog {
	configs := jobQ.schedulerOConfig.GetConfigurations()
	peers, err := jobQ.etcdService.GetPeers(context.Background())
	if err != nil {
		jobQ.logger.Error("failed to get peers", "error", err)
		return nil
	}

	// Get leader node ID from raft (matching executor service logic)
	var leaderNodeId uint64
	if jobQ.fsm != nil {
		_, leaderServerID := jobQ.fsm.LeaderWithID()
		if leaderServerID != "" {
			parsedLeaderId, parseErr := strconv.ParseUint(string(leaderServerID), 10, 64)
			if parseErr == nil {
				leaderNodeId = parsedLeaderId
			}
		}
	}

	// Build worker nodes list excluding leader (matching executor service logic)
	workerNodes := make([]uint64, 0, len(peers))
	for _, peer := range peers {
		if peer.NodeId != leaderNodeId {
			workerNodes = append(workerNodes, peer.NodeId)
		}
	}

	// If no worker nodes (single node mode or only leader), include current node if it's not leader
	if len(workerNodes) == 0 {
		if configs.NodeId != leaderNodeId {
			workerNodes = []uint64{configs.NodeId}
		} else {
			// Leader node should not execute jobs, but in single node mode we need to assign somewhere
			// Fallback to leader if no workers available
			workerNodes = []uint64{configs.NodeId}
		}
	}

	numberOfWorkerNodes := uint64(len(workerNodes))

	sort.Slice(workerNodes, func(i, j int) bool {
		return workerNodes[i] < workerNodes[j]
	})

	// Group jobs by accountId
	accountJobs := make(map[uint64][]models.Job)
	for _, job := range jobs {
		accountJobs[job.AccountId] = append(accountJobs[job.AccountId], job)
	}

	// Group jobs by (nodeId, accountId) - all jobs for an account go to the same node
	// nodeAccountJobs maps nodeId -> accountId -> []Job
	nodeAccountJobs := make(map[uint64]map[uint64][]models.Job)
	for accountId, jobsForAccount := range accountJobs {
		// Use consistent hashing: accountId % numberOfNodes
		nodeIndex := accountId % numberOfWorkerNodes
		targetNodeId := workerNodes[nodeIndex]

		if nodeAccountJobs[targetNodeId] == nil {
			nodeAccountJobs[targetNodeId] = make(map[uint64][]models.Job)
		}
		nodeAccountJobs[targetNodeId][accountId] = jobsForAccount

		jobQ.logger.Debug("assigned account to node", "accountId", accountId, "nodeId", targetNodeId, "jobCount", len(jobsForAccount))
	}

	// Create JobQueueLog entries per account to avoid overlapping ranges
	// Since JobQueueLog doesn't have AccountId, we create separate entries for each account's job ranges
	jobQueueLogs := make([]models.JobQueueLog, 0)

	for nodeId, accountJobsMap := range nodeAccountJobs {
		for _, jobsForAccount := range accountJobsMap {
			if len(jobsForAccount) == 0 {
				continue
			}

			// Sort jobs by ID to find contiguous ranges
			sort.Slice(jobsForAccount, func(i, j int) bool {
				return jobsForAccount[i].ID < jobsForAccount[j].ID
			})

			// Create separate JobQueueLog entries for contiguous job ID ranges
			// This prevents jobs from other accounts being included in the range
			rangeStart := jobsForAccount[0].ID
			rangeEnd := jobsForAccount[0].ID

			for i := 1; i < len(jobsForAccount); i++ {
				currentJobId := jobsForAccount[i].ID
				// If job IDs are contiguous or close, extend the range
				// Otherwise, create a new log entry for the previous range
				if currentJobId > rangeEnd+1 {
					// Gap detected, create log entry for previous range
					jobQueueLogs = append(jobQueueLogs, models.JobQueueLog{
						NodeId:          nodeId,
						LowerBoundJobId: rangeStart,
						UpperBoundJobId: rangeEnd,
						Version:         version,
					})
					jobQ.logger.Debug("created job queue log for contiguous range", "nodeId", nodeId, "minJobId", rangeStart, "maxJobId", rangeEnd)

					// Start new range
					rangeStart = currentJobId
					rangeEnd = currentJobId
				} else {
					// Extend current range
					rangeEnd = currentJobId
				}
			}

			// Create log entry for the final range
			jobQueueLogs = append(jobQueueLogs, models.JobQueueLog{
				NodeId:          nodeId,
				LowerBoundJobId: rangeStart,
				UpperBoundJobId: rangeEnd,
				Version:         version,
			})
			jobQ.logger.Debug("created job queue log for final range", "nodeId", nodeId, "minJobId", rangeStart, "maxJobId", rangeEnd)
		}
	}

	jobQ.logger.Info("completed account-based job assignment", "totalJobs", len(jobs), "totalAccounts", len(accountJobs), "queueLogsCount", len(jobQueueLogs))

	return jobQueueLogs
}

// AllocateQuotasForJobQueue reconciles account execution counts from committed logs and allocates
// per-account tokens to worker nodes based on account-to-node mapping.
// This implements the leader-owned token allocation pattern where:
// 1. The leader reconciles global counts from committed execution logs
// 2. The leader allocates tokens per account per node
// 3. Workers manage local decrements from their allocated tokens
func (jobQ *jobQueue) AllocateQuotasForJobQueue() error {
	startTime := time.Now()
	jobQ.logger.Info("starting quota allocation for job queue")

	// Step 1: Get all account IDs from the database
	accountIdsResult, err := jobQ.accountRepo.GetAllAccountIds()
	if err != nil {
		jobQ.logger.Error("failed to get all account IDs", "error", err)
		return fmt.Errorf("failed to get all account IDs: %s", err.Error())
	}
	accountIds := accountIdsResult
	if len(accountIds) == 0 {
		jobQ.logger.Debug("no accounts found in database, skipping quota allocation")
		return nil
	}
	jobQ.logger.Debug("retrieved account IDs", "accountCount", len(accountIds))

	// Step 2: Generate account-to-node mapping using consistent hashing
	configs := jobQ.schedulerOConfig.GetConfigurations()
	peers, peerErr := jobQ.etcdService.GetPeers(context.Background())
	if peerErr != nil {
		jobQ.logger.Error("failed to get peers", "error", peerErr)
		return fmt.Errorf("failed to get peers: %w", peerErr)
	}

	// Get leader node ID from raft (matching executor service logic)
	var leaderNodeId uint64
	if jobQ.fsm != nil {
		_, leaderServerID := jobQ.fsm.LeaderWithID()
		if leaderServerID != "" {
			parsedLeaderId, parseErr := strconv.ParseUint(string(leaderServerID), 10, 64)
			if parseErr == nil {
				leaderNodeId = parsedLeaderId
			}
		}
	}

	// Build worker nodes list excluding leader (matching executor service logic)
	workerNodes := make([]uint64, 0, len(peers))
	for _, peer := range peers {
		if peer.NodeId != leaderNodeId {
			workerNodes = append(workerNodes, peer.NodeId)
		}
	}

	// If no worker nodes (single node mode or only leader), include current node if it's not leader
	if len(workerNodes) == 0 {
		if configs.NodeId != leaderNodeId {
			workerNodes = []uint64{configs.NodeId}
		} else {
			// Leader node should not execute jobs, but in single node mode we need to assign somewhere
			// Fallback to leader if no workers available
			workerNodes = []uint64{configs.NodeId}
		}
	}

	numberOfWorkerNodes := uint64(len(workerNodes))

	// Generate account-to-node mapping: accountId -> nodeId
	accountToNodeMapping := make(map[uint64]uint64)
	for _, accountId := range accountIds {
		nodeIndex := accountId % numberOfWorkerNodes
		targetNodeId := workerNodes[nodeIndex]
		accountToNodeMapping[accountId] = targetNodeId
	}
	jobQ.logger.Debug("generated account-to-node mapping", "accountCount", len(accountIds), "nodeCount", numberOfWorkerNodes)

	// Step 3: Get the most recent job queue date (or zero time if none exist)
	mostRecentQueueDate, queueDateErr := jobQ.jobsQueueRepo.GetMostRecentJobQueueDate()
	if queueDateErr != nil {
		jobQ.logger.Error("failed to get most recent job queue date", "error", queueDateErr)
		return fmt.Errorf("failed to get most recent job queue date: %w", queueDateErr)
	}
	jobQ.logger.Debug("most recent job queue date", "date", mostRecentQueueDate, "isZero", mostRecentQueueDate.IsZero())

	// Step 4: Get account features to determine execution limits
	accountFeatures, err := jobQ.accountRepo.GetFeaturesByAccountIds(accountIds)
	if err != nil {
		jobQ.logger.Error("failed to get account features", "error", err)
		return err
	}
	jobQ.logger.Debug("retrieved account features", "accountFeaturesCount", len(accountFeatures))

	// Step 5: Get interval usage from committed logs since last queue cycle
	var intervalUsageByAccount map[uint64]uint64
	if !mostRecentQueueDate.IsZero() {
		var usageErr error
		intervalUsageByAccount, usageErr = jobQ.executionsRepo.GetExecutionUsageByAccountIds(accountIds, mostRecentQueueDate)
		if usageErr != nil {
			jobQ.logger.Error("failed to get execution usage by account ids", "error", usageErr, "startDate", mostRecentQueueDate)
			return fmt.Errorf("failed to get execution usage by account ids: %w", usageErr)
		}
		jobQ.logger.Debug("retrieved interval execution usage from committed logs", "intervalUsageByAccount", intervalUsageByAccount)
	} else {
		intervalUsageByAccount = make(map[uint64]uint64)
		jobQ.logger.Debug("no previous queue date, starting with zero interval usage")
	}

	// Step 6: Get current account execution counts (remaining quota)
	currentCounts, err := jobQ.accountJobExecutionsCountRepo.GetExecutionCountsByAccountIds(accountIds)
	if err != nil {
		jobQ.logger.Error("failed to get current execution counts", "error", err)
		return err
	}
	jobQ.logger.Debug("retrieved current execution counts", "currentCounts", currentCounts)

	// Step 7: Calculate remaining quota per account and allocate tokens
	// For each account, we need to:
	// - Determine the limit (default or with feature)
	// - Reconcile: limit - usage = remaining
	// - Allocate tokens to the assigned node based on remaining quota
	// - Update account_job_executions_count (decrease global count)
	allocationsByNode := make(map[uint64]map[uint64]uint64) // nodeId -> accountId -> allocatedCount

	for _, accountId := range accountIds {
		// Determine execution limit for this account
		executionLimit := uint64(constants.DefaultNumberOfJobExecutions10KPerMonth)
		if features, ok := accountFeatures[accountId]; ok {
			for _, feature := range features {
				if feature.Feature == constants.IncreasedNumberOfJobExecutions100KPerMonthFeature {
					executionLimit = constants.DefaultNumberOfJobExecutions100KPerMonth
					break
				}
			}
		}
		jobQ.logger.Debug("account execution limit", "accountId", accountId, "limit", executionLimit)

		// Reconcile: subtract interval usage from current remaining count
		intervalUsage := intervalUsageByAccount[accountId]
		currentRemaining := currentCounts[accountId]

		// If no current count exists, initialize to full limit
		if _, exists := currentCounts[accountId]; !exists {
			currentRemaining = executionLimit
		}

		// The reconciled remaining should be: currentRemaining - intervalUsage
		reconciledRemaining := currentRemaining
		if intervalUsage > currentRemaining {
			// More was used than available, set to 0
			jobQ.logger.Warn("interval usage exceeds current remaining, setting to 0", "accountId", accountId, "intervalUsage", intervalUsage, "currentRemaining", currentRemaining)
			reconciledRemaining = 0
		} else {
			reconciledRemaining = currentRemaining - intervalUsage
		}
		jobQ.logger.Debug("reconciled remaining quota", "accountId", accountId, "limit", executionLimit, "intervalUsage", intervalUsage, "currentRemaining", currentRemaining, "reconciledRemaining", reconciledRemaining)

		// If quota is exhausted for system account (accountId == 1), reset to 100K
		if reconciledRemaining == 0 && accountId == 1 {
			jobQ.logger.Info("quota exhausted for system account, resetting execution count to 100K", "accountId", accountId)
			reconciledRemaining = constants.DefaultNumberOfJobExecutions100KPerMonth
			updateErr := jobQ.accountJobExecutionsCountRepo.ResetExecutionCount(accountId, reconciledRemaining)
			if updateErr != nil {
				jobQ.logger.Error("failed to reset system account execution count", "error", updateErr, "accountId", accountId)
				return updateErr
			}
			jobQ.logger.Info("successfully reset system account execution count", "accountId", accountId, "newRemaining", reconciledRemaining)
		} else if reconciledRemaining == 0 && accountId != 1 {
			// If quota is exhausted for non-system account, update job status to inactive
			jobQ.logger.Info("quota exhausted for account, updating job status to inactive", "accountId", accountId)
			updateErr := jobQ.jobRepo.UpdateJobsStatusByAccountId(accountId, models.JobStatusInactive)
			if updateErr != nil {
				jobQ.logger.Error("failed to update job status for exhausted account", "error", updateErr, "accountId", accountId)
				// Don't fail quota allocation if status update fails - log and continue
			} else {
				jobQ.logger.Info("successfully updated job status to inactive for exhausted account", "accountId", accountId)
			}
		}

		// Update the account execution count to the reconciled remaining
		// If the account doesn't have a record, create it; otherwise update it
		if _, exists := currentCounts[accountId]; !exists {
			_, createErr := jobQ.accountJobExecutionsCountRepo.Create(accountId, reconciledRemaining)
			if createErr != nil {
				jobQ.logger.Error("failed to create account execution count", "error", createErr, "accountId", accountId)
				return createErr
			}
		} else {
			// Update to reconciled remaining
			updateErr := jobQ.accountJobExecutionsCountRepo.ResetExecutionCount(accountId, reconciledRemaining)
			if updateErr != nil {
				jobQ.logger.Error("failed to reset account execution count", "error", updateErr, "accountId", accountId)
				return updateErr
			}
		}

		// Get the assigned node for this account
		assignedNodeId := accountToNodeMapping[accountId]

		// Allocate tokens to the assigned node based on remaining quota
		// For simplicity, we allocate a portion of the remaining quota
		// In a more sophisticated implementation, we might consider job frequency/execution patterns
		if reconciledRemaining > 0 {
			if allocationsByNode[assignedNodeId] == nil {
				allocationsByNode[assignedNodeId] = make(map[uint64]uint64)
			}

			// Allocate a portion of the remaining quota (e.g., 10% or a fixed amount)
			// This ensures nodes have quota available for job executions
			allocation := reconciledRemaining / 10 // Allocate 10% of remaining quota
			if allocation == 0 && reconciledRemaining > 0 {
				allocation = 1 // At least 1 execution if quota is available
			}
			if allocation > reconciledRemaining {
				allocation = reconciledRemaining
			}

			allocationsByNode[assignedNodeId][accountId] = allocation
			jobQ.logger.Debug("allocated quota to node", "nodeId", assignedNodeId, "accountId", accountId, "allocation", allocation, "reconciledRemaining", reconciledRemaining)

			// Decrease the global count by the allocation
			if allocation > 0 {
				// Get current count after reconciliation
				updatedCounts, getErr := jobQ.accountJobExecutionsCountRepo.GetExecutionCountsByAccountIds([]uint64{accountId})
				if getErr != nil {
					jobQ.logger.Error("failed to get updated execution count", "error", getErr, "accountId", accountId)
					continue
				}
				currentRemaining := updatedCounts[accountId]
				if allocation > currentRemaining {
					allocation = currentRemaining
				}
				if allocation > 0 {
					newCount := currentRemaining - allocation
					updateErr := jobQ.accountJobExecutionsCountRepo.UpdateExecutionCount(accountId, newCount)
					if updateErr != nil {
						jobQ.logger.Error("failed to update execution count after allocation", "error", updateErr, "accountId", accountId, "allocation", allocation)
						continue
					}
					jobQ.logger.Debug("decreased global count after allocation", "accountId", accountId, "allocation", allocation, "newCount", newCount)
				}
			}
		}
	}

	duration := time.Since(startTime)
	jobQ.logger.Info("completed quota allocation for job queue", "accountCount", len(accountIds), "allocationsByNode", allocationsByNode, "duration", duration, "durationMs", duration.Milliseconds())

	// Send allocations to workers via TCP
	if jobQ.quotaAllocationSender != nil && jobQ.etcdService != nil && len(allocationsByNode) > 0 {
		// Create a map of nodeId -> peer for quick lookup
		peerMap := make(map[uint64]config.RaftNode)
		for _, peer := range peers {
			peerMap[peer.NodeId] = peer
		}

		// Send quota allocations to each node
		for nodeId, accountAllocations := range allocationsByNode {
			if len(accountAllocations) == 0 {
				continue
			}

			peer, found := peerMap[nodeId]
			if !found {
				jobQ.logger.Warn("peer not found for quota allocation", "nodeId", nodeId)
				continue
			}

			jobQ.logger.Debug("sending quota allocation to peer", "nodeId", nodeId, "peerAddress", peer.NodeAddress, "accountCount", len(accountAllocations))
			err := jobQ.quotaAllocationSender.SendQuotaAllocation(jobQ.context, peer, accountAllocations)
			if err != nil {
				jobQ.logger.Error("failed to send quota allocation to peer", "error", err, "nodeId", nodeId, "peerAddress", peer.NodeAddress)
				// Continue sending to other nodes even if one fails
				continue
			}
			jobQ.logger.Info("successfully sent quota allocation to peer", "nodeId", nodeId, "peerAddress", peer.NodeAddress, "accountCount", len(accountAllocations))
		}
	} else {
		if jobQ.quotaAllocationSender == nil {
			jobQ.logger.Debug("quota allocation sender not available, skipping quota allocation send")
		}
		if jobQ.etcdService == nil {
			jobQ.logger.Debug("etcd service not available, skipping quota allocation send")
		}
	}

	return nil
}
