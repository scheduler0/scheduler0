package service

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
	"scheduler0/secrets"
	"scheduler0/shared_repo"
	"scheduler0/utils"
	"testing"
)

func Test_CanAcceptRequest(t *testing.T) {
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
}
