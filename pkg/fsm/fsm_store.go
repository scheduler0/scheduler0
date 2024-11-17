package fsm

import (
	"database/sql"
	"fmt"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	boltdb "github.com/hashicorp/raft-boltdb/v2"
	"io"
	"io/ioutil"
	"log"
	"os"
	"scheduler0/pkg/config"
	"scheduler0/pkg/constants"
	"scheduler0/pkg/db"
	"scheduler0/pkg/shared_repo"
	utils "scheduler0/pkg/utils"
	"strconv"
	"sync"
	"time"
)

type store struct {
	rwMtx                 sync.RWMutex
	dataStore             db.DataStore
	logger                hclog.Logger
	raft                  *raft.Raft
	queueJobsChannel      chan []interface{}
	scheduler0Config      config.Scheduler0Config
	scheduler0RaftActions Scheduler0RaftActions
	transportManager      raft.Transport
	logDb                 *boltdb.BoltStore
	storeDb               *boltdb.BoltStore
	fileSnapShot          *raft.FileSnapshotStore
	sharedRepo            shared_repo.SharedRepo

	raft.BatchingFSM
}

//go:generate mockery --name Scheduler0RaftStore --output ./ --inpackage
type Scheduler0RaftStore interface {
	GetFSM() raft.FSM
	GetBatchingFSM() raft.BatchingFSM
	GetDataStore() db.DataStore
	GetRaft() *raft.Raft
	VerifyLeader() raft.Future
	UpdateRaft(rft *raft.Raft)
	GetServersOnRaftCluster() []raft.Server
	GetLeaderChangeChannel() <-chan bool
	InitRaft()
	LeaderWithID() (raft.ServerAddress, raft.ServerID)
	BootstrapRaftClusterWithConfig(raftServerConfiguration raft.Configuration)
	RecoverRaftState()
	GetRaftStats() map[string]string
	RegisterObserver(or *raft.Observer)
}

var _ raft.FSM = &store{}

func NewFSMStore(
	logger hclog.Logger,
	scheduler0RaftActions Scheduler0RaftActions,
	scheduler0Config config.Scheduler0Config,
	db db.DataStore,
	ldb *boltdb.BoltStore,
	stb *boltdb.BoltStore,
	fss *raft.FileSnapshotStore,
	tm raft.Transport,
	sharedRepo shared_repo.SharedRepo,
) Scheduler0RaftStore {
	fsmStoreLogger := logger.Named("fsm-store")

	return &store{
		dataStore:             db,
		queueJobsChannel:      make(chan []interface{}, 1),
		logger:                fsmStoreLogger,
		scheduler0RaftActions: scheduler0RaftActions,
		logDb:                 ldb,
		storeDb:               stb,
		fileSnapShot:          fss,
		transportManager:      tm,
		sharedRepo:            sharedRepo,
		scheduler0Config:      scheduler0Config,
	}
}

func (s *store) GetFSM() raft.FSM {
	return s
}

func (s *store) GetBatchingFSM() raft.BatchingFSM {
	return s
}

func (s *store) GetRaft() *raft.Raft {
	return s.raft
}

func (s *store) VerifyLeader() raft.Future {
	return s.raft.VerifyLeader()
}

func (s *store) LeaderWithID() (raft.ServerAddress, raft.ServerID) {
	return s.raft.LeaderWithID()
}

func (s *store) UpdateRaft(rft *raft.Raft) {
	s.raft = rft
}

func (s *store) GetDataStore() db.DataStore {
	return s.dataStore
}

func (s *store) Apply(l *raft.Log) interface{} {
	s.rwMtx.Lock()
	defer s.rwMtx.Unlock()

	return s.scheduler0RaftActions.ApplyRaftLog(
		s.logger,
		l,
		s.dataStore,
		false,
	)
}

func (s *store) ApplyBatch(logs []*raft.Log) []interface{} {
	s.rwMtx.Lock()
	defer s.rwMtx.Unlock()

	results := []interface{}{}

	for _, l := range logs {
		result := s.scheduler0RaftActions.ApplyRaftLog(
			s.logger,
			l,
			s.dataStore,
			false,
		)
		results = append(results, result)
	}

	return results
}

func (s *store) Snapshot() (raft.FSMSnapshot, error) {
	s.logger.Info("taking snapshot")
	fmsSnapshot := NewFSMSnapshot(s.dataStore)
	return fmsSnapshot, nil
}

func (s *store) Restore(r io.ReadCloser) error {
	s.dataStore.FileLock()
	defer s.dataStore.FileUnlock()

	s.logger.Info("restoring snapshot")
	b, err := utils.BytesFromSnapshot(r)
	if err != nil {
		return fmt.Errorf("restore failed: %s", err.Error())
	}
	_, filePath := utils.GetSqliteDbDirAndDbFilePath()
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return err
	}
	if b != nil {
		if err := ioutil.WriteFile(filePath, b, os.ModePerm); err != nil {
			return err
		}
	}

	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?_foreign_keys=1", filePath))
	if err != nil {
		return fmt.Errorf("restore failed to create db: %v", err)
	}

	err = db.Ping()
	if err != nil {
		return fmt.Errorf("ping error: restore failed to create db: %v", err)
	}

	s.dataStore.ConnectionLock()
	s.dataStore.UpdateOpenConnection(db)
	s.dataStore.ConnectionUnlock()

	return nil
}

func (s *store) GetServersOnRaftCluster() []raft.Server {
	return s.raft.GetConfiguration().Configuration().Servers
}

func (s *store) GetLeaderChangeChannel() <-chan bool {
	return s.raft.LeaderCh()
}

func (s *store) InitRaft() {
	configs := s.scheduler0Config.GetConfigurations()

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

	r, err := raft.NewRaft(c, s, s.logDb, s.storeDb, s.fileSnapShot, s.transportManager)
	if err != nil {
		s.logger.Error("failed to create raft object", err)
	}
	s.raft = r
}

func (s *store) BootstrapRaftClusterWithConfig(raftServerConfiguration raft.Configuration) {
	f := s.raft.BootstrapCluster(raftServerConfiguration)
	if err := f.Error(); err != nil {
		s.logger.Error("failed to bootstrap raft cluster with configuration", err)
	}
}

func (s *store) RecoverRaftState() {
	logger := log.New(os.Stderr, "[recover-raft-state] ", log.LstdFlags)

	var (
		snapshotIndex  uint64
		snapshotTerm   uint64
		snapshots, err = s.fileSnapShot.List()
	)
	if err != nil {
		logger.Fatalln("failed to load file snapshots", err)
	}

	s.logger.Debug(fmt.Sprintf("found %d snapshots", len(snapshots)))

	var lastSnapshotBytes []byte
	for _, snapshot := range snapshots {
		var source io.ReadCloser
		_, source, err = s.fileSnapShot.Open(snapshot.ID)
		if err != nil {
			// Skip this one and try the next. We will detect if we
			// couldn't open any snapshots.
			continue
		}

		lastSnapshotBytes, err = utils.BytesFromSnapshot(source)
		// Close the source after the restore has completed
		err := source.Close()
		if err != nil {
			logger.Fatalln("failed to close file snapshot io reader", err)
			return
		}
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
	dataStore := db.NewSqliteDbConnection(s.logger, mainDbPath)
	dataStore.OpenConnectionToExistingDB()

	executionLogs, err := s.sharedRepo.GetExecutionLogs(dataStore, false)
	if err != nil {
		logger.Fatalln(fmt.Errorf("failed to get uncommitted execution logs: %s \n", err))
	}

	uncommittedTasks, getErr := s.sharedRepo.GetAsyncTasksLogs(dataStore, false)
	if getErr != nil {
		s.logger.Warn("failed to get uncommitted tasks", "error", getErr)
	}

	recoverDbPath := fmt.Sprintf("%s/%s/%s", dir, constants.SqliteDir, constants.RecoveryDbFileName)

	fileCreationErr := os.WriteFile(recoverDbPath, lastSnapshotBytes, os.ModePerm)
	if fileCreationErr != nil {
		logger.Fatalln(fmt.Errorf("fatal db file creation error: %s \n", fileCreationErr))
	}

	dataStore = db.NewSqliteDbConnection(s.logger, recoverDbPath)
	conn := dataStore.OpenConnectionToExistingDB()
	dbConnection := conn.(*sql.DB)
	defer dbConnection.Close()

	migrations := db.GetSetupSQL()
	_, err = dbConnection.Exec(migrations)
	if err != nil {
		logger.Fatalln(fmt.Errorf("fatal db file migrations error: %s \n", err))
	}

	// The snapshot information is the best known end point for the data
	// until we play back the raft log entries.
	lastIndex := snapshotIndex
	lastTerm := snapshotTerm

	// Apply any raft log entries past the snapshot.
	lastLogIndex, err := s.logDb.LastIndex()
	if err != nil {
		logger.Fatalf("failed to find last log: %v", err)
	}
	for index := snapshotIndex + 1; index <= lastLogIndex; index++ {
		var entry raft.Log
		if err = s.logDb.GetLog(index, &entry); err != nil {
			logger.Fatalf("failed to get log at index %d: %v\n", index, err)
		}
		s.scheduler0RaftActions.ApplyRaftLog(
			s.logger,
			&entry,
			dataStore,
			true,
		)
		lastIndex = entry.Index
		lastTerm = entry.Term
	}

	if len(executionLogs) > 0 {
		err = s.sharedRepo.InsertExecutionLogs(dataStore, false, executionLogs)
		if err != nil {
			logger.Fatal("failed to insert uncommitted execution logs", err)
		}
	}

	if len(uncommittedTasks) > 0 {
		err = s.sharedRepo.InsertAsyncTasksLogs(dataStore, false, uncommittedTasks)
		if err != nil {
			logger.Fatal("failed to insert uncommitted async task logs", err)
		}
	}

	lastConfiguration := s.getRaftConfiguration()

	snapshot := NewFSMSnapshot(dataStore)
	sink, err := s.fileSnapShot.Create(1, lastIndex, lastTerm, lastConfiguration, 1, s.transportManager)
	if err != nil {
		logger.Fatalf("failed to create snapshot: %v", err)
	}
	if err = snapshot.Persist(sink); err != nil {
		logger.Fatalf("failed to persist snapshot: %v", err)
	}
	if err = sink.Close(); err != nil {
		logger.Fatalf("failed to finalize snapshot: %v", err)
	}

	firstLogIndex, err := s.logDb.FirstIndex()
	if err != nil {
		logger.Fatalf("failed to get first log index: %v", err)
	}
	if err := s.logDb.DeleteRange(firstLogIndex, lastLogIndex); err != nil {
		logger.Fatalf("log compaction failed: %v", err)
	}

	err = os.Remove(recoverDbPath)
	if err != nil {
		logger.Fatalf("failed to delete recovery db: %v", err)
	}
}

func (s *store) GetRaftStats() map[string]string {
	return s.raft.Stats()
}

func (s *store) RegisterObserver(or *raft.Observer) {
	s.raft.RegisterObserver(or)
}

func (s *store) getRaftConfiguration() raft.Configuration {
	configs := s.scheduler0Config.GetConfigurations()
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
