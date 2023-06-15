package service

import (
	"context"
	"fmt"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
	"scheduler0/config"
	"scheduler0/db"
	"scheduler0/fsm"
	"scheduler0/models"
	"scheduler0/repository"
	"scheduler0/shared_repo"
	"scheduler0/utils"
	"testing"
	"time"
)

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
	sqliteDb.RunMigration()
	sqliteDb.OpenConnectionToExistingDB()

	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo)

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

	callback := func(effector func(sch chan any, ech chan any), successCh chan any, errorCh chan any) {
		effector(successCh, errorCh)
	}

	dispatcher := utils.NewDispatcher(
		int64(1),
		int64(1),
		callback,
	)

	dispatcher.Run()

	queueRepo := NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, jobQueueRepo)
	// Create a new JobService instance
	service := NewJobService(ctx, logger, jobRepo, queueRepo, projectRepo, dispatcher, asyncTaskManager)

	asyncTaskManager.SingleNodeMode = true
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
	sqliteDb.RunMigration()
	sqliteDb.OpenConnectionToExistingDB()

	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo)

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

	callback := func(effector func(sch chan any, ech chan any), successCh chan any, errorCh chan any) {
		effector(successCh, errorCh)
	}

	dispatcher := utils.NewDispatcher(
		int64(1),
		int64(1),
		callback,
	)

	dispatcher.Run()

	queueRepo := NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, jobQueueRepo)
	// Create a new JobService instance
	jobService := NewJobService(ctx, logger, jobRepo, queueRepo, projectRepo, dispatcher, asyncTaskManager)

	// Create a test job
	job := models.Job{
		ID:          1,
		Spec:        "* * * * *",
		Timezone:    "UTC",
		ProjectID:   1,
		Data:        "Test data",
		CallbackUrl: "https://example.com/callback",
	}

	// Create the project using the project repo
	project := models.Project{
		ID:          job.ProjectID,
		Name:        fmt.Sprintf("Project %d", job.ProjectID),
		Description: fmt.Sprintf("Project %d description", job.ProjectID),
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

	// Update the job
	updatedJob := models.Job{
		ID:          job.ID,
		Data:        "Updated test data",
		CallbackUrl: "https://example.com/updated-callback",
	}
	_, updateErr := jobService.UpdateJob(updatedJob)
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
	sqliteDb.RunMigration()
	sqliteDb.OpenConnectionToExistingDB()

	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo)

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

	callback := func(effector func(sch chan any, ech chan any), successCh chan any, errorCh chan any) {
		effector(successCh, errorCh)
	}

	dispatcher := utils.NewDispatcher(
		int64(1),
		int64(1),
		callback,
	)

	dispatcher.Run()

	queueRepo := NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, jobQueueRepo)
	// Create a new JobService instance
	jobService := NewJobService(ctx, logger, jobRepo, queueRepo, projectRepo, dispatcher, asyncTaskManager)

	// Create a test job
	job := models.Job{
		ID:          1,
		Spec:        "* * * * *",
		Timezone:    "UTC",
		ProjectID:   1,
		Data:        "Test data",
		CallbackUrl: "https://example.com/callback",
	}

	// Create the project using the project repo
	project := models.Project{
		ID:          job.ProjectID,
		Name:        fmt.Sprintf("Project %d", job.ProjectID),
		Description: fmt.Sprintf("Project %d description", job.ProjectID),
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
	deleteErr := jobService.DeleteJob(job)
	if deleteErr != nil {
		t.Fatalf("Failed to delete job: %v", deleteErr)
	}

	// Try to retrieve the deleted job from the job repo
	getErr := jobRepo.GetOneByID(&job)

	// Assert that the getErr is not nil, indicating the job was successfully deleted
	assert.NotNil(t, getErr)
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
	sqliteDb.RunMigration()
	sqliteDb.OpenConnectionToExistingDB()

	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo)

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

	callback := func(effector func(sch chan any, ech chan any), successCh chan any, errorCh chan any) {
		effector(successCh, errorCh)
	}

	dispatcher := utils.NewDispatcher(
		int64(1),
		int64(1),
		callback,
	)

	dispatcher.Run()

	queueRepo := NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, jobQueueRepo)
	// Create a new JobService instance
	jobService := NewJobService(ctx, logger, jobRepo, queueRepo, projectRepo, dispatcher, asyncTaskManager)

	// Create a test project
	projectID := uint64(1)
	project := models.Project{
		ID:          projectID,
		Name:        "Project 1",
		Description: "Project 1 description",
	}

	// Insert the project into the project repo
	_, createErr := projectRepo.CreateOne(&project)
	if createErr != nil {
		t.Fatalf("Failed to create project: %v", createErr)
	}

	// Create test jobs associated with the project
	jobs := []models.Job{
		{
			ID:          1,
			Spec:        "* * * * *",
			Timezone:    "UTC",
			ProjectID:   projectID,
			Data:        "Test data 1",
			CallbackUrl: "https://example.com/callback1",
		},
		{
			ID:          2,
			Spec:        "0 0 * * *",
			Timezone:    "America/New_York",
			ProjectID:   projectID,
			Data:        "Test data 2",
			CallbackUrl: "https://example.com/callback2",
		},
		{
			ID:          3,
			Spec:        "0 0 * * *",
			Timezone:    "Europe/London",
			ProjectID:   projectID,
			Data:        "Test data 3",
			CallbackUrl: "https://example.com/callback3",
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
	orderBy := "ID"
	result, getErr := jobService.GetJobsByProjectID(projectID, offset, limit, orderBy)
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
	assert.Equal(t, jobs[0].CallbackUrl, result.Data[0].CallbackUrl)
	assert.Equal(t, jobs[1].ID, result.Data[1].ID)
	assert.Equal(t, jobs[1].Spec, result.Data[1].Spec)
	assert.Equal(t, jobs[1].Timezone, result.Data[1].Timezone)
	assert.Equal(t, jobs[1].ProjectID, result.Data[1].ProjectID)
	assert.Equal(t, jobs[1].Data, result.Data[1].Data)
	assert.Equal(t, jobs[1].CallbackUrl, result.Data[1].CallbackUrl)
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
	sqliteDb.RunMigration()
	sqliteDb.OpenConnectionToExistingDB()

	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo)

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

	callback := func(effector func(sch chan any, ech chan any), successCh chan any, errorCh chan any) {
		effector(successCh, errorCh)
	}

	dispatcher := utils.NewDispatcher(
		int64(1),
		int64(1),
		callback,
	)

	dispatcher.Run()

	queueRepo := NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, jobQueueRepo)
	// Create a new JobService instance
	jobService := NewJobService(ctx, logger, jobRepo, queueRepo, projectRepo, dispatcher, asyncTaskManager)

	// Create a test job
	job := models.Job{
		ID:          1,
		Spec:        "* * * * *",
		Timezone:    "UTC",
		ProjectID:   1,
		Data:        "Test data",
		CallbackUrl: "https://example.com/callback",
	}

	// Create the project using the project repo
	project := models.Project{
		ID:          job.ProjectID,
		Name:        fmt.Sprintf("Project %d", job.ProjectID),
		Description: fmt.Sprintf("Project %d description", job.ProjectID),
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
	retrievedJob, getErr := jobService.GetJob(job)
	if getErr != nil {
		t.Fatalf("Failed to get job: %v", getErr)
	}

	// Assert the correctness of the retrieved job
	assert.Equal(t, job.ID, retrievedJob.ID)
	assert.Equal(t, job.Spec, retrievedJob.Spec)
	assert.Equal(t, job.Timezone, retrievedJob.Timezone)
	assert.Equal(t, job.ProjectID, retrievedJob.ProjectID)
	assert.Equal(t, job.Data, retrievedJob.Data)
	assert.Equal(t, job.CallbackUrl, retrievedJob.CallbackUrl)
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
	sqliteDb.RunMigration()
	sqliteDb.OpenConnectionToExistingDB()

	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo)

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

	callback := func(effector func(sch chan any, ech chan any), successCh chan any, errorCh chan any) {
		effector(successCh, errorCh)
	}

	dispatcher := utils.NewDispatcher(
		int64(1),
		int64(1),
		callback,
	)

	dispatcher.Run()

	queueRepo := NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, jobQueueRepo)
	// Create a new JobService instance
	jobService := NewJobService(ctx, logger, jobRepo, queueRepo, projectRepo, dispatcher, asyncTaskManager)

	queueRepo.AddServers([]uint64{1})

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

	// Call the QueueJobs method of the job service
	jobService.QueueJobs(jobs)

	assert.Equal(t, queueRepo.allocations[1], uint64(1))
}
