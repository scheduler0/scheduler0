package repository

import (
	sq "github.com/Masterminds/squirrel"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
	"scheduler0/config"
	"scheduler0/db"
	"scheduler0/fsm"
	"scheduler0/models"
	"scheduler0/shared_repo"
	"testing"
	"time"
)

func Test_JobQueuesRepo_GetLastJobQueueLogForNode(t *testing.T) {
	t.Skip()
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "job-queues-repo-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo)
	tempFile, err := ioutil.TempFile("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.RunMigration()
	sqliteDb.OpenConnectionToExistingDB()
	scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, sqliteDb)

	// Create a mock Raft cluster
	cluster := raft.MakeClusterCustom(t, &raft.MakeClusterOpts{
		Peers:          3,
		Bootstrap:      true,
		Conf:           raft.DefaultConfig(),
		ConfigStoreFSM: false,
		MakeFSMFunc: func() raft.FSM {
			return scheduler0Store.GetFSM()
		},
	})
	cluster.FullyConnect()
	scheduler0Store.UpdateRaft(cluster.Leader())

	// Create a new JobQueuesRepo instance
	jobQueuesRepo := NewJobQueuesRepo(logger, scheduler0RaftActions, scheduler0Store)

	// Create job queue logs for testing
	logs := []models.JobQueueLog{
		{
			Id:              1,
			NodeId:          1,
			LowerBoundJobId: 1,
			UpperBoundJobId: 10,
			Version:         1,
			DateCreated:     time.Now(),
		},
		{
			Id:              2,
			NodeId:          1,
			LowerBoundJobId: 11,
			UpperBoundJobId: 20,
			Version:         1,
			DateCreated:     time.Now(),
		},
		{
			Id:              3,
			NodeId:          2,
			LowerBoundJobId: 21,
			UpperBoundJobId: 30,
			Version:         1,
			DateCreated:     time.Now(),
		},
	}

	// Insert the job queue logs into the database
	for _, log := range logs {
		ds := scheduler0Store.GetDataStore()
		insertErr := insertJobQueueLog(logger, ds, log)
		if insertErr != nil {
			t.Fatalf("Failed to insert job queue log: %v", insertErr)
		}
	}

	// Define the node ID and version to query
	nodeID := uint64(1)
	version := uint64(1)

	// Call the GetLastJobQueueLogForNode method
	result := jobQueuesRepo.GetLastJobQueueLogForNode(nodeID, version)

	// Assert the number of retrieved job queue logs
	expectedCount := 2
	assert.Equal(t, expectedCount, len(result))

	// Assert the correctness of the retrieved job queue logs
	expectedLogs := []models.JobQueueLog{logs[0], logs[1]} // Expect the logs to be sorted by date created in descending order
	assert.Equal(t, expectedLogs[0].Id, result[0].Id)
	assert.Equal(t, expectedLogs[0].NodeId, result[0].NodeId)
	assert.Equal(t, expectedLogs[0].LowerBoundJobId, result[0].LowerBoundJobId)
	assert.Equal(t, expectedLogs[0].UpperBoundJobId, result[0].UpperBoundJobId)
	assert.True(t, expectedLogs[0].DateCreated.Equal(result[0].DateCreated))

	assert.Equal(t, expectedLogs[1].Id, result[1].Id)
	assert.Equal(t, expectedLogs[1].NodeId, result[1].NodeId)
	assert.Equal(t, expectedLogs[1].LowerBoundJobId, result[1].LowerBoundJobId)
	assert.Equal(t, expectedLogs[1].UpperBoundJobId, result[1].UpperBoundJobId)
	assert.True(t, expectedLogs[1].DateCreated.Equal(result[1].DateCreated))
}

func Test_JobQueuesRepo_GetLastVersion(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "job-queues-repo-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo)
	tempFile, err := ioutil.TempFile("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.RunMigration()
	sqliteDb.OpenConnectionToExistingDB()
	scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, sqliteDb)

	// Create a mock Raft cluster
	cluster := raft.MakeClusterCustom(t, &raft.MakeClusterOpts{
		Peers:          3,
		Bootstrap:      true,
		Conf:           raft.DefaultConfig(),
		ConfigStoreFSM: false,
		MakeFSMFunc: func() raft.FSM {
			return scheduler0Store.GetFSM()
		},
	})
	cluster.FullyConnect()
	scheduler0Store.UpdateRaft(cluster.Leader())

	// Create a new JobQueuesRepo instance
	jobQueuesRepo := NewJobQueuesRepo(logger, scheduler0RaftActions, scheduler0Store)

	// Create job queue versions for testing
	versions := []uint64{1, 3, 2}

	// Insert the job queue versions into the database
	for _, version := range versions {
		insertErr := insertJobQueueVersion(logger, scheduler0Store.GetDataStore(), version)
		if insertErr != nil {
			t.Fatalf("Failed to insert job queue version: %v", insertErr)
		}
	}

	// Call the GetLastVersion method
	result := jobQueuesRepo.GetLastVersion()

	// Assert the retrieved last version
	expectedVersion := uint64(3)
	assert.Equal(t, expectedVersion, result)
}

func insertJobQueueLog(logger hclog.Logger, dataStore db.DataStore, log models.JobQueueLog) error {
	insertBuilder := sq.Insert(JobQueuesTableName).
		Columns(
			JobQueueIdColumn,
			JobQueueNodeIdColumn,
			JobQueueLowerBoundJobId,
			JobQueueUpperBound,
			JobQueueVersion,
			JobQueueDateCreatedColumn,
		).
		Values(
			log.Id,
			log.NodeId,
			log.LowerBoundJobId,
			log.UpperBoundJobId,
			log.Version,
			log.DateCreated,
		).
		RunWith(dataStore.GetOpenConnection())

	_, err := insertBuilder.Exec()
	if err != nil {
		logger.Error("failed to insert job queue log", err)
		return err
	}

	return nil
}

func insertJobQueueVersion(logger hclog.Logger, dataStore db.DataStore, version uint64) error {
	insertBuilder := sq.Insert(JobQueuesVersionTableName).
		Columns(JobQueueVersion).
		Columns(JobNumberOfActiveNodesVersion).
		Values(version, 1).
		RunWith(dataStore.GetOpenConnection())

	_, err := insertBuilder.Exec()
	if err != nil {
		logger.Error("failed to insert job queue version", err)
		return err
	}

	return nil
}
