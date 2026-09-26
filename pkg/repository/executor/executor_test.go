package executor

import (
	"context"
	"fmt"
	"os"
	"scheduler0/pkg/config"
	"scheduler0/pkg/constants"
	"scheduler0/pkg/db"
	"scheduler0/pkg/fsm"
	"scheduler0/pkg/models"
	account_repo "scheduler0/pkg/repository/account"
	"scheduler0/pkg/secrets"
	"scheduler0/pkg/shared_repo"
	"scheduler0/pkg/utils"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/assert"
)

func setupTestFSMStore(t *testing.T) (fsm.Scheduler0RaftStore, fsm.Scheduler0RaftActions, hclog.Logger, func()) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "executor-repo-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)
	tempFile, err := os.CreateTemp("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tempFile.Close()
	sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.RunMigration(logger)
	sqliteDb.OpenConnectionToExistingDB()
	scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, scheduler0config, sqliteDb, nil, nil, nil, nil, sharedRepo)

	raftConf := raft.DefaultConfig()
	raftConf.HeartbeatTimeout = 50 * time.Millisecond
	raftConf.ElectionTimeout = 50 * time.Millisecond
	raftConf.CommitTimeout = 50 * time.Millisecond
	raftConf.LeaderLeaseTimeout = 25 * time.Millisecond

	cluster := raft.MakeClusterCustom(t, &raft.MakeClusterOpts{
		Peers:          1,
		Bootstrap:      true,
		Conf:           raftConf,
		ConfigStoreFSM: false,
		MakeFSMFunc: func() raft.FSM {
			return scheduler0Store.GetFSM()
		},
	})
	cluster.FullyConnect()
	scheduler0Store.UpdateRaft(cluster.Leader())

	cleanup := func() {
		cluster.Close()
		os.Remove(tempFile.Name())
	}

	return scheduler0Store, scheduler0RaftActions, logger, cleanup
}

func createTestAccount(t *testing.T, accountRepo account_repo.AccountRepository, accountId uint64, accountName string) {
	account := &models.Account{
		ID:   accountId,
		Name: accountName,
	}
	_, createAccountErr := accountRepo.CreateAccount(account)
	if createAccountErr != nil {
		t.Fatalf("Failed to create account %d: %v", accountId, createAccountErr)
	}
}

func TestNewExecutorRepo(t *testing.T) {
	t.Parallel()
	scheduler0Store, scheduler0RaftActions, logger, cleanup := setupTestFSMStore(t)
	defer cleanup()

	repo := NewExecutorRepo(logger, scheduler0RaftActions, scheduler0Store, nil)
	assert.NotNil(t, repo)
}

func TestCreateOne_Success(t *testing.T) {
	t.Parallel()
	scheduler0Store, scheduler0RaftActions, logger, cleanup := setupTestFSMStore(t)
	defer cleanup()

	accountRepo := account_repo.NewAccountRepository(context.TODO(), logger, scheduler0RaftActions, scheduler0Store)
	createTestAccount(t, accountRepo, 1, "Test Account")

	executorRepo := NewExecutorRepo(logger, scheduler0RaftActions, scheduler0Store, nil)

	executor := models.JobExecutor{
		Name:          "Test Executor",
		Type:          "webhook_url",
		WebhookUrl:    "https://example.com/webhook",
		WebhookMethod: "POST",
		AccountId:     1,
	}

	id, err := executorRepo.CreateOne(executor)
	assert.Nil(t, err)
	assert.Equal(t, uint64(1), id)
}

func TestCreateOne_InvalidAccountId(t *testing.T) {
	t.Parallel()
	scheduler0Store, scheduler0RaftActions, logger, cleanup := setupTestFSMStore(t)
	defer cleanup()

	executorRepo := NewExecutorRepo(logger, scheduler0RaftActions, scheduler0Store, nil)

	executor := models.JobExecutor{
		Name:      "Test Executor",
		Type:      "webhook_url",
		AccountId: 0,
	}

	_, err := executorRepo.CreateOne(executor)
	assert.NotNil(t, err)
	assert.Equal(t, 400, err.Type)
	assert.Contains(t, err.Message, "account id is required")
}

func TestCreateOne_InvalidType(t *testing.T) {
	t.Parallel()
	scheduler0Store, scheduler0RaftActions, logger, cleanup := setupTestFSMStore(t)
	defer cleanup()

	accountRepo := account_repo.NewAccountRepository(context.TODO(), logger, scheduler0RaftActions, scheduler0Store)
	createTestAccount(t, accountRepo, 1, "Test Account")

	executorRepo := NewExecutorRepo(logger, scheduler0RaftActions, scheduler0Store, nil)

	executor := models.JobExecutor{
		Name:      "Test Executor",
		Type:      "invalid_type",
		AccountId: 1,
	}

	_, err := executorRepo.CreateOne(executor)
	assert.NotNil(t, err)
	assert.Equal(t, 400, err.Type)
	assert.Contains(t, err.Message, "invalid job executor type")
}

func TestCreateOne_WebhookUrlRequired(t *testing.T) {
	t.Parallel()
	scheduler0Store, scheduler0RaftActions, logger, cleanup := setupTestFSMStore(t)
	defer cleanup()

	accountRepo := account_repo.NewAccountRepository(context.TODO(), logger, scheduler0RaftActions, scheduler0Store)
	createTestAccount(t, accountRepo, 1, "Test Account")

	executorRepo := NewExecutorRepo(logger, scheduler0RaftActions, scheduler0Store, nil)

	executor := models.JobExecutor{
		Name:      "Test Executor",
		Type:      "webhook_url",
		AccountId: 1,
	}

	_, err := executorRepo.CreateOne(executor)
	assert.NotNil(t, err)
	assert.Equal(t, 400, err.Type)
	assert.Contains(t, err.Message, "webhook url is required")
}

func TestGetOneByID_Success(t *testing.T) {
	t.Parallel()
	scheduler0Store, scheduler0RaftActions, logger, cleanup := setupTestFSMStore(t)
	defer cleanup()

	accountRepo := account_repo.NewAccountRepository(context.TODO(), logger, scheduler0RaftActions, scheduler0Store)
	createTestAccount(t, accountRepo, 1, "Test Account")

	executorRepo := NewExecutorRepo(logger, scheduler0RaftActions, scheduler0Store, nil)

	executor := models.JobExecutor{
		Name:          "Test Executor",
		Type:          "webhook_url",
		WebhookUrl:    "https://example.com/webhook",
		WebhookMethod: "POST",
		AccountId:     1,
	}

	createdId, createErr := executorRepo.CreateOne(executor)
	assert.Nil(t, createErr)

	retrievedExecutor, getErr := executorRepo.GetOneByID(createdId, 1)
	assert.Nil(t, getErr)
	assert.NotNil(t, retrievedExecutor)
	assert.Equal(t, createdId, retrievedExecutor.ID)
	assert.Equal(t, "Test Executor", retrievedExecutor.Name)
	assert.Equal(t, "webhook_url", retrievedExecutor.Type)
	assert.Equal(t, "https://example.com/webhook", retrievedExecutor.WebhookUrl)
}

func TestPayloadAggregation_RoundTrip(t *testing.T) {
	t.Parallel()
	scheduler0Store, scheduler0RaftActions, logger, cleanup := setupTestFSMStore(t)
	defer cleanup()

	accountRepo := account_repo.NewAccountRepository(context.TODO(), logger, scheduler0RaftActions, scheduler0Store)
	createTestAccount(t, accountRepo, 1, "Test Account")

	executorRepo := NewExecutorRepo(logger, scheduler0RaftActions, scheduler0Store, nil)

	aggregatingId, createErr := executorRepo.CreateOne(models.JobExecutor{
		Name:               "Aggregating Executor",
		Type:               "webhook_url",
		WebhookUrl:         "https://example.com/aggregating",
		WebhookMethod:      "POST",
		PayloadAggregation: true,
		AccountId:          1,
	})
	assert.Nil(t, createErr)

	nonAggregatingId, createErr := executorRepo.CreateOne(models.JobExecutor{
		Name:          "Plain Executor",
		Type:          "webhook_url",
		WebhookUrl:    "https://example.com/plain",
		WebhookMethod: "POST",
		AccountId:     1,
	})
	assert.Nil(t, createErr)

	aggregating, getErr := executorRepo.GetOneByID(aggregatingId, 1)
	assert.Nil(t, getErr)
	assert.True(t, aggregating.PayloadAggregation, "aggregation flag should persist as true")

	plain, getErr := executorRepo.GetOneByID(nonAggregatingId, 1)
	assert.Nil(t, getErr)
	assert.False(t, plain.PayloadAggregation, "aggregation flag should default to false")

	listed, listErr := executorRepo.List(0, 10, "id", "ASC", 1)
	assert.Nil(t, listErr)
	byId := map[uint64]models.JobExecutor{}
	for _, e := range listed {
		byId[e.ID] = e
	}
	assert.True(t, byId[aggregatingId].PayloadAggregation)
	assert.False(t, byId[nonAggregatingId].PayloadAggregation)

	batched, batchErr := executorRepo.BatchGetByIds([]uint64{aggregatingId, nonAggregatingId})
	assert.Nil(t, batchErr)
	byId = map[uint64]models.JobExecutor{}
	for _, e := range batched {
		byId[e.ID] = e
	}
	assert.True(t, byId[aggregatingId].PayloadAggregation)
	assert.False(t, byId[nonAggregatingId].PayloadAggregation)

	_, updateErr := executorRepo.UpdateOneByID(models.JobExecutor{
		ID:                 aggregatingId,
		Name:               "Aggregating Executor",
		Type:               "webhook_url",
		WebhookUrl:         "https://example.com/aggregating",
		WebhookMethod:      "POST",
		PayloadAggregation: false,
		AccountId:          1,
	})
	assert.Nil(t, updateErr)

	afterUpdate, getErr := executorRepo.GetOneByID(aggregatingId, 1)
	assert.Nil(t, getErr)
	assert.False(t, afterUpdate.PayloadAggregation, "aggregation flag should be disabled after update")
}

func TestGetOneByID_InvalidAccountId(t *testing.T) {
	t.Parallel()
	scheduler0Store, scheduler0RaftActions, logger, cleanup := setupTestFSMStore(t)
	defer cleanup()

	executorRepo := NewExecutorRepo(logger, scheduler0RaftActions, scheduler0Store, nil)

	_, err := executorRepo.GetOneByID(1, 0)
	assert.NotNil(t, err)
	assert.Equal(t, 400, err.Type)
	assert.Contains(t, err.Message, "account id is required")
}

func TestGetOneByID_NotFound(t *testing.T) {
	t.Parallel()
	scheduler0Store, scheduler0RaftActions, logger, cleanup := setupTestFSMStore(t)
	defer cleanup()

	accountRepo := account_repo.NewAccountRepository(context.TODO(), logger, scheduler0RaftActions, scheduler0Store)
	createTestAccount(t, accountRepo, 1, "Test Account")

	executorRepo := NewExecutorRepo(logger, scheduler0RaftActions, scheduler0Store, nil)

	executor, err := executorRepo.GetOneByID(999, 1)
	assert.Nil(t, err)
	assert.NotNil(t, executor)
	assert.Equal(t, uint64(0), executor.ID)
}

func TestUpdateOneByID_Success(t *testing.T) {
	t.Parallel()
	scheduler0Store, scheduler0RaftActions, logger, cleanup := setupTestFSMStore(t)
	defer cleanup()

	accountRepo := account_repo.NewAccountRepository(context.TODO(), logger, scheduler0RaftActions, scheduler0Store)
	createTestAccount(t, accountRepo, 1, "Test Account")

	executorRepo := NewExecutorRepo(logger, scheduler0RaftActions, scheduler0Store, nil)

	executor := models.JobExecutor{
		Name:          "Test Executor",
		Type:          "webhook_url",
		WebhookUrl:    "https://example.com/webhook",
		WebhookMethod: "POST",
		AccountId:     1,
	}

	createdId, createErr := executorRepo.CreateOne(executor)
	assert.Nil(t, createErr)

	updatedExecutor := models.JobExecutor{
		ID:            createdId,
		Name:          "Updated Executor",
		Type:          "webhook_url",
		WebhookUrl:    "https://example.com/webhook-updated",
		WebhookMethod: "PUT",
		AccountId:     1,
	}

	count, updateErr := executorRepo.UpdateOneByID(updatedExecutor)
	assert.Nil(t, updateErr)
	assert.Equal(t, uint64(1), count)

	retrievedExecutor, getErr := executorRepo.GetOneByID(createdId, 1)
	assert.Nil(t, getErr)
	assert.Equal(t, "Updated Executor", retrievedExecutor.Name)
	assert.Equal(t, "https://example.com/webhook-updated", retrievedExecutor.WebhookUrl)
}

func TestUpdateOneByID_InvalidID(t *testing.T) {
	t.Parallel()
	scheduler0Store, scheduler0RaftActions, logger, cleanup := setupTestFSMStore(t)
	defer cleanup()

	accountRepo := account_repo.NewAccountRepository(context.TODO(), logger, scheduler0RaftActions, scheduler0Store)
	createTestAccount(t, accountRepo, 1, "Test Account")

	executorRepo := NewExecutorRepo(logger, scheduler0RaftActions, scheduler0Store, nil)

	executor := models.JobExecutor{
		ID:        0,
		Name:      "Test Executor",
		Type:      "webhook_url",
		AccountId: 1,
	}

	_, err := executorRepo.UpdateOneByID(executor)
	assert.NotNil(t, err)
	assert.Equal(t, 400, err.Type)
	assert.Contains(t, err.Message, "job executor id is required")
}

func TestUpdateOneByID_InvalidAccountId(t *testing.T) {
	t.Parallel()
	scheduler0Store, scheduler0RaftActions, logger, cleanup := setupTestFSMStore(t)
	defer cleanup()

	executorRepo := NewExecutorRepo(logger, scheduler0RaftActions, scheduler0Store, nil)

	executor := models.JobExecutor{
		ID:        1,
		Name:      "Test Executor",
		Type:      "webhook_url",
		AccountId: 0,
	}

	_, err := executorRepo.UpdateOneByID(executor)
	assert.NotNil(t, err)
	assert.Equal(t, 400, err.Type)
	assert.Contains(t, err.Message, "account id is required")
}

func TestDeleteOneByID_Success(t *testing.T) {
	t.Parallel()
	scheduler0Store, scheduler0RaftActions, logger, cleanup := setupTestFSMStore(t)
	defer cleanup()

	accountRepo := account_repo.NewAccountRepository(context.TODO(), logger, scheduler0RaftActions, scheduler0Store)
	createTestAccount(t, accountRepo, 1, "Test Account")

	executorRepo := NewExecutorRepo(logger, scheduler0RaftActions, scheduler0Store, nil)

	executor := models.JobExecutor{
		Name:          "Test Executor",
		Type:          "webhook_url",
		WebhookUrl:    "https://example.com/webhook",
		WebhookMethod: "POST",
		AccountId:     1,
	}

	createdId, createErr := executorRepo.CreateOne(executor)
	assert.Nil(t, createErr)

	deleteExecutor := models.JobExecutor{
		ID:        createdId,
		AccountId: 1,
	}

	count, deleteErr := executorRepo.DeleteOneByID(deleteExecutor)
	assert.Nil(t, deleteErr)
	assert.Equal(t, uint64(1), count)
}

func TestDeleteOneByID_InvalidAccountId(t *testing.T) {
	t.Parallel()
	scheduler0Store, scheduler0RaftActions, logger, cleanup := setupTestFSMStore(t)
	defer cleanup()

	executorRepo := NewExecutorRepo(logger, scheduler0RaftActions, scheduler0Store, nil)

	executor := models.JobExecutor{
		ID:        1,
		AccountId: 0,
	}

	_, err := executorRepo.DeleteOneByID(executor)
	assert.NotNil(t, err)
	assert.Equal(t, 400, err.Type)
	assert.Contains(t, err.Message, "account id is required")
}

func TestList_Success(t *testing.T) {
	t.Parallel()
	scheduler0Store, scheduler0RaftActions, logger, cleanup := setupTestFSMStore(t)
	defer cleanup()

	accountRepo := account_repo.NewAccountRepository(context.TODO(), logger, scheduler0RaftActions, scheduler0Store)
	createTestAccount(t, accountRepo, 1, "Test Account")

	executorRepo := NewExecutorRepo(logger, scheduler0RaftActions, scheduler0Store, nil)

	executors := []models.JobExecutor{
		{
			Name:          "Executor 1",
			Type:          "webhook_url",
			WebhookUrl:    "https://example.com/webhook1",
			WebhookMethod: "POST",
			AccountId:     1,
		},
		{
			Name:          "Executor 2",
			Type:          "webhook_url",
			WebhookUrl:    "https://example.com/webhook2",
			WebhookMethod: "GET",
			AccountId:     1,
		},
		{
			Name:          "Executor 3",
			Type:          "webhook_url",
			WebhookUrl:    "https://example.com/webhook3",
			WebhookMethod: "PUT",
			AccountId:     1,
		},
	}

	for _, executor := range executors {
		_, createErr := executorRepo.CreateOne(executor)
		assert.Nil(t, createErr)
	}

	list, listErr := executorRepo.List(0, 10, "id", "ASC", 1)
	assert.Nil(t, listErr)
	assert.Equal(t, 3, len(list))
}

func TestList_InvalidAccountId(t *testing.T) {
	t.Parallel()
	scheduler0Store, scheduler0RaftActions, logger, cleanup := setupTestFSMStore(t)
	defer cleanup()

	executorRepo := NewExecutorRepo(logger, scheduler0RaftActions, scheduler0Store, nil)

	_, err := executorRepo.List(0, 10, "id", "ASC", 0)
	assert.NotNil(t, err)
	assert.Equal(t, 400, err.Type)
	assert.Contains(t, err.Message, "account id is required")
}

func TestList_InvalidOrderByColumn(t *testing.T) {
	t.Parallel()
	scheduler0Store, scheduler0RaftActions, logger, cleanup := setupTestFSMStore(t)
	defer cleanup()

	accountRepo := account_repo.NewAccountRepository(context.TODO(), logger, scheduler0RaftActions, scheduler0Store)
	createTestAccount(t, accountRepo, 1, "Test Account")

	executorRepo := NewExecutorRepo(logger, scheduler0RaftActions, scheduler0Store, nil)

	_, err := executorRepo.List(0, 10, "invalid_column", "ASC", 1)
	assert.NotNil(t, err)
	assert.Equal(t, 400, err.Type)
	assert.Contains(t, err.Message, "invalid order by column")
}

func TestList_InvalidOrderByDirection(t *testing.T) {
	t.Parallel()
	scheduler0Store, scheduler0RaftActions, logger, cleanup := setupTestFSMStore(t)
	defer cleanup()

	accountRepo := account_repo.NewAccountRepository(context.TODO(), logger, scheduler0RaftActions, scheduler0Store)
	createTestAccount(t, accountRepo, 1, "Test Account")

	executorRepo := NewExecutorRepo(logger, scheduler0RaftActions, scheduler0Store, nil)

	_, err := executorRepo.List(0, 10, "id", "INVALID", 1)
	assert.NotNil(t, err)
	assert.Equal(t, 400, err.Type)
	assert.Contains(t, err.Message, "invalid order by direction")
}

func TestBatchGetByIds_Success(t *testing.T) {
	t.Parallel()
	scheduler0Store, scheduler0RaftActions, logger, cleanup := setupTestFSMStore(t)
	defer cleanup()

	accountRepo := account_repo.NewAccountRepository(context.TODO(), logger, scheduler0RaftActions, scheduler0Store)
	createTestAccount(t, accountRepo, 1, "Test Account")

	executorRepo := NewExecutorRepo(logger, scheduler0RaftActions, scheduler0Store, nil)

	executor1 := models.JobExecutor{
		Name:          "Executor 1",
		Type:          "webhook_url",
		WebhookUrl:    "https://example.com/webhook1",
		WebhookMethod: "POST",
		AccountId:     1,
	}
	id1, _ := executorRepo.CreateOne(executor1)

	executor2 := models.JobExecutor{
		Name:          "Executor 2",
		Type:          "webhook_url",
		WebhookUrl:    "https://example.com/webhook2",
		WebhookMethod: "GET",
		AccountId:     1,
	}
	id2, _ := executorRepo.CreateOne(executor2)

	executors, err := executorRepo.BatchGetByIds([]uint64{id1, id2})
	assert.Nil(t, err)
	assert.Equal(t, 2, len(executors))
}

func TestBatchGetByIds_EmptyList(t *testing.T) {
	t.Parallel()
	scheduler0Store, scheduler0RaftActions, logger, cleanup := setupTestFSMStore(t)
	defer cleanup()

	executorRepo := NewExecutorRepo(logger, scheduler0RaftActions, scheduler0Store, nil)

	executors, err := executorRepo.BatchGetByIds([]uint64{})
	assert.Nil(t, err)
	assert.Empty(t, executors)
}

func TestCount_Success(t *testing.T) {
	t.Parallel()
	scheduler0Store, scheduler0RaftActions, logger, cleanup := setupTestFSMStore(t)
	defer cleanup()

	accountRepo := account_repo.NewAccountRepository(context.TODO(), logger, scheduler0RaftActions, scheduler0Store)
	createTestAccount(t, accountRepo, 1, "Test Account")

	executorRepo := NewExecutorRepo(logger, scheduler0RaftActions, scheduler0Store, nil)

	executors := []models.JobExecutor{
		{
			Name:          "Executor 1",
			Type:          "webhook_url",
			WebhookUrl:    "https://example.com/webhook1",
			WebhookMethod: "POST",
			AccountId:     1,
		},
		{
			Name:          "Executor 2",
			Type:          "webhook_url",
			WebhookUrl:    "https://example.com/webhook2",
			WebhookMethod: "GET",
			AccountId:     1,
		},
	}

	for _, executor := range executors {
		_, createErr := executorRepo.CreateOne(executor)
		assert.Nil(t, createErr)
	}

	count, countErr := executorRepo.Count(1)
	assert.Nil(t, countErr)
	assert.Equal(t, uint64(2), count)
}

func TestCount_InvalidAccountId(t *testing.T) {
	t.Parallel()
	scheduler0Store, scheduler0RaftActions, logger, cleanup := setupTestFSMStore(t)
	defer cleanup()

	executorRepo := NewExecutorRepo(logger, scheduler0RaftActions, scheduler0Store, nil)

	_, err := executorRepo.Count(0)
	assert.NotNil(t, err)
	assert.Equal(t, 400, err.Type)
	assert.Contains(t, err.Message, "account id is required")
}

func TestCreateOne_EdgeCase_EmptyName(t *testing.T) {
	t.Parallel()
	scheduler0Store, scheduler0RaftActions, logger, cleanup := setupTestFSMStore(t)
	defer cleanup()

	accountRepo := account_repo.NewAccountRepository(context.TODO(), logger, scheduler0RaftActions, scheduler0Store)
	createTestAccount(t, accountRepo, 1, "Test Account")

	executorRepo := NewExecutorRepo(logger, scheduler0RaftActions, scheduler0Store, nil)

	executor := models.JobExecutor{
		Name:          "",
		Type:          "webhook_url",
		WebhookUrl:    "https://example.com/webhook",
		WebhookMethod: "POST",
		AccountId:     1,
	}

	_, err := executorRepo.CreateOne(executor)

	assert.NotNil(t, err)
	assert.Equal(t, 400, err.Type)
	assert.Contains(t, err.Message, "executor name is required")
}

func TestCreateOne_EdgeCase_WhitespaceOnlyName(t *testing.T) {
	t.Parallel()
	scheduler0Store, scheduler0RaftActions, logger, cleanup := setupTestFSMStore(t)
	defer cleanup()

	accountRepo := account_repo.NewAccountRepository(context.TODO(), logger, scheduler0RaftActions, scheduler0Store)
	createTestAccount(t, accountRepo, 1, "Test Account")

	executorRepo := NewExecutorRepo(logger, scheduler0RaftActions, scheduler0Store, nil)

	executor := models.JobExecutor{
		Name:          "   ",
		Type:          "webhook_url",
		WebhookUrl:    "https://example.com/webhook",
		WebhookMethod: "POST",
		AccountId:     1,
	}

	_, err := executorRepo.CreateOne(executor)

	assert.NotNil(t, err)
	assert.Equal(t, 400, err.Type)
	assert.Contains(t, err.Message, "executor name is required")
}

func TestCreateOne_EdgeCase_WhitespaceOnlyWebhookUrl(t *testing.T) {
	t.Parallel()
	scheduler0Store, scheduler0RaftActions, logger, cleanup := setupTestFSMStore(t)
	defer cleanup()

	accountRepo := account_repo.NewAccountRepository(context.TODO(), logger, scheduler0RaftActions, scheduler0Store)
	createTestAccount(t, accountRepo, 1, "Test Account")

	executorRepo := NewExecutorRepo(logger, scheduler0RaftActions, scheduler0Store, nil)

	executor := models.JobExecutor{
		Name:          "Test Executor",
		Type:          "webhook_url",
		WebhookUrl:    "   ",
		WebhookMethod: "POST",
		AccountId:     1,
	}

	_, err := executorRepo.CreateOne(executor)

	assert.NotNil(t, err)
	assert.Equal(t, 400, err.Type)
	assert.Contains(t, err.Message, "webhook url is required")
}

func TestGetOneByID_EdgeCase_ExecutorIDZero(t *testing.T) {
	t.Parallel()
	scheduler0Store, scheduler0RaftActions, logger, cleanup := setupTestFSMStore(t)
	defer cleanup()

	accountRepo := account_repo.NewAccountRepository(context.TODO(), logger, scheduler0RaftActions, scheduler0Store)
	createTestAccount(t, accountRepo, 1, "Test Account")

	executorRepo := NewExecutorRepo(logger, scheduler0RaftActions, scheduler0Store, nil)

	executor, err := executorRepo.GetOneByID(0, 1)

	assert.Nil(t, err)
	assert.NotNil(t, executor)
	assert.Equal(t, uint64(0), executor.ID)
}

func TestBatchGetByIds_EdgeCase_SomeNonExistent(t *testing.T) {
	t.Parallel()
	scheduler0Store, scheduler0RaftActions, logger, cleanup := setupTestFSMStore(t)
	defer cleanup()

	accountRepo := account_repo.NewAccountRepository(context.TODO(), logger, scheduler0RaftActions, scheduler0Store)
	createTestAccount(t, accountRepo, 1, "Test Account")

	executorRepo := NewExecutorRepo(logger, scheduler0RaftActions, scheduler0Store, nil)

	executor1 := models.JobExecutor{
		Name:          "Executor 1",
		Type:          "webhook_url",
		WebhookUrl:    "https://example.com/webhook1",
		WebhookMethod: "POST",
		AccountId:     1,
	}
	id1, _ := executorRepo.CreateOne(executor1)

	executors, err := executorRepo.BatchGetByIds([]uint64{id1, 99999, 88888})

	assert.Nil(t, err)
	assert.Len(t, executors, 1, "Should return only existing executors")
	assert.Equal(t, id1, executors[0].ID)
}

func TestBatchGetByIds_EdgeCase_AllNonExistent(t *testing.T) {
	t.Parallel()
	scheduler0Store, scheduler0RaftActions, logger, cleanup := setupTestFSMStore(t)
	defer cleanup()

	executorRepo := NewExecutorRepo(logger, scheduler0RaftActions, scheduler0Store, nil)

	executors, err := executorRepo.BatchGetByIds([]uint64{99999, 88888, 77777})

	assert.Nil(t, err)
	assert.Empty(t, executors, "Should return empty list when no executors exist")
}

func TestUpdateOneByID_EdgeCase_NonExistentExecutor(t *testing.T) {
	t.Parallel()
	scheduler0Store, scheduler0RaftActions, logger, cleanup := setupTestFSMStore(t)
	defer cleanup()

	accountRepo := account_repo.NewAccountRepository(context.TODO(), logger, scheduler0RaftActions, scheduler0Store)
	createTestAccount(t, accountRepo, 1, "Test Account")

	executorRepo := NewExecutorRepo(logger, scheduler0RaftActions, scheduler0Store, nil)

	executor := models.JobExecutor{
		ID:            99999,
		Name:          "Updated Executor",
		Type:          "webhook_url",
		WebhookUrl:    "https://example.com/webhook",
		WebhookMethod: "POST",
		AccountId:     1,
	}

	count, updateErr := executorRepo.UpdateOneByID(executor)

	assert.Nil(t, updateErr)
	assert.Equal(t, uint64(0), count, "Should return 0 rows affected for non-existent executor")
}

func TestDeleteOneByID_EdgeCase_NonExistentExecutor(t *testing.T) {
	t.Parallel()
	scheduler0Store, scheduler0RaftActions, logger, cleanup := setupTestFSMStore(t)
	defer cleanup()

	accountRepo := account_repo.NewAccountRepository(context.TODO(), logger, scheduler0RaftActions, scheduler0Store)
	createTestAccount(t, accountRepo, 1, "Test Account")

	executorRepo := NewExecutorRepo(logger, scheduler0RaftActions, scheduler0Store, nil)

	executor := models.JobExecutor{
		ID:        99999,
		AccountId: 1,
	}

	count, deleteErr := executorRepo.DeleteOneByID(executor)

	assert.Nil(t, deleteErr)
	assert.Equal(t, uint64(0), count, "Should return 0 rows affected for non-existent executor")
}

func TestList_EdgeCase_ZeroLimit(t *testing.T) {
	t.Parallel()
	scheduler0Store, scheduler0RaftActions, logger, cleanup := setupTestFSMStore(t)
	defer cleanup()

	accountRepo := account_repo.NewAccountRepository(context.TODO(), logger, scheduler0RaftActions, scheduler0Store)
	createTestAccount(t, accountRepo, 1, "Test Account")

	executorRepo := NewExecutorRepo(logger, scheduler0RaftActions, scheduler0Store, nil)

	executors, listErr := executorRepo.List(0, 0, "id", "ASC", 1)

	assert.Nil(t, listErr)
	assert.Empty(t, executors, "Should return empty list with zero limit")
}

func TestUpdateOneByID_EdgeCase_InvalidType(t *testing.T) {
	t.Parallel()
	scheduler0Store, scheduler0RaftActions, logger, cleanup := setupTestFSMStore(t)
	defer cleanup()

	accountRepo := account_repo.NewAccountRepository(context.TODO(), logger, scheduler0RaftActions, scheduler0Store)
	createTestAccount(t, accountRepo, 1, "Test Account")

	executorRepo := NewExecutorRepo(logger, scheduler0RaftActions, scheduler0Store, nil)

	executor := models.JobExecutor{
		Name:          "Test Executor",
		Type:          "webhook_url",
		WebhookUrl:    "https://example.com/webhook",
		WebhookMethod: "POST",
		AccountId:     1,
	}
	createdId, _ := executorRepo.CreateOne(executor)

	updatedExecutor := models.JobExecutor{
		ID:            createdId,
		Name:          "Updated Executor",
		Type:          "invalid_type",
		WebhookUrl:    "https://example.com/webhook",
		WebhookMethod: "POST",
		AccountId:     1,
	}

	_, updateErr := executorRepo.UpdateOneByID(updatedExecutor)

	assert.NotNil(t, updateErr)
	assert.Equal(t, 400, updateErr.Type)
	assert.Contains(t, updateErr.Message, "invalid job executor type")
}

func TestCloudKeysAreEncryptedAtRest(t *testing.T) {
	const testSecretKey = "AB551DED82B93DC8035D624A625920E2121367C7538C02277D2D4DB3C0BFFE94"
	t.Setenv("SCHEDULER0_SECRET_KEY", testSecretKey)

	scheduler0Store, scheduler0RaftActions, logger, cleanup := setupTestFSMStore(t)
	defer cleanup()

	accountRepo := account_repo.NewAccountRepository(context.TODO(), logger, scheduler0RaftActions, scheduler0Store)
	createTestAccount(t, accountRepo, 1, "Test Account")

	scheduler0Secrets := secrets.NewScheduler0Secrets()
	if loaded := scheduler0Secrets.GetSecrets(); loaded.SecretKey != testSecretKey {
		t.Skipf("scheduler0 secrets cache already populated by another test (got SecretKey=%q); skipping encryption-at-rest assertion", loaded.SecretKey)
	}

	executorRepo := NewExecutorRepo(logger, scheduler0RaftActions, scheduler0Store, scheduler0Secrets)

	plainApiKey := "plaintext-cloud-api-key"
	plainApiSecret := "plaintext-cloud-api-secret"

	createErr := func() *struct{} {
		_, err := executorRepo.CreateOne(models.JobExecutor{
			Name:             "Cloud Executor",
			Type:             "cloud_function",
			CloudProvider:    "aws",
			Region:           "us-east-1",
			CloudResourceUrl: "arn:aws:lambda:us-east-1:000000000000:function:test",
			CloudApiKey:      plainApiKey,
			CloudApiSecret:   plainApiSecret,
			AccountId:        1,
		})
		assert.Nil(t, err)
		return nil
	}()
	_ = createErr

	retrieved, getErr := executorRepo.GetOneByID(1, 1)
	assert.Nil(t, getErr)
	assert.Equal(t, plainApiKey, retrieved.CloudApiKey, "cloud api key should be decrypted on read")
	assert.Equal(t, plainApiSecret, retrieved.CloudApiSecret, "cloud api secret should be decrypted on read")

	scheduler0Store.GetDataStore().ConnectionLock()
	defer scheduler0Store.GetDataStore().ConnectionUnlock()
	row := scheduler0Store.GetDataStore().GetOpenConnection().QueryRow(
		fmt.Sprintf("SELECT %s, %s FROM %s WHERE %s = ?",
			constants.JobExecutorCloudApiKey,
			constants.JobExecutorCloudApiSecret,
			constants.JobExecutorTableName,
			constants.JobExecutorIdColumn,
		), 1)
	var storedApiKey, storedApiSecret string
	if err := row.Scan(&storedApiKey, &storedApiSecret); err != nil {
		t.Fatalf("failed to read raw row: %v", err)
	}
	assert.NotEqual(t, plainApiKey, storedApiKey, "cloud api key should be encrypted at rest")
	assert.NotEqual(t, plainApiSecret, storedApiSecret, "cloud api secret should be encrypted at rest")
	assert.NotEmpty(t, storedApiKey)
	assert.NotEmpty(t, storedApiSecret)
}

func TestWebhookSecretEncryptedAtRest(t *testing.T) {
	const testSecretKey = "AB551DED82B93DC8035D624A625920E2121367C7538C02277D2D4DB3C0BFFE94"
	t.Setenv("SCHEDULER0_SECRET_KEY", testSecretKey)

	scheduler0Store, scheduler0RaftActions, logger, cleanup := setupTestFSMStore(t)
	defer cleanup()

	accountRepo := account_repo.NewAccountRepository(context.TODO(), logger, scheduler0RaftActions, scheduler0Store)
	createTestAccount(t, accountRepo, 1, "Test Account")

	scheduler0Secrets := secrets.NewScheduler0Secrets()
	if loaded := scheduler0Secrets.GetSecrets(); loaded.SecretKey != testSecretKey {
		t.Skipf("scheduler0 secrets cache already populated by another test (got SecretKey=%q); skipping encryption-at-rest assertion", loaded.SecretKey)
	}

	executorRepo := NewExecutorRepo(logger, scheduler0RaftActions, scheduler0Store, scheduler0Secrets)

	plainWebhookSecret := "plaintext-webhook-secret"

	_, err := executorRepo.CreateOne(models.JobExecutor{
		Name:          "Webhook Executor",
		Type:          "webhook_url",
		WebhookUrl:    "https://example.com/webhook",
		WebhookMethod: "POST",
		WebhookSecret: plainWebhookSecret,
		AccountId:     1,
	})
	assert.Nil(t, err)

	retrieved, getErr := executorRepo.GetOneByID(1, 1)
	assert.Nil(t, getErr)
	assert.Equal(t, plainWebhookSecret, retrieved.WebhookSecret, "webhook secret should be decrypted on read")

	scheduler0Store.GetDataStore().ConnectionLock()
	defer scheduler0Store.GetDataStore().ConnectionUnlock()
	row := scheduler0Store.GetDataStore().GetOpenConnection().QueryRow(
		fmt.Sprintf("SELECT %s FROM %s WHERE %s = ?",
			constants.JobExecutorWebhookSecretColumn,
			constants.JobExecutorTableName,
			constants.JobExecutorIdColumn,
		), 1)
	var storedWebhookSecret string
	if scanErr := row.Scan(&storedWebhookSecret); scanErr != nil {
		t.Fatalf("failed to read raw row: %v", scanErr)
	}
	assert.NotEqual(t, plainWebhookSecret, storedWebhookSecret, "webhook secret should be encrypted at rest")
	assert.NotEmpty(t, storedWebhookSecret)
}

func TestReEncryptSecretsRotatesWebhookSecret(t *testing.T) {
	const oldSecretKey = "AB551DED82B93DC8035D624A625920E2121367C7538C02277D2D4DB3C0BFFE94"
	const newSecretKey = "11112222333344445555666677778888999900001111222233334444555566AA"
	t.Setenv("SCHEDULER0_SECRET_KEY", oldSecretKey)

	scheduler0Store, scheduler0RaftActions, logger, cleanup := setupTestFSMStore(t)
	defer cleanup()

	accountRepo := account_repo.NewAccountRepository(context.TODO(), logger, scheduler0RaftActions, scheduler0Store)
	createTestAccount(t, accountRepo, 1, "Test Account")

	scheduler0Secrets := secrets.NewScheduler0Secrets()
	if loaded := scheduler0Secrets.GetSecrets(); loaded.SecretKey != oldSecretKey {
		t.Skipf("scheduler0 secrets cache already populated by another test (got SecretKey=%q); skipping rotation assertion", loaded.SecretKey)
	}

	executorRepo := NewExecutorRepo(logger, scheduler0RaftActions, scheduler0Store, scheduler0Secrets)

	plainWebhookSecret := "plaintext-webhook-secret"
	_, err := executorRepo.CreateOne(models.JobExecutor{
		Name:          "Webhook Executor",
		Type:          "webhook_url",
		WebhookUrl:    "https://example.com/webhook",
		WebhookMethod: "POST",
		WebhookSecret: plainWebhookSecret,
		AccountId:     1,
	})
	assert.Nil(t, err)

	rotated, rotateErr := executorRepo.ReEncryptSecrets(oldSecretKey, newSecretKey)
	assert.Nil(t, rotateErr)
	assert.Equal(t, uint64(1), rotated, "expected one executor row to be re-encrypted")

	scheduler0Store.GetDataStore().ConnectionLock()
	defer scheduler0Store.GetDataStore().ConnectionUnlock()
	row := scheduler0Store.GetDataStore().GetOpenConnection().QueryRow(
		fmt.Sprintf("SELECT %s FROM %s WHERE %s = ?",
			constants.JobExecutorWebhookSecretColumn,
			constants.JobExecutorTableName,
			constants.JobExecutorIdColumn,
		), 1)
	var storedWebhookSecret string
	if scanErr := row.Scan(&storedWebhookSecret); scanErr != nil {
		t.Fatalf("failed to read raw row: %v", scanErr)
	}
	assert.Equal(t, plainWebhookSecret, utils.Decrypt(storedWebhookSecret, newSecretKey), "webhook secret should decrypt with the new key after rotation")
	_, okOld := utils.DecryptSafe(storedWebhookSecret, oldSecretKey)
	assert.False(t, okOld, "webhook secret should no longer decrypt with the old key after rotation")
}

func TestUpdateOneByIDRetainsStoredSecretCiphertext(t *testing.T) {
	const testSecretKey = "AB551DED82B93DC8035D624A625920E2121367C7538C02277D2D4DB3C0BFFE94"
	t.Setenv("SCHEDULER0_SECRET_KEY", testSecretKey)

	scheduler0Store, scheduler0RaftActions, logger, cleanup := setupTestFSMStore(t)
	defer cleanup()

	accountRepo := account_repo.NewAccountRepository(context.TODO(), logger, scheduler0RaftActions, scheduler0Store)
	createTestAccount(t, accountRepo, 1, "Test Account")

	scheduler0Secrets := secrets.NewScheduler0Secrets()
	if loaded := scheduler0Secrets.GetSecrets(); loaded.SecretKey != testSecretKey {
		t.Skipf("scheduler0 secrets cache already populated by another test (got SecretKey=%q); skipping retention assertion", loaded.SecretKey)
	}

	executorRepo := NewExecutorRepo(logger, scheduler0RaftActions, scheduler0Store, scheduler0Secrets)

	const plainWebhookSecret = "plaintext-webhook-secret"

	createdID, createErr := executorRepo.CreateOne(models.JobExecutor{
		Name:          "Webhook Executor",
		Type:          "webhook_url",
		WebhookUrl:    "https://example.com/webhook",
		WebhookMethod: "POST",
		WebhookSecret: plainWebhookSecret,
		AccountId:     1,
	})
	assert.Nil(t, createErr)

	readStoredSecret := func() string {
		scheduler0Store.GetDataStore().ConnectionLock()
		defer scheduler0Store.GetDataStore().ConnectionUnlock()
		row := scheduler0Store.GetDataStore().GetOpenConnection().QueryRow(
			fmt.Sprintf("SELECT %s FROM %s WHERE %s = ?",
				constants.JobExecutorWebhookSecretColumn,
				constants.JobExecutorTableName,
				constants.JobExecutorIdColumn,
			), createdID)
		var stored string
		if scanErr := row.Scan(&stored); scanErr != nil {
			t.Fatalf("failed to read raw row: %v", scanErr)
		}
		return stored
	}

	cipherBeforeUpdate := readStoredSecret()
	assert.NotEmpty(t, cipherBeforeUpdate)

	_, updateErr := executorRepo.UpdateOneByID(models.JobExecutor{
		ID:            createdID,
		Name:          "Renamed Executor",
		Type:          "webhook_url",
		WebhookUrl:    "https://example.com/webhook",
		WebhookMethod: "POST",
		WebhookSecret: "",
		AccountId:     1,
	})
	assert.Nil(t, updateErr)

	assert.Equal(t, cipherBeforeUpdate, readStoredSecret(),
		"a blank secret must leave the stored ciphertext untouched, not re-encrypt or blank it")

	retrieved, getErr := executorRepo.GetOneByID(createdID, 1)
	assert.Nil(t, getErr)
	assert.Equal(t, plainWebhookSecret, retrieved.WebhookSecret,
		"the retained secret must still decrypt to the original plaintext")
}
