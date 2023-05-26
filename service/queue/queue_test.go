package queue

import (
	"context"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
	"scheduler0/config"
	"scheduler0/db"
	"scheduler0/fsm"
	"scheduler0/repository"
	"scheduler0/shared_repo"
	"testing"
)

func Test_Queue_AddServers(t *testing.T) {
	ctx := context.Background()
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "queue-test",
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
	JobQueueRepo := repository.NewJobQueuesRepo(logger, scheduler0RaftActions, scheduler0Store)
	jobQueue := NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, JobQueueRepo)
	jobQueue.AddServers([]uint64{11, 22, 33})
	var defaultAllocations uint64 = 0
	assert.Equal(t, 3, len(jobQueue.allocations))
	assert.Equal(t, defaultAllocations, jobQueue.allocations[11])
	assert.Equal(t, defaultAllocations, jobQueue.allocations[22])
	assert.Equal(t, defaultAllocations, jobQueue.allocations[33])
}

func Test_Queue_RemoveServers(t *testing.T) {
	ctx := context.Background()
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "queue-test",
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
	JobQueueRepo := repository.NewJobQueuesRepo(logger, scheduler0RaftActions, scheduler0Store)
	jobQueue := NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, JobQueueRepo)
	jobQueue.AddServers([]uint64{11, 22, 33})
	var defaultAllocations uint64 = 0
	assert.Equal(t, 3, len(jobQueue.allocations))
	assert.Equal(t, defaultAllocations, jobQueue.allocations[11])
	assert.Equal(t, defaultAllocations, jobQueue.allocations[22])
	assert.Equal(t, defaultAllocations, jobQueue.allocations[33])
	jobQueue.RemoveServers([]uint64{11, 22})
	assert.Equal(t, 1, len(jobQueue.allocations))
	assert.Equal(t, defaultAllocations, jobQueue.allocations[33])
}

func Test_Queue_IncrementQueueVersion(t *testing.T) {
	ctx := context.Background()
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "queue-test",
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
	jobQueueRepo := repository.NewJobQueuesRepo(logger, scheduler0RaftActions, scheduler0Store)
	jobQueue := NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, jobQueueRepo)

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

	assert.Equal(t, jobQueueRepo.GetLastVersion(), uint64(0))

	jobQueue.IncrementQueueVersion()

	assert.Equal(t, jobQueueRepo.GetLastVersion(), uint64(1))
}
