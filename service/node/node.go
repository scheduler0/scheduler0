package node

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	"log"
	"math/rand"
	"scheduler0/config"
	"scheduler0/constants"
	"scheduler0/fsm"
	"scheduler0/models"
	"scheduler0/repository/async_task"
	"scheduler0/repository/job"
	"scheduler0/repository/job_execution"
	"scheduler0/repository/job_queue"
	"scheduler0/repository/project"
	"scheduler0/secrets"
	async_task_service "scheduler0/service/async_task"
	"scheduler0/service/executor"
	"scheduler0/service/processor"
	"scheduler0/service/queue"
	"scheduler0/shared_repo"
	"scheduler0/utils"
	"strconv"
	"sync"
	"time"
)

type Status struct {
	IsLeader           bool
	IsAuth             bool
	IsAlive            bool
	LastConnectionTime time.Duration
}

type Res struct {
	IsLeader bool
}

type Response struct {
	Data    Res  `json:"data"`
	Success bool `json:"success"`
}

type LogsFetchResponse struct {
	Data    []byte `json:"data"`
	Success bool   `json:"success"`
}

type State int

type nodeService struct {
	acceptClientWrites    bool
	acceptRequest         bool
	scheduler0RaftStore   fsm.Scheduler0RaftStore
	SingleNodeMode        bool
	ctx                   context.Context
	logger                hclog.Logger
	mtx                   sync.Mutex
	jobProcessor          processor.JobProcessorService
	jobQueue              queue.JobQueueService
	jobExecutor           executor.JobExecutorService
	jobQueuesRepo         job_queue.JobQueuesRepo
	jobRepo               job.JobRepo
	projectRepo           project.ProjectRepo
	jobExecutionRepo      job_execution.JobExecutionsRepo
	asyncTaskRepo         async_task.AsyncTasksRepo
	sharedRepo            shared_repo.SharedRepo
	isExistingNode        bool
	peerObserverChannels  chan raft.Observation
	asyncTaskManager      async_task_service.AsyncTaskService
	dispatcher            *utils.Dispatcher
	fanIns                sync.Map // models.PeerFanIn
	fanInCh               chan models.PeerFanIn
	completedFanInCh      sync.Map
	scheduler0Config      config.Scheduler0Config
	scheduler0Secrets     secrets.Scheduler0Secrets
	scheduler0RaftActions fsm.Scheduler0RaftActions
	nodeHTTPClient        NodeClient
	postProcessingChannel chan models.PostProcess
}

type NodeService interface {
	Start()
	GetUncommittedLogs(requestId string)
	CanAcceptClientWriteRequest() bool
	StopJobs()
	StartJobs()
	GetRaftLeaderWithId() (raft.ServerAddress, raft.ServerID)
	GetRaftStats() map[string]string
	CanAcceptRequest() bool
}

func NewNode(
	ctx context.Context,
	logger hclog.Logger,
	scheduler0Config config.Scheduler0Config,
	scheduler0Secrets secrets.Scheduler0Secrets,
	fsmStore fsm.Scheduler0RaftStore,
	fsmActions fsm.Scheduler0RaftActions,
	jobExecutor executor.JobExecutorService,
	jobQueue queue.JobQueueService,
	jobProcessor processor.JobProcessorService,
	jobRepo job.JobRepo,
	sharedRepo shared_repo.SharedRepo,
	jobExecutionRepo job_execution.JobExecutionsRepo,
	asyncTaskManager async_task_service.AsyncTaskService,
	dispatcher *utils.Dispatcher,
	nodeHTTPClient NodeClient,
	postProcessingChannel chan models.PostProcess,
	isExistingNode bool,
) NodeService {
	nodeServiceLogger := logger.Named("node-service")
	numReplicas := len(scheduler0Config.GetConfigurations().Replicas)

	nodeServiceLogger.Info("Initializing Node Service")

	return &nodeService{
		logger:                nodeServiceLogger,
		acceptClientWrites:    false,
		ctx:                   ctx,
		jobProcessor:          jobProcessor,
		jobQueue:              jobQueue,
		jobExecutor:           jobExecutor,
		jobRepo:               jobRepo,
		jobExecutionRepo:      jobExecutionRepo,
		isExistingNode:        isExistingNode,
		peerObserverChannels:  make(chan raft.Observation, numReplicas),
		asyncTaskManager:      asyncTaskManager,
		dispatcher:            dispatcher,
		fanIns:                sync.Map{},
		fanInCh:               make(chan models.PeerFanIn),
		acceptRequest:         false,
		scheduler0Config:      scheduler0Config,
		scheduler0Secrets:     scheduler0Secrets,
		scheduler0RaftActions: fsmActions,
		scheduler0RaftStore:   fsmStore,
		sharedRepo:            sharedRepo,
		nodeHTTPClient:        nodeHTTPClient,
		postProcessingChannel: postProcessingChannel,
	}
}

func (node *nodeService) Start() {
	node.logger.Info("Staring Node Service")

	if node.isExistingNode {
		node.logger.Info("discovered existing raft dir")
		node.scheduler0RaftStore.RecoverRaftState()
	}

	configs := node.scheduler0Config.GetConfigurations()

	if configs.Bootstrap && !node.isExistingNode {
		cfg := node.authRaftConfiguration()
		node.scheduler0RaftStore.BootstrapRaftClusterWithConfig(cfg)
	}
	myObserver := raft.NewObserver(node.peerObserverChannels, true, func(o *raft.Observation) bool {
		_, peerObservation := o.Data.(raft.PeerObservation)
		_, resumedHeartbeatObservation := o.Data.(raft.ResumedHeartbeatObservation)
		return peerObservation || resumedHeartbeatObservation
	})

	node.scheduler0RaftStore.RegisterObserver(myObserver)
	node.beginAcceptingClientRequest()

	go node.listenOnInputQueues()
}

func (node *nodeService) GetUncommittedLogs(requestId string) {
	taskId, addErr := node.asyncTaskManager.AddTasks("", requestId, constants.JobExecutorAsyncTaskService)
	if addErr != nil {
		node.logger.Error("failed to add new async task for job_executor", "error", addErr)
	}
	node.logger.Debug("added a new task for job_executor, task id", "task-id", taskId)

	node.dispatcher.NoBlockQueue(func(successChannel chan any, errorChannel chan any) {
		defer func() {
			close(successChannel)
			close(errorChannel)
		}()

		err := node.asyncTaskManager.UpdateTasksByRequestId(requestId, models.AsyncTaskInProgress, "")
		if err != nil {
			node.logger.Error("failed to update async task status with request id", requestId, ", error", err)
			return
		}
		uncommittedLogs := node.jobExecutor.GetUncommittedLogs()
		uncommittedTasks, err := node.asyncTaskManager.GetUnCommittedTasks()
		if err != nil {
			node.logger.Error("failed get uncommitted async tasks request id", requestId, ", error", err.Message)
			uErr := node.asyncTaskManager.UpdateTasksByRequestId(requestId, models.AsyncTaskFail, "")
			if uErr != nil {
				node.logger.Error("failed to update async task status with request id", "request id", requestId, ", error", uErr)
			}
			return
		}
		localData := models.LocalData{
			ExecutionLogs: uncommittedLogs,
			AsyncTasks:    uncommittedTasks,
		}
		data, mErr := json.Marshal(localData)
		if mErr != nil {
			node.logger.Error("failed to marshal async task result with request id", requestId, ", error", mErr)
			uErr := node.asyncTaskManager.UpdateTasksByRequestId(requestId, models.AsyncTaskFail, "")
			if uErr != nil {
				node.logger.Error("failed to update async task status with request id", requestId, ", error", uErr)
			}
			return
		}
		uErr := node.asyncTaskManager.UpdateTasksByRequestId(requestId, models.AsyncTaskSuccess, string(data))
		if uErr != nil {
			node.logger.Error("failed to update async task status with request id", requestId, ", error", uErr)
			return
		}
	})
}

func (node *nodeService) CanAcceptClientWriteRequest() bool {
	return node.acceptClientWrites
}

func (node *nodeService) CanAcceptRequest() bool {
	return node.acceptRequest
}

func (node *nodeService) StopJobs() {
	node.jobExecutor.StopAll()
}

func (node *nodeService) StartJobs() {
	node.jobProcessor.RecoverJobs()
}

func (node *nodeService) GetRaftStats() map[string]string {
	return node.scheduler0RaftStore.GetRaftStats()
}

func (node *nodeService) GetRaftLeaderWithId() (raft.ServerAddress, raft.ServerID) {
	return node.scheduler0RaftStore.LeaderWithID()
}

func (node *nodeService) authenticateWithPeersInConfig() map[string]Status {
	node.logger.Info("authenticating with nodes...")

	configs := node.scheduler0Config.GetConfigurations()
	var wg sync.WaitGroup

	results := map[string]Status{}
	wrlck := sync.Mutex{}

	for _, replica := range configs.Replicas {
		if replica.Address != utils.GetServerHTTPAddress() {
			wg.Add(1)
			go func(rep config.RaftNode, res map[string]Status, wg *sync.WaitGroup, wrlck *sync.Mutex) {
				wrlck.Lock()
				err := utils.RetryOnError(func() error {
					if peerStatus, err := node.nodeHTTPClient.ConnectNode(rep); err == nil {
						results[rep.Address] = *peerStatus
						node.logger.Info("successfully authenticated with", "node-address", rep.RaftAddress)
					} else {
						return err
					}

					return nil
				}, configs.PeerConnectRetryMax, configs.PeerConnectRetryDelaySeconds)
				wg.Done()
				wrlck.Unlock()
				if err != nil {
					node.logger.Error("failed to authenticate with peer ", "replica address", rep.Address, " error:", err)
				}
			}(replica, results, &wg, &wrlck)
		}
	}
	wg.Wait()

	return results
}

func (node *nodeService) stopAllJobsOnAllWorkerNodes() {
	node.logger.Info("stopping jobs on worker nodes.")

	configs := node.scheduler0Config.GetConfigurations()
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, constants.DefaultMaxConnectedPeers)

	for _, replica := range configs.Replicas {
		if replica.Address != utils.GetServerHTTPAddress() {
			wg.Add(1)
			go func(rep config.RaftNode, wg *sync.WaitGroup) {
				err := utils.RetryOnError(func() error {
					semaphore <- struct{}{}
					defer func() { <-semaphore }()
					return node.nodeHTTPClient.StopJobs(node.ctx, node, rep)
				}, constants.DefaultRetryMaxConfig, constants.DefaultRetryIntervalConfig)
				wg.Done()
				if err != nil {
					node.logger.Error("failed to stop jobs on worker node", "address", rep.Address, " error:", err)
				}
			}(replica, &wg)
		}
	}
	wg.Wait()
	node.logger.Error("completed stopping jobs on worker nodes")
}

func (node *nodeService) startJobsOnWorkerNodes() {
	node.logger.Info("starting jobs on worker nodes.")

	configs := node.scheduler0Config.GetConfigurations()
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, constants.DefaultMaxConnectedPeers)

	for _, replica := range configs.Replicas {
		if replica.Address != utils.GetServerHTTPAddress() {
			wg.Add(1)
			go func(rep config.RaftNode, wg *sync.WaitGroup) {
				err := utils.RetryOnError(func() error {
					semaphore <- struct{}{}
					defer func() { <-semaphore }()
					return node.nodeHTTPClient.StartJobs(node.ctx, node, rep)
				}, constants.DefaultRetryMaxConfig, constants.DefaultRetryIntervalConfig)
				wg.Done()
				if err != nil {
					node.logger.Error("failed to stop jobs on worker node", "address", rep.Address, " error:", err)
				}
			}(replica, &wg)
		}
	}
	wg.Wait()
	node.logger.Error("completed starting jobs on worker nodes")
}

func (node *nodeService) handleRaftLeadershipChanges(isLeader bool) {
	node.stopAcceptingClientWriteRequest()
	servers := node.scheduler0RaftStore.GetServersOnRaftCluster()
	nodeIds := []uint64{}
	for _, server := range servers {
		nodeId, err := utils.GetNodeIdWithRaftAddress(server.Address)
		if err != nil {
			log.Fatalln("failed to handle raft leadership changes because it failed to get node id for raft address", server.Address)
		}
		nodeIds = append(nodeIds, uint64(nodeId))
	}
	node.jobQueue.RemoveServers(nodeIds)

	if isLeader {
		node.jobQueue.AddServers(nodeIds)
		node.stopAllJobsOnAllWorkerNodes()
		singleNodeMode := len(servers) == 1
		node.jobQueue.SetSingleNodeMode(singleNodeMode)
		node.jobExecutor.SetSingleNodeMode(singleNodeMode)
		node.SingleNodeMode = singleNodeMode
		node.asyncTaskManager.SetSingleNodeMode(singleNodeMode)
		if !node.SingleNodeMode {
			uncommittedLogs := node.jobExecutor.GetUncommittedLogs()
			uncommittedAsyncTasks, err := node.asyncTaskManager.GetUnCommittedTasks()
			if err != nil {
				log.Fatalln("failed to get uncommitted async tasks after leader selection", "error", err.Error())
			}
			if len(uncommittedLogs) > 0 {
				node.jobExecutionRepo.RaftInsertExecutionLogs(uncommittedLogs, node.scheduler0Config.GetConfigurations().NodeId)
			}

			if len(uncommittedAsyncTasks) > 0 {
				_, err := node.asyncTaskRepo.RaftBatchInsert(uncommittedAsyncTasks, node.scheduler0Config.GetConfigurations().NodeId)
				if err != nil {
					node.logger.Error("failed to insert uncommitted async tasks from", "raft-leader", "error", err)
				}
			}

			node.handleUncommittedAsyncTasks(uncommittedAsyncTasks)
			node.fanInLocalDataFromPeers()
		} else {
			if node.isExistingNode {
				node.jobProcessor.StartJobs()
			} else {
				node.jobProcessor.StartJobs()
			}
			node.beginAcceptingClientWriteRequest()
		}
	}
}

func (node *nodeService) handleRaftObserverChannelChanges(o raft.Observation) {
	peerObservation, isPeerObservation := o.Data.(raft.PeerObservation)
	resumedHeartbeatObservation, isResumedHeartbeatObservation := o.Data.(raft.ResumedHeartbeatObservation)

	if isPeerObservation && !peerObservation.Removed {
		node.logger.Debug("A new node joined the cluster")
	}
	if isPeerObservation && peerObservation.Removed {
		node.logger.Debug("A node got removed from the cluster")
	}
	if isResumedHeartbeatObservation {
		node.logger.Debug(fmt.Sprintf("A node resumed execution. Peer ID %s ", string(resumedHeartbeatObservation.PeerID)))
		node.startJobsOnWorkerNodes()
	}
}

func (node *nodeService) handleUncommittedAsyncTasks(asyncTasks []models.AsyncTask) {
	for _, asyncTask := range asyncTasks {
		if asyncTask.State == models.AsyncTaskNotStated || asyncTask.State == models.AsyncTaskInProgress && asyncTask.Service == constants.CreateJobAsyncTaskService {
			var jobsPayload []models.Job
			err := json.Unmarshal([]byte(asyncTask.Input), &jobsPayload)
			if err != nil {
				node.logger.Error("failed to convert jobs payload from async task with id", "id", asyncTask.Id, "error", err.Error())
			}
			jobIds, batchInsertErr := node.jobRepo.BatchInsertJobs(jobsPayload)
			if batchInsertErr != nil {
				node.logger.Error("failed to create jobs from async task with id", "id", asyncTask.Id, "error", batchInsertErr.Error())
			}
			resObj := utils.Response{Data: jobIds, Success: true}
			updateTaskErr := node.asyncTaskManager.UpdateTasksByRequestId(asyncTask.RequestId, models.AsyncTaskSuccess, string(resObj.ToJSON()))
			if updateTaskErr != nil {
				node.logger.Error("failed to update state of uncommitted async task", "error", updateTaskErr)
			}
			node.logger.Info("successfully created jobs from async task with id", "id", asyncTask.Id, "job-ids", jobIds)
		}
		if asyncTask.State == models.AsyncTaskInProgress && asyncTask.Service == constants.JobExecutorAsyncTaskService {
			err := node.asyncTaskManager.UpdateTasksByRequestId(asyncTask.RequestId, models.AsyncTaskSuccess, "")
			if err != nil {
				node.logger.Error("failed to update state of uncommitted job executor async tasks to success", "error", err.Message)
			}
		}
	}
}

func (node *nodeService) handleCompletedPeerFanIn(peerFanIn models.PeerFanIn) {
	configs := node.scheduler0Config.GetConfigurations()
	node.completedFanInCh.Store(peerFanIn.PeerHTTPAddress, peerFanIn)
	completed := 0
	node.completedFanInCh.Range(func(key, value any) bool {
		completed += 1
		return true
	})
	if completed == len(configs.Replicas)-1 && !node.CanAcceptClientWriteRequest() {
		node.jobProcessor.StartJobs()
		node.beginAcceptingClientWriteRequest()
	}
}

func (node *nodeService) authRaftConfiguration() raft.Configuration {
	node.mtx.Lock()
	defer node.mtx.Unlock()

	configs := node.scheduler0Config.GetConfigurations()
	results := node.authenticateWithPeersInConfig()
	servers := []raft.Server{
		{
			ID:       raft.ServerID(strconv.FormatUint(configs.NodeId, 10)),
			Suffrage: raft.Voter,
			Address:  raft.ServerAddress(configs.RaftAddress),
		},
	}

	for _, replica := range configs.Replicas {
		if repStatus, ok := results[replica.Address]; ok && repStatus.IsAlive && repStatus.IsAuth {
			servers = append(servers, raft.Server{
				ID:       raft.ServerID(strconv.FormatUint(replica.NodeId, 10)),
				Suffrage: raft.Voter,
				Address:  raft.ServerAddress(replica.RaftAddress),
			})
		}
	}

	cfg := raft.Configuration{
		Servers: servers,
	}

	return cfg
}

func (node *nodeService) getRandomFanInPeerHTTPAddresses(excludeList map[string]bool) []string {
	configs := node.scheduler0Config.GetConfigurations()
	numReplicas := len(configs.Replicas)
	servers := make([]raft.ServerAddress, 0, numReplicas)
	httpAddresses := make([]string, 0, numReplicas)

	for _, server := range node.scheduler0RaftStore.GetServersOnRaftCluster() {
		if string(server.Address) != configs.RaftAddress {
			servers = append(servers, server.Address)
		}
	}

	if uint64(len(servers)) < configs.ExecutionLogFetchFanIn {
		for _, server := range servers {
			if ok := excludeList[utils.GetNodeServerAddressWithRaftAddress(server)]; !ok {
				httpAddresses = append(httpAddresses, utils.GetNodeServerAddressWithRaftAddress(server))
			}
		}
	} else {
		shuffledServers := servers
		copy(shuffledServers, servers)
		lastIndex := len(shuffledServers) - 1

		for lastIndex > 0 {
			randInt := rand.Intn(lastIndex)
			temp := shuffledServers[lastIndex]
			shuffledServers[lastIndex] = shuffledServers[randInt]
			shuffledServers[randInt] = temp
			lastIndex -= 1
		}

		for i := 0; i < len(shuffledServers); i++ {
			if _, ok := excludeList[utils.GetNodeServerAddressWithRaftAddress(shuffledServers[i])]; !ok {
				httpAddresses = append(httpAddresses, utils.GetNodeServerAddressWithRaftAddress(shuffledServers[i]))
			}
		}
	}
	if len(httpAddresses) > int(configs.ExecutionLogFetchFanIn) {
		return httpAddresses[:configs.ExecutionLogFetchFanIn]
	}

	return httpAddresses
}

func (node *nodeService) listenOnInputQueues() {
	node.logger.Info("begin listening on input channels")

	for {
		select {
		case isLeader := <-node.scheduler0RaftStore.GetLeaderChangeChannel():
			go node.handleRaftLeadershipChanges(isLeader)
		case o := <-node.peerObserverChannels:
			go node.handleRaftObserverChannelChanges(o)
		case peerFanIn := <-node.fanInCh:
			go node.handleCompletedPeerFanIn(peerFanIn)
		case postProcess := <-node.postProcessingChannel:
			{
				for _, postProcessTargetNode := range postProcess.TargetNodes {
					if postProcessTargetNode == node.scheduler0Config.GetConfigurations().NodeId {
						switch postProcess.Action {
						case constants.CommandActionQueueJob:
							go node.jobExecutor.QueueExecutions(postProcess.Data.LastInsertedId, postProcess.Data.RowsAffected)
						case constants.CommandActionCleanUncommittedAsyncTasksLogs:
							go node.asyncTaskManager.DeleteNewUncommittedAsyncLogs(postProcess.Data.LastInsertedId, postProcess.Data.RowsAffected)
						case constants.CommandActionCleanUncommittedExecutionLogs:
							go node.jobExecutor.DeleteNewUncommittedExecutionLogs(postProcess.Data.LastInsertedId, postProcess.Data.RowsAffected)
						}
					}
				}
			}
		case <-node.ctx.Done():
			return
		}
	}
}

func (node *nodeService) beginAcceptingClientWriteRequest() {
	node.acceptClientWrites = true
	node.logger.Info("ready to accept write requests")
}

func (node *nodeService) stopAcceptingClientWriteRequest() {
	node.acceptClientWrites = false
	node.logger.Info("stopped accepting httpClient write requests")
}

func (node *nodeService) beginAcceptingClientRequest() {
	node.acceptRequest = true
	node.logger.Info("being accepting httpClient requests")
}

func (node *nodeService) selectRandomPeersToFanIn() []models.PeerFanIn {
	excludeList := map[string]bool{}
	node.fanIns.Range(func(key, value any) bool {
		excludeList[key.(string)] = true
		return true
	})
	httpAddresses := node.getRandomFanInPeerHTTPAddresses(excludeList)
	peerFanIns := make([]models.PeerFanIn, 0, len(httpAddresses))
	for _, httpAddress := range httpAddresses {
		_, ok := node.fanIns.Load(httpAddress)
		if !ok {
			newFanIn := models.PeerFanIn{
				PeerHTTPAddress: httpAddress,
				State:           models.PeerFanInStateNotStated,
			}
			node.fanIns.Store(httpAddress, newFanIn)
			peerFanIns = append(peerFanIns, newFanIn)
		}
	}
	return peerFanIns
}

func (node *nodeService) fanInLocalDataFromPeers() {
	go func() {
		configs := node.scheduler0Config.GetConfigurations()
		ticker := time.NewTicker(time.Duration(configs.ExecutionLogFetchIntervalSeconds) * time.Second)
		ctx, cancelFunc := context.WithCancel(node.ctx)

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

				peers := node.selectRandomPeersToFanIn()

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

				node.fanIns.Range(func(key, value any) bool {
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
					go node.nodeHTTPClient.FetchUncommittedLogsFromPeersPhase1(ctx, node, phase1)
				}
				if len(phase2) > 0 {
					go node.nodeHTTPClient.FetchUncommittedLogsFromPeersPhase2(ctx, node, phase2)
				}
				if len(phase3) > 0 {
					go node.commitFetchedUnCommittedLogs(phase3)
				}
			case <-node.ctx.Done():
				cancelFunc()
				return
			}
		}
	}()
}

func (node *nodeService) commitFetchedUnCommittedLogs(peerFanIns []models.PeerFanIn) {
	for _, peerFanIn := range peerFanIns {

		if len(peerFanIn.Data.ExecutionLogs) > 0 {
			node.jobExecutionRepo.RaftInsertExecutionLogs(peerFanIn.Data.ExecutionLogs, node.scheduler0Config.GetConfigurations().NodeId)
		}

		if len(peerFanIn.Data.AsyncTasks) > 0 {
			_, err := node.asyncTaskRepo.RaftBatchInsert(peerFanIn.Data.AsyncTasks, node.scheduler0Config.GetConfigurations().NodeId)
			if err != nil {
				node.logger.Error("failed to insert uncommitted async tasks from", "peer", peerFanIn.PeerHTTPAddress, "error", err)
			}
		}

		node.fanIns.Delete(peerFanIn.PeerHTTPAddress)
		node.fanInCh <- peerFanIn
	}
}
