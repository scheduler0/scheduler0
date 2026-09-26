package credential

import (
	"context"
	"os"
	"scheduler0/pkg/config"
	"scheduler0/pkg/db"
	"scheduler0/pkg/fsm"
	"scheduler0/pkg/models"
	account_repo "scheduler0/pkg/repository/account"
	job_repo "scheduler0/pkg/repository/job"
	project_repo "scheduler0/pkg/repository/project"
	"scheduler0/pkg/shared_repo"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/assert"
)

func Test_CredentialRepo_CreateOne(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "credential-repo-test",
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

	// Create a new CredentialRepo instance
	credentialRepo := NewCredentialRepo(logger, scheduler0RaftActions, scheduler0Store)

	// Create a mock credential
	mockCredential := models.Credential{
		Archived:    false,
		ApiKey:      "mock-api-key",
		ApiSecret:   "mock-api-secret",
		DateCreated: time.Now(),
		AccountId:   1,
		Scopes:      []string{"read", "write", "execute"},
	}

	// Call the CreateOne function
	id, createErr := credentialRepo.CreateOne(mockCredential)
	if createErr != nil {
		t.Fatal("failed to create a credential", createErr)
	}
	// Assert the returned ID
	assert.Equal(t, uint64(1), id)
}

func Test_CredentialRepo_UpdateOneByID(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "credential-repo-test",
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

	// Create a new CredentialRepo instance
	credentialRepo := NewCredentialRepo(logger, scheduler0RaftActions, scheduler0Store)

	// Create a mock credential
	mockCredential := models.Credential{
		ID:          1,
		Archived:    false,
		ApiKey:      "mock-api-key",
		ApiSecret:   "mock-api-secret",
		DateCreated: time.Now(),
		AccountId:   1,
		Scopes:      []string{"read", "write", "execute"},
	}

	// Insert the initial credential using CreateOne
	createdId, createErr := credentialRepo.CreateOne(mockCredential)
	if createErr != nil {
		t.Fatal("failed to create a credential", createErr)
	}

	// Update the credential
	mockCredential.Archived = true
	mockCredential.ApiKey = "updated-api-key"
	mockCredential.ApiSecret = "updated-api-secret"
	count, updateErr := credentialRepo.UpdateOneByID(mockCredential)
	if updateErr != nil {
		t.Fatal("failed to update the credential", updateErr)
	}

	// Assert the returned count
	assert.Equal(t, uint64(1), count)

	// Retrieve the updated credential
	updatedCredential := models.Credential{
		ID:        createdId,
		AccountId: 1,
	}
	err = credentialRepo.GetOneID(&updatedCredential)
	if err != nil {
		t.Fatal("failed to get the updated credential", err)
	}

	// Assert the updated fields
	assert.Equal(t, true, updatedCredential.Archived)
	assert.Equal(t, "updated-api-key", updatedCredential.ApiKey)
	assert.Equal(t, "updated-api-secret", updatedCredential.ApiSecret)
}

func Test_CredentialRepo_DeleteOneByID(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "credential-repo-test",
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

	// Create a new CredentialRepo instance
	credentialRepo := NewCredentialRepo(logger, scheduler0RaftActions, scheduler0Store)

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

	// Create a mock credential
	mockCredential := models.Credential{
		ID:          1,
		Archived:    false,
		ApiKey:      "mock-api-key",
		ApiSecret:   "mock-api-secret",
		DateCreated: time.Now(),
		AccountId:   1,
		Scopes:      []string{"read", "write", "execute"},
	}

	// Insert the initial credential using CreateOne
	_, createErr := credentialRepo.CreateOne(mockCredential)
	if createErr != nil {
		t.Fatal("failed to create a credential", createErr)
	}

	// Delete the credential
	count, deleteErr := credentialRepo.DeleteOneByID(mockCredential)
	if deleteErr != nil {
		t.Fatal("failed to delete the credential", deleteErr)
	}

	// Assert the returned count
	assert.Equal(t, uint64(1), count)

	// Try to retrieve the deleted credential
	deletedCredential := models.Credential{
		ID:        mockCredential.ID,
		AccountId: 1,
	}
	gerErr := credentialRepo.GetOneID(&deletedCredential)
	if gerErr != nil {
		// Expected: credential should not be found after deletion
		assert.Contains(t, gerErr.Error(), "credential not found")
		return
	}
	t.Fatal("expected credential not found error, but credential was retrieved")
	assert.Equal(t, deletedCredential.ApiKey, "")
	assert.Equal(t, deletedCredential.ApiSecret, "")
}

func Test_CredentialRepo_List(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "credential-repo-test",
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

	// Create a new CredentialRepo instance
	credentialRepo := NewCredentialRepo(logger, scheduler0RaftActions, scheduler0Store)

	// Create mock credentials
	mockCredentials := []models.Credential{
		{
			ID:          1,
			Archived:    false,
			ApiKey:      "mock-api-key1",
			ApiSecret:   "mock-api-secret1",
			DateCreated: time.Now(),
			AccountId:   1,
			Scopes:      []string{"read", "write", "execute"},
		},
		{
			ID:          2,
			Archived:    false,
			ApiKey:      "mock-api-key2",
			ApiSecret:   "mock-api-secret2",
			DateCreated: time.Now(),
			AccountId:   1,
			Scopes:      []string{"read", "write", "execute"},
		},
		{
			ID:          3,
			Archived:    false,
			ApiKey:      "mock-api-key3",
			ApiSecret:   "mock-api-secret3",
			DateCreated: time.Now(),
			AccountId:   1,
			Scopes:      []string{"read", "write", "execute"},
		},
	}

	// Insert the mock credentials using CreateOne
	for _, credential := range mockCredentials {
		_, createErr := credentialRepo.CreateOne(credential)
		if createErr != nil {
			t.Fatal("failed to create a credential", createErr)
		}
	}

	// Call the List function with offset, limit, and orderBy
	offset := uint64(0)
	limit := uint64(2)
	orderByColumn := "id"
	orderByDirection := "ASC"
	credentials, listErr := credentialRepo.List(offset, limit, orderByColumn, orderByDirection, 1)
	if listErr != nil {
		t.Fatal("failed to retrieve credentials", listErr)
	}

	// Assert the number of returned credentials
	assert.Equal(t, 2, len(credentials))

	// Assert the order of credentials based on orderBy
	assert.Equal(t, mockCredentials[0].ID, credentials[0].ID)
	assert.Equal(t, mockCredentials[1].ID, credentials[1].ID)

	// Test invalid order by column
	_, invalidColumnErr := credentialRepo.List(offset, limit, "invalid_column", orderByDirection, 1)
	assert.NotNil(t, invalidColumnErr)
	assert.Contains(t, invalidColumnErr.Message, "invalid order by column")

	// Test invalid order by direction
	_, invalidDirectionErr := credentialRepo.List(offset, limit, orderByColumn, "INVALID", 1)
	assert.NotNil(t, invalidDirectionErr)
	assert.Contains(t, invalidDirectionErr.Message, "invalid order by direction")
}

func Test_CredentialRepo_Count(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "credential-repo-test",
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

	// Create a new CredentialRepo instance
	credentialRepo := NewCredentialRepo(logger, scheduler0RaftActions, scheduler0Store)

	// Create mock credentials
	mockCredentials := []models.Credential{
		{
			ID:          1,
			Archived:    false,
			ApiKey:      "mock-api-key1",
			ApiSecret:   "mock-api-secret1",
			DateCreated: time.Now(),
			AccountId:   1,
			Scopes:      []string{"read", "write", "execute"},
		},
		{
			ID:          2,
			Archived:    false,
			ApiKey:      "mock-api-key2",
			ApiSecret:   "mock-api-secret2",
			DateCreated: time.Now(),
			AccountId:   1,
			Scopes:      []string{"read", "write", "execute"},
		},
		{
			ID:          3,
			Archived:    false,
			ApiKey:      "mock-api-key3",
			ApiSecret:   "mock-api-secret3",
			DateCreated: time.Now(),
			AccountId:   1,
			Scopes:      []string{"read", "write", "execute"},
		},
	}

	// Insert the mock credentials using CreateOne
	for _, credential := range mockCredentials {
		_, createErr := credentialRepo.CreateOne(credential)
		if createErr != nil {
			t.Fatal("failed to create a credential", createErr)
		}
	}

	// Call the Count function
	count, countErr := credentialRepo.Count(1)
	if countErr != nil {
		t.Fatal("failed to count credentials", countErr)
	}

	// Assert the returned count
	assert.Equal(t, uint64(len(mockCredentials)), count)
}

func Test_CredentialRepo_GetByAPIKey(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "credential-repo-test",
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

	// Create a new CredentialRepo instance
	credentialRepo := NewCredentialRepo(logger, scheduler0RaftActions, scheduler0Store)

	// Create a mock credential
	mockCredential := models.Credential{
		Archived:    false,
		ApiKey:      "mock-api-key",
		ApiSecret:   "mock-api-secret",
		DateCreated: time.Now(),
		AccountId:   1,
		Scopes:      []string{"read", "write", "execute"},
	}

	// Insert the mock credential using CreateOne
	_, createErr := credentialRepo.CreateOne(mockCredential)
	if createErr != nil {
		t.Fatal("failed to create a credential", createErr)
	}
	mockCredential.ApiSecret = ""
	mockCredential.Archived = true
	// Call the GetByAPIKey function
	getErr := credentialRepo.GetByAPIKey(&mockCredential)
	if getErr != nil {
		t.Fatal("failed to get the credential by API key", getErr)
	}

	// Assert the returned credential's fields
	assert.Equal(t, mockCredential.ID, uint64(1))
	assert.Equal(t, false, mockCredential.Archived)
	assert.Equal(t, "mock-api-key", mockCredential.ApiKey)
	assert.Equal(t, "mock-api-secret", mockCredential.ApiSecret)
}

func Test_JobRepo_GetJobsTotalCount(t *testing.T) {
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
			DateCreated:    time.Now(),
			Timezone:       "UTC",
			TimezoneOffset: 0,
			Data:           "some data",
			AccountId:      1,
			CreatedBy:      "test",
			RetryMax:       3,
			Status:         "active",
		},
		{
			ID:             2,
			ProjectID:      projectID,
			Spec:           "0 12 * * *",
			DateCreated:    time.Now(),
			Timezone:       "UTC",
			TimezoneOffset: 0,
			Data:           "some data",
			AccountId:      1,
			CreatedBy:      "test",
			RetryMax:       3,
			Status:         "active",
		},
	}

	// Call the BatchInsertJobs method to insert the jobs
	_, batchInsertErr := jobRepo.BatchInsertJobs(jobs)
	if batchInsertErr != nil {
		t.Fatal("failed to insert jobs:", batchInsertErr)
	}

	// Call the GetJobsTotalCount method
	count, getCounterr := jobRepo.GetJobsTotalCount()
	if getCounterr != nil {
		t.Fatal("failed to get jobs total count:", getCounterr)
	}

	// Assert the total count of jobs
	assert.Equal(t, uint64(2), count)
}
