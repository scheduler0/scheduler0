package project

import (
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
	job_repo "scheduler0/repository/job"
	project_repo "scheduler0/repository/project"
	"scheduler0/shared_repo"
	"testing"
)

func Test_ProjectService_CreateOne(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "project-service-test",
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

	// Create a new ProjectService instance
	projectService := NewProjectService(logger, projectRepo)

	// Define the input project
	project := models.Project{
		ID:          1,
		Name:        "Test Project",
		Description: "Test project description",
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

	jobRepo := job_repo.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)

	// Create a new ProjectRepo instance
	projectRepo := project_repo.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)

	// Create a new ProjectService instance
	projectService := NewProjectService(logger, projectRepo)

	// Define the input project
	project := models.Project{
		ID:          1,
		Name:        "Test Project",
		Description: "Test project description",
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
	updatedProject := models.Project{ID: project.ID}
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

	jobRepo := job_repo.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)

	// Create a new ProjectRepo instance
	projectRepo := project_repo.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)

	// Create a new ProjectService instance
	projectService := NewProjectService(logger, projectRepo)

	// Define the input project
	project := models.Project{
		ID:          1,
		Name:        "Test Project",
		Description: "Test project description",
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

	jobRepo := job_repo.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)

	// Create a new ProjectRepo instance
	projectRepo := project_repo.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)

	// Create a new ProjectService instance
	projectService := NewProjectService(logger, projectRepo)

	// Define the input project
	project := models.Project{
		ID:          1,
		Name:        "Test Project",
		Description: "Test project description",
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

	jobRepo := job_repo.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)

	// Create a new ProjectRepo instance
	projectRepo := project_repo.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)

	// Create a new ProjectService instance
	projectService := NewProjectService(logger, projectRepo)

	// Define the input project
	project := models.Project{
		ID:          1,
		Name:        "Test Project",
		Description: "Test project description",
	}

	// Create the project using the project repo
	_, createErr := projectRepo.CreateOne(&project)
	if createErr != nil {
		t.Fatalf("Failed to create project: %v", createErr)
	}

	// Delete the project using the DeleteOneByID method of the project service
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
	expectedErrMsg := fmt.Sprintf("message: project does not exist, code: 404")
	assert.Equal(t, getErr.Error(), expectedErrMsg)
}

func Test_ProjectService_List(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "project-service-test",
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

	jobRepo := job_repo.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)

	// Create a new ProjectRepo instance
	projectRepo := project_repo.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)

	// Create a new ProjectService instance
	projectService := NewProjectService(logger, projectRepo)

	// Define the input projects
	projects := []models.Project{
		{
			ID:          1,
			Name:        "Project 1",
			Description: "Project 1 description",
		},
		{
			ID:          2,
			Name:        "Project 2",
			Description: "Project 2 description",
		},
		{
			ID:          3,
			Name:        "Project 3",
			Description: "Project 3 description",
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
	paginatedProjects, listErr := projectService.List(offset, limit)
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
}

func Test_ProjectService_BatchGetProjects(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "project-service-test",
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

	jobRepo := job_repo.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)

	// Create a new ProjectRepo instance
	projectRepo := project_repo.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)

	// Create a new ProjectService instance
	projectService := NewProjectService(logger, projectRepo)

	// Define the input projects
	projects := []models.Project{
		{
			ID:          1,
			Name:        "Project 1",
			Description: "Project 1 description",
		},
		{
			ID:          2,
			Name:        "Project 2",
			Description: "Project 2 description",
		},
		{
			ID:          3,
			Name:        "Project 3",
			Description: "Project 3 description",
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
}
