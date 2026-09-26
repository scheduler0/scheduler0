package job_test

import (
	"context"
	"net/http"
	"os"
	"scheduler0/pkg/config"
	"scheduler0/pkg/db"
	"scheduler0/pkg/fsm"
	"scheduler0/pkg/models"
	account_repo "scheduler0/pkg/repository/account"
	executor_repo "scheduler0/pkg/repository/executor"
	job_repo "scheduler0/pkg/repository/job"
	project_repo "scheduler0/pkg/repository/project"
	"scheduler0/pkg/shared_repo"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_JobRepo_BatchInsertJobs(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "job-repo-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
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

	// Create an account first (required for foreign key constraint)
	accountRepo := account_repo.NewAccountRepository(context.TODO(), logger, scheduler0RaftActions, scheduler0Store)
	account := &models.Account{
		ID:   1,
		Name: "Test Account",
	}
	_, createAccountErr := accountRepo.CreateAccount(account)
	if createAccountErr != nil {
		t.Fatalf("Failed to create account: %v", createAccountErr)
	}

	// Create a new project
	mockProject := models.Project{
		Name:        "Test Project",
		Description: "Test project description",
		AccountId:   1,
	}

	// Create a new JobRepo instance
	jobRepo := job_repo.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)

	// Create a new ProjectRepo instance
	projectRepo := project_repo.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)

	// Call the CreateOne method of the projectRepo
	projectID, createProjectErr := projectRepo.CreateOne(&mockProject)
	if createProjectErr != nil {
		t.Fatal("failed to create project:", createProjectErr)
	}

	// Create a batch of jobs using the project's ID
	jobs := []models.Job{
		{
			ProjectID:      projectID,
			Spec:           "0 * * * *",
			RetryMax:       3,
			Status:         "active",
			DateCreated:    time.Now(),
			Timezone:       "UTC",
			TimezoneOffset: 0,
			Data:           "some data",
			AccountId:      1,
			CreatedBy:      "test",
		},
		{
			ProjectID:      projectID,
			Spec:           "0 12 * * *",
			RetryMax:       3,
			Status:         "active",
			DateCreated:    time.Now(),
			Timezone:       "UTC",
			TimezoneOffset: 0,
			Data:           "some data",
			AccountId:      1,
			CreatedBy:      "test",
		},
	}

	// Call the BatchInsertJobs method
	jobIDs, batchInsertErr := jobRepo.BatchInsertJobs(jobs)
	if batchInsertErr != nil {
		t.Fatal("failed to insert jobs:", batchInsertErr)
	}

	// Assert the returned job IDs
	expectedJobIDs := []uint64{1, 2}
	assert.Equal(t, expectedJobIDs, jobIDs)
}

func Test_JobRepo_UpdateOneByID(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "job-repo-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
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

	// Create a new JobRepo instance
	jobRepo := job_repo.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)

	// Create a project to associate with the jobs
	project := models.Project{
		Name:        "Test Project",
		Description: "Test project description",
		AccountId:   1,
	}

	// Create a new ProjectRepo instance
	projectRepo := project_repo.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)

	// Call the CreateOne method of the projectRepo to create a project
	projectID, createProjectErr := projectRepo.CreateOne(&project)
	if createProjectErr != nil {
		t.Fatal("failed to create project:", createProjectErr)
	}

	// Create jobs to update
	jobs := []models.Job{
		{
			ID:             1,
			ProjectID:      projectID,
			Spec:           "0 * * * *",
			RetryMax:       3,
			Status:         "active",
			DateCreated:    time.Now(),
			Timezone:       "UTC",
			TimezoneOffset: 0,
			Data:           "some data",
			AccountId:      1,
			CreatedBy:      "test",
		},
		{
			ID:             2,
			ProjectID:      projectID,
			Spec:           "0 12 * * *",
			RetryMax:       3,
			Status:         "active",
			DateCreated:    time.Now(),
			Timezone:       "UTC",
			TimezoneOffset: 0,
			Data:           "some data",
			AccountId:      1,
			CreatedBy:      "test",
		},
	}

	// Call the BatchInsertJobs method to insert the jobs
	_, batchInsertErr := jobRepo.BatchInsertJobs(jobs)
	if batchInsertErr != nil {
		t.Fatal("failed to insert jobs:", batchInsertErr)
	}

	// Modify some properties of the first job
	jobs[0].Data = "updated data"

	// Call the UpdateOneByID method to update the first job
	updatedCount, updateErr := jobRepo.UpdateOneByID(jobs[0])
	if updateErr != nil {
		t.Fatal("failed to update job:", updateErr)
	}

	// Assert the updated count is 1
	assert.Equal(t, uint64(1), updatedCount)

	// Retrieve the updated job from the database
	updatedJob := models.Job{
		ID: 1,
	}
	getErr := jobRepo.GetOneByID(&updatedJob)
	if getErr != nil {
		t.Fatal("failed to get updated job:", getErr)
	}

	// Assert the updated properties
	assert.Equal(t, jobs[0].Spec, updatedJob.Spec)
	assert.Equal(t, jobs[0].Data, updatedJob.Data)
}

func Test_JobRepo_DeleteOneByID(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "job-repo-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
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

	// Create a new JobRepo instance
	jobRepo := job_repo.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)

	// Create a project to associate with the job
	project := models.Project{
		Name:        "Test Project",
		Description: "Test project description",
		AccountId:   1,
	}

	// Create a new ProjectRepo instance
	projectRepo := project_repo.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)

	// Call the CreateOne method of the projectRepo to create a project
	projectID, createProjectErr := projectRepo.CreateOne(&project)
	if createProjectErr != nil {
		t.Fatal("failed to create project:", createProjectErr)
	}

	// Create a job to delete
	job := models.Job{
		ID:             1,
		ProjectID:      projectID,
		Spec:           "0 * * * *",
		DateCreated:    time.Now(),
		Timezone:       "UTC",
		TimezoneOffset: 0,
		Data:           "some data",
		AccountId:      1,
		CreatedBy:      "test",
		RetryMax:       3,
		Status:         "active",
	}

	// Call the BatchInsertJobs method to insert the job
	_, batchInsertErr := jobRepo.BatchInsertJobs([]models.Job{job})
	if batchInsertErr != nil {
		t.Fatal("failed to insert job:", batchInsertErr)
	}

	// Call the DeleteOneByID method to delete the job
	deletedCount, deleteErr := jobRepo.DeleteOneByID(job)
	if deleteErr != nil {
		t.Fatal("failed to delete job:", deleteErr)
	}

	// Assert the deleted count is 1
	assert.Equal(t, uint64(1), deletedCount)

	// Try to retrieve the deleted job from the database (soft delete, so job still exists)
	deletedJob := models.Job{
		ID: 1,
	}
	getErr := jobRepo.GetOneByID(&deletedJob)
	assert.Nil(t, getErr)
	// Verify it's marked as deleted (soft delete)
	assert.NotNil(t, deletedJob.DeletedBy)
	assert.Equal(t, "system", *deletedJob.DeletedBy)
}

func Test_JobRepo_BatchGetJobsByID(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "job-repo-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
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

	// Create a new JobRepo instance
	jobRepo := job_repo.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)

	// Create a project to associate with the jobs
	project := models.Project{
		Name:        "Test Project",
		Description: "Test project description",
		AccountId:   1,
	}

	// Create a new ProjectRepo instance
	projectRepo := project_repo.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)

	// Call the CreateOne method of the projectRepo to create a project
	projectID, createProjectErr := projectRepo.CreateOne(&project)
	if createProjectErr != nil {
		t.Fatal("failed to create project:", createProjectErr)
	}

	// Create jobs to retrieve
	jobs := []models.Job{
		{
			ID:             1,
			ProjectID:      projectID,
			Spec:           "0 * * * *",
			RetryMax:       3,
			Status:         "active",
			DateCreated:    time.Now(),
			Timezone:       "UTC",
			TimezoneOffset: 0,
			Data:           "some data",
			AccountId:      1,
			CreatedBy:      "test",
		},
		{
			ID:             2,
			ProjectID:      projectID,
			Spec:           "0 12 * * *",
			RetryMax:       3,
			Status:         "active",
			DateCreated:    time.Now(),
			Timezone:       "UTC",
			TimezoneOffset: 0,
			Data:           "some data",
			AccountId:      1,
			CreatedBy:      "test",
		},
	}

	// Call the BatchInsertJobs method to insert the jobs
	_, batchInsertErr := jobRepo.BatchInsertJobs(jobs)
	if batchInsertErr != nil {
		t.Fatal("failed to insert jobs:", batchInsertErr)
	}

	// Call the BatchGetJobsByID method to retrieve the jobs
	retrievedJobs, getErr := jobRepo.BatchGetJobsByID([]uint64{jobs[0].ID, jobs[1].ID})
	if getErr != nil {
		t.Fatal("failed to retrieve jobs:", getErr)
	}

	// Assert the retrieved jobs count
	assert.Equal(t, len(jobs), len(retrievedJobs))

	// Create a map to compare the retrieved jobs based on their IDs
	retrievedJobsMap := make(map[uint64]models.Job)
	for _, job := range retrievedJobs {
		retrievedJobsMap[job.ID] = job
	}

	// Assert the properties of the retrieved jobs
	for _, job := range jobs {
		retrievedJob, exists := retrievedJobsMap[job.ID]
		assert.True(t, exists, "job with ID %d not found", job.ID)

		assert.Equal(t, job.ProjectID, retrievedJob.ProjectID)
		assert.Equal(t, job.Spec, retrievedJob.Spec)
		assert.Equal(t, job.Timezone, retrievedJob.Timezone)
		assert.Equal(t, job.TimezoneOffset, retrievedJob.TimezoneOffset)
		assert.Equal(t, job.Data, retrievedJob.Data)
		assert.Equal(t, job.AccountId, retrievedJob.AccountId)
		assert.Equal(t, job.CreatedBy, retrievedJob.CreatedBy)
	}
}

func Test_JobRepo_BatchGetJobsWithIDRange(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "job-repo-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
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

	// Create a new JobRepo instance
	jobRepo := job_repo.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)

	// Create a project to associate with the jobs
	project := models.Project{
		Name:        "Test Project",
		Description: "Test project description",
		AccountId:   1,
	}

	// Create a new ProjectRepo instance
	projectRepo := project_repo.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)

	// Call the CreateOne method of the projectRepo to create a project
	projectID, createProjectErr := projectRepo.CreateOne(&project)
	if createProjectErr != nil {
		t.Fatal("failed to create project:", createProjectErr)
	}

	// Create jobs to retrieve
	jobs := []models.Job{
		{
			ID:             1,
			ProjectID:      projectID,
			Spec:           "0 * * * *",
			RetryMax:       3,
			Status:         "active",
			DateCreated:    time.Now(),
			Timezone:       "UTC",
			TimezoneOffset: 0,
			Data:           "some data",
			AccountId:      1,
			CreatedBy:      "test",
		},
		{
			ID:             2,
			ProjectID:      projectID,
			Spec:           "0 12 * * *",
			RetryMax:       3,
			Status:         "active",
			DateCreated:    time.Now(),
			Timezone:       "UTC",
			TimezoneOffset: 0,
			Data:           "some data",
			AccountId:      1,
			CreatedBy:      "test",
		},
		{
			ID:             3,
			ProjectID:      projectID,
			Spec:           "0 */2 * * *",
			RetryMax:       3,
			Status:         "active",
			DateCreated:    time.Now(),
			Timezone:       "UTC",
			TimezoneOffset: 0,
			Data:           "some data",
			AccountId:      1,
			CreatedBy:      "test",
		},
	}

	// Call the BatchInsertJobs method to insert the jobs
	_, batchInsertErr := jobRepo.BatchInsertJobs(jobs)
	if batchInsertErr != nil {
		t.Fatal("failed to insert jobs:", batchInsertErr)
	}

	// Call the BatchGetJobsWithIDRange method to retrieve the jobs with ID range [2, 3]
	retrievedJobs, getErr := jobRepo.BatchGetJobsWithIDRange(2, 3)
	if getErr != nil {
		t.Fatal("failed to retrieve jobs:", getErr)
	}

	// Assert the retrieved jobs count
	assert.Equal(t, 2, len(retrievedJobs))

	// Create a map to compare the retrieved jobs based on their IDs
	retrievedJobsMap := make(map[uint64]models.Job)
	for _, job := range retrievedJobs {
		retrievedJobsMap[job.ID] = job
	}

	// Assert the properties of the retrieved jobs
	for _, job := range jobs[1:] {
		retrievedJob, exists := retrievedJobsMap[job.ID]
		assert.True(t, exists, "job with ID %d not found", job.ID)

		assert.Equal(t, job.ProjectID, retrievedJob.ProjectID)
		assert.Equal(t, job.Spec, retrievedJob.Spec)
		assert.Equal(t, job.Timezone, retrievedJob.Timezone)
		assert.Equal(t, job.TimezoneOffset, retrievedJob.TimezoneOffset)
		assert.Equal(t, job.Data, retrievedJob.Data)
		assert.Equal(t, job.AccountId, retrievedJob.AccountId)
		assert.Equal(t, job.CreatedBy, retrievedJob.CreatedBy)
	}
}

func Test_JobRepo_GetAllByProjectID(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "job-repo-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
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

	// Create a new JobRepo instance
	jobRepo := job_repo.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)

	// Create projects to associate with the jobs
	project1 := models.Project{
		Name:        "Test Project 1",
		Description: "Test project 1 description",
		AccountId:   1,
	}
	project2 := models.Project{
		Name:        "Test Project 2",
		Description: "Test project 2 description",
		AccountId:   1,
	}

	// Create a new ProjectRepo instance
	projectRepo := project_repo.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)

	// Call the CreateOne method of the projectRepo to create projects
	project1ID, createProjectErr := projectRepo.CreateOne(&project1)
	if createProjectErr != nil {
		t.Fatal("failed to create project 1:", createProjectErr)
	}
	project2ID, createProjectErr := projectRepo.CreateOne(&project2)
	if createProjectErr != nil {
		t.Fatal("failed to create project 2:", createProjectErr)
	}

	// Create jobs to associate with the projects
	jobs := []models.Job{
		{
			ID:             1,
			ProjectID:      project1ID,
			Spec:           "0 * * * *",
			RetryMax:       3,
			Status:         "active",
			DateCreated:    time.Now(),
			Timezone:       "UTC",
			TimezoneOffset: 0,
			Data:           "some data",
			AccountId:      1,
			CreatedBy:      "test",
		},
		{
			ID:             2,
			ProjectID:      project1ID,
			Spec:           "0 12 * * *",
			RetryMax:       3,
			Status:         "active",
			DateCreated:    time.Now(),
			Timezone:       "UTC",
			TimezoneOffset: 0,
			Data:           "some data",
			AccountId:      1,
			CreatedBy:      "test",
		},
		{
			ID:             3,
			ProjectID:      project2ID,
			Spec:           "0 */2 * * *",
			RetryMax:       3,
			Status:         "active",
			DateCreated:    time.Now(),
			Timezone:       "UTC",
			TimezoneOffset: 0,
			Data:           "some data",
			AccountId:      1,
			CreatedBy:      "test",
		},
	}

	// Call the BatchInsertJobs method to insert the jobs
	_, batchInsertErr := jobRepo.BatchInsertJobs(jobs)
	if batchInsertErr != nil {
		t.Fatal("failed to insert jobs:", batchInsertErr)
	}

	// Call the GetAllByProjectID method to retrieve the jobs associated with project1
	retrievedJobs, getErr := jobRepo.GetAllByProjectID(project1ID, 0, 10, "id", "ASC")
	if getErr != nil {
		t.Fatal("failed to retrieve jobs for project 1:", getErr)
	}

	// Assert the retrieved jobs count for project1
	assert.Equal(t, 2, len(retrievedJobs))

	// Create a map to compare the retrieved jobs based on their IDs
	retrievedJobsMap := make(map[uint64]models.Job)
	for _, job := range retrievedJobs {
		retrievedJobsMap[job.ID] = job
	}

	// Assert the properties of the retrieved jobs for project1
	for _, job := range jobs[:2] {
		retrievedJob, exists := retrievedJobsMap[job.ID]
		assert.True(t, exists, "job with ID %d not found for project 1", job.ID)

		assert.Equal(t, job.ProjectID, retrievedJob.ProjectID)
		assert.Equal(t, job.Spec, retrievedJob.Spec)
		assert.Equal(t, job.Timezone, retrievedJob.Timezone)
		assert.Equal(t, job.TimezoneOffset, retrievedJob.TimezoneOffset)
		assert.Equal(t, job.Data, retrievedJob.Data)
		assert.Equal(t, job.AccountId, retrievedJob.AccountId)
		assert.Equal(t, job.CreatedBy, retrievedJob.CreatedBy)
	}

	// Call the GetAllByProjectID method to retrieve the jobs associated with project2
	retrievedJobs, getErr = jobRepo.GetAllByProjectID(project2ID, 0, 10, "id", "ASC")
	if getErr != nil {
		t.Fatal("failed to retrieve jobs for project 2:", getErr)
	}

	// Assert the retrieved jobs count for project2
	assert.Equal(t, 1, len(retrievedJobs))

	retrievedJobsMap = make(map[uint64]models.Job)
	for _, job := range retrievedJobs {
		retrievedJobsMap[job.ID] = job
	}

	// Assert the properties of the retrieved job for project2
	retrievedJob, exists := retrievedJobsMap[jobs[2].ID]
	assert.True(t, exists, "job with ID %d not found for project 2", jobs[2].ID)

	assert.Equal(t, jobs[2].ProjectID, retrievedJob.ProjectID)
	assert.Equal(t, jobs[2].Spec, retrievedJob.Spec)
	assert.Equal(t, jobs[2].Timezone, retrievedJob.Timezone)
	assert.Equal(t, jobs[2].TimezoneOffset, retrievedJob.TimezoneOffset)
	assert.Equal(t, jobs[2].Data, retrievedJob.Data)
	assert.Equal(t, jobs[2].AccountId, retrievedJob.AccountId)
	assert.Equal(t, jobs[2].CreatedBy, retrievedJob.CreatedBy)

	// Test invalid order by column
	_, invalidColumnErr := jobRepo.GetAllByProjectID(project1ID, 0, 10, "invalid_column", "ASC")
	assert.NotNil(t, invalidColumnErr)
	assert.Contains(t, invalidColumnErr.Message, "invalid order by column")

	// Test invalid order by direction
	_, invalidDirectionErr := jobRepo.GetAllByProjectID(project1ID, 0, 10, "id", "INVALID")
	assert.NotNil(t, invalidDirectionErr)
	assert.Contains(t, invalidDirectionErr.Message, "invalid order by direction")
}

func Test_JobRepo_GetJobsTotalCountByProjectID(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "job-repo-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
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

	// Create a new JobRepo instance
	jobRepo := job_repo.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)

	// Create an account first (required for foreign key constraint)
	accountRepo := account_repo.NewAccountRepository(context.TODO(), logger, scheduler0RaftActions, scheduler0Store)
	account := &models.Account{
		ID:   1,
		Name: "Test Account",
	}
	_, createAccountErr := accountRepo.CreateAccount(account)
	if createAccountErr != nil {
		t.Fatalf("Failed to create account: %v", createAccountErr)
	}

	// Create a project to associate with the jobs
	project := models.Project{
		Name:        "Test Project",
		Description: "Test project description",
		AccountId:   1,
	}

	// Create a new ProjectRepo instance
	projectRepo := project_repo.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)

	// Call the CreateOne method of the projectRepo to create a project
	projectID, createProjectErr := projectRepo.CreateOne(&project)
	if createProjectErr != nil {
		t.Fatal("failed to create project:", createProjectErr)
	}

	// Create jobs to associate with the project
	jobs := []models.Job{
		{
			ID:             1,
			ProjectID:      projectID,
			Spec:           "0 * * * *",
			RetryMax:       3,
			Status:         "active",
			DateCreated:    time.Now(),
			Timezone:       "UTC",
			TimezoneOffset: 0,
			Data:           "some data",
			AccountId:      1,
			CreatedBy:      "test",
		},
		{
			ID:             2,
			ProjectID:      projectID,
			Spec:           "0 12 * * *",
			RetryMax:       3,
			Status:         "active",
			DateCreated:    time.Now(),
			Timezone:       "UTC",
			TimezoneOffset: 0,
			Data:           "some data",
			AccountId:      1,
			CreatedBy:      "test",
		},
	}

	// Call the BatchInsertJobs method to insert the jobs
	_, batchInsertErr := jobRepo.BatchInsertJobs(jobs)
	if batchInsertErr != nil {
		t.Fatal("failed to insert jobs:", batchInsertErr)
	}

	// Call the GetJobsTotalCountByProjectID method
	count, counterr := jobRepo.GetJobsTotalCountByProjectID(projectID)
	if counterr != nil {
		t.Fatal("failed to get jobs total count by project ID:", counterr)
	}

	// Assert the total count is 2
	assert.Equal(t, uint64(2), count)
}

func Test_JobRepo_GetJobsPaginated(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "job-repo-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
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

	// Create a new JobRepo instance
	jobRepo := job_repo.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)

	// Create a project to associate with the jobs
	project := models.Project{
		Name:        "Test Project",
		Description: "Test project description",
		AccountId:   1,
	}

	// Create a new ProjectRepo instance
	projectRepo := project_repo.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)

	// Call the CreateOne method of the projectRepo to create a project
	projectID, createProjectErr := projectRepo.CreateOne(&project)
	if createProjectErr != nil {
		t.Fatal("failed to create project:", createProjectErr)
	}

	// Create jobs to associate with the project
	jobs := []models.Job{
		{
			ID:             1,
			ProjectID:      projectID,
			Spec:           "0 * * * *",
			RetryMax:       3,
			Status:         "active",
			DateCreated:    time.Now(),
			Timezone:       "UTC",
			TimezoneOffset: 0,
			Data:           "some data",
			AccountId:      1,
			CreatedBy:      "test",
		},
		{
			ID:             2,
			ProjectID:      projectID,
			Spec:           "0 12 * * *",
			RetryMax:       3,
			Status:         "active",
			DateCreated:    time.Now(),
			Timezone:       "UTC",
			TimezoneOffset: 0,
			Data:           "some data",
			AccountId:      1,
			CreatedBy:      "test",
		},
		{
			ID:             3,
			ProjectID:      projectID,
			Spec:           "0 * * * *",
			RetryMax:       3,
			Status:         "active",
			DateCreated:    time.Now(),
			Timezone:       "UTC",
			TimezoneOffset: 0,
			Data:           "some data",
			AccountId:      1,
			CreatedBy:      "test",
		},
		{
			ID:             4,
			ProjectID:      projectID,
			Spec:           "0 12 * * *",
			RetryMax:       3,
			Status:         "active",
			DateCreated:    time.Now(),
			Timezone:       "UTC",
			TimezoneOffset: 0,
			Data:           "some data",
			AccountId:      1,
			CreatedBy:      "test",
		},
	}

	// Call the BatchInsertJobs method to insert the jobs
	_, batchInsertErr := jobRepo.BatchInsertJobs(jobs)
	if batchInsertErr != nil {
		t.Fatal("failed to insert jobs:", batchInsertErr)
	}

	// Call the GetJobsPaginated method with offset 0 and limit 2
	paginatedJobs, _, geterr := jobRepo.GetJobsPaginated(1, projectID, 0, 2, "id", "ASC")
	if geterr != nil {
		t.Fatal("failed to get paginated jobs:", geterr)
	}

	// Assert the length of the returned paginated jobs
	assert.Equal(t, 2, len(paginatedJobs))

	// Assert the job IDs and associated project ID
	assert.Equal(t, projectID, paginatedJobs[0].ProjectID)
	assert.Equal(t, projectID, paginatedJobs[1].ProjectID)
	assert.Equal(t, uint64(1), paginatedJobs[0].ID)
	assert.Equal(t, uint64(2), paginatedJobs[1].ID)
	assert.Equal(t, paginatedJobs[0].AccountId, uint64(1))
	assert.Equal(t, paginatedJobs[1].AccountId, uint64(1))
	assert.Equal(t, paginatedJobs[0].CreatedBy, "test")
	assert.Equal(t, paginatedJobs[1].CreatedBy, "test")

	// Test with different ordering
	paginatedJobs, _, geterr = jobRepo.GetJobsPaginated(1, projectID, 0, 2, "id", "DESC")
	if geterr != nil {
		t.Fatal("failed to get paginated jobs with DESC ordering:", geterr)
	}

	// Assert the length of the returned paginated jobs
	assert.Equal(t, 2, len(paginatedJobs))

	// Assert the job IDs are in descending order
	assert.Equal(t, uint64(4), paginatedJobs[0].ID)
	assert.Equal(t, uint64(3), paginatedJobs[1].ID)

	// Test invalid order by column
	_, _, invalidColumnErr := jobRepo.GetJobsPaginated(1, projectID, 0, 2, "invalid_column", "ASC")
	assert.NotNil(t, invalidColumnErr)
	assert.Contains(t, invalidColumnErr.Message, "invalid order by column")

	// Test invalid order by direction
	_, _, invalidDirectionErr := jobRepo.GetJobsPaginated(1, projectID, 0, 2, "id", "INVALID")
	assert.NotNil(t, invalidDirectionErr)
	assert.Contains(t, invalidDirectionErr.Message, "invalid order by direction")
}

// ========== Edge Case Tests ==========

func TestBatchGetJobsByID_EdgeCase_EmptyList(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "job-repo-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
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
		Conf:           raft.DefaultConfig(),
		ConfigStoreFSM: false,
		MakeFSMFunc: func() raft.FSM {
			return scheduler0Store.GetFSM()
		},
	})
	defer cluster.Close()
	cluster.FullyConnect()
	scheduler0Store.UpdateRaft(cluster.Leader())

	jobRepo := job_repo.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)

	// Batch get with empty list
	jobs, batchErr := jobRepo.BatchGetJobsByID([]uint64{})

	assert.Nil(t, batchErr)
	assert.Empty(t, jobs, "Should return empty list for empty input")
}

func TestBatchGetJobsByID_EdgeCase_SomeNonExistent(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "job-repo-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
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
		Conf:           raft.DefaultConfig(),
		ConfigStoreFSM: false,
		MakeFSMFunc: func() raft.FSM {
			return scheduler0Store.GetFSM()
		},
	})
	defer cluster.Close()
	cluster.FullyConnect()
	scheduler0Store.UpdateRaft(cluster.Leader())

	accountRepo := account_repo.NewAccountRepository(context.TODO(), logger, scheduler0RaftActions, scheduler0Store)
	account := &models.Account{Name: "Test Account"}
	_, createAccountErr := accountRepo.CreateAccount(account)
	if createAccountErr != nil {
		t.Fatalf("Failed to create account: %v", createAccountErr)
	}

	projectRepo := project_repo.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, nil)
	project := &models.Project{
		Name:        "Test Project",
		Description: "Test Description",
		AccountId:   account.ID,
	}
	projectID, createProjectErr := projectRepo.CreateOne(project)
	if createProjectErr != nil {
		t.Fatalf("Failed to create project: %v", createProjectErr)
	}

	jobRepo := job_repo.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)

	job1 := models.Job{
		ProjectID:      projectID,
		Spec:           "0 * * * *",
		DateCreated:    time.Now(),
		Timezone:       "UTC",
		TimezoneOffset: 0,
		Data:           "some data",
		AccountId:      account.ID,
		CreatedBy:      "test",
		RetryMax:       3,
		Status:         "active",
	}
	jobIDs, batchInsertErr := jobRepo.BatchInsertJobs([]models.Job{job1})
	if batchInsertErr != nil {
		t.Fatalf("Failed to insert job: %v", batchInsertErr)
	}

	// Batch get with mix of existing and non-existent IDs
	jobs, batchErr := jobRepo.BatchGetJobsByID([]uint64{jobIDs[0], 99999, 88888})

	assert.Nil(t, batchErr)
	assert.Len(t, jobs, 1, "Should return only existing jobs")
	assert.Equal(t, jobIDs[0], jobs[0].ID)
}

func TestBatchGetJobsByID_EdgeCase_AllNonExistent(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "job-repo-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
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
		Conf:           raft.DefaultConfig(),
		ConfigStoreFSM: false,
		MakeFSMFunc: func() raft.FSM {
			return scheduler0Store.GetFSM()
		},
	})
	defer cluster.Close()
	cluster.FullyConnect()
	scheduler0Store.UpdateRaft(cluster.Leader())

	jobRepo := job_repo.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)

	// Batch get with all non-existent IDs
	jobs, batchErr := jobRepo.BatchGetJobsByID([]uint64{99999, 88888, 77777})

	assert.Nil(t, batchErr)
	assert.Empty(t, jobs, "Should return empty list when no jobs exist")
}

func TestGetOneByID_EdgeCase_JobIDZero(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "job-repo-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
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
		Conf:           raft.DefaultConfig(),
		ConfigStoreFSM: false,
		MakeFSMFunc: func() raft.FSM {
			return scheduler0Store.GetFSM()
		},
	})
	defer cluster.Close()
	cluster.FullyConnect()
	scheduler0Store.UpdateRaft(cluster.Leader())

	jobRepo := job_repo.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)

	job := &models.Job{
		ID: 0, // Zero ID
	}

	getErr := jobRepo.GetOneByID(job)

	assert.NotNil(t, getErr)
	assert.Equal(t, http.StatusNotFound, getErr.Type)
	assert.Contains(t, getErr.Message, "job cannot be found")
}

func TestUpdateOneByID_EdgeCase_NonExistentJob(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "job-repo-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
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
		Conf:           raft.DefaultConfig(),
		ConfigStoreFSM: false,
		MakeFSMFunc: func() raft.FSM {
			return scheduler0Store.GetFSM()
		},
	})
	defer cluster.Close()
	cluster.FullyConnect()
	scheduler0Store.UpdateRaft(cluster.Leader())

	accountRepo := account_repo.NewAccountRepository(context.TODO(), logger, scheduler0RaftActions, scheduler0Store)
	account := &models.Account{Name: "Test Account"}
	_, createAccountErr := accountRepo.CreateAccount(account)
	if createAccountErr != nil {
		t.Fatalf("Failed to create account: %v", createAccountErr)
	}

	jobRepo := job_repo.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)

	// Try to update non-existent job
	job := models.Job{
		ID:        99999,
		AccountId: account.ID,
		Status:    "paused",
	}

	_, updateErr := jobRepo.UpdateOneByID(job)

	assert.NotNil(t, updateErr)
	assert.Equal(t, http.StatusNotFound, updateErr.Type)
	assert.Contains(t, updateErr.Message, "job cannot be found")
}

func TestDeleteOneByID_EdgeCase_NonExistentJob(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "job-repo-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
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
		Conf:           raft.DefaultConfig(),
		ConfigStoreFSM: false,
		MakeFSMFunc: func() raft.FSM {
			return scheduler0Store.GetFSM()
		},
	})
	defer cluster.Close()
	cluster.FullyConnect()
	scheduler0Store.UpdateRaft(cluster.Leader())

	accountRepo := account_repo.NewAccountRepository(context.TODO(), logger, scheduler0RaftActions, scheduler0Store)
	account := &models.Account{Name: "Test Account"}
	_, createAccountErr := accountRepo.CreateAccount(account)
	if createAccountErr != nil {
		t.Fatalf("Failed to create account: %v", createAccountErr)
	}

	jobRepo := job_repo.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)

	// Try to delete non-existent job
	job := models.Job{
		ID:        99999,
		AccountId: account.ID,
		DeletedBy: new(string),
	}
	*job.DeletedBy = "test"

	count, deleteErr := jobRepo.DeleteOneByID(job)

	// DeleteOneByID doesn't check if job exists, it just deletes
	// If no rows are affected, count will be 0
	assert.Nil(t, deleteErr)
	assert.Equal(t, uint64(0), count, "Should return 0 rows affected for non-existent job")
}

func TestGetJobsPaginated_EdgeCase_ZeroLimit(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "job-repo-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
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
		Conf:           raft.DefaultConfig(),
		ConfigStoreFSM: false,
		MakeFSMFunc: func() raft.FSM {
			return scheduler0Store.GetFSM()
		},
	})
	defer cluster.Close()
	cluster.FullyConnect()
	scheduler0Store.UpdateRaft(cluster.Leader())

	accountRepo := account_repo.NewAccountRepository(context.TODO(), logger, scheduler0RaftActions, scheduler0Store)
	account := &models.Account{Name: "Test Account"}
	_, createAccountErr := accountRepo.CreateAccount(account)
	if createAccountErr != nil {
		t.Fatalf("Failed to create account: %v", createAccountErr)
	}

	jobRepo := job_repo.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)

	// Get jobs with zero limit
	jobs, count, paginateErr := jobRepo.GetJobsPaginated(account.ID, 0, 0, 0, "id", "ASC")

	assert.Nil(t, paginateErr)
	assert.Empty(t, jobs, "Should return empty list with zero limit")
	assert.Equal(t, uint64(0), count)
}

func TestBatchInsertJobs_EdgeCase_EmptyList(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "job-repo-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
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
		Conf:           raft.DefaultConfig(),
		ConfigStoreFSM: false,
		MakeFSMFunc: func() raft.FSM {
			return scheduler0Store.GetFSM()
		},
	})
	defer cluster.Close()
	cluster.FullyConnect()
	scheduler0Store.UpdateRaft(cluster.Leader())

	jobRepo := job_repo.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)

	// Batch insert with empty list
	jobIDs, batchErr := jobRepo.BatchInsertJobs([]models.Job{})

	// BatchInsertJobs should handle empty list gracefully
	assert.Nil(t, batchErr, "Should not error on empty list")
	assert.Empty(t, jobIDs, "Should return empty list for empty input")
}

// Test_JobRepo_GetActiveJobsByExecutorID_MultipleJobsOnLocalExecutor verifies that
// several jobs can share one local executor and all are returned by the pull query
// used by GET /api/v1/local-executors/{id}/jobs.
func Test_JobRepo_GetActiveJobsByExecutorID_MultipleJobsOnLocalExecutor(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "job-repo-test",
		Level: hclog.LevelFromString("ERROR"),
	})
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)
	tempFile, err := os.CreateTemp("", "test-db")
	require.NoError(t, err)
	tempFile.Close()
	defer os.Remove(tempFile.Name())

	sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.RunMigration(logger)
	sqliteDb.OpenConnectionToExistingDB()
	scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, scheduler0config, sqliteDb, nil, nil, nil, nil, sharedRepo)

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

	accountRepo := account_repo.NewAccountRepository(context.TODO(), logger, scheduler0RaftActions, scheduler0Store)
	_, createAccountErr := accountRepo.CreateAccount(&models.Account{ID: 1, Name: "Test Account"})
	require.Nil(t, createAccountErr)

	jobRepo := job_repo.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)
	projectRepo := project_repo.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)
	executorRepo := executor_repo.NewExecutorRepo(logger, scheduler0RaftActions, scheduler0Store, nil)

	projectID, createProjectErr := projectRepo.CreateOne(&models.Project{
		Name:        "multi-job-local",
		Description: "shared local executor",
		AccountId:   1,
	})
	require.Nil(t, createProjectErr)

	localExecutorID, createExecutorErr := executorRepo.CreateOne(models.JobExecutor{
		Name:       "local-multi",
		Type:       string(models.ExecutorTypeLocal),
		Command:    "echo hello",
		WorkingDir: "/tmp",
		AccountId:  1,
		CreatedBy:  "tester",
	})
	require.Nil(t, createExecutorErr)

	otherExecutorID, createOtherErr := executorRepo.CreateOne(models.JobExecutor{
		Name:       "local-other",
		Type:       string(models.ExecutorTypeLocal),
		Command:    "echo other",
		WorkingDir: "/tmp",
		AccountId:  1,
		CreatedBy:  "tester",
	})
	require.Nil(t, createOtherErr)

	jobs := []models.Job{
		{ProjectID: projectID, Spec: "@every 1m", Timezone: "UTC", Data: `{"job":"alpha"}`, AccountId: 1, CreatedBy: "tester", Status: models.JobStatusActive, ExecutorId: &localExecutorID},
		{ProjectID: projectID, Spec: "@every 2m", Timezone: "UTC", Data: `{"job":"beta"}`, AccountId: 1, CreatedBy: "tester", Status: models.JobStatusActive, ExecutorId: &localExecutorID},
		{ProjectID: projectID, Spec: "@every 5m", Timezone: "UTC", Data: `{"job":"gamma"}`, AccountId: 1, CreatedBy: "tester", Status: models.JobStatusActive, ExecutorId: &localExecutorID},
		{ProjectID: projectID, Spec: "@every 10m", Timezone: "UTC", Data: `{"job":"other"}`, AccountId: 1, CreatedBy: "tester", Status: models.JobStatusActive, ExecutorId: &otherExecutorID},
	}
	insertedIDs, insertErr := jobRepo.BatchInsertJobs(jobs)
	require.Nil(t, insertErr)
	require.Len(t, insertedIDs, 4)

	pulled, pullErr := jobRepo.GetActiveJobsByExecutorID(localExecutorID, 1)
	require.Nil(t, pullErr)
	require.Len(t, pulled, 3)

	seenSpecs := map[string]bool{}
	for _, job := range pulled {
		require.NotNil(t, job.ExecutorId)
		assert.Equal(t, localExecutorID, *job.ExecutorId)
		seenSpecs[job.Spec] = true
	}
	assert.True(t, seenSpecs["@every 1m"])
	assert.True(t, seenSpecs["@every 2m"])
	assert.True(t, seenSpecs["@every 5m"])
	assert.False(t, seenSpecs["@every 10m"])
}
