package node

import (
	"context"
	"encoding/json"
	"math/rand"
	"scheduler0/pkg/config"
	"scheduler0/pkg/constants"
	"scheduler0/pkg/models"
	"scheduler0/pkg/utils"
	"strconv"
	"sync"
	"time"

	"github.com/hashicorp/raft"
)

type PeerCommunicator interface {
	AuthenticateWithPeersFromEtcd() map[string]Status
	WatchPeersFromEtcd()
	GetPeers() []config.RaftNode
	StopAllJobsOnAllWorkerNodes()
	StartJobsOnWorkerNodes()
	GetRandomFanInPeerHTTPAddresses(excludeList map[string]bool) []string
	FanInLocalDataFromPeersSync()
	FanInLocalDataFromPeers()
	SelectRandomPeersToFanIn() []models.PeerFanIn
	CommitFetchedUnCommittedLogs(peerFanIns []models.PeerFanIn)
	GetUncommittedLogs(requestId string)
	ReturnUncommittedLogs(requestId string)
}

type peerCommunicator struct {
	node *nodeService
}

func newPeerCommunicator(node *nodeService) *peerCommunicator {
	return &peerCommunicator{
		node: node,
	}
}

func (p *peerCommunicator) AuthenticateWithPeersFromEtcd() map[string]Status {
	p.node.logger.Info("authenticating with nodes from etcd...")

	configs := p.node.scheduler0Config.GetConfigurations()
	var wg sync.WaitGroup

	results := map[string]Status{}
	wrlck := sync.Mutex{}

	// Get peers from etcd
	var peers []config.RaftNode
	if p.node.etcdService != nil && len(configs.EtcdEndpoints) > 0 {
		var err error
		peers, err = p.node.etcdService.GetPeers(p.node.ctx)
		if err != nil {
			p.node.logger.Error("failed to get peers from etcd", "error", err)
			peers = nil
		}

		// Update cached peers (may be empty on error)
		p.node.peersMutex.Lock()
		p.node.peersFromEtcd = peers
		p.node.peersMutex.Unlock()
	} else {
		p.node.logger.Warn("etcd not configured, no peers will be discovered")
		peers = nil
	}

	for _, replica := range peers {
		if replica.NodeId != configs.NodeId {
			wg.Add(1)
			go func(rep config.RaftNode, res map[string]Status, wg *sync.WaitGroup, wrlck *sync.Mutex) {
				wrlck.Lock()
				err := utils.RetryOnError(func() error {
					if peerStatus, err := p.node.client.ConnectNode(rep); err == nil {
						results[rep.NodeAddress] = *peerStatus
						p.node.logger.Info("successfully authenticated with", "node-address", rep.NodeAddress, "nodeId", rep.NodeId)
					} else {
						return err
					}

					return nil
				}, configs.PeerConnectRetryMax, configs.PeerConnectRetryDelaySeconds)
				wg.Done()
				wrlck.Unlock()
				if err != nil {
					p.node.logger.Error("failed to authenticate with peer ", "replica address", rep.NodeAddress, "nodeId", rep.NodeId, " error:", err)
				}
			}(replica, results, &wg, &wrlck)
		}
	}
	wg.Wait()

	return results
}

func (p *peerCommunicator) WatchPeersFromEtcd() {
	if p.node.etcdService == nil {
		return
	}

	configs := p.node.scheduler0Config.GetConfigurations()
	if len(configs.EtcdEndpoints) == 0 {
		return
	}

	peerCh, err := p.node.etcdService.WatchPeers(p.node.ctx)
	if err != nil {
		p.node.logger.Error("failed to start watching peers from etcd", "error", err)
		return
	}

	p.node.logger.Info("started watching peers from etcd")

	for {
		select {
		case peers, ok := <-peerCh:
			if !ok {
				p.node.logger.Warn("peer watch channel closed")
				return
			}

			// Update cached peers
			p.node.peersMutex.Lock()
			p.node.peersFromEtcd = peers
			p.node.peersMutex.Unlock()

			p.node.logger.Info("peers updated from etcd", "peerCount", len(peers))

			// Reconcile Raft membership with latest peers.
			// Only the current Raft leader is allowed to change membership.
			go p.node.raftCluster.ReconcileRaftMembershipWithPeers(peers)
		case <-p.node.ctx.Done():
			return
		}
	}
}

func (p *peerCommunicator) GetPeers() []config.RaftNode {
	configs := p.node.scheduler0Config.GetConfigurations()

	if p.node.etcdService == nil || len(configs.EtcdEndpoints) == 0 {
		p.node.logger.Warn("getPeers called without etcd configured; returning no peers")
		return nil
	}

	// Prefer cached peers if available.
	p.node.peersMutex.RLock()
	peers := p.node.peersFromEtcd
	p.node.peersMutex.RUnlock()

	if len(peers) > 0 {
		return peers
	}

	// Otherwise try to get fresh peers
	freshPeers, err := p.node.etcdService.GetPeers(p.node.ctx)
	if err != nil {
		p.node.logger.Error("failed to get peers from etcd", "error", err)
		return nil
	}

	// Update cache
	p.node.peersMutex.Lock()
	p.node.peersFromEtcd = freshPeers
	p.node.peersMutex.Unlock()

	return freshPeers
}

func (p *peerCommunicator) StopAllJobsOnAllWorkerNodes() {
	p.node.logger.Info("stopping jobs on worker nodes.")

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, constants.DefaultMaxConnectedPeers)

	peers := p.GetPeers()
	configs := p.node.scheduler0Config.GetConfigurations()

	for _, replica := range peers {
		if replica.NodeId != configs.NodeId {
			wg.Add(1)
			go func(rep config.RaftNode, wg *sync.WaitGroup) {
				err := utils.RetryOnError(func() error {
					semaphore <- struct{}{}
					defer func() { <-semaphore }()
					return p.node.client.StopJobs(p.node.ctx, p.node, rep)
				}, constants.DefaultRetryMaxConfig, constants.DefaultRetryIntervalConfig)
				wg.Done()
				if err != nil {
					p.node.logger.Error("failed to stop jobs on worker node", "address", rep.NodeAddress, "nodeId", rep.NodeId, " error:", err)
				}
			}(replica, &wg)
		}
	}
	wg.Wait()
	p.node.logger.Error("completed stopping jobs on worker nodes")
}

func (p *peerCommunicator) StartJobsOnWorkerNodes() {
	p.node.logger.Info("starting jobs on worker nodes.")

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, constants.DefaultMaxConnectedPeers)

	peers := p.GetPeers()
	configs := p.node.scheduler0Config.GetConfigurations()

	for _, replica := range peers {
		if replica.NodeId != configs.NodeId {
			wg.Add(1)
			go func(rep config.RaftNode, wg *sync.WaitGroup) {
				err := utils.RetryOnError(func() error {
					semaphore <- struct{}{}
					defer func() { <-semaphore }()
					return p.node.client.StartJobs(p.node.ctx, p.node, rep)
				}, constants.DefaultRetryMaxConfig, constants.DefaultRetryIntervalConfig)
				wg.Done()
				if err != nil {
					p.node.logger.Error("failed to start jobs on worker node", "address", rep.NodeAddress, "nodeId", rep.NodeId, " error:", err)
				}
			}(replica, &wg)
		}
	}
	wg.Wait()
	p.node.logger.Error("completed starting jobs on worker nodes")
}

func (p *peerCommunicator) GetRandomFanInPeerHTTPAddresses(excludeList map[string]bool) []string {
	configs := p.node.scheduler0Config.GetConfigurations()
	servers := p.node.scheduler0RaftStore.GetServersOnRaftCluster()
	numServers := len(servers)
	httpAddresses := make([]string, 0, numServers)

	// Filter out self using NodeId comparison instead of address
	selfServerID := raft.ServerID(strconv.FormatUint(configs.NodeId, 10))
	filteredServers := make([]raft.ServerAddress, 0, numServers)
	for _, server := range servers {
		if server.ID != selfServerID {
			filteredServers = append(filteredServers, server.Address)
		}
	}

	if uint64(len(filteredServers)) < configs.ExecutionLogFetchFanIn {
		for _, server := range filteredServers {
			if ok := excludeList[string(server)]; !ok {
				httpAddresses = append(httpAddresses, string(server))
			}
		}
	} else {
		shuffledServers := make([]raft.ServerAddress, len(filteredServers))
		copy(shuffledServers, filteredServers)
		lastIndex := len(shuffledServers) - 1

		for lastIndex > 0 {
			randInt := rand.Intn(lastIndex)
			temp := shuffledServers[lastIndex]
			shuffledServers[lastIndex] = shuffledServers[randInt]
			shuffledServers[randInt] = temp
			lastIndex -= 1
		}

		for i := 0; i < len(shuffledServers); i++ {
			if ok := excludeList[string(shuffledServers[i])]; !ok {
				httpAddresses = append(httpAddresses, string(shuffledServers[i]))
			}
		}
	}
	if len(httpAddresses) > int(configs.ExecutionLogFetchFanIn) {
		return httpAddresses[:configs.ExecutionLogFetchFanIn]
	}

	return httpAddresses
}

func (p *peerCommunicator) SelectRandomPeersToFanIn() []models.PeerFanIn {
	excludeList := map[string]bool{}
	p.node.fanIns.Range(func(key, value any) bool {
		excludeList[key.(string)] = true
		return true
	})
	httpAddresses := p.GetRandomFanInPeerHTTPAddresses(excludeList)
	peerFanIns := make([]models.PeerFanIn, 0, len(httpAddresses))
	for _, httpAddress := range httpAddresses {
		_, ok := p.node.fanIns.Load(httpAddress)
		if !ok {
			newFanIn := models.PeerFanIn{
				PeerNodeAddress: httpAddress,
				State:           models.PeerFanInStateNotStated,
			}
			p.node.fanIns.Store(httpAddress, newFanIn)
			peerFanIns = append(peerFanIns, newFanIn)
		}
	}
	return peerFanIns
}

// FanInLocalDataFromPeersSync performs synchronous fan-in of uncommitted logs from peers.
// This is used during leader restart to ensure predictable initialization before accepting write requests.
// It processes peers sequentially (one peer through all phases, then next peer) for simplicity and predictability.
func (p *peerCommunicator) FanInLocalDataFromPeersSync() {
	startTime := time.Now()
	configs := p.node.scheduler0Config.GetConfigurations()

	p.node.logger.Info("starting synchronous fan-in from peers")

	// Get peers list (exclude self)
	excludeList := map[string]bool{}
	httpAddresses := p.GetRandomFanInPeerHTTPAddresses(excludeList)

	if len(httpAddresses) == 0 {
		p.node.logger.Warn("no peers found in multi-node mode, proceeding without fan-in")
		return
	}

	p.node.logger.Info("found peers to fan-in from", "peerCount", len(httpAddresses))

	// Calculate timeout: RaftTransportTimeout * number_of_peers * 3 (for 3 phases) or minimum 120 seconds
	timeoutSeconds := int64(configs.RaftTransportTimeout) * int64(len(httpAddresses)) * 3
	if timeoutSeconds < 120 {
		timeoutSeconds = 120
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	p.node.logger.Info("sync fan-in timeout set", "timeoutSeconds", timeoutSeconds)

	// Track statistics
	successfulPeers := 0
	failedPeers := 0
	totalExecutionLogs := 0
	totalAsyncTasks := 0

	// Process each peer sequentially through all phases
	for i, httpAddress := range httpAddresses {
		peerNum := i + 1
		p.node.logger.Info("processing peer", "peerNum", peerNum, "totalPeers", len(httpAddresses), "peerAddress", httpAddress)

		// Check context cancellation
		select {
		case <-ctx.Done():
			p.node.logger.Warn("sync fan-in cancelled due to timeout or context cancellation", "processedPeers", peerNum-1, "totalPeers", len(httpAddresses))
			goto summary
		default:
		}

		// Initialize peer fan-in state
		peerFanIn := models.PeerFanIn{
			PeerNodeAddress: httpAddress,
			State:           models.PeerFanInStateNotStated,
		}

		peerExecutionLogs := 0
		peerAsyncTasks := 0

		// Phase 1: Get request ID
		p.node.logger.Info("phase 1 started for peer", "peerAddress", httpAddress, "peerNum", peerNum)
		phase1Start := time.Now()
		p.node.client.FetchUncommittedLogsFromPeersPhase1(ctx, p.node, []models.PeerFanIn{peerFanIn})

		// Wait for phase 1 to complete by checking state in fanIns
		// The client method updates fanIns synchronously, so we poll with a reasonable timeout
		phase1Complete := false
		phase1Timeout := time.After(30 * time.Second) // Per-peer timeout for phase 1
		phase1Ticker := time.NewTicker(100 * time.Millisecond)
		defer phase1Ticker.Stop()

	phase1Loop:
		for {
			select {
			case <-phase1Timeout:
				p.node.logger.Error("phase 1 timeout for peer", "peerAddress", httpAddress, "peerNum", peerNum, "duration", time.Since(phase1Start))
				break phase1Loop
			case <-ctx.Done():
				p.node.logger.Warn("phase 1 cancelled due to context", "peerAddress", httpAddress, "peerNum", peerNum)
				break phase1Loop
			case <-phase1Ticker.C:
				if val, ok := p.node.fanIns.Load(httpAddress); ok {
					updatedFanIn := val.(models.PeerFanIn)
					if updatedFanIn.State >= models.PeerFanInStateGetRequestId && updatedFanIn.RequestId != "" {
						peerFanIn = updatedFanIn
						phase1Complete = true
						break phase1Loop
					}
				}
			}
		}

		if !phase1Complete || peerFanIn.RequestId == "" {
			p.node.logger.Error("phase 1 failed for peer", "peerAddress", httpAddress, "peerNum", peerNum, "duration", time.Since(phase1Start))
			failedPeers++
			// Clean up any partial state
			p.node.fanIns.Delete(httpAddress)
			continue
		}

		p.node.logger.Info("phase 1 completed for peer", "peerAddress", httpAddress, "peerNum", peerNum, "requestId", peerFanIn.RequestId, "duration", time.Since(phase1Start))

		// Phase 2: Get uncommitted logs using request ID
		p.node.logger.Info("phase 2 started for peer", "peerAddress", httpAddress, "peerNum", peerNum)
		phase2Start := time.Now()
		p.node.client.FetchUncommittedLogsFromPeersPhase2(ctx, p.node, []models.PeerFanIn{peerFanIn})

		// Wait for phase 2 to complete
		phase2Complete := false
		phase2Timeout := time.After(30 * time.Second) // Per-peer timeout for phase 2
		phase2Ticker := time.NewTicker(100 * time.Millisecond)
		defer phase2Ticker.Stop()

	phase2Loop:
		for {
			select {
			case <-phase2Timeout:
				p.node.logger.Error("phase 2 timeout for peer", "peerAddress", httpAddress, "peerNum", peerNum, "duration", time.Since(phase2Start))
				break phase2Loop
			case <-ctx.Done():
				p.node.logger.Warn("phase 2 cancelled due to context", "peerAddress", httpAddress, "peerNum", peerNum)
				break phase2Loop
			case <-phase2Ticker.C:
				if val, ok := p.node.fanIns.Load(httpAddress); ok {
					updatedFanIn := val.(models.PeerFanIn)
					if updatedFanIn.State >= models.PeerFanInStateGetExecutionsLogs {
						peerFanIn = updatedFanIn
						phase2Complete = true
						peerExecutionLogs = len(updatedFanIn.Data.ExecutionLogs)
						peerAsyncTasks = len(updatedFanIn.Data.AsyncTasks)
						break phase2Loop
					}
				}
			}
		}

		if !phase2Complete {
			p.node.logger.Error("phase 2 failed for peer", "peerAddress", httpAddress, "peerNum", peerNum, "duration", time.Since(phase2Start))
			failedPeers++
			// Clean up any partial state
			p.node.fanIns.Delete(httpAddress)
			continue
		}

		p.node.logger.Info("phase 2 completed for peer", "peerAddress", httpAddress, "peerNum", peerNum, "executionLogs", peerExecutionLogs, "asyncTasks", peerAsyncTasks, "duration", time.Since(phase2Start))

		// Phase 3: Commit the logs synchronously
		p.node.logger.Info("phase 3 started for peer", "peerAddress", httpAddress, "peerNum", peerNum)
		phase3Start := time.Now()
		p.CommitFetchedUnCommittedLogs([]models.PeerFanIn{peerFanIn})
		p.node.logger.Info("phase 3 completed for peer", "peerAddress", httpAddress, "peerNum", peerNum, "duration", time.Since(phase3Start))

		// Update statistics
		successfulPeers++
		totalExecutionLogs += peerExecutionLogs
		totalAsyncTasks += peerAsyncTasks

		p.node.logger.Info("peer processing completed", "peerAddress", httpAddress, "peerNum", peerNum, "success", true, "executionLogs", peerExecutionLogs, "asyncTasks", peerAsyncTasks)
	}

summary:
	duration := time.Since(startTime)
	p.node.logger.Info("sync fan-in completed",
		"totalPeers", len(httpAddresses),
		"successfulPeers", successfulPeers,
		"failedPeers", failedPeers,
		"totalExecutionLogs", totalExecutionLogs,
		"totalAsyncTasks", totalAsyncTasks,
		"duration", duration,
		"durationSeconds", duration.Seconds(),
	)
}

func (p *peerCommunicator) FanInLocalDataFromPeers() {
	go func() {
		p.node.logger.Info("fanning in local data from peers")

		configs := p.node.scheduler0Config.GetConfigurations()
		intervalSec := configs.ExecutionLogFetchIntervalSeconds
		if intervalSec == 0 {
			intervalSec = 60
			p.node.logger.Warn("ExecutionLogFetchIntervalSeconds is 0; defaulting to 60 seconds")
		}
		ticker := time.NewTicker(time.Duration(intervalSec) * time.Second)
		ctx, cancelFunc := context.WithCancel(p.node.ctx)

		var currentContext context.Context
		var currentContextCancelFunc func()

		for {
			select {
			case <-ticker.C:
				if currentContext != nil {
					currentContextCancelFunc()
					ctx, cancelFunc = context.WithCancel(context.Background())
					currentContext = ctx
					currentContextCancelFunc = cancelFunc
				}

				peers := p.SelectRandomPeersToFanIn()

				phase1 := make([]models.PeerFanIn, 0, len(peers))
				phase2 := make([]models.PeerFanIn, 0, len(peers))
				phase3 := make([]models.PeerFanIn, 0, len(peers))

				for _, peer := range peers {
					if peer.State == models.PeerFanInStateNotStated {
						phase1 = append(phase1, peer)
					}
					if peer.State == models.PeerFanInStateGetRequestId {
						phase2 = append(phase2, peer)
					}
					if peer.State == models.PeerFanInStateGetExecutionsLogs {
						phase3 = append(phase3, peer)
					}
				}

				p.node.fanIns.Range(func(key, value any) bool {
					fanIn := value.(models.PeerFanIn)
					if fanIn.State == models.PeerFanInStateNotStated {
						phase1 = append(phase1, fanIn)
					}
					if fanIn.State == models.PeerFanInStateGetRequestId {
						phase2 = append(phase2, fanIn)
					}
					if fanIn.State == models.PeerFanInStateGetExecutionsLogs {
						phase3 = append(phase3, fanIn)
					}
					return true
				})

				if len(phase1) > 0 {
					p.node.logger.Info("fetching uncommitted logs from peers phase 1", "phase1", len(phase1))
					go p.node.client.FetchUncommittedLogsFromPeersPhase1(ctx, p.node, phase1)
				}
				if len(phase2) > 0 {
					p.node.logger.Info("fetching uncommitted logs from peers phase 2", "phase2", len(phase2))
					go p.node.client.FetchUncommittedLogsFromPeersPhase2(ctx, p.node, phase2)
				}
				if len(phase3) > 0 {
					p.node.logger.Info("committing fetched uncommitted logs", "phase3", len(phase3))
					go p.CommitFetchedUnCommittedLogs(phase3)
				}

				p.node.logger.Info("fanning in local data from peers completed")
			case <-p.node.ctx.Done():
				p.node.logger.Info("stopping fanning in local data from peers")
				cancelFunc()
				return
			}
		}
	}()
}

func (p *peerCommunicator) CommitFetchedUnCommittedLogs(peerFanIns []models.PeerFanIn) {
	for _, peerFanIn := range peerFanIns {

		if len(peerFanIn.Data.ExecutionLogs) > 0 {
			p.node.jobExecutionRepo.RaftInsertExecutionLogs(peerFanIn.Data.ExecutionLogs, p.node.scheduler0Config.GetConfigurations().NodeId)
		}

		if len(peerFanIn.Data.AsyncTasks) > 0 {
			_, err := p.node.asyncTaskRepo.RaftBatchInsert(peerFanIn.Data.AsyncTasks, p.node.scheduler0Config.GetConfigurations().NodeId)
			if err != nil {
				p.node.logger.Error("failed to insert uncommitted async tasks from", "peer", peerFanIn.PeerNodeAddress, "error", err)
			}
		}

		p.node.fanIns.Delete(peerFanIn.PeerNodeAddress)
		p.node.fanInCh <- peerFanIn
	}
}

func (p *peerCommunicator) GetUncommittedLogs(requestId string) {
	taskId, addErr := p.node.asyncTaskManager.AddTasks("", requestId, constants.JobExecutorAsyncTaskService, 1)
	if addErr != nil {
		p.node.logger.Error("failed to add new async task for job_executor", "error", addErr)
	}
	p.node.logger.Debug("added a new task for job_executor, task id", "task-id", taskId)

	p.node.dispatcher.NoBlockQueue(func(successChannel chan any, errorChannel chan any) {
		defer func() {
			close(successChannel)
			close(errorChannel)
		}()

		err := p.node.asyncTaskManager.UpdateTasksByRequestId(requestId, models.AsyncTaskInProgress, "")
		if err != nil {
			p.node.logger.Error("failed to update async task status with request id", requestId, ", error", err)
			return
		}
		uncommittedLogs := p.node.jobExecutor.GetUncommittedLogs()
		uncommittedTasks, err := p.node.asyncTaskManager.GetUnCommittedTasks()
		if err != nil {
			p.node.logger.Error("failed get uncommitted async tasks request id", requestId, ", error", err.Message)
			uErr := p.node.asyncTaskManager.UpdateTasksByRequestId(requestId, models.AsyncTaskFail, "")
			if uErr != nil {
				p.node.logger.Error("failed to update async task status with request id", "request id", requestId, ", error", uErr)
			}
			return
		}
		localData := models.LocalData{
			ExecutionLogs: uncommittedLogs,
			AsyncTasks:    uncommittedTasks,
		}
		data, mErr := json.Marshal(localData)
		if mErr != nil {
			p.node.logger.Error("failed to marshal async task result with request id", requestId, ", error", mErr)
			uErr := p.node.asyncTaskManager.UpdateTasksByRequestId(requestId, models.AsyncTaskFail, "")
			if uErr != nil {
				p.node.logger.Error("failed to update async task status with request id", requestId, ", error", uErr)
			}
			return
		}
		uErr := p.node.asyncTaskManager.UpdateTasksByRequestId(requestId, models.AsyncTaskSuccess, string(data))
		if uErr != nil {
			p.node.logger.Error("failed to update async task status with request id", requestId, ", error", uErr)
			return
		}
	})
}

func (p *peerCommunicator) ReturnUncommittedLogs(requestId string) {
	taskId, addErr := p.node.asyncTaskManager.AddTasks("", requestId, constants.JobExecutorAsyncTaskService, 1)
	if addErr != nil {
		p.node.logger.Error("failed to add new async task for job_executor", addErr)
	}
	p.node.logger.Debug("added a new task for job_executor, task id", "task-id", taskId)

	p.node.dispatcher.NoBlockQueue(func(successChannel chan any, errorChannel chan any) {
		defer func() {
			close(successChannel)
			close(errorChannel)
		}()

		err := p.node.asyncTaskManager.UpdateTasksByRequestId(requestId, models.AsyncTaskInProgress, "")
		if err != nil {
			p.node.logger.Error("failed to update async task status with request id", requestId, ", error", err)
			return
		}
		uncommittedLogs := p.node.jobExecutor.GetUncommittedLogs()
		uncommittedTasks, err := p.node.asyncTaskManager.GetUnCommittedTasks()
		if err != nil {
			p.node.logger.Error("failed to uncommitted async tasks request id", requestId, ", error", err.Message)
			uErr := p.node.asyncTaskManager.UpdateTasksByRequestId(requestId, models.AsyncTaskFail, "")
			if uErr != nil {
				p.node.logger.Error("failed to update async task status with request id", "request id", requestId, ", error", uErr)
			}
			return
		}
		localData := models.LocalData{
			ExecutionLogs: uncommittedLogs,
			AsyncTasks:    uncommittedTasks,
		}
		data, mErr := json.Marshal(localData)
		if mErr != nil {
			p.node.logger.Error("failed to marshal async task result with request id", requestId, ", error", mErr)
			uErr := p.node.asyncTaskManager.UpdateTasksByRequestId(requestId, models.AsyncTaskFail, "")
			if uErr != nil {
				p.node.logger.Error("failed to update async task status with request id", requestId, ", error", uErr)
			}
			return
		}
		uErr := p.node.asyncTaskManager.UpdateTasksByRequestId(requestId, models.AsyncTaskSuccess, string(data))
		if uErr != nil {
			p.node.logger.Error("failed to update async task status with request id", requestId, ", error", uErr)
			return
		}
	})
}
