package node

import (
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"github.com/hashicorp/raft"
	boltdb "github.com/hashicorp/raft-boltdb/v2"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"scheduler0/config"
	"scheduler0/constants"
	"scheduler0/db"
	"scheduler0/fsm"
	"scheduler0/headers"
	"scheduler0/job_executor"
	"scheduler0/job_processor"
	"scheduler0/job_queue"
	"scheduler0/job_recovery"
	"scheduler0/models"
	"scheduler0/network"
	"scheduler0/repository"
	"scheduler0/secrets"
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

type State int

const (
	Cold          State = 0
	Bootstrapping       = 1
)

type Node struct {
	Tm             *raft.NetworkTransport
	Ldb            *boltdb.BoltStore
	Sdb            *boltdb.BoltStore
	Fss            *raft.FileSnapshotStore
	logger         *log.Logger
	mtx            sync.Mutex
	AcceptWrites   bool
	State          State
	FsmStore       *fsm.Store
	jobProcessor   *job_processor.JobProcessor
	jobQueue       job_queue.JobQueue
	jobExecutor    *job_executor.JobExecutor
	jobRecovery    *job_recovery.JobRecovery
	jobRepo        repository.Job
	projectRepo    repository.Project
	isExistingNode bool
}

func NewNode(
	logger *log.Logger,
	jobExecutor *job_executor.JobExecutor,
	jobQueue job_queue.JobQueue,
	jobRepo repository.Job,
	projectRepo repository.Project,
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

	return &Node{
		Tm:             tm,
		Ldb:            ldb,
		Sdb:            sdb,
		Fss:            fss,
		logger:         logger,
		AcceptWrites:   false,
		State:          Cold,
		jobProcessor:   job_processor.NewJobProcessor(jobRepo, projectRepo, jobQueue, logger),
		jobQueue:       jobQueue,
		jobExecutor:    jobExecutor,
		jobRepo:        jobRepo,
		projectRepo:    projectRepo,
		jobRecovery:    job_recovery.NewJobRecovery(logger, jobRepo, jobExecutor),
		isExistingNode: exists,
	}
}

func (node *Node) Boostrap(fsmStr *fsm.Store) {
	node.State = Bootstrapping
	if node.isExistingNode {
		node.logger.Println("discovered existing raft dir")
		node.recoverRaftState()
	}

	configs := config.GetConfigurations(node.logger)
	rft := node.newRaft(fsmStr)
	if configs.Bootstrap && !node.isExistingNode {
		node.bootstrapRaftCluster(rft)
	}
	fsmStr.Raft = rft
	node.FsmStore = fsmStr
	node.jobExecutor.Raft = fsmStr.Raft
	go node.handleLeaderChange()
	go node.jobExecutor.ListenOnInvocationChannels()
	node.listenOnInputQueues(fsmStr)
}

func (node *Node) LogJobsStatePeers(peerAddress string, jobState models.JobStateLog) {
	if node.FsmStore.Raft == nil {
		node.logger.Fatalln("raft is not set on job executors")
	}

	data := []interface{}{jobState}

	_, applyErr := fsm.AppApply(
		node.logger,
		node.FsmStore.Raft,
		jobState.State,
		peerAddress,
		data,
	)
	if applyErr != nil {
		node.logger.Fatalln("failed to apply job update states ", applyErr)
	}
}

func (node *Node) newRaft(fsm raft.FSM) *raft.Raft {
	logPrefix := node.logger.Prefix()
	node.logger.SetPrefix(fmt.Sprintf("%s[creating-new-Node-raft] ", logPrefix))
	defer node.logger.SetPrefix(logPrefix)
	configs := config.GetConfigurations(node.logger)

	c := raft.DefaultConfig()
	c.LocalID = raft.ServerID(configs.NodeId)

	// TODO: Set raft configs in scheduler0 config

	r, err := raft.NewRaft(c, fsm, node.Ldb, node.Sdb, node.Fss, node.Tm)
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

func (node *Node) recoverRaftState() raft.Configuration {
	logPrefix := node.logger.Prefix()
	node.logger.SetPrefix(fmt.Sprintf("%s[recovering-Node] ", logPrefix))
	defer node.logger.SetPrefix(logPrefix)

	var (
		snapshotIndex  uint64
		snapshotTerm   uint64
		snapshots, err = node.Fss.List()
	)
	if err != nil {
		node.logger.Fatalln(err)
	}

	node.logger.Println("found", len(snapshots), "snapshots")

	var lastSnapshotBytes []byte
	for _, snapshot := range snapshots {
		var source io.ReadCloser
		_, source, err = node.Fss.Open(snapshot.ID)
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

	recoverDbPath := fmt.Sprintf("%s/%s", dir, "recover.db")

	err = os.WriteFile(recoverDbPath, lastSnapshotBytes, os.ModePerm)
	if err != nil {
		node.logger.Fatalln(fmt.Errorf("Fatal db file creation error: %s \n", err))
	}

	dataStore := db.NewSqliteDbConnection(recoverDbPath)
	conn, err := dataStore.OpenConnection()
	dbConnection := conn.(*sql.DB)
	defer dbConnection.Close()

	migrations := db.GetSetupSQL()
	_, err = dbConnection.Exec(migrations)
	if err != nil {
		node.logger.Fatalln(fmt.Errorf("Fatal db file migrations error: %s \n", err))
	}

	fsmStr := fsm.NewFSMStore(dataStore, dbConnection, node.logger)

	// The snapshot information is the best known end point for the data
	// until we play back the Raft log entries.
	lastIndex := snapshotIndex
	lastTerm := snapshotTerm

	// Apply any Raft log entries past the snapshot.
	lastLogIndex, err := node.Ldb.LastIndex()
	if err != nil {
		node.logger.Fatalf("failed to find last log: %v", err)
	}
	for index := snapshotIndex + 1; index <= lastLogIndex; index++ {
		var entry raft.Log
		if err = node.Ldb.GetLog(index, &entry); err != nil {
			node.logger.Fatalf("failed to get log at index %d: %v\n", index, err)
		}
		fsm.ApplyCommand(node.logger, &entry, dbConnection, false, nil, nil, nil, nil, nil)
		lastIndex = entry.Index
		lastTerm = entry.Term
	}

	lastConfiguration := node.getRaftConfiguration(node.logger)

	snapshot := fsm.NewFSMSnapshot(fsmStr.SqliteDB)
	sink, err := node.Fss.Create(1, lastIndex, lastTerm, lastConfiguration, 1, node.Tm)
	if err != nil {
		node.logger.Fatalf("failed to create snapshot: %v", err)
	}
	if err = snapshot.Persist(sink); err != nil {
		node.logger.Fatalf("failed to persist snapshot: %v", err)
	}
	if err = sink.Close(); err != nil {
		node.logger.Fatalf("failed to finalize snapshot: %v", err)
	}

	firstLogIndex, err := node.Ldb.FirstIndex()
	if err != nil {
		node.logger.Fatalf("failed to get first log index: %v", err)
	}
	if err := node.Ldb.DeleteRange(firstLogIndex, lastLogIndex); err != nil {
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
		if replica.Address != fmt.Sprintf("%s://%s:%s", configs.Protocol, configs.Host, configs.Port) {
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
				}, configs.PeerConnectRetryMax, configs.PeerConnectRetryDelay)
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
		if applyErr != nil {
			node.logger.Fatalln("failed to apply job update states ", applyErr)
		}
		node.logger.Println("starting jobs")
		if node.isExistingNode && len(servers) == 1 {
			node.jobRecovery.Run()
		} else {
			node.jobProcessor.StartJobs()
		}
		node.AcceptWrites = true
		node.logger.Println("Ready to accept requests")
	}
}

func (node *Node) listenOnInputQueues(fsmStr *fsm.Store) {
	node.logger.Println("begin listening input queues")

	for {
		select {
		case job := <-fsmStr.QueueJobsChannel:
			node.jobExecutor.QueueExecutions(job)
		case job := <-fsmStr.ScheduleJobsChannel:
			node.jobExecutor.LogJobScheduledExecutions(job, true)
			node.jobRecovery.RecoverAndScheduleJob(job)
		case job := <-fsmStr.SuccessfulJobsChannel:
			node.jobExecutor.LogSuccessfulJobExecutions(job, true)
		case job := <-fsmStr.FailedJobsChannel:
			node.jobExecutor.LogFailedJobExecutions(job, true)
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
			ID:       raft.ServerID(configs.NodeId),
			Suffrage: raft.Voter,
			Address:  raft.ServerAddress(configs.RaftAddress),
		},
	}

	for _, replica := range configs.Replicas {
		if repStatus, ok := results[replica.Address]; ok && repStatus.IsAlive && repStatus.IsAuth {
			servers = append(servers, raft.Server{
				ID:       raft.ServerID(replica.NodeId),
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
			ID:       raft.ServerID(configs.NodeId),
			Suffrage: raft.Voter,
			Address:  raft.ServerAddress(configs.RaftAddress),
		},
	}

	for _, replica := range configs.Replicas {
		if replica.Address != fmt.Sprintf("%s://%s:%s", configs.Protocol, configs.Host, configs.Port) {
			servers = append(servers, raft.Server{
				ID:       raft.ServerID(replica.NodeId),
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

func (node *Node) fetchUncommittedLogsFromPeers(logger *log.Logger) {

}

func connectNode(logger *log.Logger, rep config.RaftNode) (*Status, error) {
	configs := config.GetConfigurations(logger)
	httpClient := http.Client{
		Timeout: time.Duration(configs.PeerAuthRequestTimeout) * time.Second,
	}
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/peer-handshake", rep.Address), nil)
	if err != nil {
		logger.Println("failed to create request ", err)
		return nil, err
	}
	req.Header.Set(headers.PeerHeader, "peer")
	req.Header.Set(headers.PeerAddressHeader, fmt.Sprintf("%s://%s:%s", configs.Protocol, configs.Host, configs.Port))
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
	_, err = strconv.Atoi(configs.RaftSnapshotInterval)
	if err != nil {
		logger.Fatal("Failed to convert raft snapshot interval to int", err)
	}

	_, err = strconv.Atoi(configs.RaftSnapshotThreshold)
	if err != nil {
		logger.Fatal("Failed to convert raft snapshot threshold to int", err)
	}

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

	maxPool, err := strconv.Atoi(configs.RaftTransportMaxPool)
	if err != nil {
		logger.Fatal("Failed to convert raft transport max pool to int", err)
	}

	timeout, err := strconv.Atoi(configs.RaftTransportTimeout)
	if err != nil {
		logger.Fatal("Failed to convert raft transport timeout to int", err)
	}

	tm = raft.NewNetworkTransport(network.NewTransport(muxLn), maxPool, time.Second*time.Duration(timeout), nil)
	return
}
