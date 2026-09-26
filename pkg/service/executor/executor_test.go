package executor

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"scheduler0/pkg/config"
	"scheduler0/pkg/constants"
	"scheduler0/pkg/db"
	"scheduler0/pkg/fsm"
	"scheduler0/pkg/models"
	account_repo "scheduler0/pkg/repository/account"
	account_job_executions_count_repo "scheduler0/pkg/repository/account_job_executions_count"
	async_task_repo "scheduler0/pkg/repository/async_task"
	executor_repo "scheduler0/pkg/repository/executor"
	job_repo "scheduler0/pkg/repository/job"
	job_execution_repo "scheduler0/pkg/repository/job_execution"
	job_queue_repo "scheduler0/pkg/repository/job_queue"
	project_repo "scheduler0/pkg/repository/project"
	"scheduler0/pkg/scheduler0time"
	"scheduler0/pkg/service/account"
	"scheduler0/pkg/service/async_task"
	etcd_service "scheduler0/pkg/service/etcd"
	"scheduler0/pkg/service/executor/executors"
	aws_lambda "scheduler0/pkg/service/executor/executors/aws"
	azure_function "scheduler0/pkg/service/executor/executors/azure"
	gcp_function "scheduler0/pkg/service/executor/executors/gcp"
	webhook_executor "scheduler0/pkg/service/executor/executors/webhook"
	"scheduler0/pkg/service/job"
	job_execution_service "scheduler0/pkg/service/job_execution_service"
	"scheduler0/pkg/service/queue"
	"scheduler0/pkg/shared_repo"
	"scheduler0/pkg/utils"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	"github.com/robfig/cron"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func Test_JobExecutor_QueueExecutions_JobsLessThanJobMaxBatch(t *testing.T) {
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
	sqliteDb.RunMigration(logger)
	sqliteDb.OpenConnectionToExistingDB()

	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)

	// Create a new FSM store
	scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, scheduler0config, sqliteDb, nil, nil, nil, nil, sharedRepo)

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

	ctx, canceler := context.WithCancel(context.Background())
	defer canceler()

	jobRepo := job_repo.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)
	projectRepo := project_repo.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)
	asyncTaskManagerRepo := async_task_repo.NewAsyncTasksRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)
	asyncTaskManager := async_task.NewAsyncTaskManager(ctx, logger, scheduler0Store, asyncTaskManagerRepo, scheduler0config)
	jobQueueRepo := job_queue_repo.NewJobQueuesRepo(logger, scheduler0RaftActions, scheduler0Store)
	jobExecutionsRepo := job_execution_repo.NewExecutionsRepo(
		logger,
		scheduler0RaftActions,
		scheduler0Store,
	)

	dispatcher := utils.NewDispatcher(
		ctx,
		int64(1),
		int64(1),
	)

	dispatcher.Run()

	// Create missing dependencies
	jobExecutorRepo := executor_repo.NewExecutorRepo(logger, scheduler0RaftActions, scheduler0Store, nil)
	accountRepo := account_repo.NewAccountRepository(ctx, logger, scheduler0RaftActions, scheduler0Store)
	accountJobExecutionsCountRepo := account_job_executions_count_repo.NewAccountJobExecutionsCountRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)
	jobExecutionLogService := job_execution_service.NewJobExecutionLogService(jobExecutionsRepo, logger, accountJobExecutionsCountRepo, jobQueueRepo, accountRepo, jobRepo)
	accountService := account.NewAccountService(accountRepo, accountJobExecutionsCountRepo, nil, nil, nil, nil, jobRepo, nil)

	// Create executor implementations
	awsLambdaExecutor := aws_lambda.NewLambdaExecutor(logger, ctx)
	webhookExecutor := webhook_executor.NewWebhookExecutor(logger, ctx, scheduler0config, dispatcher)
	gcpFunctionExecutor := gcp_function.NewFunctionsExecutor(logger, ctx)
	azureFunctionExecutor := azure_function.NewFunctionsExecutor(logger, ctx)

	// Create mocks for queue dependencies
	mockQuotaAllocationSender := queue.NewMockQuotaAllocationSender(t)
	mockEtcdService := etcd_service.NewMockEtcdService(t)
	mockEtcdService.On("GetPeers", mock.Anything).Return([]config.RaftNode{}, nil).Maybe()

	queueRepo := queue.NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, jobQueueRepo, jobRepo, jobExecutionsRepo, accountRepo, accountJobExecutionsCountRepo, mockQuotaAllocationSender, mockEtcdService)
	queueRepo.SetSingleNodeMode(true)
	// Create a new JobService instance
	jobService := job.NewJobService(ctx, logger, jobRepo, queueRepo, projectRepo, jobExecutorRepo, dispatcher, asyncTaskManager, jobExecutionLogService, accountService)

	service := NewJobExecutor(
		ctx,
		logger,
		scheduler0config,
		scheduler0RaftActions,
		jobRepo,
		jobExecutionsRepo,
		jobExecutorRepo,
		jobQueueRepo,
		awsLambdaExecutor,
		webhookExecutor,
		gcpFunctionExecutor,
		azureFunctionExecutor,
		dispatcher,
		accountRepo,
		accountJobExecutionsCountRepo,
		queueRepo,
	)

	service.SetSingleNodeMode(true)
	asyncTaskManager.SetSingleNodeMode(true)
	asyncTaskManager.ListenForNotifications()

	// Define the input jobs
	jobs := []models.Job{
		{
			ID:        1,
			Spec:      "* * * * *",
			Timezone:  "UTC",
			ProjectID: 1,
		},
		{
			ID:        2,
			Spec:      "0 0 * * *",
			Timezone:  "America/New_York",
			ProjectID: 2,
		},
	}

	// Create the projects using the project repo
	for _, job := range jobs {
		project := models.Project{
			ID:          job.ProjectID,
			Name:        fmt.Sprintf("Project %d", job.ProjectID),
			Description: fmt.Sprintf("Project %d description", job.ProjectID),
		}
		_, createErr := projectRepo.CreateOne(&project)
		if createErr != nil {
			t.Fatalf("Failed to create project: %v", createErr)
		}
	}

	// Call the BatchInsertJobs method of the job service
	_, batchErr := jobService.BatchInsertJobs("request123", jobs)
	if batchErr != nil {
		t.Fatalf("Failed to insert jobs: %v", batchErr)
	}

	time.Sleep(time.Second * time.Duration(1))

	serr := os.Setenv("SCHEDULER0_NODE_ID", "1")
	defer os.Unsetenv("SCHEDULER0_NODE_ID")
	if serr != nil {
		t.Fatal("failed to set env", serr)
	}

	service.QueueExecutions(
		int64(1),
		int64(2),
	)

	_, ok := service.GetExecutionsCache().Load(jobs[0].ID)
	assert.Equal(t, true, ok)
	_, ok = service.GetExecutionsCache().Load(jobs[1].ID)
	assert.Equal(t, true, ok)
}

func Test_JobExecutor_QueueExecutions_JobsMoreThanJobMaxBatch(t *testing.T) {
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
	sqliteDb.RunMigration(logger)
	sqliteDb.OpenConnectionToExistingDB()

	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)

	// Create a new FSM store
	scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, scheduler0config, sqliteDb, nil, nil, nil, nil, sharedRepo)

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

	ctx, canceler := context.WithCancel(context.Background())
	defer canceler()

	jobRepo := job_repo.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)
	projectRepo := project_repo.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)
	asyncTaskManagerRepo := async_task_repo.NewAsyncTasksRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)
	asyncTaskManager := async_task.NewAsyncTaskManager(ctx, logger, scheduler0Store, asyncTaskManagerRepo, scheduler0config)
	jobQueueRepo := job_queue_repo.NewJobQueuesRepo(logger, scheduler0RaftActions, scheduler0Store)
	jobExecutionsRepo := job_execution_repo.NewExecutionsRepo(
		logger,
		scheduler0RaftActions,
		scheduler0Store,
	)

	dispatcher := utils.NewDispatcher(
		ctx,
		int64(1),
		int64(1),
	)

	dispatcher.Run()

	// Create missing dependencies
	jobExecutorRepo := executor_repo.NewExecutorRepo(logger, scheduler0RaftActions, scheduler0Store, nil)
	accountRepo := account_repo.NewAccountRepository(ctx, logger, scheduler0RaftActions, scheduler0Store)
	accountJobExecutionsCountRepo := account_job_executions_count_repo.NewAccountJobExecutionsCountRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)
	jobExecutionLogService := job_execution_service.NewJobExecutionLogService(jobExecutionsRepo, logger, accountJobExecutionsCountRepo, jobQueueRepo, accountRepo, jobRepo)
	accountService := account.NewAccountService(accountRepo, accountJobExecutionsCountRepo, nil, nil, nil, nil, jobRepo, nil)

	// Create executor implementations
	awsLambdaExecutor := aws_lambda.NewLambdaExecutor(logger, ctx)
	webhookExecutor := webhook_executor.NewWebhookExecutor(logger, ctx, scheduler0config, dispatcher)
	gcpFunctionExecutor := gcp_function.NewFunctionsExecutor(logger, ctx)
	azureFunctionExecutor := azure_function.NewFunctionsExecutor(logger, ctx)

	// Create mocks for queue dependencies
	mockQuotaAllocationSender := queue.NewMockQuotaAllocationSender(t)
	mockEtcdService := etcd_service.NewMockEtcdService(t)
	mockEtcdService.On("GetPeers", mock.Anything).Return([]config.RaftNode{}, nil).Maybe()

	queueRepo := queue.NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, jobQueueRepo, jobRepo, jobExecutionsRepo, accountRepo, accountJobExecutionsCountRepo, mockQuotaAllocationSender, mockEtcdService)
	queueRepo.SetSingleNodeMode(true)
	// Create a new JobService instance
	jobService := job.NewJobService(ctx, logger, jobRepo, queueRepo, projectRepo, jobExecutorRepo, dispatcher, asyncTaskManager, jobExecutionLogService, accountService)

	service := NewJobExecutor(
		ctx,
		logger,
		scheduler0config,
		scheduler0RaftActions,
		jobRepo,
		jobExecutionsRepo,
		jobExecutorRepo,
		jobQueueRepo,
		awsLambdaExecutor,
		webhookExecutor,
		gcpFunctionExecutor,
		azureFunctionExecutor,
		dispatcher,
		accountRepo,
		accountJobExecutionsCountRepo,
		queueRepo,
	)

	asyncTaskManager.SetSingleNodeMode(true)
	asyncTaskManager.ListenForNotifications()

	// Define the input jobs
	jobs := []models.Job{}

	i := 1
	for i < constants.JobMaxBatchSize+100 {
		jobs = append(jobs, models.Job{
			ID:        uint64(i),
			Spec:      "0 0 * * *",
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

	time.Sleep(time.Second * time.Duration(1))

	serr := os.Setenv("SCHEDULER0_NODE_ID", "1")
	defer os.Unsetenv("SCHEDULER0_NODE_ID")
	if serr != nil {
		t.Fatal("failed to set env", serr)
	}
	service.QueueExecutions(
		int64(1),
		int64(constants.JobMaxBatchSize+100),
	)

	i = 0
	for i < constants.JobMaxBatchSize+99 {
		_, ok := service.GetExecutionsCache().Load(jobs[i].ID)
		assert.Equal(t, true, ok)
		i++
	}
}

func Test_JobExecutor_QueueExecutions_DoesNotQueueJobsForOtherServers(t *testing.T) {
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
	sqliteDb.RunMigration(logger)
	sqliteDb.OpenConnectionToExistingDB()

	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)

	// Create a new FSM store
	scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, scheduler0config, sqliteDb, nil, nil, nil, nil, sharedRepo)

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

	ctx, canceler := context.WithCancel(context.Background())
	defer canceler()

	jobRepo := job_repo.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)
	projectRepo := project_repo.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)
	asyncTaskManagerRepo := async_task_repo.NewAsyncTasksRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)
	asyncTaskManager := async_task.NewAsyncTaskManager(ctx, logger, scheduler0Store, asyncTaskManagerRepo, scheduler0config)
	jobQueueRepo := job_queue_repo.NewJobQueuesRepo(logger, scheduler0RaftActions, scheduler0Store)
	jobExecutionsRepo := job_execution_repo.NewExecutionsRepo(
		logger,
		scheduler0RaftActions,
		scheduler0Store,
	)

	dispatcher := utils.NewDispatcher(
		ctx,
		int64(1),
		int64(1),
	)

	dispatcher.Run()

	// Create missing dependencies
	jobExecutorRepo := executor_repo.NewExecutorRepo(logger, scheduler0RaftActions, scheduler0Store, nil)
	accountRepo := account_repo.NewAccountRepository(ctx, logger, scheduler0RaftActions, scheduler0Store)
	accountJobExecutionsCountRepo := account_job_executions_count_repo.NewAccountJobExecutionsCountRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)
	jobExecutionLogService := job_execution_service.NewJobExecutionLogService(jobExecutionsRepo, logger, accountJobExecutionsCountRepo, jobQueueRepo, accountRepo, jobRepo)
	accountService := account.NewAccountService(accountRepo, accountJobExecutionsCountRepo, nil, nil, nil, nil, jobRepo, nil)

	// Create executor implementations
	awsLambdaExecutor := aws_lambda.NewLambdaExecutor(logger, ctx)
	webhookExecutor := webhook_executor.NewWebhookExecutor(logger, ctx, scheduler0config, dispatcher)
	gcpFunctionExecutor := gcp_function.NewFunctionsExecutor(logger, ctx)
	azureFunctionExecutor := azure_function.NewFunctionsExecutor(logger, ctx)

	// Create mocks for queue dependencies
	mockQuotaAllocationSender := queue.NewMockQuotaAllocationSender(t)
	mockEtcdService := etcd_service.NewMockEtcdService(t)
	mockEtcdService.On("GetPeers", mock.Anything).Return([]config.RaftNode{}, nil).Maybe()

	queueRepo := queue.NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, jobQueueRepo, jobRepo, jobExecutionsRepo, accountRepo, accountJobExecutionsCountRepo, mockQuotaAllocationSender, mockEtcdService)
	// Create a new JobService instance
	jobService := job.NewJobService(ctx, logger, jobRepo, queueRepo, projectRepo, jobExecutorRepo, dispatcher, asyncTaskManager, jobExecutionLogService, accountService)

	service := NewJobExecutor(
		ctx,
		logger,
		scheduler0config,
		scheduler0RaftActions,
		jobRepo,
		jobExecutionsRepo,
		jobExecutorRepo,
		jobQueueRepo,
		awsLambdaExecutor,
		webhookExecutor,
		gcpFunctionExecutor,
		azureFunctionExecutor,
		dispatcher,
		accountRepo,
		accountJobExecutionsCountRepo,
		queueRepo,
	)

	asyncTaskManager.SetSingleNodeMode(true)
	asyncTaskManager.ListenForNotifications()

	// Define the input jobs
	jobs := []models.Job{}

	i := 1
	for i < constants.JobMaxBatchSize+100 {
		jobs = append(jobs, models.Job{
			ID:        uint64(i),
			Spec:      "0 0 * * *",
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

	time.Sleep(time.Second * time.Duration(4))

	serr := os.Setenv("SCHEDULER0_NODE_ID", "2")
	defer os.Unsetenv("SCHEDULER0_NODE_ID")
	if serr != nil {
		t.Fatal("failed to set env", serr)
	}

	service.QueueExecutions(
		int64(1),
		int64(constants.JobMaxBatchSize+100),
	)

	i = 0
	for i < constants.JobMaxBatchSize+99 {
		_, ok := service.GetExecutionsCache().Load(jobs[i].ID)
		assert.Equal(t, ok, false)
		i++
	}
}

func Test_JobExecutor_ScheduleJobs_WithScheduledStateAsLastKnowState_NextTimeExecution_One(t *testing.T) {
	testCases := []struct {
		JobState models.JobExecutionLogState
	}{{
		JobState: models.ExecutionLogScheduleState,
	}, {
		JobState: models.ExecutionLogFailedState,
	}, {
		JobState: models.ExecutionLogSuccessState,
	}}

	for _, testCase := range testCases {
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
		sqliteDb.RunMigration(logger)
		sqliteDb.OpenConnectionToExistingDB()

		scheduler0config := config.NewScheduler0Config()
		sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
		scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)

		// Create a new FSM store
		scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, scheduler0config, sqliteDb, nil, nil, nil, nil, sharedRepo)

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

		ctx, canceler := context.WithCancel(context.Background())
		defer canceler()

		jobRepo := job_repo.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)
		projectRepo := project_repo.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)
		asyncTaskManagerRepo := async_task_repo.NewAsyncTasksRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)
		asyncTaskManager := async_task.NewAsyncTaskManager(ctx, logger, scheduler0Store, asyncTaskManagerRepo, scheduler0config)
		jobQueueRepo := job_queue_repo.NewJobQueuesRepo(logger, scheduler0RaftActions, scheduler0Store)
		jobExecutionsRepo := job_execution_repo.NewExecutionsRepo(
			logger,
			scheduler0RaftActions,
			scheduler0Store,
		)

		dispatcher := utils.NewDispatcher(
			ctx,
			int64(1),
			int64(1),
		)

		dispatcher.Run()

		// Create missing dependencies
		jobExecutorRepo := executor_repo.NewExecutorRepo(logger, scheduler0RaftActions, scheduler0Store, nil)
		accountRepo := account_repo.NewAccountRepository(ctx, logger, scheduler0RaftActions, scheduler0Store)
		accountJobExecutionsCountRepo := account_job_executions_count_repo.NewAccountJobExecutionsCountRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)
		jobExecutionLogService := job_execution_service.NewJobExecutionLogService(jobExecutionsRepo, logger, accountJobExecutionsCountRepo, jobQueueRepo, accountRepo, jobRepo)
		accountService := account.NewAccountService(accountRepo, accountJobExecutionsCountRepo, nil, nil, nil, nil, jobRepo, nil)

		// Create executor implementations
		awsLambdaExecutor := aws_lambda.NewLambdaExecutor(logger, ctx)
		webhookExecutor := webhook_executor.NewWebhookExecutor(logger, ctx, scheduler0config, dispatcher)
		gcpFunctionExecutor := gcp_function.NewFunctionsExecutor(logger, ctx)
		azureFunctionExecutor := azure_function.NewFunctionsExecutor(logger, ctx)

		// Create mocks for queue dependencies
		mockQuotaAllocationSender := queue.NewMockQuotaAllocationSender(t)
		mockEtcdService := etcd_service.NewMockEtcdService(t)
		mockEtcdService.On("GetPeers", mock.Anything).Return([]config.RaftNode{}, nil).Maybe()

		queueRepo := queue.NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, jobQueueRepo, jobRepo, jobExecutionsRepo, accountRepo, accountJobExecutionsCountRepo, mockQuotaAllocationSender, mockEtcdService)
		queueRepo.SetSingleNodeMode(true)
		// Create a new JobService instance
		jobService := job.NewJobService(ctx, logger, jobRepo, queueRepo, projectRepo, jobExecutorRepo, dispatcher, asyncTaskManager, jobExecutionLogService, accountService)

		service := NewJobExecutor(
			ctx,
			logger,
			scheduler0config,
			scheduler0RaftActions,
			jobRepo,
			jobExecutionsRepo,
			jobExecutorRepo,
			jobQueueRepo,
			awsLambdaExecutor,
			webhookExecutor,
			gcpFunctionExecutor,
			azureFunctionExecutor,
			dispatcher,
			accountRepo,
			accountJobExecutionsCountRepo,
			queueRepo,
		)

		asyncTaskManager.SetSingleNodeMode(true)
		asyncTaskManager.ListenForNotifications()

		// Define the input jobs
		jobs := []models.Job{}

		schedule, parseErr := cron.Parse("@every 1m")
		if parseErr != nil {
			t.Fatal("cron spec error", parseErr)
		}

		schedulerTime := scheduler0time.GetSchedulerTime()
		now := schedulerTime.GetTime(time.Now())
		lastTime := now
		nextTime := schedule.Next(lastTime)

		i := 1
		for i < 100 {
			jobs = append(jobs, models.Job{
				ID:        uint64(i),
				Spec:      "@every 1m",
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

		time.Sleep(time.Second * time.Duration(1))

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
				State:                 testCase.JobState,
				LastExecutionDatetime: lastTime,
				NextExecutionDatetime: nextTime,
			})
			i++
		}
		err = sharedRepo.InsertExecutionLogs(sqliteDb, false, uncommittedExecutionsLogs)
		if err != nil {
			t.Fatal("failed to insert execution logs", err)
		}

		committedExecutionsLogs := []models.JobExecutionLog{}
		i = 24
		for i < 50 {
			committedExecutionsLogs = append(uncommittedExecutionsLogs, models.JobExecutionLog{
				JobId:                 jobs[i].ID,
				UniqueId:              fmt.Sprintf("%d-%d", jobs[i].ID, i),
				State:                 testCase.JobState,
				LastExecutionDatetime: lastTime,
				NextExecutionDatetime: nextTime,
			})
			i++
		}
		err = sharedRepo.InsertExecutionLogs(sqliteDb, true, committedExecutionsLogs)
		if err != nil {
			t.Fatal("failed to insert execution logs", err)
		}
		service.QueueExecutions(
			int64(1),
			int64(100),
		)

		i = 0
		for i < 99 {
			sched, ok := service.GetExecutionsCache().Load(jobs[i].ID)
			scheduler := sched.(models.JobSchedule)
			assert.Equal(t, scheduler.MemExecution.NextExecutionDatetime.Sub(nextTime).Round(1*time.Second) < time.Duration(1)*time.Second, true)
			assert.Equal(t, scheduler.MemExecution.NextExecutionDatetime.Sub(nextTime).Round(1*time.Second) > time.Duration(-1)*time.Second, true)
			assert.Equal(t, ok, true)
			i++
		}
	}
}

func Test_JobExecutor_ScheduleJobs_WithScheduledStateAsLastKnowState_NextTimeExecution_Two(t *testing.T) {
	testCases := []struct {
		JobState models.JobExecutionLogState
	}{{
		JobState: models.ExecutionLogScheduleState,
	}, {
		JobState: models.ExecutionLogFailedState,
	}, {
		JobState: models.ExecutionLogSuccessState,
	}}

	for _, testCase := range testCases {
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
		sqliteDb.RunMigration(logger)
		sqliteDb.OpenConnectionToExistingDB()

		scheduler0config := config.NewScheduler0Config()
		sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
		scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)

		// Create a new FSM store
		scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, scheduler0config, sqliteDb, nil, nil, nil, nil, sharedRepo)

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

		ctx, canceler := context.WithCancel(context.Background())
		defer canceler()

		jobRepo := job_repo.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)
		projectRepo := project_repo.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)
		asyncTaskManagerRepo := async_task_repo.NewAsyncTasksRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)
		asyncTaskManager := async_task.NewAsyncTaskManager(ctx, logger, scheduler0Store, asyncTaskManagerRepo, scheduler0config)
		jobQueueRepo := job_queue_repo.NewJobQueuesRepo(logger, scheduler0RaftActions, scheduler0Store)
		jobExecutionsRepo := job_execution_repo.NewExecutionsRepo(
			logger,
			scheduler0RaftActions,
			scheduler0Store,
		)

		dispatcher := utils.NewDispatcher(
			ctx,
			int64(1),
			int64(1),
		)

		dispatcher.Run()

		// Create missing dependencies
		jobExecutorRepo := executor_repo.NewExecutorRepo(logger, scheduler0RaftActions, scheduler0Store, nil)
		accountRepo := account_repo.NewAccountRepository(ctx, logger, scheduler0RaftActions, scheduler0Store)
		accountJobExecutionsCountRepo := account_job_executions_count_repo.NewAccountJobExecutionsCountRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)
		jobExecutionLogService := job_execution_service.NewJobExecutionLogService(jobExecutionsRepo, logger, accountJobExecutionsCountRepo, jobQueueRepo, accountRepo, jobRepo)
		accountService := account.NewAccountService(accountRepo, accountJobExecutionsCountRepo, nil, nil, nil, nil, jobRepo, nil)

		// Create executor implementations
		awsLambdaExecutor := aws_lambda.NewLambdaExecutor(logger, ctx)
		webhookExecutor := webhook_executor.NewWebhookExecutor(logger, ctx, scheduler0config, dispatcher)
		gcpFunctionExecutor := gcp_function.NewFunctionsExecutor(logger, ctx)
		azureFunctionExecutor := azure_function.NewFunctionsExecutor(logger, ctx)

		// Create mocks for queue dependencies
		mockQuotaAllocationSender := queue.NewMockQuotaAllocationSender(t)
		mockEtcdService := etcd_service.NewMockEtcdService(t)
		mockEtcdService.On("GetPeers", mock.Anything).Return([]config.RaftNode{}, nil).Maybe()

		queueRepo := queue.NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, jobQueueRepo, jobRepo, jobExecutionsRepo, accountRepo, accountJobExecutionsCountRepo, mockQuotaAllocationSender, mockEtcdService)
		queueRepo.SetSingleNodeMode(true)
		// Create a new JobService instance
		jobService := job.NewJobService(ctx, logger, jobRepo, queueRepo, projectRepo, jobExecutorRepo, dispatcher, asyncTaskManager, jobExecutionLogService, accountService)

		service := NewJobExecutor(
			ctx,
			logger,
			scheduler0config,
			scheduler0RaftActions,
			jobRepo,
			jobExecutionsRepo,
			jobExecutorRepo,
			jobQueueRepo,
			awsLambdaExecutor,
			webhookExecutor,
			gcpFunctionExecutor,
			azureFunctionExecutor,
			dispatcher,
			accountRepo,
			accountJobExecutionsCountRepo,
			queueRepo,
		)

		asyncTaskManager.SetSingleNodeMode(true)
		asyncTaskManager.ListenForNotifications()

		// Define the input jobs
		jobs := []models.Job{}

		schedule, parseErr := cron.Parse("@every 1h")
		if parseErr != nil {
			t.Fatal("cron spec error", parseErr)
		}

		schedulerTime := scheduler0time.GetSchedulerTime()
		now := schedulerTime.GetTime(time.Now())
		nextTime := schedule.Next(now)
		prevNextTime := nextTime.Add(-nextTime.Sub(now))
		lastTime := nextTime.Add(-nextTime.Sub(now)).Add(-nextTime.Sub(now))

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

		time.Sleep(time.Second * time.Duration(4))

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
				State:                 testCase.JobState,
				LastExecutionDatetime: lastTime,
				NextExecutionDatetime: prevNextTime,
			})
			i++
		}
		err = sharedRepo.InsertExecutionLogs(sqliteDb, false, uncommittedExecutionsLogs)
		if err != nil {
			t.Fatal("failed to insert execution logs", err)
		}

		committedExecutionsLogs := []models.JobExecutionLog{}
		i = 24
		for i < 50 {
			committedExecutionsLogs = append(uncommittedExecutionsLogs, models.JobExecutionLog{
				JobId:                 jobs[i].ID,
				UniqueId:              fmt.Sprintf("%d-%d", jobs[i].ID, i),
				State:                 testCase.JobState,
				LastExecutionDatetime: lastTime,
				NextExecutionDatetime: prevNextTime,
			})
			i++
		}
		err = sharedRepo.InsertExecutionLogs(sqliteDb, true, committedExecutionsLogs)
		if err != nil {
			t.Fatal("failed to insert execution logs", err)
		}
		service.QueueExecutions(
			int64(1),
			int64(100),
		)

		i = 0
		for i < 99 {
			sched, ok := service.GetExecutionsCache().Load(jobs[i].ID)
			scheduler := sched.(models.JobSchedule)
			assert.Equal(t, scheduler.MemExecution.NextExecutionDatetime.Sub(nextTime.Add(time.Duration(4)*time.Second)).Round(time.Minute*1) < time.Duration(1)*time.Second, true)
			assert.Equal(t, ok, true)
			i++
		}
	}
}

func Test_JobExecutor_ScheduleJobs_WithScheduledStateAsLastKnowState_NextTimeExecution_Three(t *testing.T) {
	testCases := []struct {
		JobState models.JobExecutionLogState
	}{{
		JobState: models.ExecutionLogScheduleState,
	}, {
		JobState: models.ExecutionLogFailedState,
	}, {
		JobState: models.ExecutionLogSuccessState,
	}}

	for _, testCase := range testCases {
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
		sqliteDb.RunMigration(logger)
		sqliteDb.OpenConnectionToExistingDB()

		scheduler0config := config.NewScheduler0Config()
		sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
		scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)

		// Create a new FSM store
		scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, scheduler0config, sqliteDb, nil, nil, nil, nil, sharedRepo)

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

		ctx, canceler := context.WithCancel(context.Background())
		defer canceler()

		jobRepo := job_repo.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)
		projectRepo := project_repo.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)
		asyncTaskManagerRepo := async_task_repo.NewAsyncTasksRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)
		asyncTaskManager := async_task.NewAsyncTaskManager(ctx, logger, scheduler0Store, asyncTaskManagerRepo, scheduler0config)
		jobQueueRepo := job_queue_repo.NewJobQueuesRepo(logger, scheduler0RaftActions, scheduler0Store)
		jobExecutionsRepo := job_execution_repo.NewExecutionsRepo(
			logger,
			scheduler0RaftActions,
			scheduler0Store,
		)

		dispatcher := utils.NewDispatcher(
			ctx,
			int64(1),
			int64(1),
		)

		dispatcher.Run()

		// Create missing dependencies
		jobExecutorRepo := executor_repo.NewExecutorRepo(logger, scheduler0RaftActions, scheduler0Store, nil)
		accountRepo := account_repo.NewAccountRepository(ctx, logger, scheduler0RaftActions, scheduler0Store)
		accountJobExecutionsCountRepo := account_job_executions_count_repo.NewAccountJobExecutionsCountRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)
		jobExecutionLogService := job_execution_service.NewJobExecutionLogService(jobExecutionsRepo, logger, accountJobExecutionsCountRepo, jobQueueRepo, accountRepo, jobRepo)
		accountService := account.NewAccountService(accountRepo, accountJobExecutionsCountRepo, nil, nil, nil, nil, jobRepo, nil)

		// Create executor implementations
		awsLambdaExecutor := aws_lambda.NewLambdaExecutor(logger, ctx)
		webhookExecutor := webhook_executor.NewWebhookExecutor(logger, ctx, scheduler0config, dispatcher)
		gcpFunctionExecutor := gcp_function.NewFunctionsExecutor(logger, ctx)
		azureFunctionExecutor := azure_function.NewFunctionsExecutor(logger, ctx)

		// Create mocks for queue dependencies
		mockQuotaAllocationSender := queue.NewMockQuotaAllocationSender(t)
		mockEtcdService := etcd_service.NewMockEtcdService(t)
		mockEtcdService.On("GetPeers", mock.Anything).Return([]config.RaftNode{}, nil).Maybe()

		queueRepo := queue.NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, jobQueueRepo, jobRepo, jobExecutionsRepo, accountRepo, accountJobExecutionsCountRepo, mockQuotaAllocationSender, mockEtcdService)
		queueRepo.SetSingleNodeMode(true)
		// Create a new JobService instance
		jobService := job.NewJobService(ctx, logger, jobRepo, queueRepo, projectRepo, jobExecutorRepo, dispatcher, asyncTaskManager, jobExecutionLogService, accountService)

		service := NewJobExecutor(
			ctx,
			logger,
			scheduler0config,
			scheduler0RaftActions,
			jobRepo,
			jobExecutionsRepo,
			jobExecutorRepo,
			jobQueueRepo,
			awsLambdaExecutor,
			webhookExecutor,
			gcpFunctionExecutor,
			azureFunctionExecutor,
			dispatcher,
			accountRepo,
			accountJobExecutionsCountRepo,
			queueRepo,
		)

		asyncTaskManager.SetSingleNodeMode(true)
		asyncTaskManager.ListenForNotifications()

		// Define the input jobs
		jobs := []models.Job{}

		schedule, parseErr := cron.Parse("@every 1h")
		if parseErr != nil {
			t.Fatal("cron spec error", parseErr)
		}

		schedulerTime := scheduler0time.GetSchedulerTime()
		now := schedulerTime.GetTime(time.Now())
		nextTime := schedule.Next(now)
		prevNextTime := nextTime.Add(-nextTime.Sub(now)).Add(-nextTime.Sub(now)).Add(-nextTime.Sub(now))
		lastTime := nextTime.Add(-nextTime.Sub(now)).Add(-nextTime.Sub(now)).Add(-nextTime.Sub(now)).Add(-nextTime.Sub(now))

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

		time.Sleep(time.Second * time.Duration(4))

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
				State:                 testCase.JobState,
				LastExecutionDatetime: lastTime,
				NextExecutionDatetime: prevNextTime,
			})
			i++
		}
		err = sharedRepo.InsertExecutionLogs(sqliteDb, false, uncommittedExecutionsLogs)
		if err != nil {
			t.Fatal("failed to insert execution logs", err)
		}

		committedExecutionsLogs := []models.JobExecutionLog{}
		i = 24
		for i < 50 {
			committedExecutionsLogs = append(uncommittedExecutionsLogs, models.JobExecutionLog{
				JobId:                 jobs[i].ID,
				UniqueId:              fmt.Sprintf("%d-%d", jobs[i].ID, i),
				State:                 testCase.JobState,
				LastExecutionDatetime: lastTime,
				NextExecutionDatetime: prevNextTime,
			})
			i++
		}
		err = sharedRepo.InsertExecutionLogs(sqliteDb, true, committedExecutionsLogs)
		if err != nil {
			t.Fatal("failed to insert execution logs", err)
		}
		service.QueueExecutions(
			int64(1),
			int64(100),
		)

		i = 0
		for i < 99 {
			sched, ok := service.GetExecutionsCache().Load(jobs[i].ID)
			scheduler := sched.(models.JobSchedule)
			assert.Equal(t, scheduler.MemExecution.NextExecutionDatetime.Sub(nextTime.Add(time.Duration(4)*time.Second)).Round(time.Minute*1) < time.Duration(1)*time.Second, true)
			assert.Equal(t, ok, true)
			i++
		}
	}
}

func Test_ListenForJobsToInvoke(t *testing.T) {
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
	sqliteDb.RunMigration(logger)
	sqliteDb.OpenConnectionToExistingDB()

	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)

	// Create a new FSM store
	scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, scheduler0config, sqliteDb, nil, nil, nil, nil, sharedRepo)

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

	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	jobRepo := job_repo.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)
	projectRepo := project_repo.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)
	asyncTaskManagerRepo := async_task_repo.NewAsyncTasksRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)
	asyncTaskManager := async_task.NewAsyncTaskManager(ctx, logger, scheduler0Store, asyncTaskManagerRepo, scheduler0config)
	jobQueueRepo := job_queue_repo.NewJobQueuesRepo(logger, scheduler0RaftActions, scheduler0Store)
	jobExecutionsRepo := job_execution_repo.NewExecutionsRepo(
		logger,
		scheduler0RaftActions,
		scheduler0Store,
	)

	dispatcher := utils.NewDispatcher(
		ctx,
		int64(1),
		int64(1),
	)

	dispatcher.Run()

	// Create missing dependencies
	jobExecutorRepo := executor_repo.NewExecutorRepo(logger, scheduler0RaftActions, scheduler0Store, nil)
	accountRepo := account_repo.NewAccountRepository(ctx, logger, scheduler0RaftActions, scheduler0Store)
	accountJobExecutionsCountRepo := account_job_executions_count_repo.NewAccountJobExecutionsCountRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)
	jobExecutionLogService := job_execution_service.NewJobExecutionLogService(jobExecutionsRepo, logger, accountJobExecutionsCountRepo, jobQueueRepo, accountRepo, jobRepo)
	accountService := account.NewAccountService(accountRepo, accountJobExecutionsCountRepo, nil, nil, nil, nil, jobRepo, nil)

	// Create executor implementations
	awsLambdaExecutor := aws_lambda.NewLambdaExecutor(logger, ctx)
	webhookExecutor := webhook_executor.NewWebhookExecutor(logger, ctx, scheduler0config, dispatcher)
	gcpFunctionExecutor := gcp_function.NewFunctionsExecutor(logger, ctx)
	azureFunctionExecutor := azure_function.NewFunctionsExecutor(logger, ctx)

	// Create mocks for queue dependencies
	mockQuotaAllocationSender := queue.NewMockQuotaAllocationSender(t)
	mockEtcdService := etcd_service.NewMockEtcdService(t)
	mockEtcdService.On("GetPeers", mock.Anything).Return([]config.RaftNode{}, nil).Maybe()

	queueRepo := queue.NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, jobQueueRepo, jobRepo, jobExecutionsRepo, accountRepo, accountJobExecutionsCountRepo, mockQuotaAllocationSender, mockEtcdService)
	// Create a new JobService instance
	jobService := job.NewJobService(ctx, logger, jobRepo, queueRepo, projectRepo, jobExecutorRepo, dispatcher, asyncTaskManager, jobExecutionLogService, accountService)
	httpJobExecutor := executors.NewMockHTTPExecutor(t)
	httpJobExecutor.On("ExecuteHTTPJob", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	service := NewJobExecutor(
		ctx,
		logger,
		scheduler0config,
		scheduler0RaftActions,
		jobRepo,
		jobExecutionsRepo,
		jobExecutorRepo,
		jobQueueRepo,
		awsLambdaExecutor,
		webhookExecutor,
		gcpFunctionExecutor,
		azureFunctionExecutor,
		dispatcher,
		accountRepo,
		accountJobExecutionsCountRepo,
		queueRepo,
	)

	asyncTaskManager.SetSingleNodeMode(true)
	asyncTaskManager.ListenForNotifications()

	// Define the input jobs
	jobs := []models.Job{}

	schedule, parseErr := cron.Parse("@every 1m")
	if parseErr != nil {
		t.Fatal("cron spec error", parseErr)
	}

	schedulerTime := scheduler0time.GetSchedulerTime()
	now := schedulerTime.GetTime(time.Now())
	nextTime := schedule.Next(now)

	job := models.Job{
		ID:        uint64(1),
		Spec:      "@every 1h",
		Timezone:  "America/New_York",
		ProjectID: 1,
		Data:      "http://someaddress",
	}

	jobs = []models.Job{job}

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

	time.Sleep(time.Second * time.Duration(1))

	serr := os.Setenv("SCHEDULER0_NODE_ID", "1")
	defer os.Unsetenv("SCHEDULER0_NODE_ID")
	if serr != nil {
		t.Fatal("failed to set env", serr)
	}

	go service.ListenForJobsToInvokeV1()

	service.GetExecutionsCache().Store(uint64(1), &models.JobSchedule{
		Job: job,
		MemExecution: models.MemJobExecution{
			NextExecutionDatetime: schedulerTime.GetTime(nextTime),
		},
	})

	time.Sleep(1*time.Minute + 2*time.Second)
}

func Test_handleFailedJobs(t *testing.T) {
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
	sqliteDb.RunMigration(logger)
	sqliteDb.OpenConnectionToExistingDB()

	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)

	// Create a new FSM store
	scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, scheduler0config, sqliteDb, nil, nil, nil, nil, sharedRepo)

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

	ctx, canceler := context.WithCancel(context.Background())
	defer canceler()

	jobRepo := job_repo.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)
	projectRepo := project_repo.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)
	asyncTaskManagerRepo := async_task_repo.NewAsyncTasksRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)
	asyncTaskManager := async_task.NewAsyncTaskManager(ctx, logger, scheduler0Store, asyncTaskManagerRepo, scheduler0config)
	jobQueueRepo := job_queue_repo.NewJobQueuesRepo(logger, scheduler0RaftActions, scheduler0Store)
	jobExecutionsRepo := job_execution_repo.NewExecutionsRepo(
		logger,
		scheduler0RaftActions,
		scheduler0Store,
	)

	dispatcher := utils.NewDispatcher(
		ctx,
		int64(1),
		int64(1),
	)

	dispatcher.Run()

	// Create missing dependencies
	jobExecutorRepo := executor_repo.NewExecutorRepo(logger, scheduler0RaftActions, scheduler0Store, nil)
	accountRepo := account_repo.NewAccountRepository(ctx, logger, scheduler0RaftActions, scheduler0Store)
	accountJobExecutionsCountRepo := account_job_executions_count_repo.NewAccountJobExecutionsCountRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)
	jobExecutionLogService := job_execution_service.NewJobExecutionLogService(jobExecutionsRepo, logger, accountJobExecutionsCountRepo, jobQueueRepo, accountRepo, jobRepo)
	accountService := account.NewAccountService(accountRepo, accountJobExecutionsCountRepo, nil, nil, nil, nil, jobRepo, nil)

	// Create executor implementations
	awsLambdaExecutor := aws_lambda.NewLambdaExecutor(logger, ctx)
	webhookExecutor := webhook_executor.NewWebhookExecutor(logger, ctx, scheduler0config, dispatcher)
	gcpFunctionExecutor := gcp_function.NewFunctionsExecutor(logger, ctx)
	azureFunctionExecutor := azure_function.NewFunctionsExecutor(logger, ctx)

	// Create mocks for queue dependencies
	mockQuotaAllocationSender := queue.NewMockQuotaAllocationSender(t)
	mockEtcdService := etcd_service.NewMockEtcdService(t)
	mockEtcdService.On("GetPeers", mock.Anything).Return([]config.RaftNode{}, nil).Maybe()

	queueRepo := queue.NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, jobQueueRepo, jobRepo, jobExecutionsRepo, accountRepo, accountJobExecutionsCountRepo, mockQuotaAllocationSender, mockEtcdService)
	// Create a new JobService instance
	jobService := job.NewJobService(ctx, logger, jobRepo, queueRepo, projectRepo, jobExecutorRepo, dispatcher, asyncTaskManager, jobExecutionLogService, accountService)

	service := NewJobExecutor(
		ctx,
		logger,
		scheduler0config,
		scheduler0RaftActions,
		jobRepo,
		jobExecutionsRepo,
		jobExecutorRepo,
		jobQueueRepo,
		awsLambdaExecutor,
		webhookExecutor,
		gcpFunctionExecutor,
		azureFunctionExecutor,
		dispatcher,
		accountRepo,
		accountJobExecutionsCountRepo,
		queueRepo,
	)

	go service.ListenForJobsToInvokeV1()

	asyncTaskManager.SetSingleNodeMode(true)
	asyncTaskManager.ListenForNotifications()

	// Define the input jobs
	jobs := []models.Job{}
	job := models.Job{
		ID:        uint64(1),
		Spec:      "@every 1m",
		Timezone:  "America/New_York",
		ProjectID: 1,
		Data:      "http://%s",
	}

	time.Sleep(1 * time.Second)

	jobs = []models.Job{job}

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

	time.Sleep(time.Second * time.Duration(1))

	serr := os.Setenv("SCHEDULER0_NODE_ID", "1")
	defer os.Unsetenv("SCHEDULER0_NODE_ID")
	if serr != nil {
		t.Fatal("failed to set env", serr)
	}

	serr = os.Setenv("SCHEDULER0_JOB_EXECUTION_RETRY_MAX", "2")
	defer os.Unsetenv("SCHEDULER0_JOB_EXECUTION_RETRY_MAX")
	if serr != nil {
		t.Fatal("failed to set env", serr)
	}

	schedule, parseErr := cron.Parse("@every 1m")
	if parseErr != nil {
		t.Fatal("cron spec error", parseErr)
	}

	schedulerTime := scheduler0time.GetSchedulerTime()
	now := schedulerTime.GetTime(time.Now())
	nextTime := schedule.Next(now)

	service.GetExecutionsCache().Store(uint64(1), &models.JobSchedule{
		Job: job,
		MemExecution: models.MemJobExecution{
			NextExecutionDatetime: nextTime,
		},
	})

	service.UpdateRaft(scheduler0Store.GetRaft())

	time.Sleep(time.Minute*1 + time.Second*5)

	exec, ok := service.GetExecutionsCache().Load(uint64(1))
	if !ok {
		t.Fatal("should store job execution in cache")
	}
	cachedJobExecutionLog := (exec).(models.MemJobExecution)
	assert.Equal(t, 2, int(cachedJobExecutionLog.FailCount))
	uncommittedExecutionLogsCount := jobExecutionsRepo.CountExecutionLogs(false)
	assert.Equal(t, 3, int(uncommittedExecutionLogsCount))
}

func Test_StopAll(t *testing.T) {
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
	sqliteDb.RunMigration(logger)
	sqliteDb.OpenConnectionToExistingDB()

	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)

	// Create a new FSM store
	scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, scheduler0config, sqliteDb, nil, nil, nil, nil, sharedRepo)

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

	ctx, canceler := context.WithCancel(context.Background())
	defer canceler()

	jobRepo := job_repo.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)
	jobQueueRepo := job_queue_repo.NewJobQueuesRepo(logger, scheduler0RaftActions, scheduler0Store)
	jobExecutionsRepo := job_execution_repo.NewExecutionsRepo(
		logger,
		scheduler0RaftActions,
		scheduler0Store,
	)

	dispatcher := utils.NewDispatcher(
		ctx,
		int64(1),
		int64(1),
	)

	dispatcher.Run()

	// Create missing dependencies
	jobExecutorRepo := executor_repo.NewExecutorRepo(logger, scheduler0RaftActions, scheduler0Store, nil)
	accountRepo := account_repo.NewAccountRepository(ctx, logger, scheduler0RaftActions, scheduler0Store)
	accountJobExecutionsCountRepo := account_job_executions_count_repo.NewAccountJobExecutionsCountRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)

	// Create executor implementations
	awsLambdaExecutor := aws_lambda.NewLambdaExecutor(logger, ctx)
	webhookExecutor := webhook_executor.NewWebhookExecutor(logger, ctx, scheduler0config, dispatcher)
	gcpFunctionExecutor := gcp_function.NewFunctionsExecutor(logger, ctx)
	azureFunctionExecutor := azure_function.NewFunctionsExecutor(logger, ctx)

	// Create mocks for queue dependencies
	mockQuotaAllocationSender := queue.NewMockQuotaAllocationSender(t)
	mockEtcdService := etcd_service.NewMockEtcdService(t)
	mockEtcdService.On("GetPeers", mock.Anything).Return([]config.RaftNode{}, nil).Maybe()

	queueRepo := queue.NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, jobQueueRepo, jobRepo, jobExecutionsRepo, accountRepo, accountJobExecutionsCountRepo, mockQuotaAllocationSender, mockEtcdService)

	service := NewJobExecutor(
		ctx,
		logger,
		scheduler0config,
		scheduler0RaftActions,
		jobRepo,
		jobExecutionsRepo,
		jobExecutorRepo,
		jobQueueRepo,
		awsLambdaExecutor,
		webhookExecutor,
		gcpFunctionExecutor,
		azureFunctionExecutor,
		dispatcher,
		accountRepo,
		accountJobExecutionsCountRepo,
		queueRepo,
	)

	for i := 0; i < 10; i++ {
		service.GetExecutionsCache().Store(i, i)
	}
	service.StopAll()
	count := 0
	service.GetExecutionsCache().Range(func(key, value any) bool {
		count += 1
		return true
	})
	assert.Equal(t, count, 0)
}
