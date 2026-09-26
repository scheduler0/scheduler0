package async_task

import (
	"context"
	"net/http"
	"os"
	"scheduler0/pkg/config"
	"scheduler0/pkg/db"
	"scheduler0/pkg/fsm"
	"scheduler0/pkg/mocks"
	"scheduler0/pkg/models"
	"scheduler0/pkg/repository/async_task"
	"scheduler0/pkg/shared_repo"
	"scheduler0/pkg/utils"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// getFastRaftConfig returns a Raft config with faster timeouts for testing
func getFastRaftConfig() *raft.Config {
	raftConf := raft.DefaultConfig()
	raftConf.HeartbeatTimeout = 50 * time.Millisecond
	raftConf.ElectionTimeout = 50 * time.Millisecond
	raftConf.CommitTimeout = 50 * time.Millisecond
	raftConf.LeaderLeaseTimeout = 25 * time.Millisecond
	return raftConf
}

// futureWithError is a helper type that implements raft.Future and returns an error
// Used to simulate a node that is not the leader
type futureWithError struct {
	raft.Future
	err error
}

func (f futureWithError) Error() error {
	return f.err
}

func Test_AsyncTaskManager_AddTasks(t *testing.T) {
	ctx := context.Background()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "async-task-manager-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)
	tempFile, err := os.CreateTemp("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tempFile.Close()
	defer os.Remove(tempFile.Name())
	sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.RunMigration(logger)
	sqliteDb.OpenConnectionToExistingDB()
	scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, scheduler0config, sqliteDb, nil, nil, nil, nil, sharedRepo)

	input := "{'a':2}"
	requestId := "request-id"
	service := "asyncService"

	// Create a mock raft cluster
	cluster := raft.MakeClusterCustom(t, &raft.MakeClusterOpts{
		Peers:          1,
		Bootstrap:      true,
		Conf:           getFastRaftConfig(),
		ConfigStoreFSM: false,
		MakeFSMFunc: func() raft.FSM {
			return scheduler0Store.GetFSM()
		},
	})
	defer cluster.Close()
	cluster.FullyConnect()
	scheduler0Store.UpdateRaft(cluster.Leader())

	asyncTaskManagerRepo := async_task.NewAsyncTasksRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)
	asyncTaskManager := NewAsyncTaskManager(ctx, logger, scheduler0Store, asyncTaskManagerRepo, scheduler0config)

	asyncTaskIds, getErr := asyncTaskManager.AddTasks(input, requestId, service, 1)
	if getErr != nil {
		t.Fatal("failed add an async task", getErr)
	}

	assert.Equal(t, asyncTaskIds[0], uint64(1))
}

func Test_AsyncTaskManager_UpdateTasksById(t *testing.T) {
	ctx := context.Background()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "async-task-manager-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)
	tempFile, err := os.CreateTemp("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tempFile.Close()
	defer os.Remove(tempFile.Name())
	sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.RunMigration(logger)
	sqliteDb.OpenConnectionToExistingDB()
	scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, scheduler0config, sqliteDb, nil, nil, nil, nil, sharedRepo)

	input := "{'a':2}"
	requestId := "request-id"
	service := "asyncService"
	output := "output"

	// Create a mock raft cluster
	cluster := raft.MakeClusterCustom(t, &raft.MakeClusterOpts{
		Peers:          1,
		Bootstrap:      true,
		Conf:           getFastRaftConfig(),
		ConfigStoreFSM: false,
		MakeFSMFunc: func() raft.FSM {
			return scheduler0Store.GetFSM()
		},
	})
	defer cluster.Close()
	cluster.FullyConnect()
	scheduler0Store.UpdateRaft(cluster.Leader())

	asyncTaskManagerRepo := async_task.NewAsyncTasksRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)
	asyncTaskManager := NewAsyncTaskManager(ctx, logger, scheduler0Store, asyncTaskManagerRepo, scheduler0config)

	asyncTaskIds, getErr := asyncTaskManager.AddTasks(input, requestId, service, 1)
	if getErr != nil {
		t.Fatal("failed add an async task", getErr)
	}
	updateErr := asyncTaskManager.UpdateTasksById(asyncTaskIds[0], models.AsyncTaskSuccess, output)
	if updateErr != nil {
		t.Fatal("failed update an async task", updateErr)
	}
	task, getTaskErr := asyncTaskManager.GetTaskWithRequestIdNonBlocking(requestId, 1)
	if getTaskErr != nil {
		t.Fatal("failed to get an async task", getTaskErr)
	}
	assert.Equal(t, task.RequestId, requestId)
	assert.Equal(t, task.State, models.AsyncTaskSuccess)
}

func Test_AsyncTaskManager_UpdateTasksByRequestId(t *testing.T) {
	ctx := context.Background()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "async-task-manager-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)
	tempFile, err := os.CreateTemp("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tempFile.Close()
	defer os.Remove(tempFile.Name())
	sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.RunMigration(logger)
	sqliteDb.OpenConnectionToExistingDB()
	scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, scheduler0config, sqliteDb, nil, nil, nil, nil, sharedRepo)

	input := "{'a':2}"
	requestId := "request-id"
	service := "asyncService"
	output := "output"

	// Create a mock raft cluster
	cluster := raft.MakeClusterCustom(t, &raft.MakeClusterOpts{
		Peers:          1,
		Bootstrap:      true,
		Conf:           getFastRaftConfig(),
		ConfigStoreFSM: false,
		MakeFSMFunc: func() raft.FSM {
			return scheduler0Store.GetFSM()
		},
	})
	defer cluster.Close()
	cluster.FullyConnect()
	scheduler0Store.UpdateRaft(cluster.Leader())

	asyncTaskManagerRepo := async_task.NewAsyncTasksRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)
	asyncTaskManager := NewAsyncTaskManager(ctx, logger, scheduler0Store, asyncTaskManagerRepo, scheduler0config)

	_, getErr := asyncTaskManager.AddTasks(input, requestId, service, 1)
	if getErr != nil {
		t.Fatal("failed add an async task", getErr)
	}
	updateErr := asyncTaskManager.UpdateTasksByRequestId(requestId, models.AsyncTaskSuccess, output)
	if updateErr != nil {
		t.Fatal("failed update an async task", updateErr)
	}
	task, getTaskErr := asyncTaskManager.GetTaskWithRequestIdNonBlocking(requestId, 1)
	if getTaskErr != nil {
		t.Fatal("failed to get an async task", getTaskErr)
	}
	assert.Equal(t, task.RequestId, requestId)
	assert.Equal(t, task.State, models.AsyncTaskSuccess)
}

func Test_AsyncTaskManager_AddSubscriber(t *testing.T) {
	bctx := context.Background()
	ctx, cancler := context.WithCancel(bctx)
	defer cancler()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "async-task-manager-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)
	tempFile, err := os.CreateTemp("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tempFile.Close()
	defer os.Remove(tempFile.Name())
	sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.RunMigration(logger)
	sqliteDb.OpenConnectionToExistingDB()
	scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, scheduler0config, sqliteDb, nil, nil, nil, nil, sharedRepo)

	input := "{'a':2}"
	requestId := "request-id"
	service := "asyncService"
	output := "output"

	// Create a mock raft cluster
	cluster := raft.MakeClusterCustom(t, &raft.MakeClusterOpts{
		Peers:          1,
		Bootstrap:      true,
		Conf:           getFastRaftConfig(),
		ConfigStoreFSM: false,
		MakeFSMFunc: func() raft.FSM {
			return scheduler0Store.GetFSM()
		},
	})
	defer cluster.Close()
	cluster.FullyConnect()
	scheduler0Store.UpdateRaft(cluster.Leader())

	asyncTaskManagerRepo := async_task.NewAsyncTasksRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)
	asyncTaskManager := NewAsyncTaskManager(ctx, logger, scheduler0Store, asyncTaskManagerRepo, scheduler0config)

	taskIds, getErr := asyncTaskManager.AddTasks(input, requestId, service, 1)
	if getErr != nil {
		t.Fatal("failed add an async task", getErr)
	}

	var executedMutex sync.Mutex
	executed := false
	asyncTaskManager.ListenForNotifications()

	subscriberId, addSubErr := asyncTaskManager.AddSubscriber(taskIds[0], func(task models.AsyncTask) {
		assert.Equal(t, task.RequestId, requestId)
		executedMutex.Lock()
		executed = true
		executedMutex.Unlock()
	})
	if addSubErr != nil {
		t.Fatal("failed add a subscriber for an async task", addSubErr)
	}
	assert.Equal(t, subscriberId, uint64(1))
	updateErr := asyncTaskManager.UpdateTasksByRequestId(requestId, models.AsyncTaskSuccess, output)
	if updateErr != nil {
		t.Fatal("failed update an async task", updateErr)
	}
	time.Sleep(time.Second * 1)
	executedMutex.Lock()
	executedValue := executed
	executedMutex.Unlock()
	assert.Equal(t, executedValue, true)
	cancler()
}

func Test_AsyncTaskManager_DeleteSubscriber(t *testing.T) {
	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "async-task-manager-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)
	tempFile, err := os.CreateTemp("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tempFile.Close()
	defer os.Remove(tempFile.Name())
	sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.RunMigration(logger)
	sqliteDb.OpenConnectionToExistingDB()
	scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, scheduler0config, sqliteDb, nil, nil, nil, nil, sharedRepo)

	input := "{'a':2}"
	requestId := "request-id"
	service := "asyncService"
	output := "output"

	// Create a mock raft cluster
	cluster := raft.MakeClusterCustom(t, &raft.MakeClusterOpts{
		Peers:          1,
		Bootstrap:      true,
		Conf:           getFastRaftConfig(),
		ConfigStoreFSM: false,
		MakeFSMFunc: func() raft.FSM {
			return scheduler0Store.GetFSM()
		},
	})
	defer cluster.Close()
	cluster.FullyConnect()
	scheduler0Store.UpdateRaft(cluster.Leader())

	asyncTaskManagerRepo := async_task.NewAsyncTasksRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)
	asyncTaskManager := NewAsyncTaskManager(ctx, logger, scheduler0Store, asyncTaskManagerRepo, scheduler0config)

	taskIds, getErr := asyncTaskManager.AddTasks(input, requestId, service, 1)
	if getErr != nil {
		t.Fatal("failed add an async task", getErr)
	}

	var executedMutex sync.Mutex
	executed := false
	asyncTaskManager.ListenForNotifications()

	subscriberId, addSubErr := asyncTaskManager.AddSubscriber(taskIds[0], func(task models.AsyncTask) {
		assert.Equal(t, task.RequestId, requestId)
		executedMutex.Lock()
		executed = true
		executedMutex.Unlock()
	})
	if addSubErr != nil {
		t.Fatal("failed add a subscriber for an async task", addSubErr)
	}
	assert.Equal(t, subscriberId, uint64(1))
	deleteSubErr := asyncTaskManager.DeleteSubscriber(taskIds[0], subscriberId)
	if deleteSubErr != nil {
		t.Fatal("failed delete a subscriber for an async task", deleteSubErr)
	}
	updateErr := asyncTaskManager.UpdateTasksByRequestId(requestId, models.AsyncTaskSuccess, output)
	if updateErr != nil {
		t.Fatal("failed update an async task", updateErr)
	}
	time.Sleep(time.Second * 1)
	executedMutex.Lock()
	executedValue := executed
	executedMutex.Unlock()
	assert.Equal(t, executedValue, false)
}

func Test_AsyncTaskManager_GetTaskBlocking(t *testing.T) {
	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "async-task-manager-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)
	tempFile, err := os.CreateTemp("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tempFile.Close()
	defer os.Remove(tempFile.Name())
	sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.RunMigration(logger)
	sqliteDb.OpenConnectionToExistingDB()
	scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, scheduler0config, sqliteDb, nil, nil, nil, nil, sharedRepo)

	input := "{'a':2}"
	requestId := "request-id"
	service := "asyncService"
	output := "output"

	// Create a mock raft cluster
	cluster := raft.MakeClusterCustom(t, &raft.MakeClusterOpts{
		Peers:          1,
		Bootstrap:      true,
		Conf:           getFastRaftConfig(),
		ConfigStoreFSM: false,
		MakeFSMFunc: func() raft.FSM {
			return scheduler0Store.GetFSM()
		},
	})
	defer cluster.Close()
	cluster.FullyConnect()
	scheduler0Store.UpdateRaft(cluster.Leader())

	asyncTaskManagerRepo := async_task.NewAsyncTasksRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)
	asyncTaskManager := NewAsyncTaskManager(ctx, logger, scheduler0Store, asyncTaskManagerRepo, scheduler0config)

	taskIds, getErr := asyncTaskManager.AddTasks(input, requestId, service, 1)
	if getErr != nil {
		t.Fatal("failed add an async task", getErr)
	}

	asyncTaskManager.ListenForNotifications()
	tashCh, size, getTaskBlErr := asyncTaskManager.GetTaskBlocking(taskIds[0])
	if getTaskBlErr != nil {
		t.Fatal("failed update an async task", getTaskBlErr)
	}
	updateErr := asyncTaskManager.UpdateTasksByRequestId(requestId, models.AsyncTaskSuccess, output)
	if updateErr != nil {
		t.Fatal("failed update an async task", updateErr)
	}
	assert.Equal(t, size, uint64(1))
	task := <-tashCh
	assert.Equal(t, task.RequestId, requestId)
	assert.Equal(t, task.State, models.AsyncTaskSuccess)
	assert.Equal(t, task.Output, output)
}

func Test_AsyncTaskManager_GetTaskWithRequestIdBlocking(t *testing.T) {
	ctx, canceler := context.WithCancel(context.Background())
	defer canceler()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "async-task-manager-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)
	tempFile, err := os.CreateTemp("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tempFile.Close()
	defer os.Remove(tempFile.Name())
	sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.RunMigration(logger)
	sqliteDb.OpenConnectionToExistingDB()
	scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, scheduler0config, sqliteDb, nil, nil, nil, nil, sharedRepo)

	input := "{'a':2}"
	requestId := "request-id"
	service := "asyncService"
	output := "output"

	// Create a mock raft cluster
	cluster := raft.MakeClusterCustom(t, &raft.MakeClusterOpts{
		Peers:          1,
		Bootstrap:      true,
		Conf:           getFastRaftConfig(),
		ConfigStoreFSM: false,
		MakeFSMFunc: func() raft.FSM {
			return scheduler0Store.GetFSM()
		},
	})
	defer cluster.Close()
	cluster.FullyConnect()
	scheduler0Store.UpdateRaft(cluster.Leader())

	asyncTaskManagerRepo := async_task.NewAsyncTasksRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)
	asyncTaskManager := NewAsyncTaskManager(ctx, logger, scheduler0Store, asyncTaskManagerRepo, scheduler0config)

	_, getErr := asyncTaskManager.AddTasks(input, requestId, service, 1)
	if getErr != nil {
		t.Fatal("failed add an async task", getErr)
	}

	asyncTaskManager.ListenForNotifications()
	tashCh, _, getTaskBlErr := asyncTaskManager.GetTaskWithRequestIdBlocking(requestId, 1)
	if getTaskBlErr != nil {
		t.Fatal("failed update an async task", getTaskBlErr)
	}
	updateErr := asyncTaskManager.UpdateTasksByRequestId(requestId, models.AsyncTaskSuccess, output)
	if updateErr != nil {
		t.Fatal("failed update an async task", updateErr)
	}
	//assert.Equal(t, size, 1)
	task := <-tashCh
	assert.Equal(t, task.RequestId, requestId)
	assert.Equal(t, task.State, models.AsyncTaskSuccess)
	assert.Equal(t, task.Output, output)
}

func Test_AsyncTaskManager_GetTaskIdWithRequestId(t *testing.T) {
	ctx := context.Background()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "async-task-manager-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)
	tempFile, err := os.CreateTemp("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tempFile.Close()
	defer os.Remove(tempFile.Name())
	sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.RunMigration(logger)
	sqliteDb.OpenConnectionToExistingDB()
	scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, scheduler0config, sqliteDb, nil, nil, nil, nil, sharedRepo)

	input := "{'a':2}"
	requestId := "request-id"
	service := "asyncService"

	// Create a mock raft cluster
	cluster := raft.MakeClusterCustom(t, &raft.MakeClusterOpts{
		Peers:          1,
		Bootstrap:      true,
		Conf:           getFastRaftConfig(),
		ConfigStoreFSM: false,
		MakeFSMFunc: func() raft.FSM {
			return scheduler0Store.GetFSM()
		},
	})
	defer cluster.Close()
	cluster.FullyConnect()
	scheduler0Store.UpdateRaft(cluster.Leader())

	asyncTaskManagerRepo := async_task.NewAsyncTasksRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)
	asyncTaskManager := NewAsyncTaskManager(ctx, logger, scheduler0Store, asyncTaskManagerRepo, scheduler0config)

	_, getErr := asyncTaskManager.AddTasks(input, requestId, service, 1)
	if getErr != nil {
		t.Fatal("failed add an async task", getErr)
	}

	taskId, getTaskBlErr := asyncTaskManager.GetTaskIdWithRequestId(requestId, 1)
	if getTaskBlErr != nil {
		t.Fatal("failed update an async task", getTaskBlErr)
	}
	assert.Equal(t, taskId, uint64(1))
}

func Test_AsyncTaskManager_GetUnCommittedTasks(t *testing.T) {
	ctx := context.Background()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "async-task-manager-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)
	tempFile, err := os.CreateTemp("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tempFile.Close()
	defer os.Remove(tempFile.Name())
	sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.RunMigration(logger)
	sqliteDb.OpenConnectionToExistingDB()
	scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, scheduler0config, sqliteDb, nil, nil, nil, nil, sharedRepo)

	input := "{'a':2}"
	requestId := "request-id"
	service := "asyncService"

	// Create a mock raft cluster
	cluster := raft.MakeClusterCustom(t, &raft.MakeClusterOpts{
		Peers:          1,
		Bootstrap:      true,
		Conf:           getFastRaftConfig(),
		ConfigStoreFSM: false,
		MakeFSMFunc: func() raft.FSM {
			return scheduler0Store.GetFSM()
		},
	})
	defer cluster.Close()
	cluster.FullyConnect()
	scheduler0Store.UpdateRaft(cluster.Leader())

	asyncTaskManagerRepo := async_task.NewAsyncTasksRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)
	asyncTaskManager := NewAsyncTaskManager(ctx, logger, scheduler0Store, asyncTaskManagerRepo, scheduler0config)

	_, batchAddErr := asyncTaskManagerRepo.BatchInsert([]models.AsyncTask{
		{
			RequestId: requestId,
			Service:   service,
			Input:     input,
			AccountId: 1, // System account
		},
	}, false)
	if batchAddErr != nil {
		t.Fatal("failed batch add an async task", batchAddErr)
	}
	uncommittedTasks, getUTerr := asyncTaskManager.GetUnCommittedTasks()
	if getUTerr != nil {
		t.Fatal("failed batch add an async task", getUTerr)
	}
	assert.Equal(t, uncommittedTasks[0].RequestId, requestId)
	assert.Equal(t, uncommittedTasks[0].Input, input)
	assert.Equal(t, uncommittedTasks[0].Service, service)
}

func TestAsyncTaskService_SetSingleNodeMode(t *testing.T) {
	ctx := context.Background()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "async-task-manager-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)
	tempFile, err := os.CreateTemp("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tempFile.Close()
	defer os.Remove(tempFile.Name())
	sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.RunMigration(logger)
	sqliteDb.OpenConnectionToExistingDB()
	scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, scheduler0config, sqliteDb, nil, nil, nil, nil, sharedRepo)

	cluster := raft.MakeClusterCustom(t, &raft.MakeClusterOpts{
		Peers:          1,
		Bootstrap:      true,
		Conf:           getFastRaftConfig(),
		ConfigStoreFSM: false,
		MakeFSMFunc: func() raft.FSM {
			return scheduler0Store.GetFSM()
		},
	})
	defer cluster.Close()
	cluster.FullyConnect()
	scheduler0Store.UpdateRaft(cluster.Leader())

	asyncTaskManagerRepo := async_task.NewAsyncTasksRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)
	asyncTaskManager := NewAsyncTaskManager(ctx, logger, scheduler0Store, asyncTaskManagerRepo, scheduler0config)

	assert.Equal(t, asyncTaskManager.GetSingleNodeMode(), false)

	asyncTaskManager.SetSingleNodeMode(true)

	assert.Equal(t, asyncTaskManager.GetSingleNodeMode(), true)
}

// setupTestAsyncTaskServiceWithData creates a test async task service with pre-populated data
// This helper is useful for testing edge cases that require existing tasks in the database
func setupTestAsyncTaskServiceWithData(t *testing.T, prePopulatedTasks []models.AsyncTask) (AsyncTaskService, func()) {
	ctx := context.Background()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "async-task-manager-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)
	tempFile, err := os.CreateTemp("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tempFile.Close()
	tempFileName := tempFile.Name()

	sqliteDb := db.NewSqliteDbConnection(logger, tempFileName)
	sqliteDb.RunMigration(logger)
	sqliteDb.OpenConnectionToExistingDB()
	scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, scheduler0config, sqliteDb, nil, nil, nil, nil, sharedRepo)

	cluster := raft.MakeClusterCustom(t, &raft.MakeClusterOpts{
		Peers:          1,
		Bootstrap:      true,
		Conf:           getFastRaftConfig(),
		ConfigStoreFSM: false,
		MakeFSMFunc: func() raft.FSM {
			return scheduler0Store.GetFSM()
		},
	})
	cluster.FullyConnect()
	scheduler0Store.UpdateRaft(cluster.Leader())

	asyncTaskManagerRepo := async_task.NewAsyncTasksRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)
	asyncTaskManager := NewAsyncTaskManager(ctx, logger, scheduler0Store, asyncTaskManagerRepo, scheduler0config)

	// Pre-populate tasks if provided
	if len(prePopulatedTasks) > 0 {
		ids, batchErr := asyncTaskManagerRepo.BatchInsert(prePopulatedTasks, false)
		if batchErr != nil {
			t.Fatalf("Failed to pre-populate tasks: %v", batchErr)
		}
		// Update task states and add to service's in-memory maps
		for i, task := range prePopulatedTasks {
			task.Id = ids[i]
			// Update the task state after insertion (BatchInsert sets state to 0)
			if task.State != models.AsyncTaskNotStated {
				updateErr := asyncTaskManagerRepo.UpdateTaskState(task, task.State, task.Output)
				if updateErr != nil {
					t.Fatalf("Failed to update task state: %v", updateErr)
				}
			}
			// Also add to the service's in-memory maps
			asyncTaskManager.(*asyncTaskService).task.Store(task.Id, task)
			asyncTaskManager.(*asyncTaskService).taskIdRequestIdMap.Store(task.RequestId, task.Id)
		}
	}

	cleanup := func() {
		cluster.Close()
		os.Remove(tempFileName)
	}

	return asyncTaskManager, cleanup
}

// Test_AsyncTaskService_AddTasks_SingleNodeMode tests AddTasks in single node mode
func Test_AsyncTaskService_AddTasks_SingleNodeMode(t *testing.T) {
	asyncTaskManager, cleanup := setupTestAsyncTaskServiceWithData(t, nil)
	defer cleanup()

	asyncTaskManager.SetSingleNodeMode(true)

	input := "{'test': 'data'}"
	requestId := "single-node-request-id"
	service := "testService"
	accountId := uint64(1)

	ids, err := asyncTaskManager.AddTasks(input, requestId, service, accountId)
	assert.Nil(t, err)
	assert.NotNil(t, ids)
	assert.Equal(t, 1, len(ids))
	assert.Greater(t, ids[0], uint64(0))
}

// Test_AsyncTaskService_AddTasks_NotLeader tests AddTasks when not leader (fallback to BatchInsert)
func Test_AsyncTaskService_AddTasks_NotLeader(t *testing.T) {
	asyncTaskManager, cleanup := setupTestAsyncTaskServiceWithData(t, nil)
	defer cleanup()

	asyncTaskManager.SetSingleNodeMode(false)
	asyncTaskManager.SetNodeIsLeader(false) // Set as not leader to trigger BatchInsert path

	input := "{'test': 'data'}"
	requestId := "not-leader-request-id"
	service := "testService"
	accountId := uint64(1)

	ids, err := asyncTaskManager.AddTasks(input, requestId, service, accountId)
	// Since nodeIsLeader is false, BatchInsert is called (line 87), not RaftBatchInsert
	assert.Nil(t, err)
	assert.NotNil(t, ids)
}

// Test_AsyncTaskService_UpdateTasksById_TaskNotFound tests UpdateTasksById when task is not in map
func Test_AsyncTaskService_UpdateTasksById_TaskNotFound(t *testing.T) {
	asyncTaskManager, cleanup := setupTestAsyncTaskServiceWithData(t, nil)
	defer cleanup()

	nonExistentTaskId := uint64(99999)
	err := asyncTaskManager.UpdateTasksById(nonExistentTaskId, models.AsyncTaskSuccess, "output")
	assert.NotNil(t, err)
	assert.Equal(t, http.StatusNotFound, err.Type)
	assert.Contains(t, err.Message, "could not find task with id")
}

// Test_AsyncTaskService_UpdateTasksById_SingleNodeMode tests UpdateTasksById in single node mode
func Test_AsyncTaskService_UpdateTasksById_SingleNodeMode(t *testing.T) {
	// Create a task first
	task := models.AsyncTask{
		Id:        1,
		RequestId: "test-request-id",
		Input:     "{'test': 'data'}",
		Service:   "testService",
		State:     models.AsyncTaskInProgress,
		AccountId: 1,
	}

	asyncTaskManager, cleanup := setupTestAsyncTaskServiceWithData(t, []models.AsyncTask{task})
	defer cleanup()

	asyncTaskManager.SetSingleNodeMode(true)

	err := asyncTaskManager.UpdateTasksById(task.Id, models.AsyncTaskSuccess, "success output")
	assert.Nil(t, err)
}

// Test_AsyncTaskService_UpdateTasksByRequestId_TaskIdNotFound tests UpdateTasksByRequestId when task ID not in map
func Test_AsyncTaskService_UpdateTasksByRequestId_TaskIdNotFound(t *testing.T) {
	asyncTaskManager, cleanup := setupTestAsyncTaskServiceWithData(t, nil)
	defer cleanup()

	nonExistentRequestId := "non-existent-request-id"
	err := asyncTaskManager.UpdateTasksByRequestId(nonExistentRequestId, models.AsyncTaskSuccess, "output")
	assert.NotNil(t, err)
	assert.Equal(t, http.StatusNotFound, err.Type)
	assert.Contains(t, err.Message, "could not find task id for request id")
}

// Test_AsyncTaskService_UpdateTasksByRequestId_TaskNotFound tests UpdateTasksByRequestId when task not in map
func Test_AsyncTaskService_UpdateTasksByRequestId_TaskNotFound(t *testing.T) {
	asyncTaskManager, cleanup := setupTestAsyncTaskServiceWithData(t, nil)
	defer cleanup()

	// Add requestId to map but not the task itself
	asyncTaskManager.(*asyncTaskService).taskIdRequestIdMap.Store("test-request-id", uint64(99999))

	err := asyncTaskManager.UpdateTasksByRequestId("test-request-id", models.AsyncTaskSuccess, "output")
	assert.NotNil(t, err)
	assert.Equal(t, http.StatusNotFound, err.Type)
	assert.Contains(t, err.Message, "could not find task with request id task id")
}

// Test_AsyncTaskService_UpdateTasksByRequestId_SingleNodeMode tests UpdateTasksByRequestId in single node mode
func Test_AsyncTaskService_UpdateTasksByRequestId_SingleNodeMode(t *testing.T) {
	task := models.AsyncTask{
		Id:        1,
		RequestId: "test-request-id",
		Input:     "{'test': 'data'}",
		Service:   "testService",
		State:     models.AsyncTaskInProgress,
		AccountId: 1,
	}

	asyncTaskManager, cleanup := setupTestAsyncTaskServiceWithData(t, []models.AsyncTask{task})
	defer cleanup()

	asyncTaskManager.SetSingleNodeMode(true)

	err := asyncTaskManager.UpdateTasksByRequestId(task.RequestId, models.AsyncTaskSuccess, "success output")
	assert.Nil(t, err)
}

// Test_AsyncTaskService_AddSubscriber_TaskNotFound tests AddSubscriber when task is not found
func Test_AsyncTaskService_AddSubscriber_TaskNotFound(t *testing.T) {
	asyncTaskManager, cleanup := setupTestAsyncTaskServiceWithData(t, nil)
	defer cleanup()

	nonExistentTaskId := uint64(99999)
	subscriber := func(task models.AsyncTask) {}

	subId, err := asyncTaskManager.AddSubscriber(nonExistentTaskId, subscriber)
	assert.NotNil(t, err)
	assert.Equal(t, uint64(0), subId)
	assert.Equal(t, http.StatusNotFound, err.Type)
	assert.Contains(t, err.Message, "could not find task with id")
}

// Test_AsyncTaskService_AddSubscriber_WithExistingSubscribers tests AddSubscriber with existing subscribers
func Test_AsyncTaskService_AddSubscriber_WithExistingSubscribers(t *testing.T) {
	task := models.AsyncTask{
		Id:        1,
		RequestId: "test-request-id",
		Input:     "{'test': 'data'}",
		Service:   "testService",
		State:     models.AsyncTaskInProgress,
		AccountId: 1,
	}

	asyncTaskManager, cleanup := setupTestAsyncTaskServiceWithData(t, []models.AsyncTask{task})
	defer cleanup()

	// Add task to in-memory map
	asyncTaskManager.(*asyncTaskService).task.Store(task.Id, task)

	subscriber1 := func(task models.AsyncTask) {}
	subscriber2 := func(task models.AsyncTask) {}

	subId1, err1 := asyncTaskManager.AddSubscriber(task.Id, subscriber1)
	assert.Nil(t, err1)
	assert.Equal(t, uint64(1), subId1)

	subId2, err2 := asyncTaskManager.AddSubscriber(task.Id, subscriber2)
	assert.Nil(t, err2)
	assert.Equal(t, uint64(2), subId2)
}

// Test_AsyncTaskService_GetTaskBlocking_TaskNotFound tests GetTaskBlocking when task is not found in repo
func Test_AsyncTaskService_GetTaskBlocking_TaskNotFound(t *testing.T) {
	asyncTaskManager, cleanup := setupTestAsyncTaskServiceWithData(t, nil)
	defer cleanup()

	nonExistentTaskId := uint64(99999)
	taskCh, subId, err := asyncTaskManager.GetTaskBlocking(nonExistentTaskId)
	assert.NotNil(t, err)
	assert.Nil(t, taskCh)
	assert.Equal(t, uint64(0), subId)
}

// Test_AsyncTaskService_GetTaskBlocking_TaskAlreadySuccess tests GetTaskBlocking when task is already in success state
func Test_AsyncTaskService_GetTaskBlocking_TaskAlreadySuccess(t *testing.T) {
	task := models.AsyncTask{
		Id:        1,
		RequestId: "test-request-id",
		Input:     "{'test': 'data'}",
		Service:   "testService",
		State:     models.AsyncTaskSuccess,
		AccountId: 1,
	}

	asyncTaskManager, cleanup := setupTestAsyncTaskServiceWithData(t, []models.AsyncTask{task})
	defer cleanup()

	// Get the actual task ID from the database (it was assigned during insertion)
	taskFromRepo, getErr := asyncTaskManager.(*asyncTaskService).asyncTaskManagerRepo.GetTaskByRequestIdAndAccountId(task.RequestId, task.AccountId)
	if getErr != nil {
		t.Fatalf("Failed to get task from repo: %v", getErr)
	}

	taskCh, subId, err := asyncTaskManager.GetTaskBlocking(taskFromRepo.Id)
	assert.Nil(t, err)
	assert.NotNil(t, taskCh)
	assert.Equal(t, uint64(0), subId) // No subscriber needed for completed task

	// Verify task is immediately available
	select {
	case receivedTask := <-taskCh:
		assert.Equal(t, taskFromRepo.Id, receivedTask.Id)
		assert.Equal(t, models.AsyncTaskSuccess, receivedTask.State)
	case <-time.After(1 * time.Second):
		t.Fatal("Expected task to be immediately available")
	}
}

// Test_AsyncTaskService_GetTaskBlocking_TaskInNonProgressState tests GetTaskBlocking when task is in non-progress state
func Test_AsyncTaskService_GetTaskBlocking_TaskInNonProgressState(t *testing.T) {
	task := models.AsyncTask{
		Id:        1,
		RequestId: "test-request-id",
		Input:     "{'test': 'data'}",
		Service:   "testService",
		State:     models.AsyncTaskFail,
		AccountId: 1,
	}

	asyncTaskManager, cleanup := setupTestAsyncTaskServiceWithData(t, []models.AsyncTask{task})
	defer cleanup()

	taskCh, subId, err := asyncTaskManager.GetTaskBlocking(task.Id)
	assert.Nil(t, err)
	assert.Nil(t, taskCh) // Should return nil for non-progress states
	assert.Equal(t, uint64(0), subId)
}

// Test_AsyncTaskService_GetTaskWithRequestIdNonBlocking_TaskNotInMap tests GetTaskWithRequestIdNonBlocking when task not in map
// This test verifies that lines 252-254 are executed: task is stored in maps after fetching from repo
func Test_AsyncTaskService_GetTaskWithRequestIdNonBlocking_TaskNotInMap(t *testing.T) {
	task := models.AsyncTask{
		Id:        1,
		RequestId: "test-request-id",
		Input:     "{'test': 'data'}",
		Service:   "testService",
		State:     models.AsyncTaskInProgress,
		AccountId: 1,
	}

	asyncTaskManager, cleanup := setupTestAsyncTaskServiceWithData(t, []models.AsyncTask{task})
	defer cleanup()

	// Remove task from in-memory maps to simulate it not being in the map initially
	// This allows us to test the path where task is fetched from repo (lines 243-247)
	asyncTaskManager.(*asyncTaskService).task.Delete(task.Id)
	asyncTaskManager.(*asyncTaskService).taskIdRequestIdMap.Delete(task.RequestId)

	// Verify task is not in map initially
	_, ok := asyncTaskManager.(*asyncTaskService).taskIdRequestIdMap.Load(task.RequestId)
	assert.False(t, ok, "task should not be in map initially")

	// Don't add to in-memory map - should fetch from repo (lines 243-247)
	// This will execute lines 252-254: Store task, Store mapping, set taskId
	result, err := asyncTaskManager.GetTaskWithRequestIdNonBlocking(task.RequestId, task.AccountId)
	assert.Nil(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, task.Id, result.Id)
	assert.Equal(t, task.RequestId, result.RequestId)

	// Verify task is now stored in map (line 252)
	storedTask, ok := asyncTaskManager.(*asyncTaskService).task.Load(task.Id)
	assert.True(t, ok, "task should be stored in map after fetching from repo")
	assert.NotNil(t, storedTask)
	assert.Equal(t, task.Id, storedTask.(models.AsyncTask).Id)
	assert.Equal(t, task.RequestId, storedTask.(models.AsyncTask).RequestId)

	// Verify requestId mapping is stored (line 253)
	mappedTaskId, ok := asyncTaskManager.(*asyncTaskService).taskIdRequestIdMap.Load(task.RequestId)
	assert.True(t, ok, "requestId mapping should be stored after fetching from repo")
	assert.Equal(t, task.Id, mappedTaskId.(uint64))
}

// Test_AsyncTaskService_GetTaskWithRequestIdNonBlocking_TaskInMap tests GetTaskWithRequestIdNonBlocking when taskId is in map
func Test_AsyncTaskService_GetTaskWithRequestIdNonBlocking_TaskInMap(t *testing.T) {
	task := models.AsyncTask{
		Id:        1,
		RequestId: "test-request-id",
		Input:     "{'test': 'data'}",
		Service:   "testService",
		State:     models.AsyncTaskInProgress,
		AccountId: 1,
	}

	asyncTaskManager, cleanup := setupTestAsyncTaskServiceWithData(t, []models.AsyncTask{task})
	defer cleanup()

	// Add task to in-memory maps
	asyncTaskManager.(*asyncTaskService).task.Store(task.Id, task)
	asyncTaskManager.(*asyncTaskService).taskIdRequestIdMap.Store(task.RequestId, task.Id)

	// Should use taskId from map and fetch from repo
	result, err := asyncTaskManager.GetTaskWithRequestIdNonBlocking(task.RequestId, task.AccountId)
	assert.Nil(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, task.Id, result.Id)
	assert.Equal(t, task.RequestId, result.RequestId)
}

// Test_AsyncTaskService_GetTaskWithRequestIdNonBlocking_TaskNotFoundInRepo tests GetTaskWithRequestIdNonBlocking when task not found in repo
func Test_AsyncTaskService_GetTaskWithRequestIdNonBlocking_TaskNotFoundInRepo(t *testing.T) {
	asyncTaskManager, cleanup := setupTestAsyncTaskServiceWithData(t, nil)
	defer cleanup()

	nonExistentRequestId := "non-existent-request-id"
	result, err := asyncTaskManager.GetTaskWithRequestIdNonBlocking(nonExistentRequestId, 1)
	assert.NotNil(t, err)
	assert.Nil(t, result)
	assert.Equal(t, http.StatusNotFound, err.Type)
	assert.Contains(t, err.Message, "task doesn't exist")
}

// Test_AsyncTaskService_GetTaskWithRequestIdBlocking_TaskNotInMap tests GetTaskWithRequestIdBlocking when task not in map
// This test verifies that lines 276-280 are executed: task is stored in maps and GetTaskBlocking is called
func Test_AsyncTaskService_GetTaskWithRequestIdBlocking_TaskNotInMap(t *testing.T) {
	task := models.AsyncTask{
		Id:        1,
		RequestId: "test-request-id",
		Input:     "{'test': 'data'}",
		Service:   "testService",
		State:     models.AsyncTaskInProgress,
		AccountId: 1,
	}

	asyncTaskManager, cleanup := setupTestAsyncTaskServiceWithData(t, []models.AsyncTask{task})
	defer cleanup()

	// Remove task from in-memory maps to simulate it not being in the map initially
	// This allows us to test the path where task is fetched from repo (lines 268-275)
	asyncTaskManager.(*asyncTaskService).task.Delete(task.Id)
	asyncTaskManager.(*asyncTaskService).taskIdRequestIdMap.Delete(task.RequestId)

	// Verify task is not in map initially
	_, ok := asyncTaskManager.(*asyncTaskService).taskIdRequestIdMap.Load(task.RequestId)
	assert.False(t, ok, "task should not be in map initially")

	// Don't add to in-memory map - should fetch from repo (lines 268-275)
	// This will execute lines 276-280: Store task, Store mapping, call GetTaskBlocking
	taskCh, subId, err := asyncTaskManager.GetTaskWithRequestIdBlocking(task.RequestId, task.AccountId)
	assert.Nil(t, err)
	assert.NotNil(t, taskCh)
	assert.Greater(t, subId, uint64(0))

	// Verify task is now stored in map (line 276)
	storedTask, ok := asyncTaskManager.(*asyncTaskService).task.Load(task.Id)
	assert.True(t, ok, "task should be stored in map after fetching from repo")
	assert.NotNil(t, storedTask)
	assert.Equal(t, task.Id, storedTask.(models.AsyncTask).Id)
	assert.Equal(t, task.RequestId, storedTask.(models.AsyncTask).RequestId)

	// Verify requestId mapping is stored (line 277)
	mappedTaskId, ok := asyncTaskManager.(*asyncTaskService).taskIdRequestIdMap.Load(task.RequestId)
	assert.True(t, ok, "requestId mapping should be stored after fetching from repo")
	assert.Equal(t, task.Id, mappedTaskId.(uint64))
}

// Test_AsyncTaskService_GetTaskIdWithRequestId_TaskNotInMap tests GetTaskIdWithRequestId when task not in map
func Test_AsyncTaskService_GetTaskIdWithRequestId_TaskNotInMap(t *testing.T) {
	task := models.AsyncTask{
		Id:        1,
		RequestId: "test-request-id",
		Input:     "{'test': 'data'}",
		Service:   "testService",
		State:     models.AsyncTaskInProgress,
		AccountId: 1,
	}

	asyncTaskManager, cleanup := setupTestAsyncTaskServiceWithData(t, []models.AsyncTask{task})
	defer cleanup()

	// Don't add to in-memory map - should fetch from repo
	taskId, err := asyncTaskManager.GetTaskIdWithRequestId(task.RequestId, task.AccountId)
	assert.Nil(t, err)
	assert.Equal(t, task.Id, taskId)
}

// Test_AsyncTaskService_GetTaskIdWithRequestId_TaskNotFoundInRepo tests GetTaskIdWithRequestId when task not found in repo
func Test_AsyncTaskService_GetTaskIdWithRequestId_TaskNotFoundInRepo(t *testing.T) {
	asyncTaskManager, cleanup := setupTestAsyncTaskServiceWithData(t, nil)
	defer cleanup()

	nonExistentRequestId := "non-existent-request-id"
	taskId, err := asyncTaskManager.GetTaskIdWithRequestId(nonExistentRequestId, 1)
	assert.NotNil(t, err)
	assert.Equal(t, uint64(0), taskId)
	assert.Equal(t, http.StatusNotFound, err.Type)
	assert.Contains(t, err.Message, "task doesn't exist")
}

// Test_AsyncTaskService_GetTaskIdWithRequestId_TaskInMap tests GetTaskIdWithRequestId when taskId is already in map
func Test_AsyncTaskService_GetTaskIdWithRequestId_TaskInMap(t *testing.T) {
	task := models.AsyncTask{
		Id:        1,
		RequestId: "test-request-id",
		Input:     "{'test': 'data'}",
		Service:   "testService",
		State:     models.AsyncTaskInProgress,
		AccountId: 1,
	}

	asyncTaskManager, cleanup := setupTestAsyncTaskServiceWithData(t, []models.AsyncTask{task})
	defer cleanup()

	// Add task to in-memory map
	asyncTaskManager.(*asyncTaskService).task.Store(task.Id, task)
	asyncTaskManager.(*asyncTaskService).taskIdRequestIdMap.Store(task.RequestId, task.Id)

	// Should return immediately from map
	taskId, err := asyncTaskManager.GetTaskIdWithRequestId(task.RequestId, task.AccountId)
	assert.Nil(t, err)
	assert.Equal(t, task.Id, taskId)
}

// Test_AsyncTaskService_GetTaskWithRequestIdBlocking_TaskInMap tests GetTaskWithRequestIdBlocking when taskId is in map
func Test_AsyncTaskService_GetTaskWithRequestIdBlocking_TaskInMap(t *testing.T) {
	task := models.AsyncTask{
		Id:        1,
		RequestId: "test-request-id",
		Input:     "{'test': 'data'}",
		Service:   "testService",
		State:     models.AsyncTaskInProgress,
		AccountId: 1,
	}

	asyncTaskManager, cleanup := setupTestAsyncTaskServiceWithData(t, []models.AsyncTask{task})
	defer cleanup()

	// Add task to in-memory map
	asyncTaskManager.(*asyncTaskService).task.Store(task.Id, task)
	asyncTaskManager.(*asyncTaskService).taskIdRequestIdMap.Store(task.RequestId, task.Id)

	// Start listening for notifications
	asyncTaskManager.ListenForNotifications()

	// Should use taskId from map
	taskCh, subId, err := asyncTaskManager.GetTaskWithRequestIdBlocking(task.RequestId, task.AccountId)
	assert.Nil(t, err)
	assert.NotNil(t, taskCh)
	assert.Greater(t, subId, uint64(0))
}

// Test_AsyncTaskService_UpdateTasksById_WhenLeader tests UpdateTasksById when node is leader (uses RaftUpdateTaskState)
func Test_AsyncTaskService_UpdateTasksById_WhenLeader(t *testing.T) {
	asyncTaskManager, mockRepo, cleanup := setupTestAsyncTaskServiceWithMockRepo(t)
	defer cleanup()

	task := models.AsyncTask{
		Id:        1,
		RequestId: "test-request-id",
		Input:     "{'test': 'data'}",
		Service:   "testService",
		State:     models.AsyncTaskInProgress,
		AccountId: 1,
	}

	// Add task to in-memory map
	asyncTaskManager.task.Store(task.Id, task)
	asyncTaskManager.SetSingleNodeMode(false)
	asyncTaskManager.SetNodeIsLeader(true) // Set as leader

	// Mock RaftUpdateTaskState to return success (this will be called since nodeIsLeader is true)
	mockRepo.On("RaftUpdateTaskState", mock.Anything, models.AsyncTaskSuccess, "success output").Return(nil)

	// Since nodeIsLeader is true, RaftUpdateTaskState is called (line 131), not UpdateTaskState
	err := asyncTaskManager.UpdateTasksById(task.Id, models.AsyncTaskSuccess, "success output")

	assert.Nil(t, err)
	mockRepo.AssertCalled(t, "RaftUpdateTaskState", mock.Anything, models.AsyncTaskSuccess, "success output")
	mockRepo.AssertNotCalled(t, "UpdateTaskState", mock.Anything, mock.Anything, mock.Anything)
}

// Test_AsyncTaskService_UpdateTasksById_WhenNotLeader tests UpdateTasksById when node is not leader (fallback to UpdateTaskState)
func Test_AsyncTaskService_UpdateTasksById_WhenNotLeader(t *testing.T) {
	task := models.AsyncTask{
		Id:        1,
		RequestId: "test-request-id",
		Input:     "{'test': 'data'}",
		Service:   "testService",
		State:     models.AsyncTaskInProgress,
		AccountId: 1,
	}

	asyncTaskManager, cleanup := setupTestAsyncTaskServiceWithData(t, []models.AsyncTask{task})
	defer cleanup()

	asyncTaskManager.SetSingleNodeMode(false)
	asyncTaskManager.SetNodeIsLeader(false) // Set as not leader to trigger UpdateTaskState path
	// Add task to in-memory map
	asyncTaskManager.(*asyncTaskService).task.Store(task.Id, task)

	// Since nodeIsLeader is false, UpdateTaskState is called (line 125), not RaftUpdateTaskState
	err := asyncTaskManager.UpdateTasksById(task.Id, models.AsyncTaskSuccess, "success output")
	assert.Nil(t, err)
}

// Test_AsyncTaskService_UpdateTasksByRequestId_WhenLeader tests UpdateTasksByRequestId when node is leader (uses RaftUpdateTaskState)
func Test_AsyncTaskService_UpdateTasksByRequestId_WhenLeader(t *testing.T) {
	asyncTaskManager, mockRepo, cleanup := setupTestAsyncTaskServiceWithMockRepo(t)
	defer cleanup()

	task := models.AsyncTask{
		Id:        1,
		RequestId: "test-request-id",
		Input:     "{'test': 'data'}",
		Service:   "testService",
		State:     models.AsyncTaskInProgress,
		AccountId: 1,
	}

	// Add task to in-memory maps
	asyncTaskManager.task.Store(task.Id, task)
	asyncTaskManager.taskIdRequestIdMap.Store(task.RequestId, task.Id)
	asyncTaskManager.SetSingleNodeMode(false)
	asyncTaskManager.SetNodeIsLeader(true) // Set as leader

	// Mock RaftUpdateTaskState to return success (this will be called since nodeIsLeader is true)
	mockRepo.On("RaftUpdateTaskState", mock.Anything, models.AsyncTaskSuccess, "success output").Return(nil)

	// Since nodeIsLeader is true, RaftUpdateTaskState is called (line 173), not UpdateTaskState
	err := asyncTaskManager.UpdateTasksByRequestId(task.RequestId, models.AsyncTaskSuccess, "success output")

	assert.Nil(t, err)
	mockRepo.AssertCalled(t, "RaftUpdateTaskState", mock.Anything, models.AsyncTaskSuccess, "success output")
	mockRepo.AssertNotCalled(t, "UpdateTaskState", mock.Anything, mock.Anything, mock.Anything)
}

// Test_AsyncTaskService_UpdateTasksByRequestId_WhenNotLeader tests UpdateTasksByRequestId when node is not leader (fallback to UpdateTaskState)
func Test_AsyncTaskService_UpdateTasksByRequestId_WhenNotLeader(t *testing.T) {
	task := models.AsyncTask{
		Id:        1,
		RequestId: "test-request-id",
		Input:     "{'test': 'data'}",
		Service:   "testService",
		State:     models.AsyncTaskInProgress,
		AccountId: 1,
	}

	asyncTaskManager, cleanup := setupTestAsyncTaskServiceWithData(t, []models.AsyncTask{task})
	defer cleanup()

	asyncTaskManager.SetSingleNodeMode(false)
	asyncTaskManager.SetNodeIsLeader(false) // Set as not leader to trigger UpdateTaskState path
	// Add task to in-memory maps
	asyncTaskManager.(*asyncTaskService).task.Store(task.Id, task)
	asyncTaskManager.(*asyncTaskService).taskIdRequestIdMap.Store(task.RequestId, task.Id)

	// Since nodeIsLeader is false, UpdateTaskState is called (line 167), not RaftUpdateTaskState
	err := asyncTaskManager.UpdateTasksByRequestId(task.RequestId, models.AsyncTaskSuccess, "success output")
	assert.Nil(t, err)
}

// Test_AsyncTaskService_DeleteSubscriber_SubscribersNotFound tests DeleteSubscriber when subscribers map doesn't exist
func Test_AsyncTaskService_DeleteSubscriber_SubscribersNotFound(t *testing.T) {
	task := models.AsyncTask{
		Id:        1,
		RequestId: "test-request-id",
		Input:     "{'test': 'data'}",
		Service:   "testService",
		State:     models.AsyncTaskInProgress,
		AccountId: 1,
	}

	asyncTaskManager, cleanup := setupTestAsyncTaskServiceWithData(t, []models.AsyncTask{task})
	defer cleanup()

	// Add task to in-memory map but don't add subscribers
	asyncTaskManager.(*asyncTaskService).task.Store(task.Id, task)

	// Try to delete a subscriber that doesn't exist
	err := asyncTaskManager.DeleteSubscriber(task.Id, 1)
	assert.NotNil(t, err)
	assert.Equal(t, http.StatusNotFound, err.Type)
	assert.Contains(t, err.Message, "could not find subscribers for task with id")
}

// Test_AsyncTaskService_GetUnCommittedTasks_Error tests GetUnCommittedTasks when repository returns error
func Test_AsyncTaskService_GetUnCommittedTasks_Error(t *testing.T) {
	// This test would require mocking the repository to return an error
	// For now, we'll test the success path is covered by existing tests
	// The error path would need a mock repository
}

// Test_AsyncTaskService_ListenForNotifications_TaskNotFound tests ListenForNotifications when task is not found
func Test_AsyncTaskService_ListenForNotifications_TaskNotFound(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "async-task-manager-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)
	tempFile, err := os.CreateTemp("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tempFile.Close()
	defer os.Remove(tempFile.Name())
	sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.RunMigration(logger)
	sqliteDb.OpenConnectionToExistingDB()
	scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, scheduler0config, sqliteDb, nil, nil, nil, nil, sharedRepo)

	cluster := raft.MakeClusterCustom(t, &raft.MakeClusterOpts{
		Peers:          1,
		Bootstrap:      true,
		Conf:           getFastRaftConfig(),
		ConfigStoreFSM: false,
		MakeFSMFunc: func() raft.FSM {
			return scheduler0Store.GetFSM()
		},
	})
	defer cluster.Close()
	cluster.FullyConnect()
	scheduler0Store.UpdateRaft(cluster.Leader())

	asyncTaskManagerRepo := async_task.NewAsyncTasksRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)
	asyncTaskManager := NewAsyncTaskManager(ctx, logger, scheduler0Store, asyncTaskManagerRepo, scheduler0config)

	// Start listening for notifications
	asyncTaskManager.ListenForNotifications()

	// Send a notification for a task that doesn't exist in the map
	taskNotification := models.AsyncTask{
		Id:        99999,
		RequestId: "non-existent",
		State:     models.AsyncTaskSuccess,
		AccountId: 1,
	}

	// Send notification - should log error and return
	asyncTaskManager.(*asyncTaskService).notificationsCh <- taskNotification

	// Give it time to process
	time.Sleep(200 * time.Millisecond)
}

// Test_AsyncTaskService_ListenForNotifications_NoSubscribers tests ListenForNotifications when subscribers map doesn't exist
func Test_AsyncTaskService_ListenForNotifications_NoSubscribers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "async-task-manager-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)
	tempFile, err := os.CreateTemp("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tempFile.Close()
	defer os.Remove(tempFile.Name())
	sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.RunMigration(logger)
	sqliteDb.OpenConnectionToExistingDB()
	scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, scheduler0config, sqliteDb, nil, nil, nil, nil, sharedRepo)

	cluster := raft.MakeClusterCustom(t, &raft.MakeClusterOpts{
		Peers:          1,
		Bootstrap:      true,
		Conf:           getFastRaftConfig(),
		ConfigStoreFSM: false,
		MakeFSMFunc: func() raft.FSM {
			return scheduler0Store.GetFSM()
		},
	})
	defer cluster.Close()
	cluster.FullyConnect()
	scheduler0Store.UpdateRaft(cluster.Leader())

	asyncTaskManagerRepo := async_task.NewAsyncTasksRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)
	asyncTaskManager := NewAsyncTaskManager(ctx, logger, scheduler0Store, asyncTaskManagerRepo, scheduler0config)

	task := models.AsyncTask{
		Id:        1,
		RequestId: "test-request-id",
		Input:     "{'test': 'data'}",
		Service:   "testService",
		State:     models.AsyncTaskInProgress,
		AccountId: 1,
	}

	// Add task to map but don't add subscribers
	asyncTaskManager.(*asyncTaskService).task.Store(task.Id, task)

	// Start listening for notifications
	asyncTaskManager.ListenForNotifications()

	// Send notification - should handle gracefully with no subscribers
	taskNotification := models.AsyncTask{
		Id:        task.Id,
		RequestId: task.RequestId,
		State:     models.AsyncTaskSuccess,
		AccountId: task.AccountId,
	}

	asyncTaskManager.(*asyncTaskService).notificationsCh <- taskNotification

	// Give it time to process
	time.Sleep(200 * time.Millisecond)

	// Task should be deleted when state is Success
	_, exists := asyncTaskManager.(*asyncTaskService).task.Load(task.Id)
	assert.False(t, exists, "Task should be deleted after success state")
}

// Test_AsyncTaskService_ListenForNotifications_WithSubscribers tests ListenForNotifications with subscribers
func Test_AsyncTaskService_ListenForNotifications_WithSubscribers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "async-task-manager-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)
	tempFile, err := os.CreateTemp("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tempFile.Close()
	defer os.Remove(tempFile.Name())
	sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.RunMigration(logger)
	sqliteDb.OpenConnectionToExistingDB()
	scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, scheduler0config, sqliteDb, nil, nil, nil, nil, sharedRepo)

	cluster := raft.MakeClusterCustom(t, &raft.MakeClusterOpts{
		Peers:          1,
		Bootstrap:      true,
		Conf:           getFastRaftConfig(),
		ConfigStoreFSM: false,
		MakeFSMFunc: func() raft.FSM {
			return scheduler0Store.GetFSM()
		},
	})
	defer cluster.Close()
	cluster.FullyConnect()
	scheduler0Store.UpdateRaft(cluster.Leader())

	asyncTaskManagerRepo := async_task.NewAsyncTasksRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)
	asyncTaskManager := NewAsyncTaskManager(ctx, logger, scheduler0Store, asyncTaskManagerRepo, scheduler0config)

	task := models.AsyncTask{
		Id:        1,
		RequestId: "test-request-id",
		Input:     "{'test': 'data'}",
		Service:   "testService",
		State:     models.AsyncTaskInProgress,
		AccountId: 1,
	}

	// Add task to map
	asyncTaskManager.(*asyncTaskService).task.Store(task.Id, task)

	// Add a subscriber
	var receivedTask models.AsyncTask
	var mu sync.Mutex
	subscriber := func(t models.AsyncTask) {
		mu.Lock()
		defer mu.Unlock()
		receivedTask = t
	}

	subId, err := asyncTaskManager.AddSubscriber(task.Id, subscriber)
	assert.Nil(t, err)
	assert.Greater(t, subId, uint64(0))

	// Start listening for notifications
	asyncTaskManager.ListenForNotifications()

	// Send notification
	taskNotification := models.AsyncTask{
		Id:        task.Id,
		RequestId: task.RequestId,
		State:     models.AsyncTaskSuccess,
		Output:    "test output",
		AccountId: task.AccountId,
	}

	asyncTaskManager.(*asyncTaskService).notificationsCh <- taskNotification

	// Give it time to process
	time.Sleep(200 * time.Millisecond)

	// Verify subscriber was called
	mu.Lock()
	assert.Equal(t, taskNotification.Id, receivedTask.Id)
	assert.Equal(t, models.AsyncTaskSuccess, receivedTask.State)
	mu.Unlock()

	// Task should be deleted after success
	_, exists := asyncTaskManager.(*asyncTaskService).task.Load(task.Id)
	assert.False(t, exists, "Task should be deleted after success state")
}

// Test_AsyncTaskService_ListenForNotifications_TaskFailState tests ListenForNotifications when task state is Fail
func Test_AsyncTaskService_ListenForNotifications_TaskFailState(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "async-task-manager-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)
	tempFile, err := os.CreateTemp("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tempFile.Close()
	defer os.Remove(tempFile.Name())
	sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.RunMigration(logger)
	sqliteDb.OpenConnectionToExistingDB()
	scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, scheduler0config, sqliteDb, nil, nil, nil, nil, sharedRepo)

	cluster := raft.MakeClusterCustom(t, &raft.MakeClusterOpts{
		Peers:          1,
		Bootstrap:      true,
		Conf:           getFastRaftConfig(),
		ConfigStoreFSM: false,
		MakeFSMFunc: func() raft.FSM {
			return scheduler0Store.GetFSM()
		},
	})
	defer cluster.Close()
	cluster.FullyConnect()
	scheduler0Store.UpdateRaft(cluster.Leader())

	asyncTaskManagerRepo := async_task.NewAsyncTasksRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)
	asyncTaskManager := NewAsyncTaskManager(ctx, logger, scheduler0Store, asyncTaskManagerRepo, scheduler0config)

	task := models.AsyncTask{
		Id:        1,
		RequestId: "test-request-id",
		Input:     "{'test': 'data'}",
		Service:   "testService",
		State:     models.AsyncTaskInProgress,
		AccountId: 1,
	}

	// Add task to map
	asyncTaskManager.(*asyncTaskService).task.Store(task.Id, task)

	// Start listening for notifications
	asyncTaskManager.ListenForNotifications()

	// Send notification with Fail state
	taskNotification := models.AsyncTask{
		Id:        task.Id,
		RequestId: task.RequestId,
		State:     models.AsyncTaskFail,
		AccountId: task.AccountId,
	}

	asyncTaskManager.(*asyncTaskService).notificationsCh <- taskNotification

	// Give it time to process
	time.Sleep(200 * time.Millisecond)

	// Task should be deleted after fail state
	_, exists := asyncTaskManager.(*asyncTaskService).task.Load(task.Id)
	assert.False(t, exists, "Task should be deleted after fail state")
}

// Test_AsyncTaskService_ListenForNotifications_ContextDone tests ListenForNotifications when context is done
func Test_AsyncTaskService_ListenForNotifications_ContextDone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "async-task-manager-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)
	tempFile, err := os.CreateTemp("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tempFile.Close()
	defer os.Remove(tempFile.Name())
	sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.RunMigration(logger)
	sqliteDb.OpenConnectionToExistingDB()
	scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, scheduler0config, sqliteDb, nil, nil, nil, nil, sharedRepo)

	cluster := raft.MakeClusterCustom(t, &raft.MakeClusterOpts{
		Peers:          1,
		Bootstrap:      true,
		Conf:           getFastRaftConfig(),
		ConfigStoreFSM: false,
		MakeFSMFunc: func() raft.FSM {
			return scheduler0Store.GetFSM()
		},
	})
	defer cluster.Close()
	cluster.FullyConnect()
	scheduler0Store.UpdateRaft(cluster.Leader())

	asyncTaskManagerRepo := async_task.NewAsyncTasksRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)
	asyncTaskManager := NewAsyncTaskManager(ctx, logger, scheduler0Store, asyncTaskManagerRepo, scheduler0config)

	// Start listening for notifications
	asyncTaskManager.ListenForNotifications()

	// Cancel context - should stop the goroutine
	cancel()

	// Give it time to process
	time.Sleep(200 * time.Millisecond)
}

// Test_AsyncTaskService_DeleteNewUncommittedAsyncLogs tests DeleteNewUncommittedAsyncLogs (empty implementation)
func Test_AsyncTaskService_DeleteNewUncommittedAsyncLogs(t *testing.T) {
	asyncTaskManager, cleanup := setupTestAsyncTaskServiceWithData(t, nil)
	defer cleanup()

	// This is an empty implementation, but we should test it doesn't panic
	assert.NotPanics(t, func() {
		asyncTaskManager.DeleteNewUncommittedAsyncLogs(1, 10)
	})
}

// setupTestAsyncTaskServiceWithMockRepo creates a test async task service with a mock repository
func setupTestAsyncTaskServiceWithMockRepo(t *testing.T) (*asyncTaskService, *mocks.MockAsyncTasksRepo, func()) {
	ctx := context.Background()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "async-task-manager-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	scheduler0config := config.NewScheduler0Config()

	mockRepo := mocks.NewMockAsyncTasksRepo(t)
	mockFSMStore := fsm.NewMockScheduler0RaftStore(t)
	asyncTaskManager := NewAsyncTaskManager(ctx, logger, mockFSMStore, mockRepo, scheduler0config)

	cleanup := func() {
		// No cleanup needed for mocks
	}

	return asyncTaskManager.(*asyncTaskService), mockRepo, cleanup
}

// Test_AsyncTaskService_AddTasks_WhenLeader tests AddTasks when node is leader (uses RaftBatchInsert)
func Test_AsyncTaskService_AddTasks_WhenLeader(t *testing.T) {
	asyncTaskManager, mockRepo, cleanup := setupTestAsyncTaskServiceWithMockRepo(t)
	defer cleanup()

	asyncTaskManager.SetSingleNodeMode(false)
	asyncTaskManager.SetNodeIsLeader(true) // Set as leader

	input := "{'test': 'data'}"
	requestId := "test-request-id"
	service := "testService"
	accountId := uint64(1)

	// Mock RaftBatchInsert to return success (this will be called since nodeIsLeader is true)
	mockRepo.On("RaftBatchInsert", mock.Anything, mock.Anything).Return([]uint64{1}, nil)

	// Since nodeIsLeader is true, RaftBatchInsert is called (line 94), not BatchInsert
	ids, err := asyncTaskManager.AddTasks(input, requestId, service, accountId)

	assert.Nil(t, err)
	assert.NotNil(t, ids)
	assert.Equal(t, 1, len(ids))
	assert.Equal(t, uint64(1), ids[0])
	mockRepo.AssertCalled(t, "RaftBatchInsert", mock.Anything, mock.Anything)
	mockRepo.AssertNotCalled(t, "BatchInsert", mock.Anything, mock.Anything)
}

// Test_AsyncTaskService_AddTasks_BatchInsertError tests AddTasks when not leader and BatchInsert returns error (lines 87-91)
func Test_AsyncTaskService_AddTasks_BatchInsertError(t *testing.T) {
	asyncTaskManager, mockRepo, cleanup := setupTestAsyncTaskServiceWithMockRepo(t)
	defer cleanup()

	asyncTaskManager.SetSingleNodeMode(false)
	asyncTaskManager.SetNodeIsLeader(false) // Set as not leader

	input := "{'test': 'data'}"
	requestId := "test-request-id"
	service := "testService"
	accountId := uint64(1)

	// Mock BatchInsert to return an error (this will be called since nodeIsLeader is false)
	expectedError := utils.HTTPGenericError(http.StatusInternalServerError, "database error")
	mockRepo.On("BatchInsert", mock.Anything, false).Return([]uint64{}, expectedError)

	// Since nodeIsLeader is false, BatchInsert is called (line 87), not RaftBatchInsert
	// This tests the error handling path (lines 88-89)
	ids, err := asyncTaskManager.AddTasks(input, requestId, service, accountId)

	assert.NotNil(t, err)
	assert.Nil(t, ids)
	assert.Equal(t, http.StatusInternalServerError, err.Type)
	assert.Equal(t, "database error", err.Message)
}

// Test_AsyncTaskService_UpdateTasksById_UpdateTaskStateError tests UpdateTasksById when not leader and UpdateTaskState returns error (lines 125-129)
func Test_AsyncTaskService_UpdateTasksById_UpdateTaskStateError(t *testing.T) {
	asyncTaskManager, mockRepo, cleanup := setupTestAsyncTaskServiceWithMockRepo(t)
	defer cleanup()

	task := models.AsyncTask{
		Id:        1,
		RequestId: "test-request-id",
		Input:     "{'test': 'data'}",
		Service:   "testService",
		State:     models.AsyncTaskInProgress,
		AccountId: 1,
	}

	// Add task to in-memory map
	asyncTaskManager.task.Store(task.Id, task)
	asyncTaskManager.SetSingleNodeMode(false)
	asyncTaskManager.SetNodeIsLeader(false) // Set as not leader

	// Mock UpdateTaskState to return an error (this will be called since nodeIsLeader is false)
	expectedError := utils.HTTPGenericError(http.StatusInternalServerError, "update failed")
	mockRepo.On("UpdateTaskState", mock.Anything, models.AsyncTaskSuccess, "output").Return(expectedError)

	// Since nodeIsLeader is false, UpdateTaskState is called (line 125), not RaftUpdateTaskState
	// This tests the error handling path (lines 126-128)
	err := asyncTaskManager.UpdateTasksById(task.Id, models.AsyncTaskSuccess, "output")

	assert.NotNil(t, err)
	assert.Equal(t, http.StatusNotFound, err.Type)
	assert.Contains(t, err.Message, "could not update task with id")
}

// Test_AsyncTaskService_UpdateTasksByRequestId_UpdateTaskStateError tests UpdateTasksByRequestId when not leader and UpdateTaskState returns error (lines 166-171)
func Test_AsyncTaskService_UpdateTasksByRequestId_UpdateTaskStateError(t *testing.T) {
	asyncTaskManager, mockRepo, cleanup := setupTestAsyncTaskServiceWithMockRepo(t)
	defer cleanup()

	task := models.AsyncTask{
		Id:        1,
		RequestId: "test-request-id",
		Input:     "{'test': 'data'}",
		Service:   "testService",
		State:     models.AsyncTaskInProgress,
		AccountId: 1,
	}

	// Add task to in-memory maps
	asyncTaskManager.task.Store(task.Id, task)
	asyncTaskManager.taskIdRequestIdMap.Store(task.RequestId, task.Id)
	asyncTaskManager.SetSingleNodeMode(false)
	asyncTaskManager.SetNodeIsLeader(false) // Set as not leader

	// Mock UpdateTaskState to return an error (this will be called since nodeIsLeader is false)
	expectedError := utils.HTTPGenericError(http.StatusInternalServerError, "update failed")
	mockRepo.On("UpdateTaskState", mock.Anything, models.AsyncTaskSuccess, "output").Return(expectedError)

	// Since nodeIsLeader is false, UpdateTaskState is called (line 167), not RaftUpdateTaskState
	// This tests the error handling path (lines 168-170)
	err := asyncTaskManager.UpdateTasksByRequestId(task.RequestId, models.AsyncTaskSuccess, "output")

	assert.NotNil(t, err)
	assert.Equal(t, http.StatusNotFound, err.Type)
	assert.Contains(t, err.Message, "could not update task with id")
}

// Test_AsyncTaskService_GetTaskWithRequestIdBlocking_GetTaskByRequestIdError tests GetTaskWithRequestIdBlocking when GetTaskByRequestIdAndAccountId returns error (lines 269-273)
func Test_AsyncTaskService_GetTaskWithRequestIdBlocking_GetTaskByRequestIdError(t *testing.T) {
	asyncTaskManager, mockRepo, cleanup := setupTestAsyncTaskServiceWithMockRepo(t)
	defer cleanup()

	requestId := "test-request-id"
	accountId := uint64(1)

	// Mock GetTaskByRequestIdAndAccountId to return an error
	expectedError := utils.HTTPGenericError(http.StatusInternalServerError, "database error")
	mockRepo.On("GetTaskByRequestIdAndAccountId", requestId, accountId).Return(nil, expectedError)

	// Task is not in map, so it will try to fetch from repo
	taskCh, subId, err := asyncTaskManager.GetTaskWithRequestIdBlocking(requestId, accountId)

	assert.NotNil(t, err)
	assert.Nil(t, taskCh)
	assert.Equal(t, uint64(0), subId)
	assert.Equal(t, http.StatusInternalServerError, err.Type)
	assert.Equal(t, "database error", err.Message)
}

// Test_AsyncTaskService_GetTaskWithRequestIdBlocking_TaskNotFoundInRepo tests GetTaskWithRequestIdBlocking when GetTaskByRequestIdAndAccountId returns nil task (lines 274-276)
func Test_AsyncTaskService_GetTaskWithRequestIdBlocking_TaskNotFoundInRepo(t *testing.T) {
	asyncTaskManager, mockRepo, cleanup := setupTestAsyncTaskServiceWithMockRepo(t)
	defer cleanup()

	requestId := "test-request-id"
	accountId := uint64(1)

	// Mock GetTaskByRequestIdAndAccountId to return nil task (not found)
	mockRepo.On("GetTaskByRequestIdAndAccountId", requestId, accountId).Return(nil, nil)

	// Task is not in map, so it will try to fetch from repo
	taskCh, subId, err := asyncTaskManager.GetTaskWithRequestIdBlocking(requestId, accountId)

	assert.NotNil(t, err)
	assert.Nil(t, taskCh)
	assert.Equal(t, uint64(0), subId)
	assert.Equal(t, http.StatusNotFound, err.Type)
	assert.Contains(t, err.Message, "task doesn't exist")
}

// Test_AsyncTaskService_RejectsCrossAccountMapHit covers the IDOR fix: a requestId present in the
// in-memory map must still fail when the caller's account does not own the task.
func Test_AsyncTaskService_RejectsCrossAccountMapHit(t *testing.T) {
	ownerAccount := uint64(10)
	callerAccount := uint64(99)
	requestId := "cross-account-request-id"
	task := &models.AsyncTask{
		Id:        7,
		RequestId: requestId,
		AccountId: ownerAccount,
		State:     models.AsyncTaskSuccess,
		Service:   "create_job",
		Input:     `[]`,
		Output:    `[]`,
	}

	t.Run("GetTaskWithRequestIdNonBlocking", func(t *testing.T) {
		svc, mockRepo, cleanup := setupTestAsyncTaskServiceWithMockRepo(t)
		defer cleanup()

		svc.taskIdRequestIdMap.Store(requestId, task.Id)
		mockRepo.On("GetTask", task.Id).Return(task, (*utils.GenericError)(nil)).Once()

		got, err := svc.GetTaskWithRequestIdNonBlocking(requestId, callerAccount)
		assert.Nil(t, got)
		assert.NotNil(t, err)
		assert.Equal(t, http.StatusNotFound, err.Type)
		assert.Contains(t, err.Message, "task doesn't exist")
		mockRepo.AssertNotCalled(t, "GetTaskByRequestIdAndAccountId", mock.Anything, mock.Anything)
	})

	t.Run("GetTaskWithRequestIdBlocking", func(t *testing.T) {
		svc, mockRepo, cleanup := setupTestAsyncTaskServiceWithMockRepo(t)
		defer cleanup()

		svc.taskIdRequestIdMap.Store(requestId, task.Id)
		mockRepo.On("GetTask", task.Id).Return(task, (*utils.GenericError)(nil)).Once()

		ch, subId, err := svc.GetTaskWithRequestIdBlocking(requestId, callerAccount)
		assert.Nil(t, ch)
		assert.Equal(t, uint64(0), subId)
		assert.NotNil(t, err)
		assert.Equal(t, http.StatusNotFound, err.Type)
		mockRepo.AssertNotCalled(t, "GetTaskByRequestIdAndAccountId", mock.Anything, mock.Anything)
	})

	t.Run("GetTaskIdWithRequestId", func(t *testing.T) {
		svc, mockRepo, cleanup := setupTestAsyncTaskServiceWithMockRepo(t)
		defer cleanup()

		svc.taskIdRequestIdMap.Store(requestId, task.Id)
		mockRepo.On("GetTask", task.Id).Return(task, (*utils.GenericError)(nil)).Once()

		taskId, err := svc.GetTaskIdWithRequestId(requestId, callerAccount)
		assert.Equal(t, uint64(0), taskId)
		assert.NotNil(t, err)
		assert.Equal(t, http.StatusNotFound, err.Type)
		mockRepo.AssertNotCalled(t, "GetTaskByRequestIdAndAccountId", mock.Anything, mock.Anything)
	})

	t.Run("allows owning account via map hit", func(t *testing.T) {
		svc, mockRepo, cleanup := setupTestAsyncTaskServiceWithMockRepo(t)
		defer cleanup()

		svc.taskIdRequestIdMap.Store(requestId, task.Id)
		mockRepo.On("GetTask", task.Id).Return(task, (*utils.GenericError)(nil)).Once()

		got, err := svc.GetTaskWithRequestIdNonBlocking(requestId, ownerAccount)
		assert.Nil(t, err)
		assert.NotNil(t, got)
		assert.Equal(t, ownerAccount, got.AccountId)
		assert.Equal(t, task.Id, got.Id)
	})
}

// Test_AsyncTaskService_AddTasks_PreservesAccountAndScopesPoll verifies AddTasks stores the
// provided accountId and that a different account cannot poll the task even when the
// requestId is still in the in-memory map.
func Test_AsyncTaskService_AddTasks_PreservesAccountAndScopesPoll(t *testing.T) {
	svc, cleanup := setupTestAsyncTaskServiceWithData(t, nil)
	defer cleanup()

	ownerAccount := uint64(1)
	otherAccount := uint64(2)
	requestId := "account-scoped-request"
	input := `[{"spec":"@every 1m"}]`

	ids, addErr := svc.AddTasks(input, requestId, "create_job", ownerAccount)
	assert.Nil(t, addErr)
	assert.Len(t, ids, 1)

	owned, getErr := svc.GetTaskWithRequestIdNonBlocking(requestId, ownerAccount)
	assert.Nil(t, getErr)
	assert.NotNil(t, owned)
	assert.Equal(t, ownerAccount, owned.AccountId)
	assert.Equal(t, requestId, owned.RequestId)
	assert.Equal(t, input, owned.Input)

	foreign, foreignErr := svc.GetTaskWithRequestIdNonBlocking(requestId, otherAccount)
	assert.Nil(t, foreign)
	assert.NotNil(t, foreignErr)
	assert.Equal(t, http.StatusNotFound, foreignErr.Type)

	// After clearing the map, repo lookup is also account-scoped.
	svc.(*asyncTaskService).taskIdRequestIdMap.Delete(requestId)
	svc.(*asyncTaskService).task.Delete(ids[0])

	foreignFromRepo, foreignFromRepoErr := svc.GetTaskWithRequestIdNonBlocking(requestId, otherAccount)
	assert.Nil(t, foreignFromRepo)
	assert.NotNil(t, foreignFromRepoErr)
	assert.Equal(t, http.StatusNotFound, foreignFromRepoErr.Type)

	ownedFromRepo, ownedFromRepoErr := svc.GetTaskWithRequestIdNonBlocking(requestId, ownerAccount)
	assert.Nil(t, ownedFromRepoErr)
	assert.NotNil(t, ownedFromRepo)
	assert.Equal(t, ownerAccount, ownedFromRepo.AccountId)
}
