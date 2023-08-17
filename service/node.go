package service

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	boltdb "github.com/hashicorp/raft-boltdb/v2"
	"io"
	"log"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"scheduler0/config"
	"scheduler0/constants"
	"scheduler0/db"
	"scheduler0/fsm"
	"scheduler0/models"
	"scheduler0/network"
	"scheduler0/repository"
	"scheduler0/secrets"
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

const (
	Cold          State = 0
	Bootstrapping       = 1
)

type Node struct {
	TransportManager      *raft.NetworkTransport
	LogDb                 *boltdb.BoltStore
	StoreDb               *boltdb.BoltStore
	FileSnapShot          *raft.FileSnapshotStore
	acceptClientWrites    bool
	acceptRequest         bool
	State                 State
	FsmStore              fsm.Scheduler0RaftStore
	SingleNodeMode        bool
	ctx                   context.Context
	logger                hclog.Logger
	mtx                   sync.Mutex
	jobProcessor          *JobProcessor
	jobQueue              *JobQueue
	jobExecutor           *JobExecutor
	jobQueuesRepo         repository.JobQueuesRepo
	jobRepo               repository.JobRepo
	projectRepo           repository.ProjectRepo
	sharedRepo            shared_repo.SharedRepo
	isExistingNode        bool
	peerObserverChannels  chan raft.Observation
	asyncTaskManager      *AsyncTaskManager
	dispatcher            *utils.Dispatcher
	fanIns                sync.Map // models.PeerFanIn
	fanInCh               chan models.PeerFanIn
	scheduler0Config      config.Scheduler0Config
	scheduler0Secrets     secrets.Scheduler0Secrets
	scheduler0RaftActions fsm.Scheduler0RaftActions
	nodeHTTPClient        NodeClient
}

func NewNode(
	ctx context.Context,
	logger hclog.Logger,
	scheduler0Config config.Scheduler0Config,
	scheduler0Secrets secrets.Scheduler0Secrets,
	fsmActions fsm.Scheduler0RaftActions,
	jobExecutor *JobExecutor,
	jobQueue *JobQueue,
	jobRepo repository.JobRepo,
	projectRepo repository.ProjectRepo,
	executionsRepo repository.JobExecutionsRepo,
	jobQueueRepo repository.JobQueuesRepo,
	sharedRepo shared_repo.SharedRepo,
	asyncTaskManager *AsyncTaskManager,
	dispatcher *utils.Dispatcher,
	nodeHTTPClient NodeClient,
) *Node {
	nodeServiceLogger := logger.Named("node-service")

	dirPath := fmt.Sprintf("%v", constants.RaftDir)
	dirPath, exists := utils.MakeDirIfNotExist(dirPath)
	configs := scheduler0Config.GetConfigurations()
	dirPath = fmt.Sprintf("%v/%v", constants.RaftDir, configs.NodeId)
	utils.MakeDirIfNotExist(dirPath)
	tm, ldb, sdb, fss, err := getLogsAndTransport(scheduler0Config)
	if err != nil {
		log.Fatal("failed essentials for Node", err)
	}

	numReplicas := len(configs.Replicas)

	return &Node{
		TransportManager:      tm,
		LogDb:                 ldb,
		StoreDb:               sdb,
		FileSnapShot:          fss,
		logger:                nodeServiceLogger,
		acceptClientWrites:    false,
		State:                 Cold,
		ctx:                   ctx,
		jobProcessor:          NewJobProcessor(ctx, nodeServiceLogger, scheduler0Config, jobRepo, projectRepo, jobQueue, jobExecutor, executionsRepo, jobQueueRepo),
		jobQueue:              jobQueue,
		jobExecutor:           jobExecutor,
		jobRepo:               jobRepo,
		isExistingNode:        exists,
		peerObserverChannels:  make(chan raft.Observation, numReplicas),
		asyncTaskManager:      asyncTaskManager,
		dispatcher:            dispatcher,
		fanIns:                sync.Map{},
		fanInCh:               make(chan models.PeerFanIn),
		acceptRequest:         false,
		scheduler0Config:      scheduler0Config,
		scheduler0Secrets:     scheduler0Secrets,
		scheduler0RaftActions: fsmActions,
		sharedRepo:            sharedRepo,
		nodeHTTPClient:        nodeHTTPClient,
	}
}

func (node *Node) Boostrap() {
	node.State = Bootstrapping

	if node.isExistingNode {
		node.logger.Info("discovered existing raft dir")
		node.recoverRaftState()
	}

	configs := node.scheduler0Config.GetConfigurations()
	fmt.Println("node.FsmStore", node.FsmStore)
	rft := node.newRaft(node.FsmStore.GetFSM())
	if configs.Bootstrap && !node.isExistingNode {
		node.bootstrapRaftCluster(rft)
	}
	node.FsmStore.UpdateRaft(rft)
	node.jobExecutor.Raft = rft
	go node.handleLeaderChange()

	myObserver := raft.NewObserver(node.peerObserverChannels, true, func(o *raft.Observation) bool {
		_, peerObservation := o.Data.(raft.PeerObservation)
		_, resumedHeartbeatObservation := o.Data.(raft.ResumedHeartbeatObservation)
		return peerObservation || resumedHeartbeatObservation
	})

	rft.RegisterObserver(myObserver)
	go node.listenToObserverChannel()
	go node.listenToFanInChannel()
	node.beginAcceptingClientRequest()
	node.listenOnInputQueues(node.FsmStore)
}

func (node *Node) ReturnUncommittedLogs(requestId string) {
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

func (node *Node) CanAcceptClientWriteRequest() bool {
	return node.acceptClientWrites
}

func (node *Node) CanAcceptRequest() bool {
	return node.acceptRequest
}

func (node *Node) commitLocalData(peerAddress string, localData models.LocalData) {
	if node.FsmStore.GetRaft() == nil {
		log.Fatalln("raft is not set on job executors")
	}

	params := models.CommitLocalData{
		Data: localData,
	}

	node.logger.Debug("committing local data from peer",
		"address", peerAddress,
		"number of async tasks", len(localData.AsyncTasks),
		"number of execution logs", len(localData.ExecutionLogs),
	)

	nodeId, err := utils.GetNodeIdWithServerAddress(peerAddress)
	if err != nil {
		log.Fatalln("failed to get node id for raft address", peerAddress)
	}

	_, writeError := node.scheduler0RaftActions.WriteCommandToRaftLog(
		node.FsmStore.GetRaft(),
		constants.CommandTypeLocalData,
		"",
		uint64(nodeId),
		[]interface{}{params},
	)
	if writeError != nil {
		node.logger.Error("failed to apply local data into raft store", "error", writeError)
	}
}

func (node *Node) newRaft(fsm raft.FSM) *raft.Raft {
	logger := log.New(os.Stderr, "[creating-new-Node-raft] ", log.LstdFlags)

	configs := node.scheduler0Config.GetConfigurations()

	c := raft.DefaultConfig()
	c.LocalID = raft.ServerID(strconv.FormatUint(configs.NodeId, 10))

	// Set raft configs from scheduler0Configurations if available, otherwise use default values
	if configs.RaftHeartbeatTimeout != 0 {
		c.HeartbeatTimeout = time.Millisecond * time.Duration(configs.RaftHeartbeatTimeout)
	}
	if configs.RaftElectionTimeout != 0 {
		c.ElectionTimeout = time.Millisecond * time.Duration(configs.RaftElectionTimeout)
	}
	if configs.RaftCommitTimeout != 0 {
		c.CommitTimeout = time.Millisecond * time.Duration(configs.RaftCommitTimeout)
	}
	if configs.RaftMaxAppendEntries != 0 {
		c.MaxAppendEntries = int(configs.RaftMaxAppendEntries)
	}

	if configs.RaftSnapshotInterval != 0 {
		c.SnapshotInterval = time.Second * time.Duration(configs.RaftSnapshotInterval)
	}
	if configs.RaftSnapshotThreshold != 0 {
		c.SnapshotThreshold = configs.RaftSnapshotThreshold
	}

	r, err := raft.NewRaft(c, fsm, node.LogDb, node.StoreDb, node.FileSnapShot, node.TransportManager)
	if err != nil {
		logger.Fatalln("failed to create raft object for Node", err)
	}
	return r
}

func (node *Node) bootstrapRaftCluster(r *raft.Raft) {
	logger := log.New(os.Stderr, "[boostrap-raft-cluster] ", log.LstdFlags)

	cfg := node.authRaftConfiguration()
	f := r.BootstrapCluster(cfg)
	if err := f.Error(); err != nil {
		logger.Fatalln("failed to bootstrap raft Node", err)
	}
}

func (node *Node) recoverRaftState() {
	logger := log.New(os.Stderr, "[recover-raft-state] ", log.LstdFlags)

	var (
		snapshotIndex  uint64
		snapshotTerm   uint64
		snapshots, err = node.FileSnapShot.List()
	)
	if err != nil {
		logger.Fatalln("failed to load file snapshots", err)
	}

	node.logger.Debug(fmt.Sprintf("found %d snapshots", len(snapshots)))

	var lastSnapshotBytes []byte
	for _, snapshot := range snapshots {
		var source io.ReadCloser
		_, source, err = node.FileSnapShot.Open(snapshot.ID)
		if err != nil {
			// Skip this one and try the next. We will detect if we
			// couldn't open any snapshots.
			continue
		}

		lastSnapshotBytes, err = utils.BytesFromSnapshot(source)
		// Close the source after the restore has completed
		source.Close()
		if err != nil {
			// Same here, skip and try the next one.
			continue
		}

		snapshotIndex = snapshot.Index
		snapshotTerm = snapshot.Term
		break
	}
	if len(snapshots) > 0 && (snapshotIndex == 0 || snapshotTerm == 0) {
		logger.Println("failed to restore any of the available snapshots")
	}

	dir, err := os.Getwd()
	if err != nil {
		logger.Fatalln(fmt.Errorf("fatal error getting working dir: %s \n", err))
	}

	mainDbPath := fmt.Sprintf("%s/%s/%s", dir, constants.SqliteDir, constants.SqliteDbFileName)
	dataStore := db.NewSqliteDbConnection(node.logger, mainDbPath)
	dataStore.OpenConnectionToExistingDB()
	executionLogs, err := node.sharedRepo.GetExecutionLogs(dataStore, false)
	if err != nil {
		logger.Fatalln(fmt.Errorf("failed to get uncommitted execution logs: %s \n", err))
	}

	uncommittedTasks, err := node.asyncTaskManager.GetUnCommittedTasks()
	if err != nil {
		node.logger.Warn("failed to get uncommitted tasks", "error", err)
	}
	recoverDbPath := fmt.Sprintf("%s/%s/%s", dir, constants.SqliteDir, constants.RecoveryDbFileName)

	err = os.WriteFile(recoverDbPath, lastSnapshotBytes, os.ModePerm)
	if err != nil {
		logger.Fatalln(fmt.Errorf("fatal db file creation error: %s \n", err))
	}

	dataStore = db.NewSqliteDbConnection(node.logger, recoverDbPath)
	conn := dataStore.OpenConnectionToExistingDB()
	dbConnection := conn.(*sql.DB)
	defer dbConnection.Close()

	migrations := db.GetSetupSQL()
	_, err = dbConnection.Exec(migrations)
	if err != nil {
		logger.Fatalln(fmt.Errorf("fatal db file migrations error: %s \n", err))
	}

	fsmStr := fsm.NewFSMStore(node.logger, node.scheduler0RaftActions, dataStore)

	// The snapshot information is the best known end point for the data
	// until we play back the raft log entries.
	lastIndex := snapshotIndex
	lastTerm := snapshotTerm

	// Apply any raft log entries past the snapshot.
	lastLogIndex, err := node.LogDb.LastIndex()
	if err != nil {
		logger.Fatalf("failed to find last log: %v", err)
	}
	for index := snapshotIndex + 1; index <= lastLogIndex; index++ {
		var entry raft.Log
		if err = node.LogDb.GetLog(index, &entry); err != nil {
			logger.Fatalf("failed to get log at index %d: %v\n", index, err)
		}
		node.scheduler0RaftActions.ApplyRaftLog(
			node.logger,
			&entry,
			dataStore,
			false,
			nil,
			nil,
			nil,
		)
		lastIndex = entry.Index
		lastTerm = entry.Term
	}

	if len(executionLogs) > 0 {
		err = node.sharedRepo.InsertExecutionLogs(dataStore, false, executionLogs)
		if err != nil {
			logger.Fatal("failed to insert uncommitted execution logs", err)
		}
	}

	if len(uncommittedTasks) > 0 {
		err = node.sharedRepo.InsertAsyncTasksLogs(dataStore, false, uncommittedTasks)
		if err != nil {
			logger.Fatal("failed to insert uncommitted async task logs", err)
		}
	}

	lastConfiguration := node.getRaftConfiguration()

	snapshot := fsm.NewFSMSnapshot(fsmStr.GetDataStore())
	sink, err := node.FileSnapShot.Create(1, lastIndex, lastTerm, lastConfiguration, 1, node.TransportManager)
	if err != nil {
		logger.Fatalf("failed to create snapshot: %v", err)
	}
	if err = snapshot.Persist(sink); err != nil {
		logger.Fatalf("failed to persist snapshot: %v", err)
	}
	if err = sink.Close(); err != nil {
		logger.Fatalf("failed to finalize snapshot: %v", err)
	}

	firstLogIndex, err := node.LogDb.FirstIndex()
	if err != nil {
		logger.Fatalf("failed to get first log index: %v", err)
	}
	if err := node.LogDb.DeleteRange(firstLogIndex, lastLogIndex); err != nil {
		logger.Fatalf("log compaction failed: %v", err)
	}

	err = os.Remove(recoverDbPath)
	if err != nil {
		logger.Fatalf("failed to delete recovery db: %v", err)
	}
}

func (node *Node) authenticateWithPeersInConfig() map[string]Status {
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

func (node *Node) stopAllJobsOnAllNodes() {
	_, applyErr := node.scheduler0RaftActions.WriteCommandToRaftLog(
		node.FsmStore.GetRaft(),
		constants.CommandTypeStopJobs,
		"",
		0,
		nil,
	)
	if applyErr != nil {
		log.Fatalln("failed to apply job update states ", applyErr)
	}
}

func (node *Node) recoverJobsOnNode(peerId raft.ServerID) {
	num, err := strconv.ParseUint(string(peerId), 10, 32)
	if err != nil {
		panic(err)
	}
	peerIdNum := uint64(num)
	_, applyErr := node.scheduler0RaftActions.WriteCommandToRaftLog(
		node.FsmStore.GetRaft(),
		constants.CommandTypeRecoverJobs,
		"",
		peerIdNum,
		nil,
	)
	if applyErr != nil {
		log.Fatalln("failed to apply job update states ", applyErr)
	}
}

func (node *Node) handleLeaderChange() {
	select {
	case <-node.FsmStore.GetRaft().LeaderCh():
		node.stopAcceptingClientWriteRequest()
		node.stopAllJobsOnAllNodes()
		configuration := node.FsmStore.GetRaft().GetConfiguration().Configuration()
		servers := configuration.Servers
		nodeIds := []uint64{}
		for _, server := range servers {
			nodeId, err := utils.GetNodeIdWithRaftAddress(server.Address)
			if err != nil {
				log.Fatalln("failed to get node id for raft address", server.Address)
			}
			nodeIds = append(nodeIds, uint64(nodeId))
		}
		node.jobQueue.RemoveServers(nodeIds)
		node.jobQueue.AddServers(nodeIds)
		node.jobQueue.SingleNodeMode = len(servers) == 1
		node.jobExecutor.SingleNodeMode = node.jobQueue.SingleNodeMode
		node.SingleNodeMode = node.jobQueue.SingleNodeMode
		node.asyncTaskManager.SingleNodeMode = node.jobQueue.SingleNodeMode
		if !node.SingleNodeMode {
			uncommittedLogs := node.jobExecutor.GetUncommittedLogs()
			uncommittedAsyncTasks, err := node.asyncTaskManager.GetUnCommittedTasks()
			if err != nil {
				log.Fatalln("failed to get uncommitted async tasks after leader selection", "error", err.Error())
			}
			localData := models.LocalData{
				ExecutionLogs: uncommittedLogs,
				AsyncTasks:    uncommittedAsyncTasks,
			}
			node.commitLocalData(utils.GetServerHTTPAddress(), localData)
			node.handleUncommittedAsyncTasks(uncommittedAsyncTasks)
			go node.fanInLocalDataFromPeers()
		} else {
			if node.isExistingNode {
				node.jobProcessor.RecoverJobs()
			} else {
				node.jobProcessor.StartJobs()
			}
			node.beginAcceptingClientWriteRequest()
		}
	}
}

func (node *Node) handleUncommittedAsyncTasks(asyncTasks []models.AsyncTask) {
	for _, asyncTask := range asyncTasks {
		if asyncTask.State == models.AsyncTaskInProgress && asyncTask.Service == constants.CreateJobAsyncTaskService {
			var jobsPayload []models.Job
			err := json.Unmarshal([]byte(asyncTask.Input), &jobsPayload)
			if err != nil {
				node.logger.Error("failed to string -> json convert jobs payload from async task with id", "id", asyncTask.Id, "error", err.Error())
			}
			jobIds, err := node.jobRepo.BatchInsertJobs(jobsPayload)
			if err != nil {
				node.logger.Error("failed to create jobs from async task with id", "id", asyncTask.Id, "error", err.Error())
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

func (node *Node) listenOnInputQueues(fsmStr fsm.Scheduler0RaftStore) {
	node.logger.Info("begin listening input queues")

	for {
		select {
		case job := <-fsmStr.GetQueueJobsChannel():
			node.jobExecutor.QueueExecutions(job)
		case _ = <-fsmStr.GetRecoverJobsChannel():
			node.jobProcessor.RecoverJobs()
		case _ = <-fsmStr.GetStopAllJobsChannel():
			node.jobExecutor.StopAll()
		case <-node.ctx.Done():
			return
		}
	}
}

func (node *Node) authRaftConfiguration() raft.Configuration {
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

func (node *Node) getRaftConfiguration() raft.Configuration {
	node.mtx.Lock()
	defer node.mtx.Unlock()

	configs := node.scheduler0Config.GetConfigurations()
	servers := []raft.Server{
		{
			ID:       raft.ServerID(strconv.FormatUint(configs.NodeId, 10)),
			Suffrage: raft.Voter,
			Address:  raft.ServerAddress(configs.RaftAddress),
		},
	}

	for _, replica := range configs.Replicas {
		if replica.Address != utils.GetServerHTTPAddress() {
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

func (node *Node) getRandomFanInPeerHTTPAddresses(excludeList map[string]bool) []string {
	configs := node.scheduler0Config.GetConfigurations()
	numReplicas := len(configs.Replicas)
	servers := make([]raft.ServerAddress, 0, numReplicas)
	httpAddresses := make([]string, 0, numReplicas)

	for _, server := range node.FsmStore.GetRaft().GetConfiguration().Configuration().Servers {
		if string(server.Address) != configs.RaftAddress {
			servers = append(servers, server.Address)
		}
	}

	fmt.Println("servers", servers)
	fmt.Println("configs.ExecutionLogFetchFanIn", configs.ExecutionLogFetchFanIn)

	if uint64(len(servers)) < configs.ExecutionLogFetchFanIn {
		for _, server := range servers {
			if ok := excludeList[string(server)]; !ok {
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
	fmt.Println("httpAddresses", httpAddresses)

	if len(httpAddresses) > int(configs.ExecutionLogFetchFanIn) {
		return httpAddresses[:configs.ExecutionLogFetchFanIn]
	}

	return httpAddresses
}

func (node *Node) listenToObserverChannel() {
	for {
		select {
		case o := <-node.peerObserverChannels:
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
				node.recoverJobsOnNode(resumedHeartbeatObservation.PeerID)
			}
		case <-node.ctx.Done():
			return
		}
	}
}

func (node *Node) listenToFanInChannel() {
	configs := node.scheduler0Config.GetConfigurations()
	completedFanIns := make(map[string]models.PeerFanIn)
	mtx := sync.Mutex{}

	go func() {
		for {
			select {
			case peerFanIn := <-node.fanInCh:
				mtx.Lock()
				completedFanIns[peerFanIn.PeerHTTPAddress] = peerFanIn
				if len(completedFanIns) == len(configs.Replicas)-1 && !node.CanAcceptClientWriteRequest() {
					node.jobProcessor.StartJobs()
					node.beginAcceptingClientWriteRequest()
				}
				mtx.Unlock()
			case <-node.ctx.Done():
				return
			}
		}
	}()
}

func (node *Node) beginAcceptingClientWriteRequest() {
	node.acceptClientWrites = true
	node.logger.Info("ready to accept write requests")
}

func (node *Node) stopAcceptingClientWriteRequest() {
	node.acceptClientWrites = false
	node.logger.Info("stopped accepting client write requests")
}

func (node *Node) beginAcceptingClientRequest() {
	node.acceptRequest = true
	node.logger.Info("being accepting client requests")
}

func (node *Node) selectRandomPeersToFanIn() []models.PeerFanIn {
	excludeList := map[string]bool{}
	node.fanIns.Range(func(key, value any) bool {
		excludeList[key.(string)] = true
		return true
	})
	fmt.Println("excludeList", excludeList)
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

func (node *Node) fanInLocalDataFromPeers() {
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
}

func (node *Node) commitFetchedUnCommittedLogs(peerFanIns []models.PeerFanIn) {
	for _, peerFanIn := range peerFanIns {
		node.commitLocalData(peerFanIn.PeerHTTPAddress, peerFanIn.Data)
		node.fanIns.Delete(peerFanIn.PeerHTTPAddress)
		node.fanInCh <- peerFanIn
	}
}

func getLogsAndTransport(scheduler0Config config.Scheduler0Config) (tm *raft.NetworkTransport, ldb *boltdb.BoltStore, sdb *boltdb.BoltStore, fss *raft.FileSnapshotStore, err error) {
	logger := log.New(os.Stderr, "[get-raft-logs-and-transport] ", log.LstdFlags)

	configs := scheduler0Config.GetConfigurations()
	dirPath := fmt.Sprintf("%v/%v", constants.RaftDir, configs.NodeId)

	ldb, err = boltdb.NewBoltStore(filepath.Join(dirPath, constants.RaftLog))
	if err != nil {
		logger.Fatal("failed to create log store", err)
	}
	sdb, err = boltdb.NewBoltStore(filepath.Join(dirPath, constants.RaftStableLog))
	if err != nil {
		logger.Fatal("failed to create stable store", err)
	}
	fss, err = raft.NewFileSnapshotStore(dirPath, 3, os.Stderr)
	if err != nil {
		logger.Fatal("failed to create snapshot store", err)
	}
	ln, err := net.Listen("tcp", configs.RaftAddress)
	if err != nil {
		logger.Fatalf("failed to listen to tcp net. raft address %v. %v", configs.RaftAddress, err)
	}

	adv := network.NameAddress{
		Address: configs.NodeAdvAddress,
	}

	mux := network.NewMux(ln, adv)
	go func() {
		err := mux.Serve()
		if err != nil {
			logger.Fatal("failed mux serve", err)
		}
	}()

	muxLn := mux.Listen(1)

	tm = raft.NewNetworkTransport(network.NewTransport(muxLn), int(configs.RaftTransportMaxPool), time.Second*time.Duration(configs.RaftTransportTimeout), nil)
	return
}
