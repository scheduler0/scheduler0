package project

import (
	"context"
	"net/http"
	"os"
	"scheduler0/pkg/config"
	"scheduler0/pkg/db"
	"scheduler0/pkg/fsm"
	"scheduler0/pkg/models"
	account_repo "scheduler0/pkg/repository/account"
	"scheduler0/pkg/repository/job"
	"scheduler0/pkg/shared_repo"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/assert"
)

func Test_ProjectRepo_GetBatchProjectsByIDs(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "project-repo-test",
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

	// Create a new ProjectRepo instance
	projectRepo := NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, nil)

	// Create projects to retrieve
	projects := []models.Project{
		{
			ID:          1,
			Name:        "Project 1",
			Description: "Description 1",
			AccountId:   1,
		},
		{
			ID:          2,
			Name:        "Project 2",
			Description: "Description 2",
			AccountId:   1,
		},
		{
			ID:          3,
			Name:        "Project 3",
			Description: "Description 3",
			AccountId:   1,
		},
	}

	// Insert the projects into the database
	for _, project := range projects {
		_, createErr := projectRepo.CreateOne(&project)
		if createErr != nil {
			t.Fatal("failed to create project:", createErr)
		}
	}

	// Get the projects by their IDs
	projectIDs := []uint64{1, 3}
	retrievedProjects, getErr := projectRepo.GetBatchProjectsByIDs(projectIDs)
	if getErr != nil {
		t.Fatal("failed to get projects:", getErr)
	}

	// Assert the number of retrieved projects
	assert.Equal(t, 2, len(retrievedProjects))

	// Assert the retrieved project IDs and names
	for _, project := range retrievedProjects {
		assert.Contains(t, projectIDs, project.ID)
		assert.Contains(t, []string{"Project 1", "Project 3"}, project.Name)
	}
}

func Test_ProjectRepo_List(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "project-repo-test",
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

	// Create a new ProjectRepo instance
	projectRepo := NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, nil)

	// Create projects to retrieve
	projects := []models.Project{
		{
			ID:          1,
			Name:        "Project 1",
			Description: "Description 1",
			AccountId:   1,
		},
		{
			ID:          2,
			Name:        "Project 2",
			Description: "Description 2",
			AccountId:   1,
		},
		{
			ID:          3,
			Name:        "Project 3",
			Description: "Description 3",
			AccountId:   1,
		},
	}

	// Insert the projects into the database
	for _, project := range projects {
		_, createErr := projectRepo.CreateOne(&project)
		if createErr != nil {
			t.Fatal("failed to create project:", createErr)
		}
	}

	// Call the List method with offset 1 and limit 2
	offset := uint64(1)
	limit := uint64(2)
	retrievedProjects, listErr := projectRepo.List(offset, limit, 1, "id", "ASC")
	if listErr != nil {
		t.Fatal("failed to list projects:", listErr)
	}

	// Assert the number of retrieved projects
	assert.Equal(t, 2, len(retrievedProjects))

	// Assert the retrieved project IDs and names
	assert.Equal(t, uint64(2), retrievedProjects[0].ID)
	assert.Equal(t, "Project 2", retrievedProjects[0].Name)
	assert.Equal(t, "Description 2", retrievedProjects[0].Description)
	assert.Equal(t, uint64(3), retrievedProjects[1].ID)
	assert.Equal(t, "Project 3", retrievedProjects[1].Name)
	assert.Equal(t, "Description 3", retrievedProjects[1].Description)

	// Test invalid order by column
	_, invalidColumnErr := projectRepo.List(offset, limit, 1, "invalid_column", "ASC")
	assert.NotNil(t, invalidColumnErr)
	assert.Equal(t, "invalid order by column", invalidColumnErr.Message)

	// Test invalid order by direction
	_, invalidDirectionErr := projectRepo.List(offset, limit, 1, "id", "INVALID")
	assert.NotNil(t, invalidDirectionErr)
	assert.Equal(t, "invalid order by direction. Must be ASC or DESC", invalidDirectionErr.Message)
}

func Test_ProjectRepo_UpdateOneByID(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "project-repo-test",
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

	// Create a new ProjectRepo instance
	projectRepo := NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, nil)

	// Create a project to update
	project := models.Project{
		ID:          1,
		Name:        "Old Name",
		Description: "Old Description",
		AccountId:   1,
	}

	// Insert the project into the database
	_, createErr := projectRepo.CreateOne(&project)
	if createErr != nil {
		t.Fatal("failed to create project:", createErr)
	}

	// Modify the project's properties
	project.Description = "New Description"

	// Call the UpdateOneByID method to update the project
	updatedCount, updateErr := projectRepo.UpdateOneByID(project)
	if updateErr != nil {
		t.Fatal("failed to update project:", updateErr)
	}

	// Assert the updated count is 1
	assert.Equal(t, uint64(1), updatedCount)

	// Retrieve the updated project from the database
	updatedProject := models.Project{
		ID:        project.ID,
		AccountId: project.AccountId,
	}
	getErr := projectRepo.GetOneByID(&updatedProject)
	if getErr != nil {
		t.Fatal("failed to get updated project:", getErr)
	}

	// Assert the updated properties
	assert.Equal(t, project.Description, updatedProject.Description)
}

func Test_ProjectRepo_DeleteOneByID(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "project-repo-test",
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

	// Create a JobRepo instance
	jobRepo := job.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)

	// Create a new ProjectRepo instance
	projectRepo := NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)

	// Create a project to delete
	project := models.Project{
		ID:          1,
		Name:        "Test Project",
		Description: "Test project description",
		AccountId:   1,
	}

	// Insert the project into the database
	_, createErr := projectRepo.CreateOne(&project)
	if createErr != nil {
		t.Fatal("failed to create project:", createErr)
	}

	// Create a job associated with the project
	job := models.Job{
		ID:             1,
		ProjectID:      project.ID,
		AccountId:      project.AccountId,
		Spec:           "0 * * * *",
		Timezone:       "UTC",
		TimezoneOffset: 0,
		CreatedBy:      "test",
		RetryMax:       3,
		Status:         "active",
		DateCreated:    time.Now(),
	}

	// Insert the job into the database
	_, insertErr := jobRepo.BatchInsertJobs([]models.Job{job})
	if insertErr != nil {
		t.Fatal("failed to insert job:", insertErr)
	}

	// Try to delete the project (should fail since it has associated jobs)
	_, deleteErr := projectRepo.DeleteOneByID(project)
	if deleteErr == nil {
		t.Fatal("project should not be deleted as it has associated jobs")
	}

	// Delete the job
	_, jobDeleteErr := jobRepo.DeleteOneByID(job)
	if jobDeleteErr != nil {
		t.Fatal("failed to delete job:", jobDeleteErr)
	}

	// Delete the project (soft delete)
	project.DeletedBy = new(string)
	*project.DeletedBy = "test"
	_, projectDeleteErr := projectRepo.DeleteOneByID(project)
	if projectDeleteErr != nil {
		t.Fatal("failed to delete project:", projectDeleteErr)
	}

	// Try to retrieve the deleted project from the database (soft delete, so project still exists but is marked as deleted)
	deletedProject := models.Project{
		ID:        project.ID,
		AccountId: project.AccountId,
	}
	getErr := projectRepo.GetOneByID(&deletedProject)
	// GetOneByID filters by deleted_by IS NULL OR deleted_by = '', so it should return 404 for soft-deleted projects
	assert.NotNil(t, getErr)
	assert.Equal(t, 404, getErr.Type)
	assert.Contains(t, getErr.Message, "project does not exist")
}

func Test_ProjectRepo_Count(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "project-repo-test",
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

	// Create a new ProjectRepo instance
	projectRepo := NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, nil)

	// Create projects for testing
	projects := []models.Project{
		{
			ID:          1,
			Name:        "Project 1",
			Description: "Description 1",
			AccountId:   1,
		},
		{
			ID:          2,
			Name:        "Project 2",
			Description: "Description 2",
			AccountId:   1,
		},
		{
			ID:          3,
			Name:        "Project 3",
			Description: "Description 3",
			AccountId:   1,
		},
	}

	// Insert the projects into the database
	for _, project := range projects {
		_, createErr := projectRepo.CreateOne(&project)
		if createErr != nil {
			t.Fatalf("Failed to create project: %v", createErr)
		}
	}

	// Call the Count method
	count, countErr := projectRepo.Count()
	if countErr != nil {
		t.Fatalf("Failed to count projects: %v", countErr)
	}

	// Assert the count is equal to the number of projects
	expectedCount := uint64(len(projects))
	assert.Equal(t, expectedCount, count)
}

func Test_ProjectRepo_ListAll(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "project-repo-test",
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

	// Create accounts first (required for foreign key constraint)
	accountRepo := account_repo.NewAccountRepository(context.TODO(), logger, scheduler0RaftActions, scheduler0Store)
	account1 := &models.Account{
		ID:   1,
		Name: "Test Account 1",
	}
	_, createAccount1Err := accountRepo.CreateAccount(account1)
	if createAccount1Err != nil {
		t.Fatalf("Failed to create account 1: %v", createAccount1Err)
	}

	account2 := &models.Account{
		ID:   2,
		Name: "Test Account 2",
	}
	_, createAccount2Err := accountRepo.CreateAccount(account2)
	if createAccount2Err != nil {
		t.Fatalf("Failed to create account 2: %v", createAccount2Err)
	}

	// Create a new ProjectRepo instance
	projectRepo := NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, nil)

	// Create projects for testing (across different accounts)
	projects := []models.Project{
		{
			ID:          1,
			Name:        "Project 1",
			Description: "Description 1",
			AccountId:   1,
		},
		{
			ID:          2,
			Name:        "Project 2",
			Description: "Description 2",
			AccountId:   1,
		},
		{
			ID:          3,
			Name:        "Project 3",
			Description: "Description 3",
			AccountId:   2,
		},
		{
			ID:          4,
			Name:        "Project 4",
			Description: "Description 4",
			AccountId:   2,
		},
	}

	// Insert the projects into the database
	for _, project := range projects {
		_, createErr := projectRepo.CreateOne(&project)
		if createErr != nil {
			t.Fatalf("Failed to create project: %v", createErr)
		}
	}

	// Test ListAll with offset 0 and limit 2
	offset := uint64(0)
	limit := uint64(2)
	retrievedProjects, listErr := projectRepo.ListAll(offset, limit)
	if listErr != nil {
		t.Fatalf("Failed to list all projects: %v", listErr)
	}

	// Assert the number of retrieved projects
	assert.Equal(t, 2, len(retrievedProjects))

	// Test ListAll with offset 2 and limit 2
	offset = uint64(2)
	retrievedProjects2, listErr2 := projectRepo.ListAll(offset, limit)
	if listErr2 != nil {
		t.Fatalf("Failed to list all projects: %v", listErr2)
	}

	// Assert the number of retrieved projects
	assert.Equal(t, 2, len(retrievedProjects2))

	// Test ListAll with offset 4 and limit 2 (should return empty)
	offset = uint64(4)
	retrievedProjects3, listErr3 := projectRepo.ListAll(offset, limit)
	if listErr3 != nil {
		t.Fatalf("Failed to list all projects: %v", listErr3)
	}

	// Assert the number of retrieved projects (should be empty)
	assert.Equal(t, 0, len(retrievedProjects3))

	// Verify that all projects are returned across different accounts
	allProjects := append(retrievedProjects, retrievedProjects2...)
	assert.Equal(t, 4, len(allProjects))

	// Verify that projects from both accounts are included
	account1Count := 0
	account2Count := 0
	for _, project := range allProjects {
		if project.AccountId == 1 {
			account1Count++
		} else if project.AccountId == 2 {
			account2Count++
		}
	}
	assert.Equal(t, 2, account1Count)
	assert.Equal(t, 2, account2Count)
}

// ========== Edge Case Tests ==========

func Test_ProjectRepo_CreateOne_EdgeCase_WhitespaceOnlyName(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "project-repo-test",
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

	projectRepo := NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, nil)

	project := &models.Project{
		Name:        "   ", // Whitespace only
		Description: "Description",
		AccountId:   account.ID,
	}

	_, createErr := projectRepo.CreateOne(project)

	assert.Error(t, createErr)
	assert.Equal(t, http.StatusBadRequest, createErr.Type)
	assert.Contains(t, createErr.Message, "name field is required")
}

func Test_ProjectRepo_CreateOne_EdgeCase_WhitespaceOnlyDescription(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "project-repo-test",
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

	projectRepo := NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, nil)

	project := &models.Project{
		Name:        "Project Name",
		Description: "   ", // Whitespace only
		AccountId:   account.ID,
	}

	_, createErr := projectRepo.CreateOne(project)

	assert.Error(t, createErr)
	assert.Equal(t, http.StatusBadRequest, createErr.Type)
	assert.Contains(t, createErr.Message, "description field is required")
}

func Test_ProjectRepo_GetBatchProjectsByIDs_EdgeCase_SomeNonExistent(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "project-repo-test",
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

	projectRepo := NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, nil)

	project1 := &models.Project{
		Name:        "Project 1",
		Description: "Description 1",
		AccountId:   account.ID,
	}
	_, createErr1 := projectRepo.CreateOne(project1)
	if createErr1 != nil {
		t.Fatalf("Failed to create project: %v", createErr1)
	}

	// Get projects with mix of existing and non-existent IDs
	projectIDs := []uint64{project1.ID, 99999, 88888}
	retrievedProjects, getErr := projectRepo.GetBatchProjectsByIDs(projectIDs)

	assert.Nil(t, getErr)
	assert.Len(t, retrievedProjects, 1, "Should return only existing projects")
	assert.Equal(t, project1.ID, retrievedProjects[0].ID)
}

func Test_ProjectRepo_GetBatchProjectsByIDs_EdgeCase_AllNonExistent(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "project-repo-test",
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

	projectRepo := NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, nil)

	// Get projects with all non-existent IDs
	projectIDs := []uint64{99999, 88888, 77777}
	retrievedProjects, getErr := projectRepo.GetBatchProjectsByIDs(projectIDs)

	assert.Nil(t, getErr)
	assert.Empty(t, retrievedProjects, "Should return empty list when no projects exist")
}

func Test_ProjectRepo_GetOneByID_EdgeCase_ProjectIDZero(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "project-repo-test",
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

	projectRepo := NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, nil)

	project := &models.Project{
		ID:        0, // Zero ID
		AccountId: account.ID,
	}

	getErr := projectRepo.GetOneByID(project)

	// GetOneByID doesn't validate project ID 0, it will query and return not found
	// This is acceptable behavior, but we could add validation
	assert.NotNil(t, getErr)
	assert.Equal(t, http.StatusNotFound, getErr.Type)
}

func Test_ProjectRepo_List_EdgeCase_ZeroLimit(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "project-repo-test",
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

	projectRepo := NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, nil)

	// List with zero limit
	projects, listErr := projectRepo.List(0, 0, account.ID, "id", "ASC")

	assert.Nil(t, listErr)
	assert.Empty(t, projects, "Should return empty list with zero limit")
}

func Test_ProjectRepo_UpdateOneByID_EdgeCase_NonExistentProject(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "project-repo-test",
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

	projectRepo := NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, nil)

	// Try to update non-existent project
	project := models.Project{
		ID:          99999,
		Name:        "Updated Name",
		Description: "Updated Description",
		AccountId:   account.ID,
	}

	_, updateErr := projectRepo.UpdateOneByID(project)

	assert.NotNil(t, updateErr)
	assert.Equal(t, http.StatusNotFound, updateErr.Type)
	assert.Contains(t, updateErr.Message, "project does not exist")
}
