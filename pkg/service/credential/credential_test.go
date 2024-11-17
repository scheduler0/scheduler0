package credential

import (
	"context"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http"
	"os"
	"scheduler0/config"
	"scheduler0/db"
	"scheduler0/fsm"
	"scheduler0/models"
	credential_repository "scheduler0/repository/credential"
	"scheduler0/secrets"
	"scheduler0/shared_repo"
	"scheduler0/utils"
	"testing"
)

func Test_CredentialService_CreateNewCredential(t *testing.T) {
	ctx := context.Background()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "async-task-manager-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	scheduler0config := config.NewScheduler0Config()
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
	scheduler0Secrets := secrets.NewScheduler0Secrets()
	credentialRepo := credential_repository.NewCredentialRepo(logger, scheduler0RaftActions, scheduler0Store)

	os.Setenv("SCHEDULER0_SECRET_KEY", "AB551DED82B93DC8035D624A625920E2121367C7538C02277D2D4DB3C0BFFE94")

	dispatcher := utils.NewDispatcher(
		ctx,
		int64(1),
		int64(1),
	)

	dispatcher.Run()

	service := NewCredentialService(ctx, logger, scheduler0Secrets, credentialRepo, dispatcher)

	id, createErr := service.CreateNewCredential(models.Credential{})
	if createErr != nil {
		t.Fatal("failed to create new credential", createErr)
	}
	assert.Equal(t, id, uint64(1))
}

func Test_CredentialService_UpdateOneCredential(t *testing.T) {
	ctx := context.Background()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "async-task-manager-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	scheduler0config := config.NewScheduler0Config()
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
	scheduler0Secrets := secrets.NewScheduler0Secrets()
	credentialRepo := credential_repository.NewCredentialRepo(logger, scheduler0RaftActions, scheduler0Store)

	os.Setenv("SCHEDULER0_SECRET_KEY", "AB551DED82B93DC8035D624A625920E2121367C7538C02277D2D4DB3C0BFFE94")

	dispatcher := utils.NewDispatcher(
		ctx,
		int64(1),
		int64(1),
	)

	dispatcher.Run()

	service := NewCredentialService(ctx, logger, scheduler0Secrets, credentialRepo, dispatcher)

	id, createErr := service.CreateNewCredential(models.Credential{})
	if createErr != nil {
		t.Fatal("failed to create new credential", createErr)
	}
	assert.Equal(t, id, uint64(1))

	_, updateErr := service.UpdateOneCredential(models.Credential{
		ApiKey:    "",
		ApiSecret: "",
	})
	if updateErr == nil {
		t.Fatal("failed to update credential should fail because credential does not exists")
	}
	_, secondUpdateErr := service.UpdateOneCredential(models.Credential{
		ID: id,
	})
	if secondUpdateErr == nil {
		t.Fatal("failed to update credential should fail because updating credential api_key and api_secret property is not allowed")
	}
	_, thirdUpdateErr := service.UpdateOneCredential(models.Credential{
		ID:        id,
		ApiSecret: "some-new-api-secret",
		ApiKey:    "some-new-api-key",
	})
	if thirdUpdateErr == nil {
		t.Fatal("failed to update credential should fail because updating credential api_key and api_secret property is not allowed")
	}
	cred, getErr := service.FindOneCredentialByID(id)
	assert.Equal(t, cred.Archived, false)
	if getErr != nil {
		t.Fatal("failed to get credential", getErr)
	}
	cred.Archived = true
	_, fourthUpdateErr := service.UpdateOneCredential(*cred)
	if fourthUpdateErr != nil {
		t.Fatal("update operation should succeed but failed with error:", fourthUpdateErr)
	}
	cred, getErr = service.FindOneCredentialByID(id)
	assert.Equal(t, cred.Archived, true)
	if getErr != nil {
		t.Fatal("failed to get credential", getErr)
	}
}

func Test_CredentialService_DeleteOneCredential(t *testing.T) {
	ctx := context.Background()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "async-task-manager-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	scheduler0config := config.NewScheduler0Config()
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
	scheduler0Secrets := secrets.NewScheduler0Secrets()
	credentialRepo := credential_repository.NewCredentialRepo(logger, scheduler0RaftActions, scheduler0Store)

	os.Setenv("SCHEDULER0_SECRET_KEY", "AB551DED82B93DC8035D624A625920E2121367C7538C02277D2D4DB3C0BFFE94")

	dispatcher := utils.NewDispatcher(
		ctx,
		int64(1),
		int64(1),
	)

	dispatcher.Run()

	service := NewCredentialService(ctx, logger, scheduler0Secrets, credentialRepo, dispatcher)

	id, createErr := service.CreateNewCredential(models.Credential{})
	if createErr != nil {
		t.Fatal("failed to create new credential", createErr)
	}
	assert.Equal(t, id, uint64(1))

	deletedCred, deleteErr := service.DeleteOneCredential(id)
	if deleteErr != nil {
		t.Fatal("failed to delete credential", deleteErr)
	}

	assert.Equal(t, deletedCred.ID, id)
}

func Test_CredentialService_ListCredentials(t *testing.T) {
	ctx := context.Background()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "async-task-manager-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	scheduler0config := config.NewScheduler0Config()
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
	scheduler0Secrets := secrets.NewScheduler0Secrets()
	credentialRepo := credential_repository.NewCredentialRepo(logger, scheduler0RaftActions, scheduler0Store)

	os.Setenv("SCHEDULER0_SECRET_KEY", "AB551DED82B93DC8035D624A625920E2121367C7538C02277D2D4DB3C0BFFE94")

	dispatcher := utils.NewDispatcher(
		ctx,
		int64(1),
		int64(1),
	)

	dispatcher.Run()

	service := NewCredentialService(ctx, logger, scheduler0Secrets, credentialRepo, dispatcher)

	credentials := []models.Credential{
		{
			ID:        1,
			ApiKey:    "api-key-1",
			ApiSecret: "api-secret-1",
		},
		{
			ID:        2,
			ApiKey:    "api-key-2",
			ApiSecret: "api-secret-2",
		},
		{
			ID:        3,
			ApiKey:    "api-key-3",
			ApiSecret: "api-secret-3",
		},
	}

	for i, credential := range credentials {
		_, createErr := service.CreateNewCredential(credential)
		if createErr != nil {
			t.Fatalf("Failed to create credential: %v", createErr)
		}
		cred, getErr := service.FindOneCredentialByID(credential.ID)
		if getErr != nil {
			t.Fatalf("Failed to create credential: %v", getErr)
		}
		credentials[i].ApiKey = cred.ApiKey
		credentials[i].ApiSecret = cred.ApiSecret
	}

	// Call the ListCredentials method
	offset := uint64(0)
	limit := uint64(10)
	orderBy := "id"
	result, listErr := service.ListCredentials(offset, limit, orderBy)
	if listErr != nil {
		t.Fatalf("Failed to list credentials: %v", listErr)
	}

	// Assert the number of retrieved credentials
	expectedCount := len(credentials)
	assert.Equal(t, expectedCount, len(result.Data))

	// Assert the correctness of the retrieved credentials
	credentialMap := make(map[uint64]models.Credential)
	for _, credential := range credentials {
		credentialMap[credential.ID] = credential
	}

	for _, credential := range result.Data {
		expectedCredential, ok := credentialMap[credential.ID]
		assert.True(t, ok, "Unexpected credential with ID:", credential.ID)
		assert.Equal(t, expectedCredential.ApiKey, credential.ApiKey)
		assert.Equal(t, expectedCredential.ApiSecret, credential.ApiSecret)
	}

	// Assert the total count, offset, and limit
	assert.Equal(t, uint64(expectedCount), result.Total)
	assert.Equal(t, offset, result.Offset)
	assert.Equal(t, limit, result.Limit)
}

func Test_CredentialService_ValidateServerAPIKey(t *testing.T) {
	ctx := context.Background()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "async-task-manager-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	scheduler0config := config.NewScheduler0Config()
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
	scheduler0Secrets := secrets.NewScheduler0Secrets()
	credentialRepo := credential_repository.NewCredentialRepo(logger, scheduler0RaftActions, scheduler0Store)

	os.Setenv("SCHEDULER0_SECRET_KEY", "AB551DED82B93DC8035D624A625920E2121367C7538C02277D2D4DB3C0BFFE94")

	dispatcher := utils.NewDispatcher(
		ctx,
		int64(1),
		int64(1),
	)

	dispatcher.Run()

	service := NewCredentialService(ctx, logger, scheduler0Secrets, credentialRepo, dispatcher)

	credential := models.Credential{
		ID:        1,
		ApiKey:    "api-key-1",
		ApiSecret: "api-secret-1",
	}

	_, createErr := service.CreateNewCredential(credential)
	if createErr != nil {
		t.Fatalf("Failed to create credential: %v", createErr)
	}
	cred, getErr := service.FindOneCredentialByID(credential.ID)
	if getErr != nil {
		t.Fatalf("Failed to create credential: %v", getErr)
	}

	// Call the ValidateServerAPIKey method with valid credentials
	isValid, validErr := service.ValidateServerAPIKey(cred.ApiKey, cred.ApiSecret)
	assert.True(t, isValid)
	assert.Nil(t, validErr)

	// Call the ValidateServerAPIKey method with invalid credentials
	invalidApiKey := "invalid-api-key"
	invalidApiSecret := "invalid-api-secret"
	isValid, invalidErr := service.ValidateServerAPIKey(invalidApiKey, invalidApiSecret)
	assert.False(t, isValid)
	assert.NotNil(t, invalidErr)
	assert.Equal(t, http.StatusNotFound, invalidErr.Type)

	isValid, invalidErr = service.ValidateServerAPIKey(cred.ApiKey, invalidApiSecret)
	assert.False(t, isValid)
	assert.Nil(t, invalidErr)
}
