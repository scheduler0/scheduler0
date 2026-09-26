package queue

import (
	"context"
	"errors"
	"io/ioutil"
	"os"
	"scheduler0/pkg/config"
	"scheduler0/pkg/constants"
	"scheduler0/pkg/db"
	"scheduler0/pkg/fsm"
	"scheduler0/pkg/mocks"
	"scheduler0/pkg/models"
	job_queue_repo "scheduler0/pkg/repository/job_queue"
	etcd_service "scheduler0/pkg/service/etcd"
	"scheduler0/pkg/shared_repo"
	"scheduler0/pkg/utils"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit/v6"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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
	sqliteDb.RunMigration(logger)
	sqliteDb.OpenConnectionToExistingDB()
	scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, scheduler0config, sqliteDb, nil, nil, nil, nil, nil)
	JobQueueRepo := job_queue_repo.NewJobQueuesRepo(logger, scheduler0RaftActions, scheduler0Store)
	jobQueue := NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, JobQueueRepo, nil, nil, nil, nil, nil, nil)
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
	sqliteDb.RunMigration(logger)
	sqliteDb.OpenConnectionToExistingDB()
	scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, scheduler0config, sqliteDb, nil, nil, nil, nil, nil)
	JobQueueRepo := job_queue_repo.NewJobQueuesRepo(logger, scheduler0RaftActions, scheduler0Store)
	jobQueue := NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, JobQueueRepo, nil, nil, nil, nil, nil, nil)
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
	sqliteDb.RunMigration(logger)
	sqliteDb.OpenConnectionToExistingDB()
	scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, scheduler0config, sqliteDb, nil, nil, nil, nil, nil)
	jobQueueRepo := job_queue_repo.NewJobQueuesRepo(logger, scheduler0RaftActions, scheduler0Store)
	jobQueue := NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, jobQueueRepo, nil, nil, nil, nil, nil, nil)

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
	sqliteDb.RunMigration(logger)
	sqliteDb.OpenConnectionToExistingDB()
	scheduler0Store := fsm.NewMockScheduler0RaftStore(t)

	os.Setenv("SCHEDULER0_NODE_ID", "1")
	defer os.Unsetenv("SCHEDULER0_NODE_ID")

	f := futureError{}
	scheduler0Store.On("VerifyLeader").Return(raft.Future(f))

	jobQueueRepo := job_queue_repo.NewMockJobQueuesRepo(t)
	jobQueueRepo.On("GetLastVersion").Return(uint64(1))
	jobQueueRepo.On("InsertJobQueueLogs", mock.Anything)
	// Mock GetMostRecentJobQueueDate for quota allocation (called when not in single node mode)
	jobQueueRepo.On("GetMostRecentJobQueueDate").Return(time.Time{}, nil)

	mockJobRepo := mocks.NewMockJobRepo(t)
	jobs := []models.Job{}
	numberOfJEL := 29
	for i := 0; i < numberOfJEL; i++ {
		var jobModel models.Job
		gofakeit.Struct(&jobModel)
		jobModel.ID = uint64(i + 1)
		jobModel.AccountId = uint64((i % 3) + 1) // Distribute across 3 accounts
		jobs = append(jobs, jobModel)
	}
	// Mock BatchGetJobsWithIDRange twice: once for queueing, once for quota allocation
	// Use On() with two separate expectations to ensure both calls return jobs
	mockJobRepo.On("BatchGetJobsWithIDRange", int64(1), int64(29)).Return(jobs, (*utils.GenericError)(nil))
	mockJobRepo.On("BatchGetJobsWithIDRange", int64(1), int64(29)).Return(jobs, (*utils.GenericError)(nil))

	jobQueue := NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, jobQueueRepo, mockJobRepo, nil, nil, nil, nil, nil)
	jobQueue.AddServers([]uint64{1, 2, 3, 4, 5})

	jobQueue.Queue(jobs)

	// Find the InsertJobQueueLogs call and verify the queue logs
	var firstQueueLogs []models.JobQueueLog
	for _, call := range jobQueueRepo.Calls {
		if call.Method == "InsertJobQueueLogs" && len(call.Arguments) > 0 {
			logs := call.Arguments[0].([]models.JobQueueLog)
			// Check if this is the first queue operation (contains job ID 1)
			for _, log := range logs {
				if log.LowerBoundJobId <= 1 && log.UpperBoundJobId >= 1 {
					firstQueueLogs = logs
					break
				}
			}
			if len(firstQueueLogs) > 0 {
				break
			}
		}
	}
	assert.NotEmpty(t, firstQueueLogs, "First queue operation should have created queue logs")

	// Count unique job IDs covered by the queue logs
	jobIdsCovered := make(map[uint64]bool)
	for _, log := range firstQueueLogs {
		for jobId := log.LowerBoundJobId; jobId <= log.UpperBoundJobId; jobId++ {
			jobIdsCovered[jobId] = true
		}
	}
	assert.Equal(t, 29, len(jobIdsCovered), "All 29 jobs should be covered by queue logs")

	jobs2 := []models.Job{}
	for i := 1; i <= 1; i++ {
		var jobModel models.Job
		gofakeit.Struct(&jobModel)
		jobModel.ID = uint64(29 + i)
		jobModel.AccountId = uint64(1)
		jobs2 = append(jobs2, jobModel)
	}
	// Mock BatchGetJobsWithIDRange twice: once for queueing, once for quota allocation
	// Use On() with two separate expectations to ensure both calls return jobs
	mockJobRepo.On("BatchGetJobsWithIDRange", int64(30), int64(30)).Return(jobs2, (*utils.GenericError)(nil))
	mockJobRepo.On("BatchGetJobsWithIDRange", int64(30), int64(30)).Return(jobs2, (*utils.GenericError)(nil))

	jobQueue.Queue(jobs2)

	// Find the InsertJobQueueLogs call for the second queue operation
	var secondQueueLogs []models.JobQueueLog
	for _, call := range jobQueueRepo.Calls {
		if call.Method == "InsertJobQueueLogs" && len(call.Arguments) > 0 {
			logs := call.Arguments[0].([]models.JobQueueLog)
			if len(logs) > 0 && logs[0].LowerBoundJobId == 30 {
				secondQueueLogs = logs
				break
			}
		}
	}
	assert.NotEmpty(t, secondQueueLogs, "Second queue operation should have created queue logs")
	// Verify the second queue log has the correct job ID range
	assert.Equal(t, uint64(30), secondQueueLogs[0].LowerBoundJobId)
	assert.Equal(t, uint64(30), secondQueueLogs[0].UpperBoundJobId)
	// The node ID should be one of the worker nodes (not deterministic due to map iteration)
	assert.Contains(t, []uint64{2, 3, 4, 5}, secondQueueLogs[0].NodeId)
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
	sqliteDb.RunMigration(logger)
	sqliteDb.OpenConnectionToExistingDB()
	scheduler0Store := fsm.NewMockScheduler0RaftStore(t)

	os.Setenv("SCHEDULER0_NODE_ID", "1")
	defer os.Unsetenv("SCHEDULER0_NODE_ID")

	f := futureError{}
	scheduler0Store.On("VerifyLeader").Return(raft.Future(f))

	jobQueueRepo := job_queue_repo.NewMockJobQueuesRepo(t)
	jobQueueRepo.On("GetLastVersion").Return(uint64(1))
	jobQueueRepo.On("InsertJobQueueLogs", mock.Anything)

	mockJobRepo := mocks.NewMockJobRepo(t)
	jobs := []models.Job{}
	numberOfJEL := 29
	for i := 0; i < numberOfJEL; i++ {
		var jobModel models.Job
		gofakeit.Struct(&jobModel)
		jobModel.ID = uint64(i + 1)
		jobModel.AccountId = uint64(1)
		jobs = append(jobs, jobModel)
	}
	mockJobRepo.On("BatchGetJobsWithIDRange", int64(1), int64(29)).Return(jobs, (*utils.GenericError)(nil))

	jobQueue := NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, jobQueueRepo, mockJobRepo, nil, nil, nil, nil, nil)
	jobQueue.SetSingleNodeMode(true)
	jobQueue.AddServers([]uint64{1, 2, 3, 4})

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
	sqliteDb.RunMigration(logger)
	sqliteDb.OpenConnectionToExistingDB()
	scheduler0Store := fsm.NewMockScheduler0RaftStore(t)

	os.Setenv("SCHEDULER0_NODE_ID", "1")
	defer os.Unsetenv("SCHEDULER0_NODE_ID")
	jobQueueRepo := job_queue_repo.NewMockJobQueuesRepo(t)
	jobQueue := NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, jobQueueRepo, nil, nil, nil, nil, nil, nil)

	jobQueue.SetSingleNodeMode(true)

	assert.Equal(t, true, jobQueue.GetSingleNodeMode())
}

type futureWithError struct {
	raft.Future
	err error
}

func (f futureWithError) Error() error {
	return f.err
}

func Test_Queue_Queue_EmptyJobs(t *testing.T) {
	ctx := context.Background()
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "queue-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)
	scheduler0Store := fsm.NewMockScheduler0RaftStore(t)
	jobQueueRepo := job_queue_repo.NewMockJobQueuesRepo(t)

	jobQueue := NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, jobQueueRepo, nil, nil, nil, nil, nil, nil)

	// Queue with empty jobs should return early
	jobQueue.Queue([]models.Job{})

	// Verify no calls were made to repositories
	jobQueueRepo.AssertNotCalled(t, "GetLastVersion")
	jobQueueRepo.AssertNotCalled(t, "InsertJobQueueLogs", mock.Anything)
}

func Test_Queue_Queue_NotLeader(t *testing.T) {
	ctx := context.Background()
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "queue-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)
	scheduler0Store := fsm.NewMockScheduler0RaftStore(t)

	os.Setenv("SCHEDULER0_NODE_ID", "1")
	defer os.Unsetenv("SCHEDULER0_NODE_ID")

	// Mock VerifyLeader to return an error (not leader)
	f := futureWithError{err: errors.New("not leader")}
	scheduler0Store.On("VerifyLeader").Return(raft.Future(f))

	jobQueueRepo := job_queue_repo.NewMockJobQueuesRepo(t)
	mockJobRepo := mocks.NewMockJobRepo(t)

	jobQueue := NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, jobQueueRepo, mockJobRepo, nil, nil, nil, nil, nil)
	jobQueue.AddServers([]uint64{1, 2, 3})

	jobs := []models.Job{
		{ID: 1, AccountId: 1},
		{ID: 2, AccountId: 1},
	}

	// Queue should return early when not leader
	jobQueue.Queue(jobs)

	// Verify no calls were made to repositories
	mockJobRepo.AssertNotCalled(t, "BatchGetJobsWithIDRange", mock.Anything, mock.Anything)
	jobQueueRepo.AssertNotCalled(t, "GetLastVersion")
	jobQueueRepo.AssertNotCalled(t, "InsertJobQueueLogs", mock.Anything)
}

func Test_Queue_Queue_BatchGetJobsError(t *testing.T) {
	ctx := context.Background()
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "queue-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)
	scheduler0Store := fsm.NewMockScheduler0RaftStore(t)

	os.Setenv("SCHEDULER0_NODE_ID", "1")
	defer os.Unsetenv("SCHEDULER0_NODE_ID")

	f := futureError{}
	scheduler0Store.On("VerifyLeader").Return(raft.Future(f))

	jobQueueRepo := job_queue_repo.NewMockJobQueuesRepo(t)
	mockJobRepo := mocks.NewMockJobRepo(t)

	// Mock BatchGetJobsWithIDRange to return an error (must be *utils.GenericError, not error)
	mockJobRepo.On("BatchGetJobsWithIDRange", int64(1), int64(2)).Return([]models.Job{}, utils.HTTPGenericError(500, "database error"))

	jobQueue := NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, jobQueueRepo, mockJobRepo, nil, nil, nil, nil, nil)
	jobQueue.AddServers([]uint64{1, 2, 3})

	jobs := []models.Job{
		{ID: 1, AccountId: 1},
		{ID: 2, AccountId: 1},
	}

	// Queue should handle error gracefully
	jobQueue.Queue(jobs)

	// Verify no queue logs were inserted
	jobQueueRepo.AssertNotCalled(t, "InsertJobQueueLogs", mock.Anything)
}

func Test_Queue_Queue_NoJobsFound(t *testing.T) {
	ctx := context.Background()
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "queue-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)
	scheduler0Store := fsm.NewMockScheduler0RaftStore(t)

	os.Setenv("SCHEDULER0_NODE_ID", "1")
	defer os.Unsetenv("SCHEDULER0_NODE_ID")

	f := futureError{}
	scheduler0Store.On("VerifyLeader").Return(raft.Future(f))

	jobQueueRepo := job_queue_repo.NewMockJobQueuesRepo(t)
	mockJobRepo := mocks.NewMockJobRepo(t)

	// Mock BatchGetJobsWithIDRange to return empty slice
	mockJobRepo.On("BatchGetJobsWithIDRange", int64(1), int64(2)).Return([]models.Job{}, (*utils.GenericError)(nil))

	jobQueue := NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, jobQueueRepo, mockJobRepo, nil, nil, nil, nil, nil)
	jobQueue.AddServers([]uint64{1, 2, 3})

	jobs := []models.Job{
		{ID: 1, AccountId: 1},
		{ID: 2, AccountId: 1},
	}

	// Queue should handle empty result gracefully
	jobQueue.Queue(jobs)

	// Verify no queue logs were inserted
	jobQueueRepo.AssertNotCalled(t, "InsertJobQueueLogs", mock.Anything)
}

func Test_Queue_Queue_NoWorkerNodes(t *testing.T) {
	ctx := context.Background()
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "queue-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)
	scheduler0Store := fsm.NewMockScheduler0RaftStore(t)

	os.Setenv("SCHEDULER0_NODE_ID", "1")
	defer os.Unsetenv("SCHEDULER0_NODE_ID")

	f := futureError{}
	scheduler0Store.On("VerifyLeader").Return(raft.Future(f))

	jobQueueRepo := job_queue_repo.NewMockJobQueuesRepo(t)
	jobQueueRepo.On("GetLastVersion").Return(uint64(1))
	jobQueueRepo.On("InsertJobQueueLogs", mock.Anything)

	mockJobRepo := mocks.NewMockJobRepo(t)
	jobs := []models.Job{
		{ID: 1, AccountId: 1},
		{ID: 2, AccountId: 2},
	}
	// Mock twice: once for queueing, once for quota allocation
	// Use On() with two separate expectations to ensure both calls return jobs
	mockJobRepo.On("BatchGetJobsWithIDRange", int64(1), int64(2)).Return(jobs, (*utils.GenericError)(nil))
	mockJobRepo.On("BatchGetJobsWithIDRange", int64(1), int64(2)).Return(jobs, (*utils.GenericError)(nil))

	jobQueueRepo.On("GetMostRecentJobQueueDate").Return(time.Time{}, nil)

	jobQueue := NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, jobQueueRepo, mockJobRepo, nil, nil, nil, nil, nil)
	// Don't add any servers - should fallback to leader
	jobQueue.AddServers([]uint64{1}) // Only leader node

	jobQueue.Queue(jobs)

	// Verify queue logs were inserted with leader node
	jobQueueRepo.AssertCalled(t, "InsertJobQueueLogs", mock.MatchedBy(func(logs []models.JobQueueLog) bool {
		if len(logs) == 0 {
			return false
		}
		// Should assign to leader (node 1) when no worker nodes
		return logs[0].NodeId == 1
	}))
}

func Test_Queue_Queue_AccountBasedAssignment(t *testing.T) {
	ctx := context.Background()
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "queue-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)
	scheduler0Store := fsm.NewMockScheduler0RaftStore(t)

	os.Setenv("SCHEDULER0_NODE_ID", "1")
	defer os.Unsetenv("SCHEDULER0_NODE_ID")

	f := futureError{}
	scheduler0Store.On("VerifyLeader").Return(raft.Future(f))

	jobQueueRepo := job_queue_repo.NewMockJobQueuesRepo(t)
	jobQueueRepo.On("GetLastVersion").Return(uint64(1))
	jobQueueRepo.On("InsertJobQueueLogs", mock.Anything)

	mockJobRepo := mocks.NewMockJobRepo(t)
	// Create jobs with different account IDs
	jobs := []models.Job{
		{ID: 1, AccountId: 1},
		{ID: 2, AccountId: 1}, // Same account
		{ID: 3, AccountId: 2},
		{ID: 4, AccountId: 2}, // Same account
		{ID: 5, AccountId: 3},
	}
	// Mock twice: once for queueing, once for quota allocation
	// The Queue method calculates minId=1 and maxId=5 from the jobs array
	// Use On() with two separate expectations to ensure both calls return jobs
	mockJobRepo.On("BatchGetJobsWithIDRange", int64(1), int64(5)).Return(jobs, (*utils.GenericError)(nil))
	mockJobRepo.On("BatchGetJobsWithIDRange", int64(1), int64(5)).Return(jobs, (*utils.GenericError)(nil))

	jobQueueRepo.On("GetMostRecentJobQueueDate").Return(time.Time{}, nil)

	jobQueue := NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, jobQueueRepo, mockJobRepo, nil, nil, nil, nil, nil)
	// Add worker nodes (excluding leader)
	jobQueue.AddServers([]uint64{1, 2, 3}) // Leader is 1, workers are 2, 3

	jobQueue.Queue(jobs)

	// Verify queue logs were inserted
	jobQueueRepo.AssertCalled(t, "InsertJobQueueLogs", mock.Anything)

	// Verify allocations were updated
	allocations := jobQueue.GetJobAllocations()
	assert.Greater(t, allocations[2], uint64(0))
	assert.Greater(t, allocations[3], uint64(0))
}

func Test_Queue_AllocateQuotas_FirstTime(t *testing.T) {
	ctx := context.Background()
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "queue-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)
	scheduler0Store := fsm.NewMockScheduler0RaftStore(t)

	os.Setenv("SCHEDULER0_NODE_ID", "1")
	defer os.Unsetenv("SCHEDULER0_NODE_ID")

	f := futureError{}
	scheduler0Store.On("VerifyLeader").Return(raft.Future(f))

	jobQueueRepo := job_queue_repo.NewMockJobQueuesRepo(t)
	jobQueueRepo.On("GetLastVersion").Return(uint64(1))
	jobQueueRepo.On("InsertJobQueueLogs", mock.Anything)
	// First time - no previous queue date
	jobQueueRepo.On("GetMostRecentJobQueueDate").Return(time.Time{}, nil)

	mockJobRepo := mocks.NewMockJobRepo(t)
	jobs := []models.Job{
		{ID: 1, AccountId: 1},
		{ID: 2, AccountId: 1},
	}
	// Mock twice: once for queueing, once for quota allocation
	// The issue: testify might be reusing the same slice reference for both calls
	// Solution: Use On() with RunAndReturn via the typed expecter to create a fresh slice on each call
	// We need to use the typed expecter's RunAndReturn which properly handles function returns
	mockJobRepo.EXPECT().BatchGetJobsWithIDRange(int64(1), int64(2)).RunAndReturn(func(lowerBound int64, upperBound int64) ([]models.Job, *utils.GenericError) {
		// Create a fresh slice each time to avoid any potential modification
		return []models.Job{
			{ID: 1, AccountId: 1},
			{ID: 2, AccountId: 1},
		}, (*utils.GenericError)(nil)
	})
	mockJobRepo.EXPECT().BatchGetJobsWithIDRange(int64(1), int64(2)).RunAndReturn(func(lowerBound int64, upperBound int64) ([]models.Job, *utils.GenericError) {
		// Create a fresh slice each time to avoid any potential modification
		return []models.Job{
			{ID: 1, AccountId: 1},
			{ID: 2, AccountId: 1},
		}, (*utils.GenericError)(nil)
	})
	mockJobRepo.On("UpdateJobsStatusByAccountId", mock.Anything, mock.Anything).Return((*utils.GenericError)(nil))

	mockAccountRepo := mocks.NewMockAccountRepository(t)
	mockAccountRepo.On("GetFeaturesByAccountIds", []uint64{1}).Return(map[uint64][]models.AccountFeature{}, (*utils.GenericError)(nil))

	mockExecutionsRepo := mocks.NewMockJobExecutionsRepo(t)
	// No previous queue date, so GetExecutionUsageByAccountIds won't be called

	mockAccountExecutionsCountRepo := mocks.NewMockAccountJobExecutionsCountRepo(t)
	mockAccountExecutionsCountRepo.On("GetExecutionCountsByAccountIds", []uint64{1}).Return(map[uint64]uint64{}, nil)
	mockAccountExecutionsCountRepo.On("Create", uint64(1), mock.AnythingOfType("uint64")).Return(&models.AccountJobExecutionsCount{}, nil)
	mockAccountExecutionsCountRepo.On("GetExecutionCountsByAccountIds", []uint64{1}).Return(map[uint64]uint64{1: constants.DefaultNumberOfJobExecutions10KPerMonth}, nil)
	mockAccountExecutionsCountRepo.On("UpdateExecutionCount", uint64(1), mock.AnythingOfType("uint64")).Return(nil)

	mockQuotaAllocationSender := NewMockQuotaAllocationSender(t)
	mockEtcdService := etcd_service.NewMockEtcdService(t)
	peers := []config.RaftNode{
		{NodeId: 2, NodeAddress: "127.0.0.1:8080"},
	}
	mockEtcdService.On("GetPeers", ctx).Return(peers, nil)
	mockQuotaAllocationSender.On("SendQuotaAllocation", ctx, peers[0], mock.Anything).Return(nil)

	jobQueue := NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, jobQueueRepo, mockJobRepo, mockExecutionsRepo, mockAccountRepo, mockAccountExecutionsCountRepo, mockQuotaAllocationSender, mockEtcdService)
	jobQueue.AddServers([]uint64{1, 2})

	jobQueue.Queue(jobs)

	// Debug: Check how many times BatchGetJobsWithIDRange was called and what it returned
	// This will help us understand if the slice is being modified
	calls := mockJobRepo.Calls
	batchGetCalls := 0
	for _, call := range calls {
		if call.Method == "BatchGetJobsWithIDRange" {
			batchGetCalls++
			if len(call.ReturnArguments) > 0 {
				if jobsReturned, ok := call.ReturnArguments[0].([]models.Job); ok {
					t.Logf("BatchGetJobsWithIDRange call %d: args=%v, returned %d jobs, slice len=%d, cap=%d",
						batchGetCalls, call.Arguments, len(jobsReturned), len(jobsReturned), cap(jobsReturned))
					if len(jobsReturned) > 0 {
						t.Logf("  First job: ID=%d, AccountId=%d", jobsReturned[0].ID, jobsReturned[0].AccountId)
					} else {
						t.Logf("  WARNING: Returned slice is EMPTY!")
					}
				}
			}
		}
	}
	t.Logf("Total BatchGetJobsWithIDRange calls: %d", batchGetCalls)

	// Verify account execution count was created
	mockAccountExecutionsCountRepo.AssertCalled(t, "Create", uint64(1), mock.AnythingOfType("uint64"))
}

func Test_Queue_AllocateQuotas_WithPreviousQueueDate(t *testing.T) {
	ctx := context.Background()
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "queue-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)
	scheduler0Store := fsm.NewMockScheduler0RaftStore(t)

	os.Setenv("SCHEDULER0_NODE_ID", "1")
	defer os.Unsetenv("SCHEDULER0_NODE_ID")

	f := futureError{}
	scheduler0Store.On("VerifyLeader").Return(raft.Future(f))

	jobQueueRepo := job_queue_repo.NewMockJobQueuesRepo(t)
	jobQueueRepo.On("GetLastVersion").Return(uint64(1))
	jobQueueRepo.On("InsertJobQueueLogs", mock.Anything)
	// Has previous queue date
	previousDate := time.Now().Add(-24 * time.Hour)
	jobQueueRepo.On("GetMostRecentJobQueueDate").Return(previousDate, nil)

	mockJobRepo := mocks.NewMockJobRepo(t)
	jobs := []models.Job{
		{ID: 1, AccountId: 1},
		{ID: 2, AccountId: 1},
	}
	// Mock twice: once for queueing, once for quota allocation
	// The issue: testify might be reusing the same slice reference for both calls
	// Solution: Use On() with RunAndReturn via the typed expecter to create a fresh slice on each call
	// We need to use the typed expecter's RunAndReturn which properly handles function returns
	mockJobRepo.EXPECT().BatchGetJobsWithIDRange(int64(1), int64(2)).RunAndReturn(func(lowerBound int64, upperBound int64) ([]models.Job, *utils.GenericError) {
		// Create a fresh slice each time to avoid any potential modification
		return []models.Job{
			{ID: 1, AccountId: 1},
			{ID: 2, AccountId: 1},
		}, (*utils.GenericError)(nil)
	})
	mockJobRepo.EXPECT().BatchGetJobsWithIDRange(int64(1), int64(2)).RunAndReturn(func(lowerBound int64, upperBound int64) ([]models.Job, *utils.GenericError) {
		// Create a fresh slice each time to avoid any potential modification
		return []models.Job{
			{ID: 1, AccountId: 1},
			{ID: 2, AccountId: 1},
		}, (*utils.GenericError)(nil)
	})
	mockJobRepo.On("UpdateJobsStatusByAccountId", mock.Anything, mock.Anything).Return((*utils.GenericError)(nil))

	mockAccountRepo := mocks.NewMockAccountRepository(t)
	mockAccountRepo.On("GetFeaturesByAccountIds", []uint64{1}).Return(map[uint64][]models.AccountFeature{}, (*utils.GenericError)(nil))

	mockExecutionsRepo := mocks.NewMockJobExecutionsRepo(t)
	// Should be called when there's a previous queue date
	mockExecutionsRepo.On("GetExecutionUsageByAccountIds", []uint64{1}, previousDate).Return(map[uint64]uint64{1: 100}, (*utils.GenericError)(nil))

	mockAccountExecutionsCountRepo := mocks.NewMockAccountJobExecutionsCountRepo(t)
	mockAccountExecutionsCountRepo.On("GetExecutionCountsByAccountIds", []uint64{1}).Return(map[uint64]uint64{1: 900}, nil)
	mockAccountExecutionsCountRepo.On("ResetExecutionCount", uint64(1), mock.AnythingOfType("uint64")).Return(nil)
	mockAccountExecutionsCountRepo.On("GetExecutionCountsByAccountIds", []uint64{1}).Return(map[uint64]uint64{1: 900}, nil)
	mockAccountExecutionsCountRepo.On("UpdateExecutionCount", uint64(1), mock.AnythingOfType("uint64")).Return(nil)

	mockQuotaAllocationSender := NewMockQuotaAllocationSender(t)
	mockEtcdService := etcd_service.NewMockEtcdService(t)
	peers := []config.RaftNode{
		{NodeId: 2, NodeAddress: "127.0.0.1:8080"},
	}
	mockEtcdService.On("GetPeers", ctx).Return(peers, nil)
	mockQuotaAllocationSender.On("SendQuotaAllocation", ctx, peers[0], mock.Anything).Return(nil)

	jobQueue := NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, jobQueueRepo, mockJobRepo, mockExecutionsRepo, mockAccountRepo, mockAccountExecutionsCountRepo, mockQuotaAllocationSender, mockEtcdService)
	jobQueue.AddServers([]uint64{1, 2})

	jobQueue.Queue(jobs)

	// Verify execution usage was retrieved
	mockExecutionsRepo.AssertCalled(t, "GetExecutionUsageByAccountIds", []uint64{1}, previousDate)
	// Verify account execution count was reset (not created)
	mockAccountExecutionsCountRepo.AssertCalled(t, "ResetExecutionCount", uint64(1), mock.AnythingOfType("uint64"))
}

func Test_Queue_AllocateQuotas_QuotaExhausted(t *testing.T) {
	ctx := context.Background()
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "queue-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)
	scheduler0Store := fsm.NewMockScheduler0RaftStore(t)

	os.Setenv("SCHEDULER0_NODE_ID", "1")
	defer os.Unsetenv("SCHEDULER0_NODE_ID")

	f := futureError{}
	scheduler0Store.On("VerifyLeader").Return(raft.Future(f))

	jobQueueRepo := job_queue_repo.NewMockJobQueuesRepo(t)
	jobQueueRepo.On("GetLastVersion").Return(uint64(1))
	jobQueueRepo.On("InsertJobQueueLogs", mock.Anything)
	previousDate := time.Now().Add(-24 * time.Hour)
	jobQueueRepo.On("GetMostRecentJobQueueDate").Return(previousDate, nil)

	mockJobRepo := mocks.NewMockJobRepo(t)
	jobs := []models.Job{
		{ID: 1, AccountId: 1},
		{ID: 2, AccountId: 1},
	}
	// Mock twice: once for queueing, once for quota allocation
	// The issue: testify might be reusing the same slice reference for both calls
	// Solution: Use On() with RunAndReturn via the typed expecter to create a fresh slice on each call
	// We need to use the typed expecter's RunAndReturn which properly handles function returns
	mockJobRepo.EXPECT().BatchGetJobsWithIDRange(int64(1), int64(2)).RunAndReturn(func(lowerBound int64, upperBound int64) ([]models.Job, *utils.GenericError) {
		// Create a fresh slice each time to avoid any potential modification
		return []models.Job{
			{ID: 1, AccountId: 1},
			{ID: 2, AccountId: 1},
		}, (*utils.GenericError)(nil)
	})
	mockJobRepo.EXPECT().BatchGetJobsWithIDRange(int64(1), int64(2)).RunAndReturn(func(lowerBound int64, upperBound int64) ([]models.Job, *utils.GenericError) {
		// Create a fresh slice each time to avoid any potential modification
		return []models.Job{
			{ID: 1, AccountId: 1},
			{ID: 2, AccountId: 1},
		}, (*utils.GenericError)(nil)
	})
	// Quota exhausted - should update job status to inactive
	mockJobRepo.On("UpdateJobsStatusByAccountId", uint64(1), models.JobStatusInactive).Return((*utils.GenericError)(nil))

	mockAccountRepo := mocks.NewMockAccountRepository(t)
	mockAccountRepo.On("GetFeaturesByAccountIds", []uint64{1}).Return(map[uint64][]models.AccountFeature{}, (*utils.GenericError)(nil))

	mockExecutionsRepo := mocks.NewMockJobExecutionsRepo(t)
	// Usage equals limit - quota exhausted
	mockExecutionsRepo.On("GetExecutionUsageByAccountIds", []uint64{1}, previousDate).Return(map[uint64]uint64{1: constants.DefaultNumberOfJobExecutions10KPerMonth}, (*utils.GenericError)(nil))

	mockAccountExecutionsCountRepo := mocks.NewMockAccountJobExecutionsCountRepo(t)
	mockAccountExecutionsCountRepo.On("GetExecutionCountsByAccountIds", []uint64{1}).Return(map[uint64]uint64{}, nil)
	// Remaining quota is 0
	mockAccountExecutionsCountRepo.On("Create", uint64(1), uint64(0)).Return(&models.AccountJobExecutionsCount{}, nil)

	mockQuotaAllocationSender := NewMockQuotaAllocationSender(t)
	mockEtcdService := etcd_service.NewMockEtcdService(t)
	peers := []config.RaftNode{
		{NodeId: 2, NodeAddress: "127.0.0.1:8080"},
	}
	mockEtcdService.On("GetPeers", ctx).Return(peers, nil)
	mockQuotaAllocationSender.On("SendQuotaAllocation", ctx, peers[0], mock.Anything).Return(nil)

	jobQueue := NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, jobQueueRepo, mockJobRepo, mockExecutionsRepo, mockAccountRepo, mockAccountExecutionsCountRepo, mockQuotaAllocationSender, mockEtcdService)
	jobQueue.AddServers([]uint64{1, 2})

	jobQueue.Queue(jobs)

	// Verify job status was updated to inactive
	mockJobRepo.AssertCalled(t, "UpdateJobsStatusByAccountId", uint64(1), models.JobStatusInactive)
}

func Test_Queue_AllocateQuotas_With100KFeature(t *testing.T) {
	ctx := context.Background()
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "queue-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)
	scheduler0Store := fsm.NewMockScheduler0RaftStore(t)

	os.Setenv("SCHEDULER0_NODE_ID", "1")
	defer os.Unsetenv("SCHEDULER0_NODE_ID")

	f := futureError{}
	scheduler0Store.On("VerifyLeader").Return(raft.Future(f))

	jobQueueRepo := job_queue_repo.NewMockJobQueuesRepo(t)
	jobQueueRepo.On("GetLastVersion").Return(uint64(1))
	jobQueueRepo.On("InsertJobQueueLogs", mock.Anything)
	jobQueueRepo.On("GetMostRecentJobQueueDate").Return(time.Time{}, nil)

	mockJobRepo := mocks.NewMockJobRepo(t)
	jobs := []models.Job{
		{ID: 1, AccountId: 1},
		{ID: 2, AccountId: 1},
	}
	// Mock twice: once for queueing, once for quota allocation
	// The issue: testify might be reusing the same slice reference for both calls
	// Solution: Use On() with RunAndReturn via the typed expecter to create a fresh slice on each call
	// We need to use the typed expecter's RunAndReturn which properly handles function returns
	mockJobRepo.EXPECT().BatchGetJobsWithIDRange(int64(1), int64(2)).RunAndReturn(func(lowerBound int64, upperBound int64) ([]models.Job, *utils.GenericError) {
		// Create a fresh slice each time to avoid any potential modification
		return []models.Job{
			{ID: 1, AccountId: 1},
			{ID: 2, AccountId: 1},
		}, (*utils.GenericError)(nil)
	})
	mockJobRepo.EXPECT().BatchGetJobsWithIDRange(int64(1), int64(2)).RunAndReturn(func(lowerBound int64, upperBound int64) ([]models.Job, *utils.GenericError) {
		// Create a fresh slice each time to avoid any potential modification
		return []models.Job{
			{ID: 1, AccountId: 1},
			{ID: 2, AccountId: 1},
		}, (*utils.GenericError)(nil)
	})
	mockJobRepo.On("UpdateJobsStatusByAccountId", mock.Anything, mock.Anything).Return((*utils.GenericError)(nil))

	mockAccountRepo := mocks.NewMockAccountRepository(t)
	// Account has 100K feature
	features := map[uint64][]models.AccountFeature{
		1: {
			{Feature: constants.IncreasedNumberOfJobExecutions100KPerMonthFeature},
		},
	}
	mockAccountRepo.On("GetFeaturesByAccountIds", []uint64{1}).Return(features, (*utils.GenericError)(nil))

	mockExecutionsRepo := mocks.NewMockJobExecutionsRepo(t)

	mockAccountExecutionsCountRepo := mocks.NewMockAccountJobExecutionsCountRepo(t)
	mockAccountExecutionsCountRepo.On("GetExecutionCountsByAccountIds", []uint64{1}).Return(map[uint64]uint64{}, nil)
	// Should use 100K limit
	mockAccountExecutionsCountRepo.On("Create", uint64(1), constants.DefaultNumberOfJobExecutions100KPerMonth).Return(&models.AccountJobExecutionsCount{}, nil)
	mockAccountExecutionsCountRepo.On("GetExecutionCountsByAccountIds", []uint64{1}).Return(map[uint64]uint64{1: constants.DefaultNumberOfJobExecutions100KPerMonth}, nil)
	mockAccountExecutionsCountRepo.On("UpdateExecutionCount", uint64(1), mock.AnythingOfType("uint64")).Return(nil)

	mockQuotaAllocationSender := NewMockQuotaAllocationSender(t)
	mockEtcdService := etcd_service.NewMockEtcdService(t)
	peers := []config.RaftNode{
		{NodeId: 2, NodeAddress: "127.0.0.1:8080"},
	}
	mockEtcdService.On("GetPeers", ctx).Return(peers, nil)
	mockQuotaAllocationSender.On("SendQuotaAllocation", ctx, peers[0], mock.Anything).Return(nil)

	jobQueue := NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, jobQueueRepo, mockJobRepo, mockExecutionsRepo, mockAccountRepo, mockAccountExecutionsCountRepo, mockQuotaAllocationSender, mockEtcdService)
	jobQueue.AddServers([]uint64{1, 2})

	jobQueue.Queue(jobs)

	// Verify account execution count was created with 100K limit
	mockAccountExecutionsCountRepo.AssertCalled(t, "Create", uint64(1), constants.DefaultNumberOfJobExecutions100KPerMonth)
}

func Test_Queue_GetJobAllocations_ThreadSafety(t *testing.T) {
	ctx := context.Background()
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "queue-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)
	scheduler0Store := fsm.NewMockScheduler0RaftStore(t)
	jobQueueRepo := job_queue_repo.NewMockJobQueuesRepo(t)

	jobQueue := NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, jobQueueRepo, nil, nil, nil, nil, nil, nil)

	// Test concurrent access
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id uint64) {
			jobQueue.AddServers([]uint64{id})
			allocations := jobQueue.GetJobAllocations()
			assert.NotNil(t, allocations)
			done <- true
		}(uint64(i))
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all servers were added
	allocations := jobQueue.GetJobAllocations()
	assert.GreaterOrEqual(t, len(allocations), 10)
}
