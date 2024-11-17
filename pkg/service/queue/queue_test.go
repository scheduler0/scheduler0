package queue

import (
	"context"
	"github.com/brianvoe/gofakeit/v6"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"io/ioutil"
	"math"
	"os"
	"scheduler0/pkg/config"
	"scheduler0/pkg/db"
	"scheduler0/pkg/fsm"
	"scheduler0/pkg/models"
	job_queue_repo "scheduler0/pkg/repository/job_queue"
	"scheduler0/pkg/shared_repo"
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
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)
	tempFile, err := ioutil.TempFile("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.RunMigration()
	sqliteDb.OpenConnectionToExistingDB()
	scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, scheduler0config, sqliteDb, nil, nil, nil, nil, nil)
	JobQueueRepo := job_queue_repo.NewJobQueuesRepo(logger, scheduler0RaftActions, scheduler0Store)
	jobQueue := NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, JobQueueRepo)
	jobQueue.AddServers([]uint64{11, 22, 33})
	var defaultAllocations uint64 = 0
	assert.Equal(t, 3, len(jobQueue.GetJobAllocations()))
	assert.Equal(t, defaultAllocations, jobQueue.GetJobAllocations()[11])
	assert.Equal(t, defaultAllocations, jobQueue.GetJobAllocations()[22])
	assert.Equal(t, defaultAllocations, jobQueue.GetJobAllocations()[33])
}

func Test_Queue_RemoveServers(t *testing.T) {
	ctx := context.Background()
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "queue-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)
	tempFile, err := ioutil.TempFile("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.RunMigration()
	sqliteDb.OpenConnectionToExistingDB()
	scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, scheduler0config, sqliteDb, nil, nil, nil, nil, nil)
	JobQueueRepo := job_queue_repo.NewJobQueuesRepo(logger, scheduler0RaftActions, scheduler0Store)
	jobQueue := NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, JobQueueRepo)
	jobQueue.AddServers([]uint64{11, 22, 33})
	var defaultAllocations uint64 = 0
	assert.Equal(t, 3, len(jobQueue.GetJobAllocations()))
	assert.Equal(t, defaultAllocations, jobQueue.GetJobAllocations()[11])
	assert.Equal(t, defaultAllocations, jobQueue.GetJobAllocations()[22])
	assert.Equal(t, defaultAllocations, jobQueue.GetJobAllocations()[33])
	jobQueue.RemoveServers([]uint64{11, 22})
	assert.Equal(t, 1, len(jobQueue.GetJobAllocations()))
	assert.Equal(t, defaultAllocations, jobQueue.GetJobAllocations()[33])
}

func Test_Queue_IncrementQueueVersion(t *testing.T) {
	ctx := context.Background()
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "queue-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)
	tempFile, err := ioutil.TempFile("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.RunMigration()
	sqliteDb.OpenConnectionToExistingDB()
	scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, scheduler0config, sqliteDb, nil, nil, nil, nil, nil)
	jobQueueRepo := job_queue_repo.NewJobQueuesRepo(logger, scheduler0RaftActions, scheduler0Store)
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
	defer cluster.Close()
	cluster.FullyConnect()
	scheduler0Store.UpdateRaft(cluster.Leader())

	assert.Equal(t, jobQueueRepo.GetLastVersion(), uint64(0))

	jobQueue.IncrementQueueVersion()

	assert.Equal(t, jobQueueRepo.GetLastVersion(), uint64(1))
}

type futureError struct {
	raft.Future
}

func (f futureError) Error() error {
	return nil
}

func Test_Queue_Queue(t *testing.T) {
	ctx := context.Background()
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "queue-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)
	tempFile, err := ioutil.TempFile("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.RunMigration()
	sqliteDb.OpenConnectionToExistingDB()
	scheduler0Store := fsm.NewMockScheduler0RaftStore(t)

	os.Setenv("SCHEDULER0_NODE_ID", "1")
	defer os.Unsetenv("SCHEDULER0_NODE_ID")

	f := futureError{}
	scheduler0Store.On("VerifyLeader").Return(raft.Future(f))

	jobQueueRepo := job_queue_repo.NewMockJobQueuesRepo(t)
	jobQueueRepo.On("GetLastVersion").Return(uint64(1))
	jobQueueRepo.On("InsertJobQueueLogs", mock.Anything)
	jobQueue := NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, jobQueueRepo)
	jobQueue.AddServers([]uint64{1, 2, 3, 4, 5})

	jobs := []models.Job{}
	numberOfJEL := 29
	for i := 0; i < numberOfJEL; i++ {
		var jobModel models.Job
		gofakeit.Struct(&jobModel)
		jobModel.ID = uint64(i + 1)
		jobs = append(jobs, jobModel)
	}

	jobQueue.Queue(jobs)
	totalJobsQueued := 0
	minNodeId := math.MaxInt
	minQueueJob := math.MaxInt

	for _, args := range jobQueueRepo.Calls[1].Arguments {
		logs := args.([]models.JobQueueLog)
		for _, jobQueueLog := range logs {
			numQueueJobForNode := int(jobQueueLog.UpperBoundJobId-jobQueueLog.LowerBoundJobId) + 1
			totalJobsQueued += numQueueJobForNode
			if numQueueJobForNode < minQueueJob {
				minNodeId = int(jobQueueLog.NodeId)
				minQueueJob = numQueueJobForNode
			}
		}
	}

	assert.Equal(t, 29, totalJobsQueued)

	jobs = []models.Job{}
	for i := 1; i <= 1; i++ {
		var jobModel models.Job
		gofakeit.Struct(&jobModel)
		jobModel.ID = uint64(29 + i)
		jobs = append(jobs, jobModel)
	}

	jobQueue.Queue(jobs)

	assert.Equal(t, uint64(minNodeId), jobQueueRepo.Calls[3].Arguments[0].([]models.JobQueueLog)[0].NodeId)
}

func Test_Queue_Queue_SingleNodeMode(t *testing.T) {
	ctx := context.Background()
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "queue-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)
	tempFile, err := ioutil.TempFile("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.RunMigration()
	sqliteDb.OpenConnectionToExistingDB()
	scheduler0Store := fsm.NewMockScheduler0RaftStore(t)

	os.Setenv("SCHEDULER0_NODE_ID", "1")
	defer os.Unsetenv("SCHEDULER0_NODE_ID")

	f := futureError{}
	scheduler0Store.On("VerifyLeader").Return(raft.Future(f))

	jobQueueRepo := job_queue_repo.NewMockJobQueuesRepo(t)
	jobQueueRepo.On("GetLastVersion").Return(uint64(1))
	jobQueueRepo.On("InsertJobQueueLogs", mock.Anything)
	jobQueue := NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, jobQueueRepo)
	jobQueue.SetSingleNodeMode(true)
	jobQueue.AddServers([]uint64{1, 2, 3, 4})

	jobs := []models.Job{}
	numberOfJEL := 29
	for i := 0; i < numberOfJEL; i++ {
		var jobModel models.Job
		gofakeit.Struct(&jobModel)
		jobModel.ID = uint64(i + 1)
		jobs = append(jobs, jobModel)
	}

	jobQueue.Queue(jobs)
	totalJobsQueued := 0
	numberOfNodes := 0

	for _, args := range jobQueueRepo.Calls[1].Arguments {
		logs := args.([]models.JobQueueLog)
		for _, jobQueueLog := range logs {
			numQueueJobForNode := int(jobQueueLog.UpperBoundJobId-jobQueueLog.LowerBoundJobId) + 1
			totalJobsQueued += numQueueJobForNode
			numberOfNodes += 1
		}
	}

	assert.Equal(t, 29, totalJobsQueued)
	assert.Equal(t, 1, numberOfNodes)
}

func Test_Queue_SetSingleNodeMode(t *testing.T) {
	ctx := context.Background()
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "queue-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)
	tempFile, err := ioutil.TempFile("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.RunMigration()
	sqliteDb.OpenConnectionToExistingDB()
	scheduler0Store := fsm.NewMockScheduler0RaftStore(t)

	os.Setenv("SCHEDULER0_NODE_ID", "1")
	defer os.Unsetenv("SCHEDULER0_NODE_ID")
	jobQueueRepo := job_queue_repo.NewMockJobQueuesRepo(t)
	jobQueue := NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, jobQueueRepo)

	jobQueue.SetSingleNodeMode(true)

	assert.Equal(t, true, jobQueue.GetSingleNodeMode())
}
