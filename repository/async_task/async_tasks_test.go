package async_task

import (
	"context"
	"github.com/brianvoe/gofakeit/v6"
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
)

func Test_AsyncTask_BatchInsert(t *testing.T) {
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
	asyncTasksRepo := NewAsyncTasksRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)

	var mockAsyncTasks []models.AsyncTask
	var numberOfTasks = 20
	for i := 0; i < numberOfTasks; i++ {
		var mockAsyncTask models.AsyncTask
		err := gofakeit.Struct(&mockAsyncTask)
		if err != nil {
			t.Fatal("failed to create async task", err)
		}
		mockAsyncTasks = append(mockAsyncTasks, mockAsyncTask)
	}
	ids, createErr := asyncTasksRepo.BatchInsert(mockAsyncTasks, true)
	if createErr != nil {
		t.Fatal("failed to create async task", createErr)
	}
	assert.Equal(t, len(ids), 20)
}

func Test_AsyncTask_GetTask(t *testing.T) {
	var testCases = []struct {
		committed bool
		name      string
	}{
		{
			committed: true,
			name:      "gets from committed",
		}, {
			committed: false,
			name:      "gets from uncommitted",
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
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
			asyncTasksRepo := NewAsyncTasksRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)

			var mockAsyncTask models.AsyncTask
			err = gofakeit.Struct(&mockAsyncTask)
			if err != nil {
				t.Fatal("failed to create async task", err)
			}
			// Insert the mock task into the mockAsyncTask
			ids, createErr := asyncTasksRepo.BatchInsert([]models.AsyncTask{mockAsyncTask}, testCase.committed)
			if createErr != nil {
				t.Fatalf("Failed to create async task: %v", createErr)
			}

			// Retrieve the task from the database
			task, getErr := asyncTasksRepo.GetTask(ids[0])
			if getErr != nil {
				t.Fatalf("Failed to retrieve async task: %v", getErr)
			}

			// Assert that the retrieved task is equal to the original mock task
			assert.Equal(t, mockAsyncTask.RequestId, task.RequestId)
			assert.Equal(t, mockAsyncTask.Input, task.Input)
			assert.Equal(t, mockAsyncTask.Output, task.Output)
			assert.Equal(t, models.AsyncTaskNotStated, task.State)
			assert.Equal(t, mockAsyncTask.Service, task.Service)
		})
	}
}

func Test_AsyncTask_RaftBatchInsert(t *testing.T) {
	ctx := context.Background()
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "async-task-test",
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
	asyncTasksRepo := NewAsyncTasksRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)

	// Create a mock raft cluster
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

	// Generate mock async tasks
	var mockAsyncTasks []models.AsyncTask
	var numberOfTasks = 20
	for i := 0; i < numberOfTasks; i++ {
		var mockAsyncTask models.AsyncTask
		err := gofakeit.Struct(&mockAsyncTask)
		if err != nil {
			t.Fatal("failed to create async task", err)
		}
		mockAsyncTasks = append(mockAsyncTasks, mockAsyncTask)
	}

	// Execute RaftBatchInsert
	_, rinsererr := asyncTasksRepo.RaftBatchInsert(mockAsyncTasks, 1)
	if rinsererr != nil {
		t.Fatal("failed to raft insert async task", rinsererr)
	}
	allTasks, getErr := asyncTasksRepo.GetAllTasks(true)
	if getErr != nil {
		t.Fatal("failed to get all async task", getErr)
	}
	assert.Equal(t, len(allTasks), numberOfTasks)
}

func Test_AsyncTask_RaftUpdateTaskState(t *testing.T) {
	ctx := context.Background()
	scheduler0Config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "async-task-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0Config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)
	tempFile, err := ioutil.TempFile("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.RunMigration()
	sqliteDb.OpenConnectionToExistingDB()
	scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, scheduler0Config, sqliteDb, nil, nil, nil, nil, nil)
	asyncTasksRepo := NewAsyncTasksRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)

	// Create a mock raft cluster
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

	// Generate a mock async task
	var mockAsyncTask models.AsyncTask
	err = gofakeit.Struct(&mockAsyncTask)
	if err != nil {
		t.Fatal("failed to create async task", err)
	}

	// Batch insert the mock async task
	ids, insertErr := asyncTasksRepo.BatchInsert([]models.AsyncTask{mockAsyncTask}, true)
	if insertErr != nil {
		t.Fatal("failed to batch insert async task", insertErr)
	}
	id := ids[0]
	mockAsyncTask.Id = id
	// Update the task state using RaftUpdateTaskState
	ruerr := asyncTasksRepo.RaftUpdateTaskState(mockAsyncTask, models.AsyncTaskInProgress, "")
	if ruerr != nil {
		t.Fatal("failed to update task state", ruerr)
	}

	// Retrieve the updated task
	updatedTask, getErr := asyncTasksRepo.GetTask(id)
	if getErr != nil {
		t.Fatal("failed to get task", getErr)
	}

	// Verify that the task state has been updated
	assert.Equal(t, updatedTask.State, models.AsyncTaskInProgress)
}

func Test_AsyncTask_UpdateTaskState(t *testing.T) {
	ctx := context.Background()
	scheduler0Config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "async-task-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0Config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)
	tempFile, err := ioutil.TempFile("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.RunMigration()
	sqliteDb.OpenConnectionToExistingDB()
	scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, scheduler0Config, sqliteDb, nil, nil, nil, nil, nil)
	asyncTasksRepo := NewAsyncTasksRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)

	// Create a mock raft cluster
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

	// Generate a mock async task
	var mockAsyncTask models.AsyncTask
	err = gofakeit.Struct(&mockAsyncTask)
	if err != nil {
		t.Fatal("failed to create async task", err)
	}

	// Insert the mock async task
	ids, insertErr := asyncTasksRepo.BatchInsert([]models.AsyncTask{mockAsyncTask}, false)
	if insertErr != nil {
		t.Fatal("failed to insert async task", insertErr)
	}

	mockAsyncTask.Id = ids[0]

	// Update the task state using UpdateTaskState
	uterr := asyncTasksRepo.UpdateTaskState(mockAsyncTask, models.AsyncTaskSuccess, "")
	if uterr != nil {
		t.Fatal("failed to update task state", uterr)
	}

	// Retrieve the updated task
	updatedTask, getErr := asyncTasksRepo.GetTask(ids[0])
	if getErr != nil {
		t.Fatal("failed to get task", getErr)
	}

	// Verify that the task state has been updated
	assert.Equal(t, updatedTask.State, models.AsyncTaskSuccess)
}

func Test_AsyncTask_GetAllTasks(t *testing.T) {
	var testCases = []struct {
		committed bool
		name      string
	}{
		{
			committed: true,
			name:      "gets all from committed",
		}, {
			committed: false,
			name:      "gets all from uncommitted",
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := context.Background()
			scheduler0Config := config.NewScheduler0Config()
			logger := hclog.New(&hclog.LoggerOptions{
				Name:  "async-task-test",
				Level: hclog.LevelFromString("DEBUG"),
			})
			sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0Config)
			scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)
			tempFile, err := ioutil.TempFile("", "test-db")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tempFile.Name())

			sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
			sqliteDb.RunMigration()
			sqliteDb.OpenConnectionToExistingDB()
			scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, scheduler0Config, sqliteDb, nil, nil, nil, nil, nil)
			asyncTasksRepo := NewAsyncTasksRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)

			// Create a mock raft cluster
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

			// Generate mock async tasks
			var mockAsyncTasks []models.AsyncTask
			var numberOfTasks = 10
			for i := 0; i < numberOfTasks; i++ {
				var mockAsyncTask models.AsyncTask
				err := gofakeit.Struct(&mockAsyncTask)
				if err != nil {
					t.Fatal("failed to create async task", err)
				}
				mockAsyncTasks = append(mockAsyncTasks, mockAsyncTask)
			}

			// Batch insert the mock async tasks
			_, insertErr := asyncTasksRepo.BatchInsert(mockAsyncTasks, testCase.committed)
			if insertErr != nil {
				t.Fatal("failed to batch insert async tasks", insertErr)
			}

			// Retrieve all tasks
			allTasks, getErr := asyncTasksRepo.GetAllTasks(testCase.committed)
			if getErr != nil {
				t.Fatal("failed to get all tasks", getErr)
			}

			// Verify the number of tasks retrieved
			assert.Equal(t, len(allTasks), numberOfTasks)
		})
	}
}
