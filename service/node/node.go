package node

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	boltdb "github.com/hashicorp/raft-boltdb/v2"
	"google.golang.org/protobuf/proto"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"scheduler0/config"
	"scheduler0/constants"
	"scheduler0/constants/headers"
	"scheduler0/db"
	"scheduler0/fsm"
	"scheduler0/models"
	"scheduler0/network"
	"scheduler0/protobuffs"
	"scheduler0/repository"
	"scheduler0/secrets"
	"scheduler0/service/async_task_manager"
	"scheduler0/service/executor"
	"scheduler0/service/processor"
	"scheduler0/service/queue"
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
	TransportManager     *raft.NetworkTransport
	LogDb                *boltdb.BoltStore
	StoreDb              *boltdb.BoltStore
	FileSnapShot         *raft.FileSnapshotStore
	acceptClientWrites   bool
	acceptRequest        bool
	State                State
	FsmStore             *fsm.Store
	SingleNodeMode       bool
	ctx                  context.Context
	logger               hclog.Logger
	mtx                  sync.Mutex
	jobProcessor         *processor.JobProcessor
	jobQueue             *queue.JobQueue
	jobExecutor          *executor.JobExecutor
	jobQueuesRepo        repository.JobQueuesRepo
	jobRepo              repository.Job
	projectRepo          repository.Project
	isExistingNode       bool
	peerObserverChannels chan raft.Observation
	asyncTaskManager     *async_task_manager.AsyncTaskManager
	dispatcher           *utils.Dispatcher
	fanIns               sync.Map // models.PeerFanIn
	fanInCh              chan models.PeerFanIn
}

func NewNode(
	ctx context.Context,
	logger hclog.Logger,
	jobExecutor *executor.JobExecutor,
	jobQueue *queue.JobQueue,
	jobRepo repository.Job,
	projectRepo repository.Project,
	executionsRepo repository.ExecutionsRepo,
	jobQueueRepo repository.JobQueuesRepo,
	asyncTaskManager *async_task_manager.AsyncTaskManager,
	dispatcher *utils.Dispatcher,
) *Node {
	nodeServiceLogger := logger.Named("node-service")

	dirPath := fmt.Sprintf("%v", constants.RaftDir)
	dirPath, exists := utils.MakeDirIfNotExist(dirPath)
	configs := config.GetConfigurations()
	dirPath = fmt.Sprintf("%v/%v", constants.RaftDir, configs.NodeId)
	utils.MakeDirIfNotExist(dirPath)
	tm, ldb, sdb, fss, err := getLogsAndTransport()
	if err != nil {
		log.Fatal("failed essentials for Node", err)
	}

	numReplicas := len(configs.Replicas)

	return &Node{
		TransportManager:     tm,
		LogDb:                ldb,
		StoreDb:              sdb,
		FileSnapShot:         fss,
		logger:               nodeServiceLogger,
		acceptClientWrites:   false,
		State:                Cold,
		ctx:                  ctx,
		jobProcessor:         processor.NewJobProcessor(ctx, nodeServiceLogger, jobRepo, projectRepo, jobQueue, jobExecutor, executionsRepo, jobQueueRepo),
		jobQueue:             jobQueue,
		jobExecutor:          jobExecutor,
		jobRepo:              jobRepo,
		projectRepo:          projectRepo,
		isExistingNode:       exists,
		peerObserverChannels: make(chan raft.Observation, numReplicas),
		asyncTaskManager:     asyncTaskManager,
		dispatcher:           dispatcher,
		fanIns:               sync.Map{},
		fanInCh:              make(chan models.PeerFanIn),
		acceptRequest:        false,
	}
}

func (node *Node) Boostrap() {
	node.State = Bootstrapping

	if node.isExistingNode {
		node.logger.Info("discovered existing raft dir")
		node.recoverRaftState()
	}

	configs := config.GetConfigurations()
	rft := node.newRaft(node.FsmStore)
	if configs.Bootstrap && !node.isExistingNode {
		node.bootstrapRaftCluster(rft)
	}
	node.FsmStore.Raft = rft
	node.jobExecutor.Raft = node.FsmStore.Raft
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

func (node *Node) commitLocalData(peerAddress string, localData models.LocalData) {
	if node.FsmStore.Raft == nil {
		log.Fatalln("raft is not set on job executors")
	}

	params := models.CommitLocalData{
		Address: peerAddress,
		Data:    localData,
	}

	node.logger.Debug("committing local data from peer",
		"address", peerAddress,
		"number of async tasks", len(localData.AsyncTasks),
		"number of execution logs", len(localData.ExecutionLogs),
	)

	data, err := json.Marshal(params)
	if err != nil {
		log.Fatalln("failed to marshal json")
	}

	nodeId, err := utils.GetNodeIdWithServerAddress(peerAddress)
	if err != nil {
		log.Fatalln("failed to get node with id for peer address", peerAddress)
	}

	commitCommand := &protobuffs.Command{
		Type:       protobuffs.Command_Type(constants.CommandTypeLocalData),
		Sql:        peerAddress,
		Data:       data,
		TargetNode: uint64(nodeId),
	}

	commitCommandData, err := proto.Marshal(commitCommand)
	if err != nil {
		log.Fatalln("failed to marshal json")
	}

	configs := config.GetConfigurations()

	af := node.FsmStore.Raft.Apply(commitCommandData, time.Second*time.Duration(configs.RaftApplyTimeout)).(raft.ApplyFuture)
	if af.Error() != nil {
		node.logger.Error("failed to apply local data into raft store", "error", af.Error())
	}
}

func (node *Node) ReturnUncommittedLogs(requestId string) {
	taskId, addErr := node.asyncTaskManager.AddTasks("", requestId, constants.JobExecutorAsyncTaskService)
	if addErr != nil {
		node.logger.Error("failed to add new async task for job_executor", addErr)
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
			node.logger.Error("failed to uncommitted async tasks request id", requestId, ", error", err.Message)
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

func (node *Node) newRaft(fsm raft.FSM) *raft.Raft {
	logger := log.New(os.Stderr, "[creating-new-Node-raft] ", log.LstdFlags)

	configs := config.GetConfigurations()

	c := raft.DefaultConfig()
	c.LocalID = raft.ServerID(strconv.FormatUint(configs.NodeId, 10))

	// Set Raft configs from Scheduler0Configurations if available, otherwise use default values
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

func (node *Node) getUncommittedLogs() []models.JobExecutionLog {
	logger := log.New(os.Stderr, "[get-uncommitted-logs] ", log.LstdFlags)

	executionLogs := []models.JobExecutionLog{}

	configs := config.GetConfigurations()

	dir, err := os.Getwd()
	if err != nil {
		logger.Fatalln(fmt.Errorf("Fatal error getting working dir: %s \n", err))
	}

	mainDbPath := fmt.Sprintf("%s/%s/%s", dir, constants.SqliteDir, constants.SqliteDbFileName)
	dataStore := db.NewSqliteDbConnection(node.logger, mainDbPath)
	conn := dataStore.OpenConnection()
	dbConnection := conn.(*sql.DB)

	rows, err := dbConnection.Query(fmt.Sprintf(
		"select count(*) from %s",
		fsm.ExecutionsUnCommittedTableName,
	), configs.NodeId, false)
	if err != nil {
		logger.Fatalln("failed to query for the count of uncommitted logs", err.Error())
	}
	var count int64 = 0
	for rows.Next() {
		scanErr := rows.Scan(&count)
		if scanErr != nil {
			logger.Fatalln("failed to scan count value", scanErr.Error())
		}
	}
	if rows.Err() != nil {
		logger.Fatalln("rows error", rows.Err())
	}
	err = rows.Close()
	if err != nil {
		logger.Fatalln("failed to close rows", err)
	}

	rows, err = dbConnection.Query(fmt.Sprintf(
		"select max(id) as maxId, min(id) as minId from %s",
		fsm.ExecutionsUnCommittedTableName,
	), configs.NodeId, false)
	if err != nil {
		logger.Fatalln("failed to query for max and min id in uncommitted logs", err.Error())
	}

	node.logger.Debug(fmt.Sprintf("found %d uncommitted logs", count))

	if count < 1 {
		return executionLogs
	}

	var maxId int64 = 0
	var minId int64 = 0
	for rows.Next() {
		scanErr := rows.Scan(&maxId, &minId)
		if scanErr != nil {
			logger.Fatalln("failed to scan max and min id  on uncommitted logs", scanErr.Error())
		}
	}
	if rows.Err() != nil {
		logger.Fatalln("rows error", rows.Err())
	}
	err = rows.Close()
	if err != nil {
		logger.Fatalln("failed to close rows", err)
	}

	node.logger.Debug(fmt.Sprintf("found max id %d and min id %d in uncommitted jobs \n", maxId, minId))

	ids := []int64{}
	for i := minId; i <= maxId; i++ {
		ids = append(ids, i)
	}

	batches := utils.Batch[int64](ids, 7)

	for _, batch := range batches {
		batchIds := []interface{}{batch[0]}
		params := "?"

		for _, id := range batch[1:] {
			batchIds = append(batchIds, id)
			params += ",?"
		}

		rows, err = dbConnection.Query(fmt.Sprintf(
			"select  %s, %s, %s, %s, %s, %s, %s from %s where id in (%s)",
			fsm.ExecutionsUniqueIdColumn,
			fsm.ExecutionsStateColumn,
			fsm.ExecutionsNodeIdColumn,
			fsm.ExecutionsLastExecutionTimeColumn,
			fsm.ExecutionsNextExecutionTime,
			fsm.ExecutionsJobIdColumn,
			fsm.ExecutionsDateCreatedColumn,
			fsm.ExecutionsUnCommittedTableName,
			params,
		), batchIds...)
		if err != nil {
			logger.Fatalln("failed to query for the uncommitted logs", err.Error())
		}
		for rows.Next() {
			var jobExecutionLog models.JobExecutionLog
			scanErr := rows.Scan(
				&jobExecutionLog.UniqueId,
				&jobExecutionLog.State,
				&jobExecutionLog.NodeId,
				&jobExecutionLog.LastExecutionDatetime,
				&jobExecutionLog.NextExecutionDatetime,
				&jobExecutionLog.JobId,
				&jobExecutionLog.DataCreated,
			)
			if scanErr != nil {
				logger.Fatalln("failed to scan job execution columns", scanErr.Error())
			}
			executionLogs = append(executionLogs, jobExecutionLog)
		}
		err = rows.Close()
		if err != nil {
			logger.Fatalln("failed to close rows", err)
		}
	}
	err = dbConnection.Close()
	if err != nil {
		logger.Fatalln("failed to close database connection", err.Error())
	}

	return executionLogs
}

func (node *Node) insertUncommittedLogsIntoRecoverDb(executionLogs []models.JobExecutionLog, dbConnection *sql.DB) {
	logger := log.New(os.Stderr, "[insert-uncommitted-execution-logs-into-recoverDb] ", log.LstdFlags)

	executionLogsBatches := utils.Batch[models.JobExecutionLog](executionLogs, 9)

	for _, executionLogsBatch := range executionLogsBatches {
		query := fmt.Sprintf("INSERT INTO %s (%s, %s, %s, %s, %s, %s, %s, %s, %s) VALUES ",
			fsm.ExecutionsUnCommittedTableName,
			fsm.ExecutionsUniqueIdColumn,
			fsm.ExecutionsStateColumn,
			fsm.ExecutionsNodeIdColumn,
			fsm.ExecutionsLastExecutionTimeColumn,
			fsm.ExecutionsNextExecutionTime,
			fsm.ExecutionsJobIdColumn,
			fsm.ExecutionsDateCreatedColumn,
			fsm.ExecutionsJobQueueVersion,
			fsm.ExecutionsVersion,
		)

		query += "(?, ?, ?, ?, ?, ?, ?, ?, ?)"
		params := []interface{}{
			executionLogsBatch[0].UniqueId,
			executionLogsBatch[0].State,
			executionLogsBatch[0].NodeId,
			executionLogsBatch[0].LastExecutionDatetime,
			executionLogsBatch[0].NextExecutionDatetime,
			executionLogsBatch[0].JobId,
			executionLogsBatch[0].DataCreated,
			executionLogsBatch[0].JobQueueVersion,
			executionLogsBatch[0].ExecutionVersion,
		}

		for _, executionLog := range executionLogsBatch[1:] {
			params = append(params,
				executionLog.UniqueId,
				executionLog.State,
				executionLog.NodeId,
				executionLog.LastExecutionDatetime,
				executionLog.NextExecutionDatetime,
				executionLog.JobId,
				executionLog.DataCreated,
				executionLog.JobQueueVersion,
				executionLog.ExecutionVersion,
			)
			query += ",(?, ?, ?, ?, ?, ?, ?, ?, ?)"
		}

		query += ";"

		ctx := context.Background()
		tx, err := dbConnection.BeginTx(ctx, nil)
		if err != nil {
			logger.Fatalln("failed to create transaction for batch insertion", err)
		}
		_, err = tx.Exec(query, params...)
		if err != nil {
			trxErr := tx.Rollback()
			if trxErr != nil {
				logger.Fatalln("failed to rollback update transition", trxErr)
			}
			logger.Fatalln("failed to insert un committed executions to recovery db", err)
		}
		err = tx.Commit()
		if err != nil {
			logger.Fatalln("failed to commit transition", err)
		}
	}
}

func (node *Node) insertUncommittedAsyncTaskLogsIntoRecoveryDb(asyncTasks []models.AsyncTask, dbConnection *sql.DB) {
	logger := log.New(os.Stderr, "[insert-uncommitted-async-task-logs-into-recoverDb] ", log.LstdFlags)
	batches := utils.Batch[models.AsyncTask](asyncTasks, 5)

	for _, batch := range batches {
		query := fmt.Sprintf("INSERT INTO %s (%s, %s, %s, %s, %s, %s) VALUES (?, ?, ?, ?, ?, ?)",
			fsm.UnCommittedAsyncTableName,
			fsm.AsyncTasksRequestIdColumn,
			fsm.AsyncTasksInputColumn,
			fsm.AsyncTasksOutputColumn,
			fsm.AsyncTasksStateColumn,
			fsm.AsyncTasksServiceColumn,
			fsm.AsyncTasksDateCreatedColumn,
		)
		params := []interface{}{
			batch[0].RequestId,
			batch[0].Input,
			batch[0].Output,
			batch[0].State,
			batch[0].Service,
			batch[0].DateCreated,
		}

		for _, row := range batch[1:] {
			query += ",(?, ?, ?, ?, ?, ?)"
			params = append(params, row.RequestId, row.Input, row.Output, row.State, row.Service, row.DateCreated)
		}

		query += ";"

		_, err := dbConnection.Exec(query, params...)
		if err != nil {
			logger.Fatalln("failed to insert uncommitted async tasks", err.Error())
		}
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

	executionLogs := node.getUncommittedLogs()
	uncommittedTasks, err := node.asyncTaskManager.GetUnCommittedTasks()
	if err != nil {
		node.logger.Warn("failed to get uncommitted tasks", "error", err)
	}
	recoverDbPath := fmt.Sprintf("%s/%s/%s", dir, constants.SqliteDir, constants.RecoveryDbFileName)

	err = os.WriteFile(recoverDbPath, lastSnapshotBytes, os.ModePerm)
	if err != nil {
		logger.Fatalln(fmt.Errorf("fatal db file creation error: %s \n", err))
	}

	dataStore := db.NewSqliteDbConnection(node.logger, recoverDbPath)
	conn := dataStore.OpenConnection()
	dbConnection := conn.(*sql.DB)
	defer dbConnection.Close()

	migrations := db.GetSetupSQL()
	_, err = dbConnection.Exec(migrations)
	if err != nil {
		logger.Fatalln(fmt.Errorf("fatal db file migrations error: %s \n", err))
	}

	fsmStr := fsm.NewFSMStore(dataStore, node.logger)

	// The snapshot information is the best known end point for the data
	// until we play back the Raft log entries.
	lastIndex := snapshotIndex
	lastTerm := snapshotTerm

	// Apply any Raft log entries past the snapshot.
	lastLogIndex, err := node.LogDb.LastIndex()
	if err != nil {
		logger.Fatalf("failed to find last log: %v", err)
	}
	for index := snapshotIndex + 1; index <= lastLogIndex; index++ {
		var entry raft.Log
		if err = node.LogDb.GetLog(index, &entry); err != nil {
			logger.Fatalf("failed to get log at index %d: %v\n", index, err)
		}
		fsm.ApplyCommand(
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
		node.insertUncommittedLogsIntoRecoverDb(executionLogs, dbConnection)
	}

	if len(uncommittedTasks) > 0 {
		node.insertUncommittedAsyncTaskLogsIntoRecoveryDb(uncommittedTasks, dbConnection)
	}

	lastConfiguration := node.getRaftConfiguration()

	snapshot := fsm.NewFSMSnapshot(fsmStr.DataStore)
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

func (node *Node) authenticateWithPeersInConfig(logger hclog.Logger) map[string]Status {
	node.logger.Info("authenticating with nodes...")

	configs := config.GetConfigurations()
	var wg sync.WaitGroup

	results := map[string]Status{}
	wrlck := sync.Mutex{}

	for _, replica := range configs.Replicas {
		if replica.Address != utils.GetServerHTTPAddress() {
			wg.Add(1)
			go func(rep config.RaftNode, res map[string]Status, wg *sync.WaitGroup, wrlck *sync.Mutex) {
				wrlck.Lock()
				err := utils.RetryOnError(func() error {
					if peerStatus, err := connectNode(logger, rep); err == nil {
						results[rep.Address] = *peerStatus
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
	_, applyErr := fsm.AppApply(
		node.FsmStore.Raft,
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
	_, applyErr := fsm.AppApply(
		node.FsmStore.Raft,
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
	case <-node.FsmStore.Raft.LeaderCh():
		node.stopAcceptingClientWriteRequest()
		node.stopAllJobsOnAllNodes()
		configuration := node.FsmStore.Raft.GetConfiguration().Configuration()
		servers := configuration.Servers
		nodeIds := []uint64{}
		for _, server := range servers {
			nodeId, err := utils.GetNodeIdWithRaftAddress(server.Address)
			if err != nil {
				log.Fatalln("failed to get node with id for peer address", server.Address)
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
			var jobsPayload []models.JobModel
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

func (node *Node) listenOnInputQueues(fsmStr *fsm.Store) {
	node.logger.Info("begin listening input queues")

	for {
		select {
		case job := <-fsmStr.QueueJobsChannel:
			node.jobExecutor.QueueExecutions(job)
		case _ = <-fsmStr.RecoverJobs:
			node.jobProcessor.RecoverJobs()
		case _ = <-fsmStr.StopAllJobs:
			node.jobExecutor.StopAll()
		case <-node.ctx.Done():
			return
		}
	}
}

func (node *Node) authRaftConfiguration() raft.Configuration {
	node.mtx.Lock()
	defer node.mtx.Unlock()

	configs := config.GetConfigurations()
	results := node.authenticateWithPeersInConfig(node.logger)
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

	configs := config.GetConfigurations()
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

func (node *Node) getRandomFanInPeerHTTPAddresses() []string {
	configs := config.GetConfigurations()
	numReplicas := len(configs.Replicas)
	servers := make([]raft.ServerAddress, 0, numReplicas)
	httpAddresses := make([]string, 0, numReplicas)

	for _, server := range node.FsmStore.Raft.GetConfiguration().Configuration().Servers {
		if string(server.Address) != configs.RaftAddress {
			servers = append(servers, server.Address)
		}
	}

	if uint64(len(servers)) < configs.ExecutionLogFetchFanIn {
		for _, server := range servers {
			httpAddresses = append(httpAddresses, utils.GetNodeServerAddressWithRaftAddress(server))
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
			httpAddresses = append(httpAddresses, utils.GetNodeServerAddressWithRaftAddress(shuffledServers[i]))
		}
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
	configs := config.GetConfigurations()
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
}

func (node *Node) beginAcceptingClientRequest() {
	node.acceptRequest = true
	node.logger.Info("ready to accept requests")
}

func (node *Node) stopAcceptingClientRequest() {
	node.acceptRequest = false
}

func (node *Node) CanAcceptClientWriteRequest() bool {
	return node.acceptClientWrites
}

func (node *Node) CanAcceptRequest() bool {
	return node.acceptRequest
}

func (node *Node) selectRandomPeersToFanIn() []models.PeerFanIn {
	httpAddresses := node.getRandomFanInPeerHTTPAddresses()
	peerFanIns := make([]models.PeerFanIn, 0, len(httpAddresses))
	for _, httpAddress := range httpAddresses {
		var peerFanIn models.PeerFanIn
		storeFanIn, ok := node.fanIns.Load(httpAddress)
		if !ok {
			newFanIn := models.PeerFanIn{
				PeerHTTPAddress: httpAddress,
				State:           models.PeerFanInStateNotStated,
			}
			node.fanIns.Store(httpAddress, newFanIn)
			peerFanIn = newFanIn
			peerFanIns = append(peerFanIns, newFanIn)
		} else {
			peerFanIn = storeFanIn.(models.PeerFanIn)
			peerFanIns = append(peerFanIns, peerFanIn)
		}
	}
	return peerFanIns
}

func (node *Node) fanInLocalDataFromPeers() {
	configs := config.GetConfigurations()
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

			go node.fetchUncommittedLogsFromPeersPhase1(ctx, phase1)
			go node.fetchUncommittedLogsFromPeersPhase2(ctx, phase2)
			go node.commitFetchedUnCommittedLogs(phase3)
		case <-node.ctx.Done():
			cancelFunc()
			return
		}
	}
}

func (node *Node) fetchUncommittedLogsFromPeersPhase1(ctx context.Context, peerFanIns []models.PeerFanIn) {
	for _, peerFanIn := range peerFanIns {
		httpClient := &http.Client{}
		httpRequest, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%v/execution-logs", peerFanIn.PeerHTTPAddress), nil)
		if reqErr != nil {
			node.logger.Error("failed to create request to execution logs from", "node address", peerFanIn.PeerHTTPAddress, "error", reqErr.Error())
		} else {
			httpRequest.Header.Set(headers.PeerHeader, headers.PeerHeaderValue)
			httpRequest.Header.Set(headers.PeerAddressHeader, utils.GetServerHTTPAddress())
			credentials := secrets.GetSecrets()
			httpRequest.SetBasicAuth(credentials.AuthUsername, credentials.AuthPassword)
			res, err := httpClient.Do(httpRequest)
			if err != nil {
				node.logger.Error("failed to get uncommitted execution logs from", "node address", peerFanIn.PeerHTTPAddress, "error", err.Error())
			} else {
				if res.StatusCode == http.StatusAccepted {
					location := res.Header.Get("Location")
					peerFanIn.RequestId = location
					closeErr := res.Body.Close()
					if closeErr != nil {
						node.logger.Error("failed to close body", "error", closeErr.Error())
					}
					peerFanIn.State = models.PeerFanInStateGetRequestId
					node.fanIns.Store(peerFanIn.PeerHTTPAddress, peerFanIn)
					node.logger.Info("successfully fetch execution logs from", "node address", peerFanIn.PeerHTTPAddress)
				} else {
					node.logger.Error("failed to get uncommitted execution logs from", "node address", peerFanIn.PeerHTTPAddress, "state code", res.StatusCode)
				}
			}
		}
	}
}

func (node *Node) fetchUncommittedLogsFromPeersPhase2(ctx context.Context, peerFanIns []models.PeerFanIn) {
	for _, peerFanIn := range peerFanIns {
		httpClient := &http.Client{}
		httpRequest, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s%s", peerFanIn.PeerHTTPAddress, peerFanIn.RequestId), nil)
		if reqErr != nil {
			node.logger.Error("failed to create request to execution logs from", "node address", peerFanIn.PeerHTTPAddress, "error", reqErr.Error())
		} else {
			httpRequest.Header.Set(headers.PeerHeader, headers.PeerHeaderValue)
			httpRequest.Header.Set(headers.PeerAddressHeader, utils.GetServerHTTPAddress())
			credentials := secrets.GetSecrets()
			httpRequest.SetBasicAuth(credentials.AuthUsername, credentials.AuthPassword)
			res, err := httpClient.Do(httpRequest)
			if err != nil {
				node.logger.Error("failed to get uncommitted execution logs from", "node address", peerFanIn.PeerHTTPAddress, "error", err.Error())
			} else {
				if res.StatusCode == http.StatusOK {
					data, readErr := io.ReadAll(res.Body)
					if readErr != nil {
						node.logger.Error("failed to read uncommitted execution logs from", peerFanIn.PeerHTTPAddress, "error", readErr.Error())
					} else {
						var asyncTaskRes models.AsyncTaskRes
						marshalErr := json.Unmarshal([]byte(data), &asyncTaskRes)
						if marshalErr != nil {
							node.logger.Error("failed to read uncommitted execution logs from", "node address", peerFanIn.PeerHTTPAddress, "error", marshalErr.Error())
						} else {
							var localData models.LocalData
							marshalErr = json.Unmarshal([]byte(asyncTaskRes.Data.Output), &localData)
							if marshalErr != nil {
								node.logger.Error("failed to read uncommitted execution logs from", "node address", peerFanIn.PeerHTTPAddress, "error", marshalErr.Error())
							} else {
								peerFanIn.Data = localData
								closeErr := res.Body.Close()
								if closeErr != nil {
									node.logger.Error("failed to close body", "error", closeErr.Error())
								} else {
									peerFanIn.State = models.PeerFanInStateGetExecutionsLogs
									node.fanIns.Store(peerFanIn.PeerHTTPAddress, peerFanIn)
									node.logger.Info("successfully fetch execution logs from", "node address", peerFanIn.PeerHTTPAddress)
								}
							}
						}
					}
				} else {
					node.logger.Error("failed to get uncommitted execution logs from", "node address", peerFanIn.PeerHTTPAddress, "state code", res.StatusCode)
				}
			}
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

func connectNode(logger hclog.Logger, rep config.RaftNode) (*Status, error) {
	configs := config.GetConfigurations()
	httpClient := http.Client{
		Timeout: time.Duration(configs.PeerAuthRequestTimeoutMs) * time.Millisecond,
	}
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/peer-handshake", rep.Address), nil)
	if err != nil {
		logger.Error("failed to create request", "error", err)
		return nil, err
	}
	req.Header.Set(headers.PeerHeader, headers.PeerHeaderValue)
	req.Header.Set(headers.PeerAddressHeader, utils.GetServerHTTPAddress())
	credentials := secrets.GetSecrets()
	req.SetBasicAuth(credentials.AuthUsername, credentials.AuthPassword)

	start := time.Now()

	resp, err := httpClient.Do(req)
	if err != nil {
		logger.Error("failed to send request", "error", err.Error())
		return nil, err
	}
	defer resp.Body.Close()

	connectionTime := time.Since(start)

	if resp.StatusCode == http.StatusOK {
		data, ioErr := io.ReadAll(resp.Body)
		if ioErr != nil {
			logger.Error("failed to response", "error:", ioErr.Error())
			return nil, ioErr
		}

		body := Response{}

		unMarshalErr := json.Unmarshal(data, &body)
		if unMarshalErr != nil {
			logger.Error("failed to unmarshal response ", "error", unMarshalErr.Error())
			return nil, unMarshalErr
		}

		logger.Info("successfully authenticated", "replica-address", rep.Address)

		return &Status{
			IsAlive:            true,
			IsAuth:             true,
			IsLeader:           body.Data.IsLeader,
			LastConnectionTime: connectionTime,
		}, nil
	}

	logger.Error("could not authenticate", "replica-address", rep.Address, " status code:", resp.StatusCode)

	if resp.StatusCode == http.StatusUnauthorized {
		return &Status{
			IsAlive:            true,
			IsAuth:             false,
			IsLeader:           false,
			LastConnectionTime: connectionTime,
		}, nil
	}

	if resp.StatusCode == http.StatusServiceUnavailable {
		return &Status{
			IsAlive:            false,
			IsAuth:             false,
			IsLeader:           false,
			LastConnectionTime: connectionTime,
		}, nil
	}

	return &Status{
		IsAlive:            false,
		IsAuth:             false,
		IsLeader:           false,
		LastConnectionTime: connectionTime,
	}, nil
}

func getLogsAndTransport() (tm *raft.NetworkTransport, ldb *boltdb.BoltStore, sdb *boltdb.BoltStore, fss *raft.FileSnapshotStore, err error) {
	logger := log.New(os.Stderr, "[get-raft-logs-and-transport] ", log.LstdFlags)

	configs := config.GetConfigurations()
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
		logger.Fatal("failed to listen to tcp net", err)
	}

	mux := network.NewMux(ln)

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
