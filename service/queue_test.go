package service

import (
	"context"
	"github.com/brianvoe/gofakeit/v6"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"math"
	"os"
	"scheduler0/config"
	"scheduler0/db"
	"scheduler0/fsm"
	"scheduler0/models"
	"scheduler0/repository"
	"scheduler0/shared_repo"
	"testing"
	"time"
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

func Test_getNextServerToQueue(t *testing.T) {
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
	jobQueue.AddServers([]uint64{11, 22, 33})
	jobQueue.allocations[11] = 3
	jobQueue.allocations[22] = 3
	jobQueue.allocations[33] = 1
	nextNodeId := jobQueue.getNextServerToQueue()
	assert.Equal(t, nextNodeId, uint64(33))
	jobQueue.allocations[11] = 2
	jobQueue.allocations[22] = 1
	jobQueue.allocations[33] = 3
	nextNodeId = jobQueue.getNextServerToQueue()
	assert.Equal(t, nextNodeId, uint64(22))
	jobQueue.allocations[11] = 0
	jobQueue.allocations[22] = 1
	jobQueue.allocations[33] = 1
	nextNodeId = jobQueue.getNextServerToQueue()
	assert.Equal(t, nextNodeId, uint64(11))
}

func Test_queue(t *testing.T) {
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
		Peers:          5,
		Bootstrap:      true,
		Conf:           raft.DefaultConfig(),
		ConfigStoreFSM: false,
		MakeFSMFunc: func() raft.FSM {
			return scheduler0Store.GetFSM()
		},
	})
	cluster.FullyConnect()
	scheduler0Store.UpdateRaft(cluster.Leader())
	jobQueue.AddServers([]uint64{1, 2, 3, 4, 5})

	go func() {
		time.Sleep(time.Second * 3)
		jobQueue.queue(1, 29)
	}()

	var actualAllocations = map[uint64][]int64{}

	for {
		queue := <-scheduler0Store.GetQueueJobsChannel()
		actualAllocations[queue[0].(uint64)] = []int64{
			queue[1].(int64),
			queue[2].(int64),
		}
		if len(actualAllocations) == 4 {
			break
		}
	}
	var minId = math.MaxInt
	var maxId = math.MinInt

	time.Sleep(time.Second * 1)

	for serverId, allocation := range actualAllocations {
		minId = int(math.Min(float64(allocation[0]), float64(minId)))
		maxId = int(math.Max(float64(allocation[1]), float64(maxId)))
		assert.Equal(t, jobQueue.allocations[serverId], uint64(allocation[1]-allocation[0]))
	}

	assert.Equal(t, len(actualAllocations), 4)
	assert.Equal(t, minId, 1)
	assert.Equal(t, maxId, 29)
}

func Test_assignJobRangeToServers_single_server(t *testing.T) {
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
		Peers:          1,
		Bootstrap:      true,
		Conf:           raft.DefaultConfig(),
		ConfigStoreFSM: false,
		MakeFSMFunc: func() raft.FSM {
			return scheduler0Store.GetFSM()
		},
	})
	cluster.FullyConnect()
	scheduler0Store.UpdateRaft(cluster.Leader())
	jobQueue.AddServers([]uint64{11})

	assignments := jobQueue.assignJobRangeToServers(11, 31)

	assert.Equal(t, len(assignments), 1)
}

func Test_assignJobRangeToServers_multi_server(t *testing.T) {
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
		Peers:          4,
		Bootstrap:      true,
		Conf:           raft.DefaultConfig(),
		ConfigStoreFSM: false,
		MakeFSMFunc: func() raft.FSM {
			return scheduler0Store.GetFSM()
		},
	})
	cluster.FullyConnect()
	scheduler0Store.UpdateRaft(cluster.Leader())
	jobQueue.AddServers([]uint64{1, 2, 3, 4})

	assignments := jobQueue.assignJobRangeToServers(11, 29)

	assert.Equal(t, len(assignments), 3)
	assert.Equal(t, assignments[0][0], uint64(11))
	assert.Equal(t, assignments[2][1], uint64(29))
}

func Test_Queue_Queue(t *testing.T) {
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
		Peers:          5,
		Bootstrap:      true,
		Conf:           raft.DefaultConfig(),
		ConfigStoreFSM: false,
		MakeFSMFunc: func() raft.FSM {
			return scheduler0Store.GetFSM()
		},
	})
	cluster.FullyConnect()
	scheduler0Store.UpdateRaft(cluster.Leader())
	jobQueue.AddServers([]uint64{1, 2, 3, 4, 5})

	go func() {
		time.Sleep(time.Second * 3)
		jobs := []models.Job{}
		numberOfJEL := 29
		for i := 0; i < numberOfJEL; i++ {
			var jobModel models.Job
			gofakeit.Struct(&jobModel)
			jobModel.ID = uint64(i + 1)
			jobs = append(jobs, jobModel)
		}
		jobQueue.Queue(jobs)
	}()

	var actualAllocations = map[uint64][]int64{}

	for {
		queue := <-scheduler0Store.GetQueueJobsChannel()
		actualAllocations[queue[0].(uint64)] = []int64{
			queue[1].(int64),
			queue[2].(int64),
		}
		if len(actualAllocations) == 4 {
			break
		}
	}
	var minId = math.MaxInt
	var maxId = math.MinInt

	time.Sleep(time.Second * 1)

	for serverId, allocation := range actualAllocations {
		minId = int(math.Min(float64(allocation[0]), float64(minId)))
		maxId = int(math.Max(float64(allocation[1]), float64(maxId)))
		assert.Equal(t, jobQueue.allocations[serverId], uint64(allocation[1]-allocation[0]))
	}

	assert.Equal(t, len(actualAllocations), 4)
	assert.Equal(t, minId, 1)
	assert.Equal(t, maxId, 29)
	assert.Equal(t, jobQueue.maxId, int64(math.MinInt16))
	assert.Equal(t, jobQueue.minId, int64(math.MaxInt64))
}
