package job

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"scheduler0-private/pkg/config"
	"scheduler0-private/pkg/db"
	"scheduler0-private/pkg/fsm"
	"scheduler0-private/pkg/mocks"
	"scheduler0-private/pkg/models"
	account_repo "scheduler0-private/pkg/repository/account"
	account_job_executions_count_repo "scheduler0-private/pkg/repository/account_job_executions_count"
	async_task_repo "scheduler0-private/pkg/repository/async_task"
	executor_repo "scheduler0-private/pkg/repository/executor"
	job_repo "scheduler0-private/pkg/repository/job"
	job_execution_repo "scheduler0-private/pkg/repository/job_execution"
	job_queue_repo "scheduler0-private/pkg/repository/job_queue"
	project_repo "scheduler0-private/pkg/repository/project"
	"scheduler0-private/pkg/service/account"
	"scheduler0-private/pkg/service/async_task"
	etcd_service "scheduler0-private/pkg/service/etcd"
	"scheduler0-private/pkg/service/job_execution_service"
	"scheduler0-private/pkg/service/queue"
	"scheduler0-private/pkg/shared_repo"
	"scheduler0-private/pkg/utils"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// setupIntegrationTestJobService creates all dependencies needed for integration tests
func setupIntegrationTestJobService(t *testing.T, ctx context.Context, logger hclog.Logger, scheduler0config config.Scheduler0Config, scheduler0RaftActions fsm.Scheduler0RaftActions, scheduler0Store fsm.Scheduler0RaftStore) (JobService, job_repo.JobRepo, project_repo.ProjectRepo, account_repo.AccountRepository, executor_repo.JobExecutorRepo, *queue.MockQuotaAllocationSender, *etcd_service.MockEtcdService) {
	jobRepo := job_repo.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)
	projectRepo := project_repo.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)
	asyncTaskManagerRepo := async_task_repo.NewAsyncTasksRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)
	asyncTaskManager := async_task.NewAsyncTaskManager(ctx, logger, scheduler0Store, asyncTaskManagerRepo, scheduler0config)
	jobQueueRepo := job_queue_repo.NewJobQueuesRepo(logger, scheduler0RaftActions, scheduler0Store)
	jobExecutorRepo := executor_repo.NewExecutorRepo(logger, scheduler0RaftActions, scheduler0Store, nil)
	jobExecutionRepo := job_execution_repo.NewExecutionsRepo(logger, scheduler0RaftActions, scheduler0Store)
	accountRepo := account_repo.NewAccountRepository(ctx, logger, scheduler0RaftActions, scheduler0Store)
	accountJobExecutionsCountRepo := account_job_executions_count_repo.NewAccountJobExecutionsCountRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)
	jobExecutionLogService := job_execution_service.NewJobExecutionLogService(jobExecutionRepo, logger, accountJobExecutionsCountRepo, jobQueueRepo, accountRepo, jobRepo)

	dispatcher := utils.NewDispatcher(ctx, int64(1), int64(1))
	dispatcher.Run()

	mockQuotaAllocationSender := queue.NewMockQuotaAllocationSender(t)
	mockEtcdService := etcd_service.NewMockEtcdService(t)
	mockEtcdService.On("GetPeers", mock.AnythingOfType("*context.emptyCtx")).Return([]config.RaftNode{}, nil).Maybe()
	mockEtcdService.On("GetPeers", mock.AnythingOfType("*context.cancelCtx")).Return([]config.RaftNode{}, nil).Maybe()
	mockEtcdService.On("GetPeers", mock.AnythingOfType("*context.timerCtx")).Return([]config.RaftNode{}, nil).Maybe()
	mockEtcdService.On("GetPeers", mock.AnythingOfType("*context.valueCtx")).Return([]config.RaftNode{}, nil).Maybe()
	mockEtcdService.On("GetPeers", mock.Anything).Return([]config.RaftNode{}, nil).Maybe()

	queueRepo := queue.NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, jobQueueRepo, jobRepo, jobExecutionRepo, accountRepo, accountJobExecutionsCountRepo, mockQuotaAllocationSender, mockEtcdService)
	// prompt/classify/quota/feature repos are unused by these job-service tests (AccountId=1 skips feature checks).
	accountService := account.NewAccountService(accountRepo, accountJobExecutionsCountRepo, nil, nil, nil, nil, jobRepo, queueRepo)
	service := NewJobService(ctx, logger, jobRepo, queueRepo, projectRepo, jobExecutorRepo, dispatcher, asyncTaskManager, jobExecutionLogService, accountService)

	return service, jobRepo, projectRepo, accountRepo, jobExecutorRepo, mockQuotaAllocationSender, mockEtcdService
}

func Test_JobService_BatchInsertJobs(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "job-service-test",
		Level: hclog.LevelFromString("ERROR"),
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
	scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, scheduler0config, sqliteDb, nil, nil, nil, nil, nil)

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
	jobExecutorRepo := executor_repo.NewExecutorRepo(logger, scheduler0RaftActions, scheduler0Store, nil)
	jobExecutionRepo := job_execution_repo.NewExecutionsRepo(logger, scheduler0RaftActions, scheduler0Store)
	accountRepo := account_repo.NewAccountRepository(ctx, logger, scheduler0RaftActions, scheduler0Store)
	accountJobExecutionsCountRepo := account_job_executions_count_repo.NewAccountJobExecutionsCountRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)
	jobExecutionLogService := job_execution_service.NewJobExecutionLogService(jobExecutionRepo, logger, accountJobExecutionsCountRepo, jobQueueRepo, accountRepo, jobRepo)

	dispatcher := utils.NewDispatcher(
		ctx,
		int64(1),
		int64(1),
	)

	dispatcher.Run()

	mockQuotaAllocationSender := queue.NewMockQuotaAllocationSender(t)
	mockEtcdService := etcd_service.NewMockEtcdService(t)
	mockEtcdService.On("GetPeers", mock.AnythingOfType("*context.emptyCtx")).Return([]config.RaftNode{}, nil).Maybe()
	mockEtcdService.On("GetPeers", mock.AnythingOfType("*context.cancelCtx")).Return([]config.RaftNode{}, nil).Maybe()
	mockEtcdService.On("GetPeers", mock.AnythingOfType("*context.timerCtx")).Return([]config.RaftNode{}, nil).Maybe()
	mockEtcdService.On("GetPeers", mock.AnythingOfType("*context.valueCtx")).Return([]config.RaftNode{}, nil).Maybe()
	mockEtcdService.On("GetPeers", mock.Anything).Return([]config.RaftNode{}, nil).Maybe()

	queueRepo := queue.NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, jobQueueRepo, jobRepo, jobExecutionRepo, accountRepo, accountJobExecutionsCountRepo, mockQuotaAllocationSender, mockEtcdService)
	// prompt/classify/quota/feature repos are unused by these job-service tests (AccountId=1 skips feature checks).
	accountService := account.NewAccountService(accountRepo, accountJobExecutionsCountRepo, nil, nil, nil, nil, jobRepo, queueRepo)
	// Create a new JobService instance
	service := NewJobService(ctx, logger, jobRepo, queueRepo, projectRepo, jobExecutorRepo, dispatcher, asyncTaskManager, jobExecutionLogService, accountService)

	asyncTaskManager.SetSingleNodeMode(true)
	asyncTaskManager.ListenForNotifications()

	// Create an account first (required for foreign key constraint)
	account := &models.Account{
		ID:   1,
		Name: "Test Account",
	}
	_, createAccountErr := accountRepo.CreateAccount(account)
	if createAccountErr != nil {
		t.Fatalf("Failed to create account: %v", createAccountErr)
	}

	// Create an executor first (required for jobs)
	executor := models.JobExecutor{
		Name:          "Test Executor",
		Type:          "webhook_url",
		WebhookUrl:    "https://example.com/webhook",
		WebhookMethod: "POST",
		AccountId:     1,
	}
	executorId, createExecutorErr := jobExecutorRepo.CreateOne(executor)
	if createExecutorErr != nil {
		t.Fatalf("Failed to create executor: %v", createExecutorErr)
	}

	// Define the input jobs
	jobs := []models.Job{
		{
			ID:         1,
			Spec:       "* * * * *",
			Timezone:   "UTC",
			ProjectID:  1,
			AccountId:  1,
			ExecutorId: &executorId,
		},
		{
			ID:         2,
			Spec:       "0 0 * * *",
			Timezone:   "America/New_York",
			ProjectID:  2,
			AccountId:  1,
			ExecutorId: &executorId,
		},
	}

	// Create the projects using the project repo
	for _, job := range jobs {
		project := models.Project{
			ID:          job.ProjectID,
			Name:        fmt.Sprintf("Project %d", job.ProjectID),
			Description: fmt.Sprintf("Project %d description", job.ProjectID),
			AccountId:   1,
		}
		_, createErr := projectRepo.CreateOne(&project)
		if createErr != nil {
			t.Fatalf("Failed to create project: %v", createErr)
		}
	}

	// Call the BatchInsertJobs method of the job service
	taskIds, batchErr := service.BatchInsertJobs("request123", jobs)
	if batchErr != nil {
		t.Fatalf("Failed to insert jobs: %v", batchErr)
	}

	time.Sleep(time.Second * time.Duration(2))

	assert.Equal(t, taskIds[0], uint64(1))

	jobsMap := map[uint64]models.Job{}
	for _, job := range jobs {
		jobsMap[job.ID] = job
	}

	// Assert the correctness of the job state after insertion
	for _, job := range jobs {
		retrievedJob, getErr := service.GetJob(job)
		if getErr != nil {
			t.Fatalf("Failed to get job: %v", getErr)
		}

		assert.Equal(t, job.Spec, retrievedJob.Spec)
		assert.Equal(t, job.Timezone, retrievedJob.Timezone)
		assert.Equal(t, job.ProjectID, retrievedJob.ProjectID)
	}
}

func Test_JobService_UpdateJob(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "job-service-test",
		Level: hclog.LevelFromString("DEBUG"),
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
	scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, scheduler0config, sqliteDb, nil, nil, nil, nil, nil)

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

	ctx := context.Background()

	service, jobRepo, projectRepo, accountRepo, jobExecutorRepo, _, _ := setupIntegrationTestJobService(t, ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store)

	// Create an account first (required for foreign key constraint)
	account := &models.Account{
		ID:   1,
		Name: "Test Account",
	}
	_, createAccountErr := accountRepo.CreateAccount(account)
	if createAccountErr != nil {
		t.Fatalf("Failed to create account: %v", createAccountErr)
	}

	// Create an executor first (required for jobs)
	executor := models.JobExecutor{
		Name:          "Test Executor",
		Type:          "webhook_url",
		WebhookUrl:    "https://example.com/webhook",
		WebhookMethod: "POST",
		AccountId:     1,
	}
	executorId, createExecutorErr := jobExecutorRepo.CreateOne(executor)
	if createExecutorErr != nil {
		t.Fatalf("Failed to create executor: %v", createExecutorErr)
	}

	// Create a test job
	job := models.Job{
		ID:         1,
		Spec:       "* * * * *",
		Timezone:   "UTC",
		ProjectID:  1,
		AccountId:  1,
		ExecutorId: &executorId,
		Data:       "Test data",
	}

	// Create the project using the project repo
	project := models.Project{
		ID:          job.ProjectID,
		Name:        fmt.Sprintf("Project %d", job.ProjectID),
		Description: fmt.Sprintf("Project %d description", job.ProjectID),
		AccountId:   1,
	}
	_, createErr := projectRepo.CreateOne(&project)
	if createErr != nil {
		t.Fatalf("Failed to create project: %v", createErr)
	}

	// Insert the job into the job repo
	_, insertErr := jobRepo.BatchInsertJobs([]models.Job{job})
	if insertErr != nil {
		t.Fatalf("Failed to insert job: %v", insertErr)
	}

	// Update the job (preserve required fields like Spec, Timezone, and ExecutorId)
	updatedJob := models.Job{
		ID:         job.ID,
		ProjectID:  job.ProjectID,
		AccountId:  job.AccountId,
		Spec:       job.Spec,       // Preserve the spec (required for validation)
		Timezone:   job.Timezone,   // Preserve the timezone (required for validation)
		ExecutorId: job.ExecutorId, // Preserve the executor (required for validation)
		Data:       "Updated test data",
	}
	_, updateErr := service.UpdateJob(updatedJob)
	if updateErr != nil {
		t.Fatalf("Failed to update job: %v", updateErr)
	}

	// Retrieve the updated job from the job repo
	getErr := jobRepo.GetOneByID(&updatedJob)
	if getErr != nil {
		t.Fatalf("Failed to get job: %v", getErr)
	}
}

func Test_JobService_DeleteJob(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "job-service-test",
		Level: hclog.LevelFromString("DEBUG"),
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
	scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, scheduler0config, sqliteDb, nil, nil, nil, nil, nil)

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

	ctx := context.Background()

	service, jobRepo, projectRepo, accountRepo, jobExecutorRepo, _, _ := setupIntegrationTestJobService(t, ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store)

	// Create an account first (required for foreign key constraint)
	account := &models.Account{
		ID:   1,
		Name: "Test Account",
	}
	_, createAccountErr := accountRepo.CreateAccount(account)
	if createAccountErr != nil {
		t.Fatalf("Failed to create account: %v", createAccountErr)
	}

	// Create an executor first (required for jobs)
	executor := models.JobExecutor{
		Name:          "Test Executor",
		Type:          "webhook_url",
		WebhookUrl:    "https://example.com/webhook",
		WebhookMethod: "POST",
		AccountId:     1,
	}
	executorId, createExecutorErr := jobExecutorRepo.CreateOne(executor)
	if createExecutorErr != nil {
		t.Fatalf("Failed to create executor: %v", createExecutorErr)
	}

	// Create a test job
	job := models.Job{
		ID:         1,
		Spec:       "* * * * *",
		Timezone:   "UTC",
		ProjectID:  1,
		AccountId:  1,
		ExecutorId: &executorId,
		Data:       "Test data",
	}

	// Create the project using the project repo
	project := models.Project{
		ID:          job.ProjectID,
		Name:        fmt.Sprintf("Project %d", job.ProjectID),
		Description: fmt.Sprintf("Project %d description", job.ProjectID),
		AccountId:   1,
	}
	_, createErr := projectRepo.CreateOne(&project)
	if createErr != nil {
		t.Fatalf("Failed to create project: %v", createErr)
	}

	// Insert the job into the job repo
	_, insertErr := jobRepo.BatchInsertJobs([]models.Job{job})
	if insertErr != nil {
		t.Fatalf("Failed to insert job: %v", insertErr)
	}

	// Delete the job
	deleteErr := service.DeleteJob(job)
	if deleteErr != nil {
		t.Fatalf("Failed to delete job: %v", deleteErr)
	}

	// Try to retrieve the deleted job from the job repo (soft delete, so job still exists)
	getErr := jobRepo.GetOneByID(&job)
	if getErr != nil {
		t.Fatalf("Failed to get deleted job: %v", getErr)
	}

	// Assert that DeletedBy is set, indicating the job was successfully soft-deleted
	assert.NotNil(t, job.DeletedBy)
	assert.Equal(t, "system", *job.DeletedBy)
}

func Test_JobService_GetJobsByProjectID(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "job-service-test",
		Level: hclog.LevelFromString("DEBUG"),
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
	scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, scheduler0config, sqliteDb, nil, nil, nil, nil, nil)

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

	ctx := context.Background()

	service, jobRepo, projectRepo, accountRepo, jobExecutorRepo, _, _ := setupIntegrationTestJobService(t, ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store)

	// Create an account first (required for foreign key constraint)
	account := &models.Account{
		ID:   1,
		Name: "Test Account",
	}
	_, createAccountErr := accountRepo.CreateAccount(account)
	if createAccountErr != nil {
		t.Fatalf("Failed to create account: %v", createAccountErr)
	}

	// Create an executor first (required for jobs)
	executor := models.JobExecutor{
		Name:          "Test Executor",
		Type:          "webhook_url",
		WebhookUrl:    "https://example.com/webhook",
		WebhookMethod: "POST",
		AccountId:     1,
	}
	executorId, createExecutorErr := jobExecutorRepo.CreateOne(executor)
	if createExecutorErr != nil {
		t.Fatalf("Failed to create executor: %v", createExecutorErr)
	}

	// Create a test project
	projectID := uint64(1)
	project := models.Project{
		ID:          projectID,
		Name:        "Project 1",
		Description: "Project 1 description",
		AccountId:   1,
	}

	// Insert the project into the project repo
	_, createErr := projectRepo.CreateOne(&project)
	if createErr != nil {
		t.Fatalf("Failed to create project: %v", createErr)
	}

	// Create test jobs associated with the project
	jobs := []models.Job{
		{
			ID:         1,
			Spec:       "* * * * *",
			Timezone:   "UTC",
			ProjectID:  projectID,
			AccountId:  1,
			ExecutorId: &executorId,
			Data:       "Test data 1",
		},
		{
			ID:         2,
			Spec:       "0 0 * * *",
			Timezone:   "America/New_York",
			ProjectID:  projectID,
			AccountId:  1,
			ExecutorId: &executorId,
			Data:       "Test data 2",
		},
		{
			ID:         3,
			Spec:       "0 0 * * *",
			Timezone:   "Europe/London",
			ProjectID:  projectID,
			AccountId:  1,
			ExecutorId: &executorId,
			Data:       "Test data 3",
		},
	}

	// Insert the jobs into the job repo
	_, insertErr := jobRepo.BatchInsertJobs(jobs)
	if insertErr != nil {
		t.Fatalf("Failed to insert jobs: %v", insertErr)
	}

	// Call the GetJobsByProjectID method
	offset := uint64(0)
	limit := uint64(2)
	orderBy := "id"
	result, getErr := service.GetJobsByProjectID(projectID, offset, limit, orderBy, "asc")
	if getErr != nil {
		t.Fatalf("Failed to get jobs: %v", getErr)
	}

	// Assert the correctness of the retrieved jobs
	assert.Equal(t, 2, len(result.Data))
	assert.Equal(t, uint64(len(jobs)), result.Total)
	assert.Equal(t, jobs[0].ID, result.Data[0].ID)
	assert.Equal(t, jobs[0].Spec, result.Data[0].Spec)
	assert.Equal(t, jobs[0].Timezone, result.Data[0].Timezone)
	assert.Equal(t, jobs[0].ProjectID, result.Data[0].ProjectID)
	assert.Equal(t, jobs[0].Data, result.Data[0].Data)
	assert.Equal(t, jobs[1].ID, result.Data[1].ID)
	assert.Equal(t, jobs[1].Spec, result.Data[1].Spec)
	assert.Equal(t, jobs[1].Timezone, result.Data[1].Timezone)
	assert.Equal(t, jobs[1].ProjectID, result.Data[1].ProjectID)
	assert.Equal(t, jobs[1].Data, result.Data[1].Data)
}

func Test_JobService_GetJob(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "job-service-test",
		Level: hclog.LevelFromString("DEBUG"),
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
	scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, scheduler0config, sqliteDb, nil, nil, nil, nil, nil)

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

	ctx := context.Background()

	service, jobRepo, projectRepo, accountRepo, jobExecutorRepo, _, _ := setupIntegrationTestJobService(t, ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store)

	// Create an account first (required for foreign key constraint)
	account := &models.Account{
		ID:   1,
		Name: "Test Account",
	}
	_, createAccountErr := accountRepo.CreateAccount(account)
	if createAccountErr != nil {
		t.Fatalf("Failed to create account: %v", createAccountErr)
	}

	// Create an executor first (required for jobs)
	executor := models.JobExecutor{
		Name:          "Test Executor",
		Type:          "webhook_url",
		WebhookUrl:    "https://example.com/webhook",
		WebhookMethod: "POST",
		AccountId:     1,
	}
	executorId, createExecutorErr := jobExecutorRepo.CreateOne(executor)
	if createExecutorErr != nil {
		t.Fatalf("Failed to create executor: %v", createExecutorErr)
	}

	// Create a test job
	job := models.Job{
		ID:         1,
		Spec:       "* * * * *",
		Timezone:   "UTC",
		ProjectID:  1,
		AccountId:  1,
		ExecutorId: &executorId,
		Data:       "Test data",
	}

	// Create the project using the project repo
	project := models.Project{
		ID:          job.ProjectID,
		Name:        fmt.Sprintf("Project %d", job.ProjectID),
		Description: fmt.Sprintf("Project %d description", job.ProjectID),
		AccountId:   1,
	}
	_, createErr := projectRepo.CreateOne(&project)
	if createErr != nil {
		t.Fatalf("Failed to create project: %v", createErr)
	}

	// Insert the job into the job repo
	_, insertErr := jobRepo.BatchInsertJobs([]models.Job{job})
	if insertErr != nil {
		t.Fatalf("Failed to insert job: %v", insertErr)
	}

	// Call the GetJob method
	retrievedJob, getErr := service.GetJob(job)
	if getErr != nil {
		t.Fatalf("Failed to get job: %v", getErr)
	}

	// Assert the correctness of the retrieved job
	assert.Equal(t, job.ID, retrievedJob.ID)
	assert.Equal(t, job.Spec, retrievedJob.Spec)
	assert.Equal(t, job.Timezone, retrievedJob.Timezone)
	assert.Equal(t, job.ProjectID, retrievedJob.ProjectID)
	assert.Equal(t, job.Data, retrievedJob.Data)
}

func Test_JobService_QueueJobs(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "job-service-test",
		Level: hclog.LevelFromString("DEBUG"),
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
	scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, scheduler0config, sqliteDb, nil, nil, nil, nil, nil)

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

	ctx := context.Background()

	service, jobRepo, projectRepo, accountRepo, jobExecutorRepo, _, _ := setupIntegrationTestJobService(t, ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store)

	// Create an account first (required for foreign key constraint)
	account := &models.Account{
		ID:   1,
		Name: "Test Account",
	}
	_, createAccountErr := accountRepo.CreateAccount(account)
	if createAccountErr != nil {
		t.Fatalf("Failed to create account: %v", createAccountErr)
	}

	// Create an executor first (required for jobs)
	executor := models.JobExecutor{
		Name:          "Test Executor",
		Type:          "webhook_url",
		WebhookUrl:    "https://example.com/webhook",
		WebhookMethod: "POST",
		AccountId:     1,
	}
	executorId, createExecutorErr := jobExecutorRepo.CreateOne(executor)
	if createExecutorErr != nil {
		t.Fatalf("Failed to create executor: %v", createExecutorErr)
	}

	// Define the input jobs
	jobs := []models.Job{
		{
			ID:         1,
			Spec:       "* * * * *",
			Timezone:   "UTC",
			ProjectID:  1,
			AccountId:  1,
			ExecutorId: &executorId,
		},
		{
			ID:         2,
			Spec:       "0 0 * * *",
			Timezone:   "America/New_York",
			ProjectID:  2,
			AccountId:  1,
			ExecutorId: &executorId,
		},
	}

	// Create the projects using the project repo
	for _, job := range jobs {
		project := models.Project{
			ID:          job.ProjectID,
			Name:        fmt.Sprintf("Project %d", job.ProjectID),
			Description: fmt.Sprintf("Project %d description", job.ProjectID),
			AccountId:   1,
		}
		_, createErr := projectRepo.CreateOne(&project)
		if createErr != nil {
			t.Fatalf("Failed to create project: %v", createErr)
		}
	}

	// Insert the jobs first (required before queuing)
	_, insertErr := jobRepo.BatchInsertJobs(jobs)
	if insertErr != nil {
		t.Fatalf("Failed to insert jobs: %v", insertErr)
	}

	os.Setenv("SCHEDULER0_NODE_ID", "1")
	defer os.Unsetenv("SCHEDULER0_NODE_ID")
	service.QueueJobs(jobs)
}

// ==================== Unit Tests with Mocks ====================

// setupUnitTestJobService creates a job service with mocked dependencies for unit testing
func setupUnitTestJobService(t *testing.T) (*jobService, *mocks.MockJobRepo, *mocks.MockProjectRepo, *queue.MockJobQueueService, *async_task.MockAsyncTaskService, *mocks.MockJobExecutionLogService, *mocks.MockAccountService) {
	service, mockJobRepo, mockProjectRepo, _, mockQueueService, mockAsyncTaskService, mockJobExecutionLogService, mockAccountService := setupUnitTestJobServiceWithExecutorRepo(t)
	return service, mockJobRepo, mockProjectRepo, mockQueueService, mockAsyncTaskService, mockJobExecutionLogService, mockAccountService
}

func setupUnitTestJobServiceWithExecutorRepo(t *testing.T) (*jobService, *mocks.MockJobRepo, *mocks.MockProjectRepo, *mocks.MockJobExecutorRepo, *queue.MockJobQueueService, *async_task.MockAsyncTaskService, *mocks.MockJobExecutionLogService, *mocks.MockAccountService) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "job-service-unit-test",
		Level: hclog.LevelFromString("ERROR"),
	})

	ctx := context.Background()

	// Create mocks
	mockJobRepo := mocks.NewMockJobRepo(t)
	mockProjectRepo := mocks.NewMockProjectRepo(t)
	mockQueueService := queue.NewMockJobQueueService(t)
	mockAsyncTaskService := async_task.NewMockAsyncTaskService(t)
	mockAsyncTaskService.On("SetNodeIsLeader", mock.Anything).Return().Maybe()
	mockJobExecutionLogService := mocks.NewMockJobExecutionLogService(t)
	mockAccountService := mocks.NewMockAccountService(t)

	// Create a mock dispatcher (we'll use a real one but it won't be used in most tests)
	dispatcher := utils.NewDispatcher(ctx, int64(1), int64(1))

	// Use mockery-generated mock for JobExecutorRepo
	mockJobExecutorRepo := mocks.NewMockJobExecutorRepo(t)

	service := NewJobService(
		ctx,
		logger,
		mockJobRepo,
		mockQueueService,
		mockProjectRepo,
		mockJobExecutorRepo,
		dispatcher,
		mockAsyncTaskService,
		mockJobExecutionLogService,
		mockAccountService,
	).(*jobService)

	return service, mockJobRepo, mockProjectRepo, mockJobExecutorRepo, mockQueueService, mockAsyncTaskService, mockJobExecutionLogService, mockAccountService
}

func Test_JobService_ValidateBatch_RejectsCrossAccountExecutorAndProject(t *testing.T) {
	executorID := uint64(42)
	otherAccountExecutorID := uint64(99)
	projectID := uint64(7)
	otherAccountProjectID := uint64(8)

	tests := []struct {
		name                string
		job                 models.Job
		projects            []models.Project
		executors           []models.JobExecutor
		expectedErrorCode   int
		expectedErrorSubstr string
	}{
		{
			name: "rejects executor owned by another account",
			job: models.Job{
				Spec:       "* * * * *",
				Timezone:   "UTC",
				ProjectID:  projectID,
				AccountId:  1,
				ExecutorId: &otherAccountExecutorID,
			},
			projects: []models.Project{
				{ID: projectID, AccountId: 1},
			},
			executors: []models.JobExecutor{
				{ID: otherAccountExecutorID, AccountId: 2},
			},
			expectedErrorCode:   http.StatusForbidden,
			expectedErrorSubstr: "executor does not belong to account",
		},
		{
			name: "rejects project owned by another account",
			job: models.Job{
				Spec:       "* * * * *",
				Timezone:   "UTC",
				ProjectID:  otherAccountProjectID,
				AccountId:  1,
				ExecutorId: &executorID,
			},
			projects: []models.Project{
				{ID: otherAccountProjectID, AccountId: 2},
			},
			executors: []models.JobExecutor{
				{ID: executorID, AccountId: 1},
			},
			expectedErrorCode:   http.StatusForbidden,
			expectedErrorSubstr: "project does not belong to account",
		},
		{
			name: "allows matching account ownership",
			job: models.Job{
				Spec:       "* * * * *",
				Timezone:   "UTC",
				ProjectID:  projectID,
				AccountId:  1,
				ExecutorId: &executorID,
			},
			projects: []models.Project{
				{ID: projectID, AccountId: 1},
			},
			executors: []models.JobExecutor{
				{ID: executorID, AccountId: 1},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, _, mockProjectRepo, mockJobExecutorRepo, _, _, _, _ := setupUnitTestJobServiceWithExecutorRepo(t)

			mockProjectRepo.On("GetBatchProjectsByIDs", mock.Anything).Return(tt.projects, (*utils.GenericError)(nil))
			mockJobExecutorRepo.On("BatchGetByIds", mock.Anything).Return(tt.executors, (*utils.GenericError)(nil))

			err := service.validateBatch([]models.Job{tt.job})
			if tt.expectedErrorSubstr != "" {
				assert.NotNil(t, err)
				assert.Equal(t, tt.expectedErrorCode, err.Type)
				assert.Contains(t, err.Message, tt.expectedErrorSubstr)
				return
			}
			assert.Nil(t, err)
		})
	}
}

func Test_JobService_GetJob_RequiresAccountScope(t *testing.T) {
	service, mockJobRepo, _, _, _, _, _ := setupUnitTestJobService(t)

	t.Run("rejects missing account id", func(t *testing.T) {
		_, err := service.GetJob(models.Job{ID: 1})
		assert.NotNil(t, err)
		assert.Equal(t, http.StatusBadRequest, err.Type)
		mockJobRepo.AssertNotCalled(t, "GetOneByID", mock.Anything)
	})

	t.Run("passes account-scoped lookup", func(t *testing.T) {
		job := models.Job{ID: 9, AccountId: 3}
		mockJobRepo.On("GetOneByID", mock.MatchedBy(func(j *models.Job) bool {
			return j.ID == 9 && j.AccountId == 3
		})).Run(func(args mock.Arguments) {
			j := args.Get(0).(*models.Job)
			j.Spec = "@every 1m"
			j.ProjectID = 4
		}).Return((*utils.GenericError)(nil)).Once()

		got, err := service.GetJob(job)
		assert.Nil(t, err)
		assert.Equal(t, "@every 1m", got.Spec)
		assert.Equal(t, uint64(4), got.ProjectID)
	})
}

func Test_JobService_GetJobsByProjectID_Unit(t *testing.T) {
	tests := []struct {
		name          string
		projectID     uint64
		offset        uint64
		limit         uint64
		orderBy       string
		orderDir      string
		setupMocks    func(*mocks.MockJobRepo)
		expectedError bool
		expectedCode  int
	}{
		{
			name:      "successful retrieval",
			projectID: 1,
			offset:    0,
			limit:     10,
			orderBy:   "id",
			orderDir:  "asc",
			setupMocks: func(mockJobRepo *mocks.MockJobRepo) {
				mockJobRepo.On("GetJobsTotalCountByProjectID", uint64(1)).Return(uint64(5), (*utils.GenericError)(nil))
				mockJobRepo.On("GetAllByProjectID", uint64(1), uint64(0), uint64(10), "id", "asc").Return([]models.Job{
					{ID: 1, ProjectID: 1},
					{ID: 2, ProjectID: 1},
				}, (*utils.GenericError)(nil))
			},
			expectedError: false,
		},
		{
			name:      "limit exceeds maximum",
			projectID: 1,
			offset:    0,
			limit:     101,
			orderBy:   "id",
			orderDir:  "asc",
			setupMocks: func(mockJobRepo *mocks.MockJobRepo) {
				// No repo calls expected
			},
			expectedError: true,
			expectedCode:  http.StatusTooManyRequests,
		},
		{
			name:      "limit is zero",
			projectID: 1,
			offset:    0,
			limit:     0,
			orderBy:   "id",
			orderDir:  "asc",
			setupMocks: func(mockJobRepo *mocks.MockJobRepo) {
				// No repo calls expected
			},
			expectedError: true,
			expectedCode:  http.StatusBadRequest,
		},
		{
			name:      "offset exceeds total count",
			projectID: 1,
			offset:    100,
			limit:     10,
			orderBy:   "id",
			orderDir:  "asc",
			setupMocks: func(mockJobRepo *mocks.MockJobRepo) {
				mockJobRepo.On("GetJobsTotalCountByProjectID", uint64(1)).Return(uint64(5), (*utils.GenericError)(nil))
				mockJobRepo.On("GetAllByProjectID", uint64(1), uint64(5), uint64(10), "id", "asc").Return([]models.Job{}, (*utils.GenericError)(nil))
			},
			expectedError: false,
		},
		{
			name:      "repository error on count",
			projectID: 1,
			offset:    0,
			limit:     10,
			orderBy:   "id",
			orderDir:  "asc",
			setupMocks: func(mockJobRepo *mocks.MockJobRepo) {
				mockJobRepo.On("GetJobsTotalCountByProjectID", uint64(1)).Return(uint64(0), utils.HTTPGenericError(http.StatusInternalServerError, "database error"))
			},
			expectedError: true,
			expectedCode:  http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fresh mocks for each test case
			service, mockJobRepo, _, _, _, _, _ := setupUnitTestJobService(t)
			tt.setupMocks(mockJobRepo)
			result, err := service.GetJobsByProjectID(tt.projectID, tt.offset, tt.limit, tt.orderBy, tt.orderDir)

			if tt.expectedError {
				assert.NotNil(t, err)
				if tt.expectedCode != 0 && err != nil {
					assert.Equal(t, tt.expectedCode, err.Type)
				}
				assert.Nil(t, result)
			} else {
				assert.Nil(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func Test_JobService_GetJobsByAccountID_Unit(t *testing.T) {
	service, mockJobRepo, _, _, _, _, _ := setupUnitTestJobService(t)

	tests := []struct {
		name          string
		accountID     uint64
		projectID     *uint64
		offset        uint64
		limit         uint64
		orderBy       string
		orderDir      string
		setupMocks    func()
		expectedError bool
		expectedCode  int
	}{
		{
			name:      "successful retrieval",
			accountID: 1,
			projectID: nil,
			offset:    0,
			limit:     10,
			orderBy:   "id",
			orderDir:  "asc",
			setupMocks: func() {
				mockJobRepo.On("GetJobsPaginated", uint64(1), uint64(0), uint64(0), uint64(10), "id", "asc").Return([]models.Job{
					{ID: 1, AccountId: 1},
					{ID: 2, AccountId: 1},
				}, uint64(2), (*utils.GenericError)(nil))
			},
			expectedError: false,
		},
		{
			name:      "with project filter",
			accountID: 1,
			projectID: func() *uint64 { p := uint64(5); return &p }(),
			offset:    0,
			limit:     10,
			orderBy:   "id",
			orderDir:  "asc",
			setupMocks: func() {
				mockJobRepo.On("GetJobsPaginated", uint64(1), uint64(5), uint64(0), uint64(10), "id", "asc").Return([]models.Job{
					{ID: 1, AccountId: 1, ProjectID: 5},
				}, uint64(1), (*utils.GenericError)(nil))
			},
			expectedError: false,
		},
		{
			name:      "limit exceeds maximum",
			accountID: 1,
			projectID: nil,
			offset:    0,
			limit:     101,
			orderBy:   "id",
			orderDir:  "asc",
			setupMocks: func() {
				// No repo calls expected
			},
			expectedError: true,
			expectedCode:  http.StatusTooManyRequests,
		},
		{
			name:      "limit is zero",
			accountID: 1,
			projectID: nil,
			offset:    0,
			limit:     0,
			orderBy:   "id",
			orderDir:  "asc",
			setupMocks: func() {
				// No repo calls expected
			},
			expectedError: true,
			expectedCode:  http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMocks()
			result, err := service.GetJobsByAccountID(tt.accountID, tt.projectID, tt.offset, tt.limit, tt.orderBy, tt.orderDir)

			if tt.expectedError {
				assert.NotNil(t, err)
				if tt.expectedCode != 0 {
					assert.Equal(t, tt.expectedCode, err.Type)
				}
				assert.Nil(t, result)
			} else {
				assert.Nil(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func Test_JobService_GetJob_Unit(t *testing.T) {
	service, mockJobRepo, _, _, _, _, _ := setupUnitTestJobService(t)

	tests := []struct {
		name          string
		job           models.Job
		setupMocks    func()
		expectedError bool
	}{
		{
			name: "successful retrieval",
			job:  models.Job{ID: 1, AccountId: 1},
			setupMocks: func() {
				mockJobRepo.On("GetOneByID", mock.MatchedBy(func(job *models.Job) bool {
					return job.ID == 1 && job.AccountId == 1
				})).Return((*utils.GenericError)(nil)).Run(func(args mock.Arguments) {
					job := args.Get(0).(*models.Job)
					job.ID = 1
					job.ProjectID = 1
					job.AccountId = 1
				})
			},
			expectedError: false,
		},
		{
			name: "job not found",
			job:  models.Job{ID: 999, AccountId: 1},
			setupMocks: func() {
				mockJobRepo.On("GetOneByID", mock.MatchedBy(func(job *models.Job) bool {
					return job.ID == 999 && job.AccountId == 1
				})).Return(utils.HTTPGenericError(http.StatusNotFound, "job not found"))
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMocks()
			result, err := service.GetJob(tt.job)

			if tt.expectedError {
				assert.NotNil(t, err)
				assert.Nil(t, result)
			} else {
				assert.Nil(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.job.ID, result.ID)
			}
		})
	}
}

func Test_JobService_DeleteJob_Unit(t *testing.T) {
	tests := []struct {
		name          string
		job           models.Job
		setupMocks    func(*mocks.MockJobRepo)
		expectedError bool
		expectedCode  int
	}{
		{
			name: "successful deletion",
			job:  models.Job{ID: 1, AccountId: 1},
			setupMocks: func(mockJobRepo *mocks.MockJobRepo) {
				mockJobRepo.On("GetOneByID", mock.MatchedBy(func(job *models.Job) bool {
					return job.ID == 1
				})).Return((*utils.GenericError)(nil))
				mockJobRepo.On("DeleteOneByID", models.Job{ID: 1, AccountId: 1}).Return(uint64(1), (*utils.GenericError)(nil))
			},
			expectedError: false,
		},
		{
			name: "job not found",
			job:  models.Job{ID: 999, AccountId: 1},
			setupMocks: func(mockJobRepo *mocks.MockJobRepo) {
				mockJobRepo.On("GetOneByID", mock.MatchedBy(func(job *models.Job) bool {
					return job.ID == 999
				})).Return(utils.HTTPGenericError(http.StatusNotFound, "job not found"))
			},
			expectedError: true,
			expectedCode:  http.StatusNotFound,
		},
		{
			name: "no rows affected",
			job:  models.Job{ID: 1, AccountId: 1},
			setupMocks: func(mockJobRepo *mocks.MockJobRepo) {
				mockJobRepo.On("GetOneByID", mock.MatchedBy(func(job *models.Job) bool {
					return job.ID == 1
				})).Return((*utils.GenericError)(nil))
				mockJobRepo.On("DeleteOneByID", models.Job{ID: 1, AccountId: 1}).Return(uint64(0), (*utils.GenericError)(nil))
			},
			expectedError: true,
			expectedCode:  http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, mockJobRepo, _, _, _, _, _ := setupUnitTestJobService(t)
			tt.setupMocks(mockJobRepo)
			err := service.DeleteJob(tt.job)

			if tt.expectedError {
				assert.NotNil(t, err)
				if tt.expectedCode != 0 {
					assert.Equal(t, tt.expectedCode, err.Type)
				}
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func Test_JobService_DeleteJobsByProjectID_Unit(t *testing.T) {
	tests := []struct {
		name          string
		projectID     uint64
		accountId     uint64
		deletedBy     string
		setupMocks    func(*mocks.MockJobRepo)
		expectedError bool
		expectedRows  int64
	}{
		{
			name:      "successful deletion",
			projectID: 1,
			accountId: 2,
			deletedBy: "test-user",
			setupMocks: func(mockJobRepo *mocks.MockJobRepo) {
				mockJobRepo.On("DeleteJobsByProjectID", uint64(1), uint64(2), "test-user").Return(uint64(5), (*utils.GenericError)(nil))
			},
			expectedError: false,
			expectedRows:  5,
		},
		{
			name:      "repository error",
			projectID: 1,
			accountId: 2,
			deletedBy: "test-user",
			setupMocks: func(mockJobRepo *mocks.MockJobRepo) {
				mockJobRepo.On("DeleteJobsByProjectID", uint64(1), uint64(2), "test-user").Return(uint64(0), utils.HTTPGenericError(http.StatusInternalServerError, "database error"))
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, mockJobRepo, _, _, _, _, _ := setupUnitTestJobService(t)
			tt.setupMocks(mockJobRepo)
			err := service.DeleteJobsByProjectID(tt.projectID, tt.accountId, tt.deletedBy)

			if tt.expectedError {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func Test_JobService_QueueJobs_Unit(t *testing.T) {
	service, _, _, mockQueueService, _, _, _ := setupUnitTestJobService(t)

	jobs := []models.Job{
		{ID: 1, ProjectID: 1, AccountId: 1},
		{ID: 2, ProjectID: 1, AccountId: 1},
	}

	mockQueueService.On("Queue", jobs).Return()

	service.QueueJobs(jobs)

	mockQueueService.AssertExpectations(t)
}

func Test_JobService_validateJob_Unit(t *testing.T) {
	service, _, _, _, _, _, _ := setupUnitTestJobService(t)

	futureTime := time.Now().Add(24 * time.Hour)
	pastTime := time.Now().Add(-24 * time.Hour)

	tests := []struct {
		name                string
		job                 models.Job
		hasJobPayloadOf1Mb  bool
		hasJobRetryMaxBy5   bool
		expectedError       bool
		expectedErrorCode   int
		expectedErrorSubstr string
	}{
		{
			name: "valid job with spec",
			job: models.Job{
				Spec:       "* * * * *",
				Timezone:   "UTC",
				ExecutorId: func() *uint64 { id := uint64(1); return &id }(),
				Data:       "test data",
				RetryMax:   3,
				Status:     models.JobStatusActive,
			},
			hasJobPayloadOf1Mb: false,
			hasJobRetryMaxBy5:  false,
			expectedError:      false,
		},
		{
			name: "valid one-time job with startDate",
			job: models.Job{
				Spec:       "",
				StartDate:  futureTime,
				Timezone:   "UTC",
				ExecutorId: func() *uint64 { id := uint64(1); return &id }(),
				Data:       "test data",
				RetryMax:   3,
				Status:     models.JobStatusActive,
			},
			hasJobPayloadOf1Mb: false,
			hasJobRetryMaxBy5:  false,
			expectedError:      false,
		},
		{
			name: "invalid cron spec",
			job: models.Job{
				Spec:       "invalid cron",
				Timezone:   "UTC",
				ExecutorId: func() *uint64 { id := uint64(1); return &id }(),
				Data:       "test data",
			},
			hasJobPayloadOf1Mb: false,
			hasJobRetryMaxBy5:  false,
			expectedError:      true,
			expectedErrorCode:  http.StatusBadRequest,
		},
		{
			name: "missing spec and startDate",
			job: models.Job{
				Spec:       "",
				Timezone:   "UTC",
				ExecutorId: func() *uint64 { id := uint64(1); return &id }(),
				Data:       "test data",
			},
			hasJobPayloadOf1Mb: false,
			hasJobRetryMaxBy5:  false,
			expectedError:      true,
			expectedErrorCode:  http.StatusBadRequest,
		},
		{
			name: "invalid timezone",
			job: models.Job{
				Spec:       "* * * * *",
				Timezone:   "Invalid/Timezone",
				ExecutorId: func() *uint64 { id := uint64(1); return &id }(),
				Data:       "test data",
			},
			hasJobPayloadOf1Mb: false,
			hasJobRetryMaxBy5:  false,
			expectedError:      true,
			expectedErrorCode:  http.StatusBadRequest,
		},
		{
			name: "empty timezone",
			job: models.Job{
				Spec:       "* * * * *",
				Timezone:   "",
				ExecutorId: func() *uint64 { id := uint64(1); return &id }(),
				Data:       "test data",
			},
			hasJobPayloadOf1Mb: false,
			hasJobRetryMaxBy5:  false,
			expectedError:      true,
			expectedErrorCode:  http.StatusBadRequest,
		},
		{
			name: "missing executor",
			job: models.Job{
				Spec:     "* * * * *",
				Timezone: "UTC",
				Data:     "test data",
			},
			hasJobPayloadOf1Mb: false,
			hasJobRetryMaxBy5:  false,
			expectedError:      true,
			expectedErrorCode:  http.StatusBadRequest,
		},
		{
			name: "payload exceeds 3KB limit",
			job: models.Job{
				Spec:       "* * * * *",
				Timezone:   "UTC",
				ExecutorId: func() *uint64 { id := uint64(1); return &id }(),
				Data:       string(make([]byte, 3073)), // 3KB + 1 byte
			},
			hasJobPayloadOf1Mb: false,
			hasJobRetryMaxBy5:  false,
			expectedError:      true,
			expectedErrorCode:  http.StatusBadRequest,
		},
		{
			name: "payload exceeds 1MB limit with feature",
			job: models.Job{
				Spec:       "* * * * *",
				Timezone:   "UTC",
				ExecutorId: func() *uint64 { id := uint64(1); return &id }(),
				Data:       string(make([]byte, 1024*1024+1)), // 1MB + 1 byte
			},
			hasJobPayloadOf1Mb: true,
			hasJobRetryMaxBy5:  false,
			expectedError:      true,
			expectedErrorCode:  http.StatusBadRequest,
		},
		{
			name: "startDate in the past",
			job: models.Job{
				Spec:       "",
				StartDate:  pastTime,
				Timezone:   "UTC",
				ExecutorId: func() *uint64 { id := uint64(1); return &id }(),
				Data:       "test data",
			},
			hasJobPayloadOf1Mb: false,
			hasJobRetryMaxBy5:  false,
			expectedError:      true,
			expectedErrorCode:  http.StatusBadRequest,
		},
		{
			name: "endDate in the past",
			job: models.Job{
				Spec:       "* * * * *",
				EndDate:    pastTime,
				Timezone:   "UTC",
				ExecutorId: func() *uint64 { id := uint64(1); return &id }(),
				Data:       "test data",
			},
			hasJobPayloadOf1Mb: false,
			hasJobRetryMaxBy5:  false,
			expectedError:      true,
			expectedErrorCode:  http.StatusBadRequest,
		},
		{
			name: "endDate before startDate",
			job: models.Job{
				Spec:       "",
				StartDate:  futureTime,
				EndDate:    pastTime,
				Timezone:   "UTC",
				ExecutorId: func() *uint64 { id := uint64(1); return &id }(),
				Data:       "test data",
			},
			hasJobPayloadOf1Mb: false,
			hasJobRetryMaxBy5:  false,
			expectedError:      true,
			expectedErrorCode:  http.StatusBadRequest,
		},
		{
			name: "retryMax exceeds 3 without feature",
			job: models.Job{
				Spec:       "* * * * *",
				Timezone:   "UTC",
				ExecutorId: func() *uint64 { id := uint64(1); return &id }(),
				Data:       "test data",
				RetryMax:   4,
			},
			hasJobPayloadOf1Mb: false,
			hasJobRetryMaxBy5:  false,
			expectedError:      true,
			expectedErrorCode:  http.StatusBadRequest,
		},
		{
			name: "retryMax exceeds 15 with feature",
			job: models.Job{
				Spec:       "* * * * *",
				Timezone:   "UTC",
				ExecutorId: func() *uint64 { id := uint64(1); return &id }(),
				Data:       "test data",
				RetryMax:   16,
			},
			hasJobPayloadOf1Mb: false,
			hasJobRetryMaxBy5:  true,
			expectedError:      true,
			expectedErrorCode:  http.StatusBadRequest,
		},
		{
			name: "invalid status",
			job: models.Job{
				Spec:       "* * * * *",
				Timezone:   "UTC",
				ExecutorId: func() *uint64 { id := uint64(1); return &id }(),
				Data:       "test data",
				Status:     "invalid",
			},
			hasJobPayloadOf1Mb: false,
			hasJobRetryMaxBy5:  false,
			expectedError:      true,
			expectedErrorCode:  http.StatusBadRequest,
		},
		{
			name: "default status to active",
			job: models.Job{
				Spec:       "* * * * *",
				Timezone:   "UTC",
				ExecutorId: func() *uint64 { id := uint64(1); return &id }(),
				Data:       "test data",
				Status:     "",
			},
			hasJobPayloadOf1Mb: false,
			hasJobRetryMaxBy5:  false,
			expectedError:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.validateJob(&tt.job, tt.hasJobPayloadOf1Mb, tt.hasJobRetryMaxBy5)

			if tt.expectedError {
				assert.NotNil(t, err)
				if tt.expectedErrorCode != 0 {
					assert.Equal(t, tt.expectedErrorCode, err.Type)
				}
				if tt.expectedErrorSubstr != "" {
					assert.Contains(t, err.Message, tt.expectedErrorSubstr)
				}
			} else {
				assert.Nil(t, err)
				if tt.job.Status == "" {
					assert.Equal(t, models.JobStatusActive, tt.job.Status)
				}
			}
		})
	}
}
