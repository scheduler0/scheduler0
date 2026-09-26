package account_ai_settings

import (
	"context"
	"fmt"
	"os"
	"scheduler0-private/pkg/config"
	"scheduler0-private/pkg/db"
	"scheduler0-private/pkg/fsm"
	"scheduler0-private/pkg/models"
	account_repo "scheduler0-private/pkg/repository/account"
	"scheduler0-private/pkg/secrets"
	"scheduler0-private/pkg/shared_repo"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/assert"
)

func setupTestFSMStore(t *testing.T) (fsm.Scheduler0RaftStore, fsm.Scheduler0RaftActions, hclog.Logger, func()) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "account-ai-settings-repo-test",
		Level: hclog.LevelFromString("ERROR"),
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

func createTestAccount(t *testing.T, accountRepo account_repo.AccountRepository, accountID uint64, name string) {
	if _, err := accountRepo.CreateAccount(&models.Account{ID: accountID, Name: name}); err != nil {
		t.Fatalf("Failed to create account %d: %v", accountID, err)
	}
}

func TestAIKeysAreEncryptedAtRest(t *testing.T) {
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

	repo := NewAccountAISettingsRepo(logger, scheduler0RaftActions, scheduler0Store, scheduler0Secrets)

	plain := models.AccountAISettings{
		AccountID:          1,
		ActiveModels:       []models.ActiveModel{{Provider: "openai", Model: "gpt-4.1-mini"}},
		OpenAIAPIKey:       "sk-test-openai-key",
		AnthropicAPIKey:    "sk-ant-test-key",
		BedrockAccessKeyID: "AKIA-TEST-ACCESS-KEY",
		BedrockSecretKey:   "test-bedrock-secret-key",
		BedrockRegion:      "us-east-1",
	}
	if err := repo.Upsert(plain); err != nil {
		t.Fatalf("Upsert failed: %v", err)
	}

	stored, err := repo.Get(1)
	assert.Nil(t, err)
	if assert.NotNil(t, stored) {
		assert.NotEqual(t, plain.OpenAIAPIKey, stored.OpenAIAPIKey, "openai key should be encrypted on default Get")
		assert.NotEqual(t, plain.AnthropicAPIKey, stored.AnthropicAPIKey, "anthropic key should be encrypted on default Get")
		assert.NotEqual(t, plain.BedrockAccessKeyID, stored.BedrockAccessKeyID, "bedrock access key should be encrypted on default Get")
		assert.NotEqual(t, plain.BedrockSecretKey, stored.BedrockSecretKey, "bedrock secret key should be encrypted on default Get")
		assert.Equal(t, plain.ActiveModels, stored.ActiveModels)
		assert.Equal(t, plain.BedrockRegion, stored.BedrockRegion)
	}

	executable, err := repo.GetForExecution(1)
	assert.Nil(t, err)
	if assert.NotNil(t, executable) {
		assert.Equal(t, plain.OpenAIAPIKey, executable.OpenAIAPIKey)
		assert.Equal(t, plain.AnthropicAPIKey, executable.AnthropicAPIKey)
		assert.Equal(t, plain.BedrockAccessKeyID, executable.BedrockAccessKeyID)
		assert.Equal(t, plain.BedrockSecretKey, executable.BedrockSecretKey)
	}

	scheduler0Store.GetDataStore().ConnectionLock()
	defer scheduler0Store.GetDataStore().ConnectionUnlock()
	row := scheduler0Store.GetDataStore().GetOpenConnection().QueryRow(
		fmt.Sprintf("SELECT %s, %s, %s, %s FROM %s WHERE %s = ?",
			ColOpenAIAPIKey, ColAnthropicAPIKey,
			ColBedrockAccessKey, ColBedrockSecretKey,
			TableName, ColAccountID,
		), 1)
	var rawOpenAI, rawAnthropic, rawBedrockAccess, rawBedrockSecret string
	if scanErr := row.Scan(&rawOpenAI, &rawAnthropic, &rawBedrockAccess, &rawBedrockSecret); scanErr != nil {
		t.Fatalf("failed to read raw row: %v", scanErr)
	}
	assert.NotEqual(t, plain.OpenAIAPIKey, rawOpenAI)
	assert.NotEqual(t, plain.AnthropicAPIKey, rawAnthropic)
	assert.NotEqual(t, plain.BedrockAccessKeyID, rawBedrockAccess)
	assert.NotEqual(t, plain.BedrockSecretKey, rawBedrockSecret)
	assert.NotEmpty(t, rawOpenAI)
	assert.NotEmpty(t, rawAnthropic)
	assert.NotEmpty(t, rawBedrockAccess)
	assert.NotEmpty(t, rawBedrockSecret)
}

func TestUpsertPreservesExistingKeysWhenBlank(t *testing.T) {
	const testSecretKey = "AB551DED82B93DC8035D624A625920E2121367C7538C02277D2D4DB3C0BFFE94"
	t.Setenv("SCHEDULER0_SECRET_KEY", testSecretKey)

	scheduler0Store, scheduler0RaftActions, logger, cleanup := setupTestFSMStore(t)
	defer cleanup()

	accountRepo := account_repo.NewAccountRepository(context.TODO(), logger, scheduler0RaftActions, scheduler0Store)
	createTestAccount(t, accountRepo, 1, "Test Account")

	scheduler0Secrets := secrets.NewScheduler0Secrets()
	if loaded := scheduler0Secrets.GetSecrets(); loaded.SecretKey != testSecretKey {
		t.Skipf("scheduler0 secrets cache already populated by another test (got SecretKey=%q); skipping", loaded.SecretKey)
	}

	repo := NewAccountAISettingsRepo(logger, scheduler0RaftActions, scheduler0Store, scheduler0Secrets)

	if err := repo.Upsert(models.AccountAISettings{
		AccountID:    1,
		ActiveModels: []models.ActiveModel{{Provider: "openai", Model: "gpt-4.1-mini"}},
		OpenAIAPIKey: "sk-original-key",
	}); err != nil {
		t.Fatalf("initial Upsert failed: %v", err)
	}

	if err := repo.Upsert(models.AccountAISettings{
		AccountID:    1,
		ActiveModels: []models.ActiveModel{{Provider: "openai", Model: "gpt-4.1"}},
		OpenAIAPIKey: "",
	}); err != nil {
		t.Fatalf("follow-up Upsert failed: %v", err)
	}

	executable, err := repo.GetForExecution(1)
	assert.Nil(t, err)
	if assert.NotNil(t, executable) {
		assert.Equal(t, []models.ActiveModel{{Provider: "openai", Model: "gpt-4.1"}}, executable.ActiveModels, "active models should reflect the new upsert")
		assert.Equal(t, "sk-original-key", executable.OpenAIAPIKey, "blank openai key should retain the previously stored value")
	}
}
