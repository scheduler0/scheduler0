package node

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
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
	"scheduler0/service/executor"
	"scheduler0/service/processor"
	"scheduler0/service/queue"
	"scheduler0/utils"
	"scheduler0/utils/batcher"
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
	AcceptWrites         bool
	State                State
	FsmStore             *fsm.Store
	SingleNodeMode       bool
	ctx                  context.Context
	logger               *log.Logger
	mtx                  sync.Mutex
	jobProcessor         *processor.JobProcessor
	jobQueue             *queue.JobQueue
	jobExecutor          *executor.JobExecutor
	jobQueuesRepo        repository.JobQueuesRepo
	jobRepo              repository.Job
	projectRepo          repository.Project
	isExistingNode       bool
	peerObserverChannels chan raft.Observation
}

func NewNode(
	ctx context.Context,
	logger *log.Logger,
	jobExecutor *executor.JobExecutor,
	jobQueue *queue.JobQueue,
	jobRepo repository.Job,
	projectRepo repository.Project,
	executionsRepo repository.ExecutionsRepo,
	jobQueueRepo repository.JobQueuesRepo,
) *Node {
	logPrefix := logger.Prefix()
	logger.SetPrefix(fmt.Sprintf("%s[creating-new-Node] ", logPrefix))
	defer logger.SetPrefix(logPrefix)
	dirPath := fmt.Sprintf("%v", constants.RaftDir)
	dirPath, exists := utils.MakeDirIfNotExist(logger, dirPath)
	configs := config.GetConfigurations(logger)
	dirPath = fmt.Sprintf("%v/%v", constants.RaftDir, configs.NodeId)
	utils.MakeDirIfNotExist(logger, dirPath)
	tm, ldb, sdb, fss, err := getLogsAndTransport(logger)
	if err != nil {
		logger.Fatal("failed essentials for Node", err)
	}

	numReplicas := len(configs.Replicas)

	return &Node{
		TransportManager:     tm,
		LogDb:                ldb,
		StoreDb:              sdb,
		FileSnapShot:         fss,
		logger:               logger,
		AcceptWrites:         false,
		State:                Cold,
		ctx:                  ctx,
		jobProcessor:         processor.NewJobProcessor(ctx, logger, jobRepo, projectRepo, jobQueue, jobExecutor, executionsRepo, jobQueueRepo),
		jobQueue:             jobQueue,
		jobExecutor:          jobExecutor,
		jobRepo:              jobRepo,
		projectRepo:          projectRepo,
		isExistingNode:       exists,
		peerObserverChannels: make(chan raft.Observation, numReplicas),
	}
}

func (node *Node) Boostrap() {
	node.State = Bootstrapping

	if node.isExistingNode {
		node.logger.Println("discovered existing raft dir")
		node.recoverRaftState()
	}

	configs := config.GetConfigurations(node.logger)
	rft := node.newRaft(node.FsmStore)
	if configs.Bootstrap && !node.isExistingNode {
		node.bootstrapRaftCluster(rft)
	}
	node.FsmStore.Raft = rft
	node.jobExecutor.Raft = node.FsmStore.Raft
	go node.handleLeaderChange()
	//go node.jobExecutor.ListenOnInvocationChannels()

	if node.isExistingNode &&
		!node.SingleNodeMode && rft.State().String() == "Follower" {
		node.jobProcessor.RecoverJobs()
	}

	myObserver := raft.NewObserver(node.peerObserverChannels, true, func(o *raft.Observation) bool {
		_, peerObservation := o.Data.(raft.PeerObservation)
		_, resumedHeartbeatObservation := o.Data.(raft.ResumedHeartbeatObservation)
		return peerObservation || resumedHeartbeatObservation
	})

	rft.RegisterObserver(myObserver)
	go node.listenToObserverChannel()

	node.listenOnInputQueues(node.FsmStore)
}

func (node *Node) commitJobLogState(peerAddress string, jobState []models.JobExecutionLog) {
	if node.FsmStore.Raft == nil {
		node.logger.Fatalln("raft is not set on job executors")
	}

	params := models.CommitJobStateLog{
		Address: peerAddress,
		Logs:    jobState,
	}

	data, err := json.Marshal(params)
	if err != nil {
		node.logger.Fatalln("failed to marshal json")
	}

	createCommand := &protobuffs.Command{
		Type:         protobuffs.Command_Type(constants.CommandTypeJobExecutionLogs),
		Sql:          peerAddress,
		Data:         data,
		ActionTarget: peerAddress,
	}

	createCommandData, err := proto.Marshal(createCommand)
	if err != nil {
		node.logger.Fatalln("failed to marshal json")
	}

	configs := config.GetConfigurations(node.logger)

	_ = node.FsmStore.Raft.Apply(createCommandData, time.Second*time.Duration(configs.RaftApplyTimeout)).(raft.ApplyFuture)
}

func (node *Node) ReturnUncommittedLogs() []byte {
	return node.jobExecutor.GetUncommittedLogs()
}

func (node *Node) newRaft(fsm raft.FSM) *raft.Raft {
	logPrefix := node.logger.Prefix()
	node.logger.SetPrefix(fmt.Sprintf("%s[creating-new-Node-raft] ", logPrefix))
	defer node.logger.SetPrefix(logPrefix)
	configs := config.GetConfigurations(node.logger)

	c := raft.DefaultConfig()
	c.LocalID = raft.ServerID(rune(configs.NodeId))

	// TODO: Set raft configs in scheduler0 config

	r, err := raft.NewRaft(c, fsm, node.LogDb, node.StoreDb, node.FileSnapShot, node.TransportManager)
	if err != nil {
		node.logger.Fatalln("failed to create raft object for Node", err)
	}
	return r
}

func (node *Node) bootstrapRaftCluster(r *raft.Raft) {
	logPrefix := node.logger.Prefix()
	node.logger.SetPrefix(fmt.Sprintf("%s[boostraping-raft-cluster] ", logPrefix))
	defer node.logger.SetPrefix(logPrefix)

	cfg := node.authRaftConfiguration(node.logger)
	f := r.BootstrapCluster(cfg)
	if err := f.Error(); err != nil {
		node.logger.Fatalln("failed to bootstrap raft Node", err)
	}
}

func (node *Node) getUncommittedLogs() []models.JobExecutionLog {
	executionLogs := []models.JobExecutionLog{}

	configs := config.GetConfigurations(node.logger)

	dir, err := os.Getwd()
	if err != nil {
		node.logger.Fatalln(fmt.Errorf("Fatal error getting working dir: %s \n", err))
	}

	mainDbPath := fmt.Sprintf("%s/%s", dir, constants.SqliteDbFileName)
	dataStore := db.NewSqliteDbConnection(mainDbPath)
	conn := dataStore.OpenConnection()
	dbConnection := conn.(*sql.DB)

	rows, err := dbConnection.Query(fmt.Sprintf(
		"select count(*) from %s",
		fsm.ExecutionsUnCommittedTableName,
	), configs.NodeId, false)
	if err != nil {
		node.logger.Fatalln("failed to query for the count of uncommitted logs", err.Error())
	}
	var count int64 = 0
	for rows.Next() {
		scanErr := rows.Scan(&count)
		if scanErr != nil {
			node.logger.Fatalln("failed to scan count value", scanErr.Error())
		}
	}
	if rows.Err() != nil {
		node.logger.Fatalln("rows error", rows.Err())
	}
	err = rows.Close()
	if err != nil {
		node.logger.Fatalln("failed to close rows", err)
	}

	rows, err = dbConnection.Query(fmt.Sprintf(
		"select max(id) as maxId, min(id) as minId from %s",
		fsm.ExecutionsUnCommittedTableName,
	), configs.NodeId, false)
	if err != nil {
		node.logger.Fatalln("failed to query for max and min id in uncommitted logs", err.Error())
	}

	node.logger.Println("found", count, "uncommitted logs")

	if count < 1 {
		return executionLogs
	}

	var maxId int64 = 0
	var minId int64 = 0
	for rows.Next() {
		scanErr := rows.Scan(&maxId, &minId)
		if scanErr != nil {
			node.logger.Fatalln("failed to scan max and min id  on uncommitted logs", scanErr.Error())
		}
	}
	if rows.Err() != nil {
		node.logger.Fatalln("rows error", rows.Err())
	}
	err = rows.Close()
	if err != nil {
		node.logger.Fatalln("failed to close rows", err)
	}
	fmt.Printf("found max id %d and min id %d in uncommited jobs \n", maxId, minId)

	ids := []int64{}
	for i := minId; i <= maxId; i++ {
		ids = append(ids, i)
	}

	batches := batcher.Batch[int64](ids, 7)

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
			node.logger.Fatalln("failed to query for the uncommitted logs", err.Error())
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
				node.logger.Fatalln("failed to scan job execution columns", scanErr.Error())
			}
			executionLogs = append(executionLogs, jobExecutionLog)
		}
		err = rows.Close()
		if err != nil {
			node.logger.Fatalln("failed to close rows", err)
		}
	}
	err = dbConnection.Close()
	if err != nil {
		node.logger.Fatalln("failed to close database connection", err.Error())
	}

	return executionLogs
}

func (node *Node) insertUncommittedLogsIntoRecoverDb(executionLogs []models.JobExecutionLog, dbConnection *sql.DB) {
	executionLogsBatches := batcher.Batch[models.JobExecutionLog](executionLogs, 9)

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
			node.logger.Fatalln("failed to create transaction for batch insertion", err)
		}
		_, err = tx.Exec(query, params...)
		if err != nil {
			trxErr := tx.Rollback()
			if trxErr != nil {
				node.logger.Fatalln("failed to rollback update transition", trxErr)
			}
			node.logger.Fatalln("failed to insert un committed executions to recovery db", err)
		}
		err = tx.Commit()
		if err != nil {
			node.logger.Fatalln("failed to commit transition", err)
		}
	}
}

func (node *Node) recoverRaftState() raft.Configuration {
	logPrefix := node.logger.Prefix()
	node.logger.SetPrefix(fmt.Sprintf("%s[recovering-Node] ", logPrefix))
	defer node.logger.SetPrefix(logPrefix)

	var (
		snapshotIndex  uint64
		snapshotTerm   uint64
		snapshots, err = node.FileSnapShot.List()
	)
	if err != nil {
		node.logger.Fatalln(err)
	}

	node.logger.Println("found", len(snapshots), "snapshots")

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
		node.logger.Println("failed to restore any of the available snapshots")
	}

	dir, err := os.Getwd()
	if err != nil {
		node.logger.Fatalln(fmt.Errorf("Fatal error getting working dir: %s \n", err))
	}

	executionLogs := node.getUncommittedLogs()
	recoverDbPath := fmt.Sprintf("%s/%s", dir, constants.RecoveryDbFileName)

	err = os.WriteFile(recoverDbPath, lastSnapshotBytes, os.ModePerm)
	if err != nil {
		node.logger.Fatalln(fmt.Errorf("Fatal db file creation error: %s \n", err))
	}

	dataStore := db.NewSqliteDbConnection(recoverDbPath)
	conn := dataStore.OpenConnection()
	dbConnection := conn.(*sql.DB)
	defer dbConnection.Close()

	migrations := db.GetSetupSQL()
	_, err = dbConnection.Exec(migrations)
	if err != nil {
		node.logger.Fatalln(fmt.Errorf("Fatal db file migrations error: %s \n", err))
	}

	fsmStr := fsm.NewFSMStore(dataStore, node.logger)

	// The snapshot information is the best known end point for the data
	// until we play back the Raft log entries.
	lastIndex := snapshotIndex
	lastTerm := snapshotTerm

	// Apply any Raft log entries past the snapshot.
	lastLogIndex, err := node.LogDb.LastIndex()
	if err != nil {
		node.logger.Fatalf("failed to find last log: %v", err)
	}
	for index := snapshotIndex + 1; index <= lastLogIndex; index++ {
		var entry raft.Log
		if err = node.LogDb.GetLog(index, &entry); err != nil {
			node.logger.Fatalf("failed to get log at index %d: %v\n", index, err)
		}
		fsm.ApplyCommand(node.logger, &entry, dataStore, false, nil, nil)
		lastIndex = entry.Index
		lastTerm = entry.Term
	}

	if len(executionLogs) > 0 {
		node.insertUncommittedLogsIntoRecoverDb(executionLogs, dbConnection)
	}

	lastConfiguration := node.getRaftConfiguration(node.logger)

	snapshot := fsm.NewFSMSnapshot(fsmStr.DataStore)
	sink, err := node.FileSnapShot.Create(1, lastIndex, lastTerm, lastConfiguration, 1, node.TransportManager)
	if err != nil {
		node.logger.Fatalf("failed to create snapshot: %v", err)
	}
	if err = snapshot.Persist(sink); err != nil {
		node.logger.Fatalf("failed to persist snapshot: %v", err)
	}
	if err = sink.Close(); err != nil {
		node.logger.Fatalf("failed to finalize snapshot: %v", err)
	}

	firstLogIndex, err := node.LogDb.FirstIndex()
	if err != nil {
		node.logger.Fatalf("failed to get first log index: %v", err)
	}
	if err := node.LogDb.DeleteRange(firstLogIndex, lastLogIndex); err != nil {
		node.logger.Fatalf("log compaction failed: %v", err)
	}

	err = os.Remove(recoverDbPath)
	if err != nil {
		node.logger.Fatalf("failed to delete recovery db: %v", err)
	}

	return lastConfiguration
}

func (node *Node) authenticateWithPeersInConfig(logger *log.Logger) map[string]Status {
	node.logger.Println("Authenticating with node...")

	configs := config.GetConfigurations(logger)
	var wg sync.WaitGroup

	results := map[string]Status{}
	wrlck := sync.Mutex{}

	for _, replica := range configs.Replicas {
		if replica.Address != utils.GetServerHTTPAddress(node.logger) {
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
					node.logger.Println("failed to authenticate with peer ", rep.Address, " error:", err)
				}
			}(replica, results, &wg, &wrlck)
		}
	}
	wg.Wait()

	return results
}

func (node *Node) handleLeaderChange() {
	select {
	case <-node.FsmStore.Raft.LeaderCh():
		node.AcceptWrites = false
		_, applyErr := fsm.AppApply(
			node.logger,
			node.FsmStore.Raft,
			constants.CommandTypeStopJobs,
			"",
			nil,
		)
		configuration := node.FsmStore.Raft.GetConfiguration().Configuration()
		servers := configuration.Servers
		node.jobQueue.RemoveServers(servers)
		node.jobQueue.AddServers(servers)
		node.jobQueue.SingleNodeMode = len(servers) == 1
		node.jobExecutor.SingleNodeMode = node.jobQueue.SingleNodeMode
		node.SingleNodeMode = node.jobQueue.SingleNodeMode
		if applyErr != nil {
			node.logger.Fatalln("failed to apply job update states ", applyErr)
		}
		if !node.SingleNodeMode {
			compressedLogs := node.jobExecutor.GetUncommittedLogs()
			uncompressedLogs, err := utils.GzUncompress(compressedLogs)
			if err != nil {
				node.logger.Println("failed to uncompress gzipped logs fetch response data", err.Error())
			}

			jobStateLogs := []models.JobExecutionLog{}
			err = json.Unmarshal(uncompressedLogs, &jobStateLogs)
			if err != nil {
				node.logger.Println("failed to unmarshal uncompressed logs to jobStateLogs instance", err.Error())
			}

			if len(jobStateLogs) > 0 {
				node.commitJobLogState(utils.GetServerHTTPAddress(node.logger), jobStateLogs)
			}

			go node.fetchUncommittedLogsFromPeers()
		} else {
			if node.isExistingNode {
				node.jobProcessor.RecoverJobs()
			} else {
				node.jobProcessor.StartJobs()
			}
			node.AcceptWrites = true
		}
		node.logger.Println("Ready to accept requests")
	}
}

func (node *Node) listenOnInputQueues(fsmStr *fsm.Store) {
	node.logger.Println("begin listening input queues")

	for {
		select {
		case job := <-fsmStr.QueueJobsChannel:
			node.jobExecutor.QueueExecutions(job)
		case _ = <-fsmStr.StopAllJobs:
			node.jobExecutor.StopAll()
		}
	}
}

func (node *Node) authRaftConfiguration(logger *log.Logger) raft.Configuration {
	node.mtx.Lock()
	defer node.mtx.Unlock()

	configs := config.GetConfigurations(logger)
	results := node.authenticateWithPeersInConfig(node.logger)
	servers := []raft.Server{
		{
			ID:       raft.ServerID(rune(configs.NodeId)),
			Suffrage: raft.Voter,
			Address:  raft.ServerAddress(configs.RaftAddress),
		},
	}

	for _, replica := range configs.Replicas {
		if repStatus, ok := results[replica.Address]; ok && repStatus.IsAlive && repStatus.IsAuth {
			servers = append(servers, raft.Server{
				ID:       raft.ServerID(rune(configs.NodeId)),
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

func (node *Node) getRaftConfiguration(logger *log.Logger) raft.Configuration {
	node.mtx.Lock()
	defer node.mtx.Unlock()

	configs := config.GetConfigurations(logger)
	servers := []raft.Server{
		{
			ID:       raft.ServerID(rune(configs.NodeId)),
			Suffrage: raft.Voter,
			Address:  raft.ServerAddress(configs.RaftAddress),
		},
	}

	for _, replica := range configs.Replicas {
		if replica.Address != utils.GetServerHTTPAddress(node.logger) {
			servers = append(servers, raft.Server{
				ID:       raft.ServerID(rune(configs.NodeId)),
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

func (node *Node) getHTTPAddressOfServersToFetchFrom() []string {
	configs := config.GetConfigurations(node.logger)
	servers := []raft.ServerAddress{}
	httpAddresses := []string{}

	for _, server := range node.FsmStore.Raft.GetConfiguration().Configuration().Servers {
		if string(server.Address) != configs.RaftAddress {
			servers = append(servers, server.Address)
		}
	}

	if uint64(len(servers)) < configs.ExecutionLogFetchFanIn {
		for _, server := range servers {
			httpAddresses = append(httpAddresses, utils.GetNodeServerAddressWithRaftAddress(node.logger, server))
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
			httpAddresses = append(httpAddresses, utils.GetNodeServerAddressWithRaftAddress(node.logger, shuffledServers[i]))
		}
	}

	return httpAddresses
}

func (node *Node) fetchUncommittedLogsFromPeers() {
	configs := config.GetConfigurations(node.logger)
	uncommittedLogs := make(chan []interface{}, configs.ExecutionLogFetchFanIn)
	ctx, cancelFunc := context.WithCancel(node.ctx)
	ticker := time.NewTicker(time.Duration(configs.ExecutionLogFetchIntervalSeconds) * time.Second)
	fetchStatus := map[string]bool{}
	pendingFetches := 0
	mtx := sync.Mutex{}
	completedFirstRound := map[string]bool{}
	startedJobs := false

	for {
		select {
		case <-ticker.C:
			mtx.Lock()

			if pendingFetches > 0 {
				cancelFunc()
				ctx, cancelFunc = context.WithCancel(context.Background())
			}

			serversNotFetched := []string{}
			for server, status := range fetchStatus {
				if !status {
					serversNotFetched = append(serversNotFetched, server)
				}
			}

			if len(completedFirstRound) < 1 {
				httpAddresses := node.getHTTPAddressOfServersToFetchFrom()
				for i := 0; i < len(httpAddresses); i++ {
					completedFirstRound[httpAddresses[i]] = false
				}
			}

			notCompletedFirstRound := []string{}
			for server, status := range completedFirstRound {
				if !status {
					notCompletedFirstRound = append(notCompletedFirstRound, server)
				}
			}

			if len(notCompletedFirstRound) == 0 && !startedJobs {
				startedJobs = true
				node.AcceptWrites = true
				go node.jobProcessor.StartJobs()
			}

			if len(serversNotFetched) < 1 {
				httpAddresses := node.getHTTPAddressOfServersToFetchFrom()
				fetchStatus = map[string]bool{}
				for i := 0; i < len(httpAddresses); i++ {
					fetchStatus[httpAddresses[i]] = false
				}
				serversNotFetched = httpAddresses
			}

			if uint64(len(serversNotFetched)) > configs.ExecutionLogFetchFanIn {
				fetchUncommittedLogsFromPeers(ctx, node.logger, serversNotFetched[:configs.ExecutionLogFetchFanIn], uncommittedLogs)
				pendingFetches += len(serversNotFetched[:configs.ExecutionLogFetchFanIn])
			} else {
				fetchUncommittedLogsFromPeers(ctx, node.logger, serversNotFetched, uncommittedLogs)
				pendingFetches += len(serversNotFetched)
			}
			mtx.Unlock()
		case results := <-uncommittedLogs:
			serverAddress := results[0].(string)
			executionLogs := results[1].([]byte)
			logsFetchResponse := LogsFetchResponse{}
			err := json.Unmarshal(executionLogs, &logsFetchResponse)
			if err != nil {
				node.logger.Println("failed to json unmarshal logs fetch response", err.Error())
			}

			uncompressedLogs, err := utils.GzUncompress(logsFetchResponse.Data)
			if err != nil {
				node.logger.Println("failed to uncompress gzipped logs fetch response data", err.Error())
			}

			jobStateLogs := []models.JobExecutionLog{}
			err = json.Unmarshal(uncompressedLogs, &jobStateLogs)
			if err != nil {
				node.logger.Println("failed to unmarshal uncompressed logs to jobStateLogs instance", err.Error())
			}

			if len(jobStateLogs) > 0 {
				node.commitJobLogState(serverAddress, jobStateLogs)
			}

			mtx.Lock()
			pendingFetches -= 1
			fetchStatus[serverAddress] = true
			completedFirstRound[serverAddress] = true
			mtx.Unlock()
		}
	}
}

func (node *Node) listenToObserverChannel() {
	for {
		select {
		case o := <-node.peerObserverChannels:
			peerObservation, isPeerObservation := o.Data.(raft.PeerObservation)
			resumedHeartbeatObservation, isResumedHeartbeatObservation := o.Data.(raft.ResumedHeartbeatObservation)

			if isPeerObservation && !peerObservation.Removed {
				node.logger.Println("A new node joined the cluster")
			}
			if isPeerObservation && peerObservation.Removed {
				node.logger.Println("A node got removed from the cluster")
			}
			if isResumedHeartbeatObservation {
				fmt.Println("resumedHeartbeatObservation.PeerID", resumedHeartbeatObservation.PeerID)
				node.logger.Println(fmt.Sprintf("A node resumed execution. Peer ID %s ", string(resumedHeartbeatObservation.PeerID)))
			}
		}
	}
}

func fetchUncommittedLogsFromPeers(ctx context.Context, logger *log.Logger, peerAddresses []string, uncommittedLogs chan []interface{}) {
	configs := config.GetConfigurations(logger)

	httpClient := &http.Client{
		Timeout: time.Duration(configs.UncommittedExecutionLogsFetchTimeout) * time.Millisecond,
	}

	wg := &sync.WaitGroup{}
	wg.Add(len(peerAddresses))

	for _, peerAddress := range peerAddresses {
		go func(address string, wg *sync.WaitGroup, client *http.Client, results chan []interface{}) {
			httpRequest, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%v/execution-logs", address), nil)
			if reqErr != nil {
				logger.Println("failed to create request to execution logs from", address, "error", reqErr.Error())
				wg.Done()
				return
			}
			httpRequest.Header.Set(headers.PeerHeader, headers.PeerHeaderValue)
			httpRequest.Header.Set(headers.PeerAddressHeader, utils.GetServerHTTPAddress(logger))
			credentials := secrets.GetSecrets(logger)
			httpRequest.SetBasicAuth(credentials.AuthUsername, credentials.AuthPassword)
			res, err := httpClient.Do(httpRequest)
			if err != nil {
				wg.Done()
				logger.Println("failed to get uncommitted execution logs from", address, "error", err.Error())
				return
			}
			if res.StatusCode == http.StatusOK {
				data, readErr := io.ReadAll(res.Body)
				if readErr != nil {
					logger.Println("failed to read uncommitted execution logs from", address, "error", err.Error())
					wg.Done()
					return
				}
				results <- []interface{}{address, data}
				res.Body.Close()
				logger.Println("successfully fetch execution logs from", address)
				wg.Done()
			} else {
				logger.Println("failed to get uncommitted execution logs from", address, "state code", res.StatusCode)
				wg.Done()
			}
		}(peerAddress, wg, httpClient, uncommittedLogs)
	}
}

func connectNode(logger *log.Logger, rep config.RaftNode) (*Status, error) {
	configs := config.GetConfigurations(logger)
	httpClient := http.Client{
		Timeout: time.Duration(configs.PeerAuthRequestTimeoutMs) * time.Millisecond,
	}
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/peer-handshake", rep.Address), nil)
	if err != nil {
		logger.Println("failed to create request ", err)
		return nil, err
	}
	req.Header.Set(headers.PeerHeader, headers.PeerHeaderValue)
	req.Header.Set(headers.PeerAddressHeader, utils.GetServerHTTPAddress(logger))
	credentials := secrets.GetSecrets(logger)
	req.SetBasicAuth(credentials.AuthUsername, credentials.AuthPassword)

	start := time.Now()

	resp, err := httpClient.Do(req)
	if err != nil {
		logger.Println("failed to send request ", err)
		return nil, err
	}
	defer resp.Body.Close()

	connectionTime := time.Since(start)

	if resp.StatusCode == http.StatusOK {
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			logger.Println("failed to response", err)
			return nil, err
		}

		body := Response{}

		err = json.Unmarshal(data, &body)
		if err != nil {
			logger.Println("failed to unmarshal response ", err)
			return nil, err
		}

		logger.Println("successfully authenticated ", rep.Address)

		return &Status{
			IsAlive:            true,
			IsAuth:             true,
			IsLeader:           body.Data.IsLeader,
			LastConnectionTime: connectionTime,
		}, nil
	}

	logger.Println("could not authenticate ", rep.Address, " status code:", resp.StatusCode)

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

func getLogsAndTransport(logger *log.Logger) (tm *raft.NetworkTransport, ldb *boltdb.BoltStore, sdb *boltdb.BoltStore, fss *raft.FileSnapshotStore, err error) {
	logPrefix := logger.Prefix()
	logger.SetPrefix(fmt.Sprintf("%s[creating-new-Node-essentials] ", logPrefix))
	defer logger.SetPrefix(logPrefix)

	configs := config.GetConfigurations(logger)

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

	mux := network.NewMux(logger, ln)

	go mux.Serve()

	muxLn := mux.Listen(1)

	tm = raft.NewNetworkTransport(network.NewTransport(muxLn), int(configs.RaftTransportMaxPool), time.Second*time.Duration(configs.RaftTransportTimeout), nil)
	return
}
