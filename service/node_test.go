package service

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	"github.com/robfig/cron"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
	"scheduler0/config"
	"scheduler0/db"
	"scheduler0/fsm"
	"scheduler0/models"
	"scheduler0/repository"
	"scheduler0/scheduler0time"
	"scheduler0/secrets"
	"scheduler0/shared_repo"
	"scheduler0/utils"
	"testing"
	"time"
)

func Test_CanAcceptRequest(t *testing.T) {
	t.Cleanup(func() {
		err := os.RemoveAll("./raft_data")
		if err != nil {
			fmt.Println("failed to remove raft_data dir for test", err)
		}
	})

	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "job-service-test",
		Level: hclog.LevelFromString("trace"),
	})

	// Create a temporary SQLite database file
	tempFile, err := ioutil.TempFile("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	// Create a new SQLite database connection
	sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.RunMigration()
	sqliteDb.OpenConnectionToExistingDB()

	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo)

	scheduler0Secrets := secrets.NewScheduler0Secrets()
	// Create a new FSM store
	scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, sqliteDb)

	// Create a mock Raft cluster
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

	ctx := context.Background()

	jobRepo := repository.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)
	projectRepo := repository.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)
	asyncTaskManagerRepo := repository.NewAsyncTasksRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)
	asyncTaskManager := NewAsyncTaskManager(ctx, logger, scheduler0Store, asyncTaskManagerRepo)
	jobQueueRepo := repository.NewJobQueuesRepo(logger, scheduler0RaftActions, scheduler0Store)
	jobExecutionsRepo := repository.NewExecutionsRepo(
		logger,
		scheduler0RaftActions,
		scheduler0Store,
	)

	callback := func(effector func(sch chan any, ech chan any), successCh chan any, errorCh chan any) {
		effector(successCh, errorCh)
	}

	dispatcher := utils.NewDispatcher(
		int64(1),
		int64(1),
		callback,
	)

	dispatcher.Run()

	queueService := NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, jobQueueRepo)
	jobExecutorService := NewJobExecutor(
		ctx,
		logger,
		scheduler0config,
		scheduler0RaftActions,
		jobRepo,
		jobExecutionsRepo,
		jobQueueRepo,
		dispatcher,
	)

	nodeHTTPClient := NewHTTPClient(logger, scheduler0config, scheduler0Secrets)

	nodeService := NewNode(
		ctx,
		logger,
		scheduler0config,
		scheduler0Secrets,
		scheduler0RaftActions,
		jobExecutorService,
		queueService,
		jobRepo,
		projectRepo,
		jobExecutionsRepo,
		jobQueueRepo,
		sharedRepo,
		asyncTaskManager,
		dispatcher,
		nodeHTTPClient,
	)

	canAcceptRequest := nodeService.CanAcceptRequest()
	assert.Equal(t, false, canAcceptRequest)
	nodeService.beginAcceptingClientRequest()
	canAcceptRequest = nodeService.CanAcceptRequest()
	assert.Equal(t, true, canAcceptRequest)
}

func Test_CanAcceptClientWriteRequest(t *testing.T) {
	t.Cleanup(func() {
		err := os.RemoveAll("./raft_data")
		if err != nil {
			fmt.Println("failed to remove raft_data dir for test", err)
		}
	})

	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "job-service-test",
		Level: hclog.LevelFromString("trace"),
	})

	// Create a temporary SQLite database file
	tempFile, err := ioutil.TempFile("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	// Create a new SQLite database connection
	sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.RunMigration()
	sqliteDb.OpenConnectionToExistingDB()

	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo)

	scheduler0Secrets := secrets.NewScheduler0Secrets()
	// Create a new FSM store
	scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, sqliteDb)

	// Create a mock Raft cluster
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

	ctx := context.Background()

	jobRepo := repository.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)
	projectRepo := repository.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)
	asyncTaskManagerRepo := repository.NewAsyncTasksRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)
	asyncTaskManager := NewAsyncTaskManager(ctx, logger, scheduler0Store, asyncTaskManagerRepo)
	jobQueueRepo := repository.NewJobQueuesRepo(logger, scheduler0RaftActions, scheduler0Store)
	jobExecutionsRepo := repository.NewExecutionsRepo(
		logger,
		scheduler0RaftActions,
		scheduler0Store,
	)

	callback := func(effector func(sch chan any, ech chan any), successCh chan any, errorCh chan any) {
		effector(successCh, errorCh)
	}

	dispatcher := utils.NewDispatcher(
		int64(1),
		int64(1),
		callback,
	)

	dispatcher.Run()

	queueService := NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, jobQueueRepo)
	jobExecutorService := NewJobExecutor(
		ctx,
		logger,
		scheduler0config,
		scheduler0RaftActions,
		jobRepo,
		jobExecutionsRepo,
		jobQueueRepo,
		dispatcher,
	)

	nodeHTTPClient := NewHTTPClient(logger, scheduler0config, scheduler0Secrets)

	nodeService := NewNode(
		ctx,
		logger,
		scheduler0config,
		scheduler0Secrets,
		scheduler0RaftActions,
		jobExecutorService,
		queueService,
		jobRepo,
		projectRepo,
		jobExecutionsRepo,
		jobQueueRepo,
		sharedRepo,
		asyncTaskManager,
		dispatcher,
		nodeHTTPClient,
	)

	canAcceptClientWriteRequest := nodeService.CanAcceptClientWriteRequest()
	assert.Equal(t, false, canAcceptClientWriteRequest)
	nodeService.beginAcceptingClientWriteRequest()
	canAcceptClientWriteRequest = nodeService.CanAcceptClientWriteRequest()
	assert.Equal(t, true, canAcceptClientWriteRequest)
	nodeService.stopAcceptingClientWriteRequest()
	canAcceptClientWriteRequest = nodeService.CanAcceptClientWriteRequest()
	assert.Equal(t, false, canAcceptClientWriteRequest)
}

func Test_ReturnUncommittedLogs(t *testing.T) {
	t.Cleanup(func() {
		err := os.RemoveAll("./raft_data")
		if err != nil {
			fmt.Println("failed to remove raft_data dir for test", err)
		}
	})

	bctx := context.Background()
	ctx, cancler := context.WithCancel(bctx)
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "job-service-test",
		Level: hclog.LevelFromString("trace"),
	})

	// Create a temporary SQLite database file
	tempFile, err := ioutil.TempFile("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	// Create a new SQLite database connection
	sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.RunMigration()
	sqliteDb.OpenConnectionToExistingDB()

	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo)

	scheduler0Secrets := secrets.NewScheduler0Secrets()
	// Create a new FSM store
	scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, sqliteDb)

	// Create a mock Raft cluster
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

	jobRepo := repository.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)
	projectRepo := repository.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)
	asyncTaskManagerRepo := repository.NewAsyncTasksRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)
	asyncTaskManager := NewAsyncTaskManager(ctx, logger, scheduler0Store, asyncTaskManagerRepo)
	jobQueueRepo := repository.NewJobQueuesRepo(logger, scheduler0RaftActions, scheduler0Store)
	jobExecutionsRepo := repository.NewExecutionsRepo(
		logger,
		scheduler0RaftActions,
		scheduler0Store,
	)

	callback := func(effector func(sch chan any, ech chan any), successCh chan any, errorCh chan any) {
		effector(successCh, errorCh)
	}

	dispatcher := utils.NewDispatcher(
		int64(1),
		int64(1),
		callback,
	)

	dispatcher.Run()

	queueService := NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, jobQueueRepo)
	jobExecutorService := NewJobExecutor(
		ctx,
		logger,
		scheduler0config,
		scheduler0RaftActions,
		jobRepo,
		jobExecutionsRepo,
		jobQueueRepo,
		dispatcher,
	)
	jobService := NewJobService(ctx, logger, jobRepo, queueService, projectRepo, dispatcher, asyncTaskManager)

	nodeHTTPClient := NewHTTPClient(logger, scheduler0config, scheduler0Secrets)

	nodeService := NewNode(
		ctx,
		logger,
		scheduler0config,
		scheduler0Secrets,
		scheduler0RaftActions,
		jobExecutorService,
		queueService,
		jobRepo,
		projectRepo,
		jobExecutionsRepo,
		jobQueueRepo,
		sharedRepo,
		asyncTaskManager,
		dispatcher,
		nodeHTTPClient,
	)

	requestId := "some-request-id"
	asyncTaskManager.ListenForNotifications()
	// Define the input jobs
	jobs := []models.Job{}

	i := 1
	for i < 100 {
		jobs = append(jobs, models.Job{
			ID:        uint64(i),
			Spec:      "@every 1h",
			Timezone:  "America/New_York",
			ProjectID: 1,
		})
		i++
	}

	// Create the projects using the project repo
	project := models.Project{
		ID:          1,
		Name:        fmt.Sprintf("Project %d", 1),
		Description: fmt.Sprintf("Project %d description", 1),
	}
	_, createErr := projectRepo.CreateOne(&project)
	if createErr != nil {
		t.Fatalf("Failed to create project: %v", createErr)
	}

	// Call the BatchInsertJobs method of the job service
	_, batchErr := jobService.BatchInsertJobs("request123", jobs)
	if batchErr != nil {
		t.Fatalf("Failed to insert jobs: %v", batchErr)
	}

	time.Sleep(time.Second * time.Duration(2))

	schedule, parseErr := cron.Parse("@every 1h")
	if parseErr != nil {
		t.Fatal("cron spec error", parseErr)
	}

	schedulerTime := scheduler0time.GetSchedulerTime()
	now := schedulerTime.GetTime(time.Now())
	nextTime := schedule.Next(now)
	prevNextTime := nextTime.Add(-nextTime.Sub(now))
	lastTime := nextTime.Add(-nextTime.Sub(now)).Add(-nextTime.Sub(now))

	serr := os.Setenv("SCHEDULER0_NODE_ID", "1")
	defer os.Unsetenv("SCHEDULER0_NODE_ID")
	if serr != nil {
		t.Fatal("failed to set env", serr)
	}
	uncommittedExecutionsLogs := []models.JobExecutionLog{}
	i = 0
	for i < 24 {
		uncommittedExecutionsLogs = append(uncommittedExecutionsLogs, models.JobExecutionLog{
			JobId:                 jobs[i].ID,
			UniqueId:              fmt.Sprintf("%d-%d", jobs[i].ID, i),
			State:                 models.ExecutionLogSuccessState,
			LastExecutionDatetime: lastTime,
			NextExecutionDatetime: prevNextTime,
			NodeId:                1,
		})
		i++
	}
	err = sharedRepo.InsertExecutionLogs(sqliteDb, false, uncommittedExecutionsLogs)
	if err != nil {
		t.Fatal("failed to insert execution logs", err)
	}

	nodeService.ReturnUncommittedLogs(requestId)

	time.Sleep(time.Second * time.Duration(2))

	task, getTaskErr := asyncTaskManager.GetTaskWithRequestIdNonBlocking(requestId)
	if getTaskErr != nil {
		t.Fatal("failed update an async task", getTaskErr)
	}
	data := []byte(task.Output)
	var asyncTaskRes models.LocalData
	marshalErr := json.Unmarshal(data, &asyncTaskRes)

	if marshalErr != nil {
		t.Fatal("failed to unmarshal", marshalErr)
	}

	assert.Equal(t, task.RequestId, requestId)
	assert.Equal(t, task.State, models.AsyncTaskSuccess)
	assert.Equal(t, len(asyncTaskRes.ExecutionLogs), len(uncommittedExecutionsLogs))
	cancler()
}
