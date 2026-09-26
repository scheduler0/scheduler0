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
	job_repo "scheduler0/pkg/repository/job"
	project_repo "scheduler0/pkg/repository/project"
	"scheduler0/pkg/shared_repo"
	"scheduler0/pkg/utils"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/assert"
)

// mockJobService is a simple mock implementation of JobService for testing
type mockJobService struct{}

func (m *mockJobService) DeleteJobsByProjectID(projectID uint64, accountId uint64, deletedBy string) *utils.GenericError {
	return nil
}

func Test_ProjectService_CreateOne(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "project-service-test",
		Level: hclog.LevelFromString("DEBUG"),
	})

	// Create a temporary SQLite database file
	tempFile, err := os.CreateTemp("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tempFile.Close()
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

	jobRepo := job_repo.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)

	// Create a new ProjectRepo instance
	projectRepo := project_repo.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)

	// Create a mock JobService
	mockJobService := &mockJobService{}

	// Create a new ProjectService instance
	projectService := NewProjectService(logger, projectRepo, mockJobService)

	// Define the input project
	project := models.Project{
		ID:          1,
		Name:        "Test Project",
		Description: "Test project description",
		AccountId:   1,
	}

	// Call the CreateOne method of the project service
	createdProject, createErr := projectService.CreateOne(project)
	if createErr != nil {
		t.Fatalf("Failed to create project: %v", createErr)
	}

	// Assert the correctness of the created project
	assert.Equal(t, project.ID, createdProject.ID)
	assert.Equal(t, project.Name, createdProject.Name)
	assert.Equal(t, project.Description, createdProject.Description)
}

func Test_ProjectService_UpdateOneByID(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "project-service-test",
		Level: hclog.LevelFromString("DEBUG"),
	})

	// Create a temporary SQLite database file
	tempFile, err := os.CreateTemp("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tempFile.Close()
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

	jobRepo := job_repo.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)

	// Create a new ProjectRepo instance
	projectRepo := project_repo.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)

	// Create a mock JobService
	mockJobService := &mockJobService{}

	// Create a new ProjectService instance
	projectService := NewProjectService(logger, projectRepo, mockJobService)

	// Define the input project
	project := models.Project{
		ID:          1,
		Name:        "Test Project",
		Description: "Test project description",
		AccountId:   1,
	}

	// Create the project using the project repo
	_, createErr := projectRepo.CreateOne(&project)
	if createErr != nil {
		t.Fatalf("Failed to create project: %v", createErr)
	}

	// Update the project's description
	project.Description = "Updated project description"

	// Call the UpdateOneByID method of the project service
	updateErr := projectService.UpdateOneByID(&project)
	if updateErr != nil {
		t.Fatalf("Failed to update project: %v", updateErr)
	}

	// Retrieve the project from the project repo
	updatedProject := models.Project{
		ID:        project.ID,
		AccountId: project.AccountId,
	}
	getErr := projectRepo.GetOneByID(&updatedProject)
	if getErr != nil {
		t.Fatalf("Failed to get project: %v", getErr)
	}

	// Assert the correctness of the updated project
	assert.Equal(t, project.ID, updatedProject.ID)
	assert.Equal(t, project.Name, updatedProject.Name)
	assert.Equal(t, project.Description, updatedProject.Description)
}

func Test_ProjectService_GetOneByID(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "project-service-test",
		Level: hclog.LevelFromString("DEBUG"),
	})

	// Create a temporary SQLite database file
	tempFile, err := os.CreateTemp("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tempFile.Close()
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

	jobRepo := job_repo.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)

	// Create a new ProjectRepo instance
	projectRepo := project_repo.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)

	// Create a mock JobService
	mockJobService := &mockJobService{}

	// Create a new ProjectService instance
	projectService := NewProjectService(logger, projectRepo, mockJobService)

	// Define the input project
	project := models.Project{
		ID:          1,
		Name:        "Test Project",
		Description: "Test project description",
		AccountId:   1,
	}

	// Create the project using the project repo
	_, createErr := projectRepo.CreateOne(&project)
	if createErr != nil {
		t.Fatalf("Failed to create project: %v", createErr)
	}

	// Retrieve the project by ID using the GetOneByID method of the project service
	getErr := projectService.GetOneByID(&project)
	if getErr != nil {
		t.Fatalf("Failed to get project by ID: %v", getErr)
	}

	// Assert the correctness of the retrieved project
	assert.Equal(t, project.ID, uint64(1))
	assert.Equal(t, project.Name, "Test Project")
	assert.Equal(t, project.Description, "Test project description")
}

func Test_ProjectService_GetOneByName(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "project-service-test",
		Level: hclog.LevelFromString("DEBUG"),
	})

	// Create a temporary SQLite database file
	tempFile, err := os.CreateTemp("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tempFile.Close()
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

	jobRepo := job_repo.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)

	// Create a new ProjectRepo instance
	projectRepo := project_repo.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)

	// Create a mock JobService
	mockJobService := &mockJobService{}

	// Create a new ProjectService instance
	projectService := NewProjectService(logger, projectRepo, mockJobService)

	// Define the input project
	project := models.Project{
		ID:          1,
		Name:        "Test Project",
		Description: "Test project description",
		AccountId:   1,
	}

	// Create the project using the project repo
	_, createErr := projectRepo.CreateOne(&project)
	if createErr != nil {
		t.Fatalf("Failed to create project: %v", createErr)
	}

	// Retrieve the project by name using the GetOneByName method of the project service
	getErr := projectService.GetOneByName(&project)
	if getErr != nil {
		t.Fatalf("Failed to get project by name: %v", getErr)
	}

	// Assert the correctness of the retrieved project
	assert.Equal(t, project.ID, uint64(1))
	assert.Equal(t, project.Name, "Test Project")
	assert.Equal(t, project.Description, "Test project description")
}

func Test_ProjectService_DeleteOneByID(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "project-service-test",
		Level: hclog.LevelFromString("DEBUG"),
	})

	// Create a temporary SQLite database file
	tempFile, err := os.CreateTemp("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tempFile.Close()
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

	// Create an account first (required for foreign key constraint)
	// Use account ID 2 since account ID 1 is the system account and cannot be deleted
	accountRepo := account_repo.NewAccountRepository(context.TODO(), logger, scheduler0RaftActions, scheduler0Store)
	account := &models.Account{
		ID:   2,
		Name: "Test Account",
	}
	_, createAccountErr := accountRepo.CreateAccount(account)
	if createAccountErr != nil {
		t.Fatalf("Failed to create account: %v", createAccountErr)
	}

	jobRepo := job_repo.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)

	// Create a new ProjectRepo instance
	projectRepo := project_repo.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)

	// Create a mock JobService
	mockJobService := &mockJobService{}

	// Create a new ProjectService instance
	projectService := NewProjectService(logger, projectRepo, mockJobService)

	// Define the input project
	project := models.Project{
		ID:          1,
		Name:        "Test Project",
		Description: "Test project description",
		AccountId:   2, // Use account ID 2 (not system account)
	}

	// Create the project using the project repo
	_, createErr := projectRepo.CreateOne(&project)
	if createErr != nil {
		t.Fatalf("Failed to create project: %v", createErr)
	}

	// Delete the project using the DeleteOneByID method of the project service
	deletedBy := "test"
	project.DeletedBy = &deletedBy
	deleteErr := projectService.DeleteOneByID(project)
	if deleteErr != nil {
		t.Fatalf("Failed to delete project: %v", deleteErr)
	}

	// Attempt to retrieve the deleted project
	getErr := projectService.GetOneByID(&project)
	if getErr == nil {
		t.Fatal("Expected error when retrieving deleted project, but got nil")
	}
	// Assert the correctness of the error message
	expectedErrMsg := "message: project does not exist, code: 404"
	assert.Equal(t, getErr.Error(), expectedErrMsg)
}

func Test_ProjectService_List(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "project-service-test",
		Level: hclog.LevelFromString("DEBUG"),
	})

	// Create a temporary SQLite database file
	tempFile, err := os.CreateTemp("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tempFile.Close()
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

	jobRepo := job_repo.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)

	// Create a new ProjectRepo instance
	projectRepo := project_repo.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)

	// Create a mock JobService
	mockJobService := &mockJobService{}

	// Create a new ProjectService instance
	projectService := NewProjectService(logger, projectRepo, mockJobService)

	// Define the input projects
	projects := []models.Project{
		{
			ID:          1,
			Name:        "Project 1",
			Description: "Project 1 description",
			AccountId:   1,
		},
		{
			ID:          2,
			Name:        "Project 2",
			Description: "Project 2 description",
			AccountId:   1,
		},
		{
			ID:          3,
			Name:        "Project 3",
			Description: "Project 3 description",
			AccountId:   1,
		},
	}

	// Create the projects using the project repo
	for _, project := range projects {
		_, createErr := projectRepo.CreateOne(&project)
		if createErr != nil {
			t.Fatalf("Failed to create project: %v", createErr)
		}
	}

	// Call the List method of the project service
	offset := uint64(0)
	limit := uint64(10)
	paginatedProjects, listErr := projectService.List(offset, limit, 1, "id", "ASC")
	if listErr != nil {
		t.Fatalf("Failed to retrieve projects: %v", listErr)
	}

	// Assert the correctness of the retrieved projects
	assert.Equal(t, len(projects), len(paginatedProjects.Data))
	assert.Equal(t, uint64(len(projects)), paginatedProjects.Total)
	assert.Equal(t, offset, paginatedProjects.Offset)
	assert.Equal(t, limit, paginatedProjects.Limit)

	// Assert the correctness of the retrieved projects by ID
	for _, project := range paginatedProjects.Data {
		found := false
		for _, expectedProject := range projects {
			if project.ID == expectedProject.ID {
				found = true
				assert.Equal(t, expectedProject.Name, project.Name)
				assert.Equal(t, expectedProject.Description, project.Description)
				break
			}
		}
		assert.True(t, found, "Unexpected project found")
	}

	// Test invalid order by column
	_, invalidColumnErr := projectService.List(offset, limit, 1, "invalid_column", "ASC")
	assert.NotNil(t, invalidColumnErr)
	assert.Equal(t, "invalid order by column", invalidColumnErr.Message)

	// Test invalid order by direction
	_, invalidDirectionErr := projectService.List(offset, limit, 1, "id", "INVALID")
	assert.NotNil(t, invalidDirectionErr)
	assert.Equal(t, "invalid order by direction. Must be ASC or DESC", invalidDirectionErr.Message)
}

func Test_ProjectService_BatchGetProjects(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "project-service-test",
		Level: hclog.LevelFromString("DEBUG"),
	})

	// Create a temporary SQLite database file
	tempFile, err := os.CreateTemp("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tempFile.Close()
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

	jobRepo := job_repo.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)

	// Create a new ProjectRepo instance
	projectRepo := project_repo.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)

	// Create a mock JobService
	mockJobService := &mockJobService{}

	// Create a new ProjectService instance
	projectService := NewProjectService(logger, projectRepo, mockJobService)

	// Define the input projects
	projects := []models.Project{
		{
			ID:          1,
			Name:        "Project 1",
			Description: "Project 1 description",
			AccountId:   1,
		},
		{
			ID:          2,
			Name:        "Project 2",
			Description: "Project 2 description",
			AccountId:   1,
		},
		{
			ID:          3,
			Name:        "Project 3",
			Description: "Project 3 description",
			AccountId:   1,
		},
	}

	// Create the projects using the project repo
	for _, project := range projects {
		_, createErr := projectRepo.CreateOne(&project)
		if createErr != nil {
			t.Fatalf("Failed to create project: %v", createErr)
		}
	}

	// Define the project IDs to retrieve
	projectIds := []uint64{1, 3}

	// Call the BatchGetProjects method of the project service
	retrievedProjects, batchErr := projectService.BatchGetProjects(projectIds)
	if batchErr != nil {
		t.Fatalf("Failed to retrieve projects: %v", batchErr)
	}

	// Assert the correctness of the retrieved projects
	assert.Equal(t, len(projectIds), len(retrievedProjects))

	// Assert the correctness of the retrieved projects by ID
	for _, project := range retrievedProjects {
		found := false
		for _, expectedID := range projectIds {
			if project.ID == expectedID {
				found = true
				break
			}
		}
		assert.True(t, found, "Unexpected project found")
	}

	// Test with empty project IDs array
	emptyProjects, emptyErr := projectService.BatchGetProjects([]uint64{})
	if emptyErr != nil {
		t.Fatalf("Failed to retrieve empty projects: %v", emptyErr)
	}
	assert.Equal(t, 0, len(emptyProjects))
}

func Test_ProjectService_DeleteOneByID_MissingDeletedBy(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "project-service-test",
		Level: hclog.LevelFromString("DEBUG"),
	})

	// Create a temporary SQLite database file
	tempFile, err := os.CreateTemp("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tempFile.Close()
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

	jobRepo := job_repo.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)

	// Create a new ProjectRepo instance
	projectRepo := project_repo.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)

	// Create a mock JobService
	mockJobService := &mockJobService{}

	// Create a new ProjectService instance
	projectService := NewProjectService(logger, projectRepo, mockJobService)

	// Test with nil deletedBy
	project := models.Project{
		ID:        1,
		AccountId: 2,
		DeletedBy: nil,
	}

	deleteErr := projectService.DeleteOneByID(project)
	assert.NotNil(t, deleteErr)
	assert.Equal(t, "deletedBy is required", deleteErr.Message)

	// Test with empty deletedBy
	emptyDeletedBy := ""
	project.DeletedBy = &emptyDeletedBy

	deleteErr = projectService.DeleteOneByID(project)
	assert.NotNil(t, deleteErr)
	assert.Equal(t, "deletedBy is required", deleteErr.Message)
}

func Test_ProjectService_DeleteOneByID_SystemAccount(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "project-service-test",
		Level: hclog.LevelFromString("DEBUG"),
	})

	// Create a temporary SQLite database file
	tempFile, err := os.CreateTemp("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tempFile.Close()
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

	jobRepo := job_repo.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)

	// Create a new ProjectRepo instance
	projectRepo := project_repo.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)

	// Create a mock JobService
	mockJobService := &mockJobService{}

	// Create a new ProjectService instance
	projectService := NewProjectService(logger, projectRepo, mockJobService)

	// Test with system account (ID = 1)
	deletedBy := "test"
	project := models.Project{
		ID:        1,
		AccountId: 1, // System account
		DeletedBy: &deletedBy,
	}

	deleteErr := projectService.DeleteOneByID(project)
	assert.NotNil(t, deleteErr)
	assert.Equal(t, "system projects cannot be deleted", deleteErr.Message)
}

func Test_ProjectService_DeleteOneByID_JobDeletionFailure(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "project-service-test",
		Level: hclog.LevelFromString("DEBUG"),
	})

	// Create a temporary SQLite database file
	tempFile, err := os.CreateTemp("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tempFile.Close()
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

	jobRepo := job_repo.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)

	// Create a new ProjectRepo instance
	projectRepo := project_repo.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)

	// Create a mock JobService that returns an error
	mockJobServiceError := &mockJobServiceWithError{}

	// Create a new ProjectService instance
	projectService := NewProjectService(logger, projectRepo, mockJobServiceError)

	// Create an account first
	accountRepo := account_repo.NewAccountRepository(context.TODO(), logger, scheduler0RaftActions, scheduler0Store)
	account := &models.Account{
		ID:   2,
		Name: "Test Account",
	}
	_, createAccountErr := accountRepo.CreateAccount(account)
	if createAccountErr != nil {
		t.Fatalf("Failed to create account: %v", createAccountErr)
	}

	// Create a project
	project := models.Project{
		ID:          1,
		Name:        "Test Project",
		Description: "Test project description",
		AccountId:   2,
	}
	_, createErr := projectRepo.CreateOne(&project)
	if createErr != nil {
		t.Fatalf("Failed to create project: %v", createErr)
	}

	// Try to delete the project - should fail because job deletion fails
	deletedBy := "test"
	project.DeletedBy = &deletedBy
	deleteErr := projectService.DeleteOneByID(project)
	assert.NotNil(t, deleteErr)
	assert.Equal(t, "failed to delete jobs", deleteErr.Message)
}

func Test_ProjectService_DeleteOneByID_ProjectNotFound(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "project-service-test",
		Level: hclog.LevelFromString("DEBUG"),
	})

	// Create a temporary SQLite database file
	tempFile, err := os.CreateTemp("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tempFile.Close()
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

	jobRepo := job_repo.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)

	// Create a new ProjectRepo instance
	projectRepo := project_repo.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)

	// Create a mock JobService
	mockJobService := &mockJobService{}

	// Create a new ProjectService instance
	projectService := NewProjectService(logger, projectRepo, mockJobService)

	// Try to delete a non-existent project
	deletedBy := "test"
	project := models.Project{
		ID:        999,
		AccountId: 2,
		DeletedBy: &deletedBy,
	}

	deleteErr := projectService.DeleteOneByID(project)
	assert.NotNil(t, deleteErr)
	assert.Contains(t, deleteErr.Message, "project does not exist")
}

func Test_ProjectService_UpdateOneByID_ProjectNotFound(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "project-service-test",
		Level: hclog.LevelFromString("DEBUG"),
	})

	// Create a temporary SQLite database file
	tempFile, err := os.CreateTemp("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tempFile.Close()
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

	jobRepo := job_repo.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)

	// Create a new ProjectRepo instance
	projectRepo := project_repo.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)

	// Create a mock JobService
	mockJobService := &mockJobService{}

	// Create a new ProjectService instance
	projectService := NewProjectService(logger, projectRepo, mockJobService)

	// Try to update a non-existent project
	project := &models.Project{
		ID:          999,
		Name:        "Non-existent Project",
		Description: "This project does not exist",
		AccountId:   1,
	}

	updateErr := projectService.UpdateOneByID(project)
	assert.NotNil(t, updateErr)
	assert.Contains(t, updateErr.Message, "Cannot find ProjectID")
}

func Test_ProjectService_List_ValidationErrors(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "project-service-test",
		Level: hclog.LevelFromString("DEBUG"),
	})

	// Create a temporary SQLite database file
	tempFile, err := os.CreateTemp("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tempFile.Close()
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

	jobRepo := job_repo.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)

	// Create a new ProjectRepo instance
	projectRepo := project_repo.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)

	// Create a mock JobService
	mockJobService := &mockJobService{}

	// Create a new ProjectService instance
	projectService := NewProjectService(logger, projectRepo, mockJobService)

	// Test limit > 100
	_, limitTooHighErr := projectService.List(0, 101, 1, "id", "ASC")
	assert.NotNil(t, limitTooHighErr)
	assert.Equal(t, "too many projects. limit should be less than 100", limitTooHighErr.Message)

	// Test limit < 1
	_, limitTooLowErr := projectService.List(0, 0, 1, "id", "ASC")
	assert.NotNil(t, limitTooLowErr)
	assert.Equal(t, "limit should be greater than 0", limitTooLowErr.Message)

	// Test offset < 0 (though uint64 can't be negative, testing the check)
	// Since offset is uint64, we can't actually pass a negative value
	// But we can test with a very large offset that would be invalid in practice
	// The actual validation checks for offset < 0, but since it's uint64, this check will never be true
	// However, the code has the check, so we'll test the boundary case
	_, offsetErr := projectService.List(0, 10, 1, "id", "ASC")
	// offset 0 should be valid, so no error expected
	assert.Nil(t, offsetErr)
}

func Test_ProjectService_GetOneByID_ProjectNotFound(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "project-service-test",
		Level: hclog.LevelFromString("DEBUG"),
	})

	// Create a temporary SQLite database file
	tempFile, err := os.CreateTemp("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tempFile.Close()
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

	jobRepo := job_repo.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)

	// Create a new ProjectRepo instance
	projectRepo := project_repo.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)

	// Create a mock JobService
	mockJobService := &mockJobService{}

	// Create a new ProjectService instance
	projectService := NewProjectService(logger, projectRepo, mockJobService)

	// Try to get a non-existent project
	project := &models.Project{
		ID:        999,
		AccountId: 1,
	}

	getErr := projectService.GetOneByID(project)
	assert.NotNil(t, getErr)
}

func Test_ProjectService_GetOneByName_ProjectNotFound(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "project-service-test",
		Level: hclog.LevelFromString("DEBUG"),
	})

	// Create a temporary SQLite database file
	tempFile, err := os.CreateTemp("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tempFile.Close()
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

	jobRepo := job_repo.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)

	// Create a new ProjectRepo instance
	projectRepo := project_repo.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)

	// Create a mock JobService
	mockJobService := &mockJobService{}

	// Create a new ProjectService instance
	projectService := NewProjectService(logger, projectRepo, mockJobService)

	// Try to get a non-existent project by name
	project := &models.Project{
		Name:      "Non-existent Project",
		AccountId: 1,
	}

	getErr := projectService.GetOneByName(project)
	assert.NotNil(t, getErr)
}

// mockJobServiceWithError is a mock implementation that returns an error
type mockJobServiceWithError struct{}

func (m *mockJobServiceWithError) DeleteJobsByProjectID(projectID uint64, accountId uint64, deletedBy string) *utils.GenericError {
	return utils.HTTPGenericError(http.StatusInternalServerError, "failed to delete jobs")
}
