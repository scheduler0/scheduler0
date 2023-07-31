package repository

import (
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
	"time"
)

func Test_JobRepo_BatchInsertJobs(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "job-repo-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo)
	tempFile, err := ioutil.TempFile("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.RunMigration()
	sqliteDb.OpenConnectionToExistingDB()
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

	// Create a new project
	mockProject := models.Project{
		Name:        "Test Project",
		Description: "Test project description",
	}

	// Create a new JobRepo instance
	jobRepo := NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)

	// Create a new ProjectRepo instance
	projectRepo := NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)

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
			CallbackUrl:    "http://example.com/callback",
			ExecutionType:  "cron",
			DateCreated:    time.Now(),
			Timezone:       "UTC",
			TimezoneOffset: 0,
			Data:           "some data",
		},
		{
			ProjectID:      projectID,
			Spec:           "0 12 * * *",
			CallbackUrl:    "http://example.com/callback",
			ExecutionType:  "cron",
			DateCreated:    time.Now(),
			Timezone:       "UTC",
			TimezoneOffset: 0,
			Data:           "some data",
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
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo)
	tempFile, err := ioutil.TempFile("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.RunMigration()
	sqliteDb.OpenConnectionToExistingDB()
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

	// Create a new JobRepo instance
	jobRepo := NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)

	// Create a project to associate with the jobs
	project := models.Project{
		Name:        "Test Project",
		Description: "Test project description",
	}

	// Create a new ProjectRepo instance
	projectRepo := NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)

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
			CallbackUrl:    "http://example.com/callback",
			ExecutionType:  "cron",
			DateCreated:    time.Now(),
			Timezone:       "UTC",
			TimezoneOffset: 0,
			Data:           "some data",
		},
		{
			ID:             2,
			ProjectID:      projectID,
			Spec:           "0 12 * * *",
			CallbackUrl:    "http://example.com/callback",
			ExecutionType:  "cron",
			DateCreated:    time.Now(),
			Timezone:       "UTC",
			TimezoneOffset: 0,
			Data:           "some data",
		},
	}

	// Call the BatchInsertJobs method to insert the jobs
	_, batchInsertErr := jobRepo.BatchInsertJobs(jobs)
	if batchInsertErr != nil {
		t.Fatal("failed to insert jobs:", batchInsertErr)
	}

	// Modify some properties of the first job
	jobs[0].CallbackUrl = "http://example.com/new-callback"

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
	assert.Equal(t, jobs[0].CallbackUrl, updatedJob.CallbackUrl)
}

func Test_JobRepo_DeleteOneByID(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "job-repo-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo)
	tempFile, err := ioutil.TempFile("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.RunMigration()
	sqliteDb.OpenConnectionToExistingDB()
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

	// Create a new JobRepo instance
	jobRepo := NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)

	// Create a project to associate with the job
	project := models.Project{
		Name:        "Test Project",
		Description: "Test project description",
	}

	// Create a new ProjectRepo instance
	projectRepo := NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)

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
		CallbackUrl:    "http://example.com/callback",
		ExecutionType:  "cron",
		DateCreated:    time.Now(),
		Timezone:       "UTC",
		TimezoneOffset: 0,
		Data:           "some data",
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

	// Try to retrieve the deleted job from the database
	deletedJob := models.Job{
		ID: 1,
	}
	getErr := jobRepo.GetOneByID(&deletedJob)
	if getErr == nil {
		t.Fatal("expected job to be deleted, but it was found in the database", getErr)
	}
	assert.Equal(t, deletedJob.CallbackUrl, "")
}

func Test_JobRepo_BatchGetJobsByID(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "job-repo-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo)
	tempFile, err := ioutil.TempFile("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.RunMigration()
	sqliteDb.OpenConnectionToExistingDB()
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

	// Create a new JobRepo instance
	jobRepo := NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)

	// Create a project to associate with the jobs
	project := models.Project{
		Name:        "Test Project",
		Description: "Test project description",
	}

	// Create a new ProjectRepo instance
	projectRepo := NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)

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
			CallbackUrl:    "http://example.com/callback",
			ExecutionType:  "cron",
			DateCreated:    time.Now(),
			Timezone:       "UTC",
			TimezoneOffset: 0,
			Data:           "some data",
		},
		{
			ID:             2,
			ProjectID:      projectID,
			Spec:           "0 12 * * *",
			CallbackUrl:    "http://example.com/callback",
			ExecutionType:  "cron",
			DateCreated:    time.Now(),
			Timezone:       "UTC",
			TimezoneOffset: 0,
			Data:           "some data",
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
		assert.Equal(t, job.CallbackUrl, retrievedJob.CallbackUrl)
		assert.Equal(t, job.ExecutionType, retrievedJob.ExecutionType)
		assert.Equal(t, job.Timezone, retrievedJob.Timezone)
		assert.Equal(t, job.TimezoneOffset, retrievedJob.TimezoneOffset)
		assert.Equal(t, job.Data, retrievedJob.Data)
	}
}

func Test_JobRepo_BatchGetJobsWithIDRange(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "job-repo-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo)
	tempFile, err := ioutil.TempFile("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.RunMigration()
	sqliteDb.OpenConnectionToExistingDB()
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

	// Create a new JobRepo instance
	jobRepo := NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)

	// Create a project to associate with the jobs
	project := models.Project{
		Name:        "Test Project",
		Description: "Test project description",
	}

	// Create a new ProjectRepo instance
	projectRepo := NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)

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
			CallbackUrl:    "http://example.com/callback",
			ExecutionType:  "cron",
			DateCreated:    time.Now(),
			Timezone:       "UTC",
			TimezoneOffset: 0,
			Data:           "some data",
		},
		{
			ID:             2,
			ProjectID:      projectID,
			Spec:           "0 12 * * *",
			CallbackUrl:    "http://example.com/callback",
			ExecutionType:  "cron",
			DateCreated:    time.Now(),
			Timezone:       "UTC",
			TimezoneOffset: 0,
			Data:           "some data",
		},
		{
			ID:             3,
			ProjectID:      projectID,
			Spec:           "0 */2 * * *",
			CallbackUrl:    "http://example.com/callback",
			ExecutionType:  "cron",
			DateCreated:    time.Now(),
			Timezone:       "UTC",
			TimezoneOffset: 0,
			Data:           "some data",
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
		assert.Equal(t, job.CallbackUrl, retrievedJob.CallbackUrl)
		assert.Equal(t, job.ExecutionType, retrievedJob.ExecutionType)
		assert.Equal(t, job.Timezone, retrievedJob.Timezone)
		assert.Equal(t, job.TimezoneOffset, retrievedJob.TimezoneOffset)
		assert.Equal(t, job.Data, retrievedJob.Data)
	}
}

func Test_JobRepo_GetAllByProjectID(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "job-repo-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo)
	tempFile, err := ioutil.TempFile("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.RunMigration()
	sqliteDb.OpenConnectionToExistingDB()
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

	// Create a new JobRepo instance
	jobRepo := NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)

	// Create projects to associate with the jobs
	project1 := models.Project{
		Name:        "Test Project 1",
		Description: "Test project 1 description",
	}
	project2 := models.Project{
		Name:        "Test Project 2",
		Description: "Test project 2 description",
	}

	// Create a new ProjectRepo instance
	projectRepo := NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)

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
			CallbackUrl:    "http://example.com/callback",
			ExecutionType:  "cron",
			DateCreated:    time.Now(),
			Timezone:       "UTC",
			TimezoneOffset: 0,
			Data:           "some data",
		},
		{
			ID:             2,
			ProjectID:      project1ID,
			Spec:           "0 12 * * *",
			CallbackUrl:    "http://example.com/callback",
			ExecutionType:  "cron",
			DateCreated:    time.Now(),
			Timezone:       "UTC",
			TimezoneOffset: 0,
			Data:           "some data",
		},
		{
			ID:             3,
			ProjectID:      project2ID,
			Spec:           "0 */2 * * *",
			CallbackUrl:    "http://example.com/callback",
			ExecutionType:  "cron",
			DateCreated:    time.Now(),
			Timezone:       "UTC",
			TimezoneOffset: 0,
			Data:           "some data",
		},
	}

	// Call the BatchInsertJobs method to insert the jobs
	_, batchInsertErr := jobRepo.BatchInsertJobs(jobs)
	if batchInsertErr != nil {
		t.Fatal("failed to insert jobs:", batchInsertErr)
	}

	// Call the GetAllByProjectID method to retrieve the jobs associated with project1
	retrievedJobs, getErr := jobRepo.GetAllByProjectID(project1ID, 0, 10, "id")
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
		assert.Equal(t, job.CallbackUrl, retrievedJob.CallbackUrl)
		assert.Equal(t, job.ExecutionType, retrievedJob.ExecutionType)
		assert.Equal(t, job.Timezone, retrievedJob.Timezone)
		assert.Equal(t, job.TimezoneOffset, retrievedJob.TimezoneOffset)
		assert.Equal(t, job.Data, retrievedJob.Data)
	}

	// Call the GetAllByProjectID method to retrieve the jobs associated with project2
	retrievedJobs, getErr = jobRepo.GetAllByProjectID(project2ID, 0, 10, "id ASC")
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
	assert.Equal(t, jobs[2].CallbackUrl, retrievedJob.CallbackUrl)
	assert.Equal(t, jobs[2].ExecutionType, retrievedJob.ExecutionType)
	assert.Equal(t, jobs[2].Timezone, retrievedJob.Timezone)
	assert.Equal(t, jobs[2].TimezoneOffset, retrievedJob.TimezoneOffset)
	assert.Equal(t, jobs[2].Data, retrievedJob.Data)
}

func Test_JobRepo_GetJobsTotalCountByProjectID(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "job-repo-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo)
	tempFile, err := ioutil.TempFile("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.RunMigration()
	sqliteDb.OpenConnectionToExistingDB()
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

	// Create a new JobRepo instance
	jobRepo := NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)

	// Create a project to associate with the jobs
	project := models.Project{
		Name:        "Test Project",
		Description: "Test project description",
	}

	// Create a new ProjectRepo instance
	projectRepo := NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)

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
			CallbackUrl:    "http://example.com/callback",
			ExecutionType:  "cron",
			DateCreated:    time.Now(),
			Timezone:       "UTC",
			TimezoneOffset: 0,
			Data:           "some data",
		},
		{
			ID:             2,
			ProjectID:      projectID,
			Spec:           "0 12 * * *",
			CallbackUrl:    "http://example.com/callback",
			ExecutionType:  "cron",
			DateCreated:    time.Now(),
			Timezone:       "UTC",
			TimezoneOffset: 0,
			Data:           "some data",
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
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo)
	tempFile, err := ioutil.TempFile("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.RunMigration()
	sqliteDb.OpenConnectionToExistingDB()
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

	// Create a new JobRepo instance
	jobRepo := NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)

	// Create a project to associate with the jobs
	project := models.Project{
		Name:        "Test Project",
		Description: "Test project description",
	}

	// Create a new ProjectRepo instance
	projectRepo := NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)

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
			CallbackUrl:    "http://example.com/callback",
			ExecutionType:  "cron",
			DateCreated:    time.Now(),
			Timezone:       "UTC",
			TimezoneOffset: 0,
			Data:           "some data",
		},
		{
			ID:             2,
			ProjectID:      projectID,
			Spec:           "0 12 * * *",
			CallbackUrl:    "http://example.com/callback",
			ExecutionType:  "cron",
			DateCreated:    time.Now(),
			Timezone:       "UTC",
			TimezoneOffset: 0,
			Data:           "some data",
		},
		{
			ID:             3,
			ProjectID:      projectID,
			Spec:           "0 * * * *",
			CallbackUrl:    "http://example.com/callback",
			ExecutionType:  "cron",
			DateCreated:    time.Now(),
			Timezone:       "UTC",
			TimezoneOffset: 0,
			Data:           "some data",
		},
		{
			ID:             3,
			ProjectID:      projectID,
			Spec:           "0 12 * * *",
			CallbackUrl:    "http://example.com/callback",
			ExecutionType:  "cron",
			DateCreated:    time.Now(),
			Timezone:       "UTC",
			TimezoneOffset: 0,
			Data:           "some data",
		},
	}

	// Call the BatchInsertJobs method to insert the jobs
	_, batchInsertErr := jobRepo.BatchInsertJobs(jobs)
	if batchInsertErr != nil {
		t.Fatal("failed to insert jobs:", batchInsertErr)
	}

	// Call the GetJobsPaginated method with offset 0 and limit 2
	paginatedJobs, _, geterr := jobRepo.GetJobsPaginated(projectID, 0, 2)
	if geterr != nil {
		t.Fatal("failed to get paginated jobs:", geterr)
	}

	// Assert the length of the returned paginated jobs
	assert.Equal(t, 2, len(paginatedJobs))

	// Assert the job IDs and associated project ID
	assert.Equal(t, projectID, paginatedJobs[0].ProjectID)
	assert.Equal(t, projectID, paginatedJobs[1].ProjectID)
	assert.Contains(t, []uint64{1, 2}, paginatedJobs[0].ID)
	assert.Contains(t, []uint64{1, 2}, paginatedJobs[1].ID)
	assert.NotEqual(t, paginatedJobs[0].ID, paginatedJobs[1].ID)
}
