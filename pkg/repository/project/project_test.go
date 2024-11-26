package project

import (
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
	"scheduler0/pkg/config"
	"scheduler0/pkg/db"
	"scheduler0/pkg/fsm"
	"scheduler0/pkg/models"
	"scheduler0/pkg/repository/job"
	"scheduler0/pkg/shared_repo"
	"testing"
)

func Test_ProjectRepo_GetBatchProjectsByIDs(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "project-repo-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)
	tempFile, err := ioutil.TempFile("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.RunMigration()
	sqliteDb.OpenConnectionToExistingDB()
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

	// Create a new ProjectRepo instance
	projectRepo := NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, nil)

	// Create projects to retrieve
	projects := []models.Project{
		{
			ID:          1,
			Name:        "Project 1",
			Description: "Description 1",
		},
		{
			ID:          2,
			Name:        "Project 2",
			Description: "Description 2",
		},
		{
			ID:          3,
			Name:        "Project 3",
			Description: "Description 3",
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
	tempFile, err := ioutil.TempFile("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.RunMigration()
	sqliteDb.OpenConnectionToExistingDB()
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

	// Create a new ProjectRepo instance
	projectRepo := NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, nil)

	// Create projects to retrieve
	projects := []models.Project{
		{
			ID:          1,
			Name:        "Project 1",
			Description: "Description 1",
		},
		{
			ID:          2,
			Name:        "Project 2",
			Description: "Description 2",
		},
		{
			ID:          3,
			Name:        "Project 3",
			Description: "Description 3",
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
	retrievedProjects, listErr := projectRepo.List(offset, limit)
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
}

func Test_ProjectRepo_UpdateOneByID(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "project-repo-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)
	tempFile, err := ioutil.TempFile("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.RunMigration()
	sqliteDb.OpenConnectionToExistingDB()
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

	// Create a new ProjectRepo instance
	projectRepo := NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, nil)

	// Create a project to update
	project := models.Project{
		ID:          1,
		Name:        "Old Name",
		Description: "Old Description",
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
	updatedProject := models.Project{ID: project.ID}
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
	tempFile, err := ioutil.TempFile("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.RunMigration()
	sqliteDb.OpenConnectionToExistingDB()
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

	// Create a JobRepo instance
	jobRepo := job.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)

	// Create a new ProjectRepo instance
	projectRepo := NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)

	// Create a project to delete
	project := models.Project{
		ID:          1,
		Name:        "Test Project",
		Description: "Test project description",
	}

	// Insert the project into the database
	_, createErr := projectRepo.CreateOne(&project)
	if createErr != nil {
		t.Fatal("failed to create project:", createErr)
	}

	// Create a job associated with the project
	job := models.Job{
		ID:        1,
		ProjectID: project.ID,
		// Set other job properties...
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

	// Delete the project
	_, projectDeleteErr := projectRepo.DeleteOneByID(project)
	if projectDeleteErr != nil {
		t.Fatal("failed to delete project:", projectDeleteErr)
	}

	// Try to retrieve the deleted project from the database
	deletedProject := models.Project{ID: project.ID}
	getErr := projectRepo.GetOneByID(&deletedProject)
	if getErr == nil {
		t.Fatal("project should not exist after deletion")
	}
}

func Test_ProjectRepo_Count(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "project-repo-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)
	tempFile, err := ioutil.TempFile("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.RunMigration()
	sqliteDb.OpenConnectionToExistingDB()
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

	// Create a new ProjectRepo instance
	projectRepo := NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, nil)

	// Create projects for testing
	projects := []models.Project{
		{
			ID:          1,
			Name:        "Project 1",
			Description: "Description 1",
		},
		{
			ID:          2,
			Name:        "Project 2",
			Description: "Description 2",
		},
		{
			ID:          3,
			Name:        "Project 3",
			Description: "Description 3",
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
