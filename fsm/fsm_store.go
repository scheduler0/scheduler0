package fsm

import (
	"database/sql"
	"fmt"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	"io"
	"io/ioutil"
	"os"
	"scheduler0/config"
	"scheduler0/constants"
	"scheduler0/db"
	"scheduler0/models"
	"scheduler0/utils"
	"sync"
)

type store struct {
	rwMtx                 sync.RWMutex
	dataStore             db.DataStore
	logger                hclog.Logger
	raft                  *raft.Raft
	queueJobsChannel      chan []interface{}
	localDataChannel      chan models.LocalData
	stopAllJobs           chan bool
	recoverJobs           chan bool
	scheduler0Config      config.Scheduler0Config
	scheduler0RaftActions Scheduler0RaftActions

	raft.BatchingFSM
}

//go:generate mockery --name Scheduler0RaftStore --output ../mocks
type Scheduler0RaftStore interface {
	GetFSM() raft.FSM
	GetBatchingFSM() raft.BatchingFSM
	GetDataStore() db.DataStore
	GetRaft() *raft.Raft
	UpdateRaft(rft *raft.Raft)
	GetStopAllJobsChannel() chan bool
	GetRecoverJobsChannel() chan bool
	GetQueueJobsChannel() chan []interface{}
}

var _ raft.FSM = &store{}

func NewFSMStore(logger hclog.Logger, scheduler0RaftActions Scheduler0RaftActions, db db.DataStore) Scheduler0RaftStore {
	fsmStoreLogger := logger.Named("fsm-store")

	return &store{
		dataStore:             db,
		queueJobsChannel:      make(chan []interface{}, 1),
		localDataChannel:      make(chan models.LocalData, 1),
		stopAllJobs:           make(chan bool, 1),
		recoverJobs:           make(chan bool, 1),
		logger:                fsmStoreLogger,
		scheduler0RaftActions: scheduler0RaftActions,
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

func (s *store) UpdateRaft(rft *raft.Raft) {
	s.raft = rft
}

func (s *store) GetDataStore() db.DataStore {
	return s.dataStore
}

func (s *store) GetStopAllJobsChannel() chan bool {
	return s.stopAllJobs
}

func (s *store) GetRecoverJobsChannel() chan bool {
	return s.recoverJobs
}

func (s *store) GetQueueJobsChannel() chan []interface{} {
	return s.queueJobsChannel
}

func (s *store) Apply(l *raft.Log) interface{} {
	s.rwMtx.Lock()
	defer s.rwMtx.Unlock()

	fmt.Println("----IN APPLY BLOCK----")

	return s.scheduler0RaftActions.ApplyRaftLog(
		s.logger,
		l,
		s.dataStore,
		true,
		s.queueJobsChannel,
		s.stopAllJobs,
		s.recoverJobs,
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
			true,
			s.queueJobsChannel,
			s.stopAllJobs,
			s.recoverJobs,
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
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("Fatal error getting working dir: %s \n", err)
	}
	dbFilePath := fmt.Sprintf("%v/%v", dir, constants.SqliteDbFileName)
	if err := os.Remove(dbFilePath); err != nil && !os.IsNotExist(err) {
		return err
	}
	if b != nil {
		if err := ioutil.WriteFile(dbFilePath, b, os.ModePerm); err != nil {
			return err
		}
	}

	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?_foreign_keys=1", dbFilePath))
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
