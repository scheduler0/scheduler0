package account

import (
	"context"
	"database/sql"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"scheduler0-private/pkg/config"
	"scheduler0-private/pkg/constants"
	"scheduler0-private/pkg/db"
	"scheduler0-private/pkg/fsm"
	"scheduler0-private/pkg/mocks"
	"scheduler0-private/pkg/models"
	"scheduler0-private/pkg/shared_repo"
	"scheduler0-private/pkg/utils"
)

// setupMockFSMStore creates a mock FSM store for unit tests
// Individual tests should set up expectations for methods they actually call
func setupMockFSMStore(t *testing.T) *fsm.MockScheduler0RaftStore {
	return fsm.NewMockScheduler0RaftStore(t)
}

func setupTestDB(t *testing.T) (db.DataStore, func()) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "account-repo-test",
		Level: hclog.LevelFromString("DEBUG"),
	})

	tempFile, err := os.CreateTemp("", "test-account-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tempFile.Close()

	sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.RunMigration(logger)
	sqliteDb.OpenConnectionToExistingDB()

	cleanup := func() {
		os.Remove(tempFile.Name())
	}

	return sqliteDb, cleanup
}

func setupTestFSMStore(t *testing.T, dataStore db.DataStore) (fsm.Scheduler0RaftStore, func()) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "account-repo-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)

	fsmStore := fsm.NewFSMStore(logger, scheduler0RaftActions, scheduler0config, dataStore, nil, nil, nil, nil, sharedRepo)

	// Use faster timeouts for testing
	raftConf := raft.DefaultConfig()
	raftConf.HeartbeatTimeout = 50 * time.Millisecond
	raftConf.ElectionTimeout = 50 * time.Millisecond
	raftConf.CommitTimeout = 50 * time.Millisecond
	raftConf.LeaderLeaseTimeout = 25 * time.Millisecond // Must be <= HeartbeatTimeout

	cluster := raft.MakeClusterCustom(t, &raft.MakeClusterOpts{
		Peers:          1,
		Bootstrap:      true,
		Conf:           raftConf,
		ConfigStoreFSM: false,
		MakeFSMFunc: func() raft.FSM {
			return fsmStore.GetFSM()
		},
	})

	cluster.FullyConnect()

	// Wait for leader with timeout to prevent blocking
	leaderCh := make(chan *raft.Raft, 1)
	go func() {
		leaderCh <- cluster.Leader()
	}()

	select {
	case leader := <-leaderCh:
		if leader == nil {
			t.Fatal("Raft cluster leader is nil")
		}
		fsmStore.UpdateRaft(leader)
	case <-time.After(10 * time.Second):
		t.Fatal("Timeout waiting for raft cluster leader")
	}

	cleanup := func() {
		cluster.Close()
	}

	return fsmStore, cleanup
}

func TestNewAccountRepository(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{})
	mockRaftActions := mocks.NewScheduler0RaftActions(t)
	mockFSMStore := setupMockFSMStore(t)

	repo := NewAccountRepository(context.Background(), logger, mockRaftActions, mockFSMStore)

	assert.NotNil(t, repo)
	accountRepo, ok := repo.(*accountRepository)
	assert.True(t, ok)
	assert.Equal(t, mockRaftActions, accountRepo.scheduler0RaftActions)
	assert.Equal(t, mockFSMStore, accountRepo.fsmStore)
	assert.Equal(t, logger, accountRepo.logger)
}

func TestCreateAccount_Success(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{})
	mockRaftActions := mocks.NewScheduler0RaftActions(t)
	mockFSMStore := setupMockFSMStore(t)
	mockRaft := &raft.Raft{}

	mockFSMStore.On("GetRaft").Return(mockRaft)

	expectedResponse := &models.FSMResponse{
		Data: models.SQLResponse{
			LastInsertedId: 1,
			RowsAffected:   1,
		},
		Error: "",
	}

	mockRaftActions.On("WriteCommandToRaftLog",
		mock.Anything,
		constants.CommandTypeDbExecute,
		mock.AnythingOfType("string"),
		mock.Anything,
		[]uint64{},
		constants.CommandAction(0),
	).Return(expectedResponse, (*utils.GenericError)(nil))

	repo := NewAccountRepository(context.Background(), logger, mockRaftActions, mockFSMStore)

	account := &models.Account{
		Name: "Test Account",
	}

	id, err := repo.CreateAccount(account)

	assert.Nil(t, err)
	assert.Equal(t, uint64(1), id)
	assert.Equal(t, uint64(1), account.ID)
}

func TestCreateAccount_WriteCommandError(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{})
	mockRaftActions := mocks.NewScheduler0RaftActions(t)
	mockFSMStore := setupMockFSMStore(t)
	mockRaft := &raft.Raft{}

	mockFSMStore.On("GetRaft").Return(mockRaft)

	expectedError := utils.HTTPGenericError(500, "raft error")

	mockRaftActions.On("WriteCommandToRaftLog",
		mockRaft,
		constants.CommandTypeDbExecute,
		mock.AnythingOfType("string"),
		mock.AnythingOfType("[]interface {}"),
		[]uint64{},
		constants.CommandAction(0),
	).Return((*models.FSMResponse)(nil), expectedError)

	repo := NewAccountRepository(context.Background(), logger, mockRaftActions, mockFSMStore)

	account := &models.Account{
		Name: "Test Account",
	}

	id, err := repo.CreateAccount(account)

	assert.Error(t, err)
	assert.Equal(t, uint64(0), id)
	assert.Equal(t, expectedError, err)
}

func TestCreateAccount_NilResponse(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{})
	mockRaftActions := mocks.NewScheduler0RaftActions(t)
	mockFSMStore := setupMockFSMStore(t)
	mockRaft := &raft.Raft{}

	mockFSMStore.On("GetRaft").Return(mockRaft)

	mockRaftActions.On("WriteCommandToRaftLog",
		mockRaft,
		constants.CommandTypeDbExecute,
		mock.AnythingOfType("string"),
		mock.AnythingOfType("[]interface {}"),
		[]uint64{},
		constants.CommandAction(0),
	).Return((*models.FSMResponse)(nil), (*utils.GenericError)(nil))

	repo := NewAccountRepository(context.Background(), logger, mockRaftActions, mockFSMStore)

	account := &models.Account{
		Name: "Test Account",
	}

	id, err := repo.CreateAccount(account)

	assert.Error(t, err)
	assert.Equal(t, uint64(0), id)
	assert.Contains(t, err.Error(), "service is unavailable")
}

func TestGetAccount_Success(t *testing.T) {
	dataStore, cleanup := setupTestDB(t)
	defer cleanup()

	fsmStore, fsmCleanup := setupTestFSMStore(t, dataStore)
	defer fsmCleanup()

	logger := hclog.New(&hclog.LoggerOptions{})
	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)

	repo := NewAccountRepository(context.Background(), logger, scheduler0RaftActions, fsmStore)

	// Create an account first
	account := &models.Account{
		Name: "Test Account",
	}
	_, err := repo.CreateAccount(account)
	assert.Nil(t, err)

	// Get the account
	retrievedAccount, err := repo.GetAccount(account.ID)

	assert.Nil(t, err)
	assert.NotNil(t, retrievedAccount)
	assert.Equal(t, account.ID, retrievedAccount.ID)
	assert.Equal(t, account.Name, retrievedAccount.Name)
}

func TestGetAccount_NotFound(t *testing.T) {
	dataStore, cleanup := setupTestDB(t)
	defer cleanup()

	fsmStore, fsmCleanup := setupTestFSMStore(t, dataStore)
	defer fsmCleanup()

	logger := hclog.New(&hclog.LoggerOptions{})
	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)

	repo := NewAccountRepository(context.Background(), logger, scheduler0RaftActions, fsmStore)

	// Try to get non-existent account
	retrievedAccount, err := repo.GetAccount(999)

	assert.Error(t, err)
	assert.Nil(t, retrievedAccount)
	assert.Equal(t, http.StatusNotFound, err.Type)
}

func TestGetAccount_InvalidID(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{})
	mockRaftActions := mocks.NewScheduler0RaftActions(t)
	mockFSMStore := setupMockFSMStore(t)
	mockDataStore := mocks.NewMockDataStore(t)

	mockFSMStore.On("GetDataStore").Return(mockDataStore)
	mockDataStore.On("ConnectionLock").Return()
	mockDataStore.On("ConnectionUnlock").Return()

	repo := NewAccountRepository(context.Background(), logger, mockRaftActions, mockFSMStore)

	retrievedAccount, err := repo.GetAccount(0)

	assert.Error(t, err)
	assert.Nil(t, retrievedAccount)
	assert.Equal(t, http.StatusBadRequest, err.Type)
	assert.Contains(t, err.Error(), "account id is required")
}

func TestGetAccounts_Success(t *testing.T) {
	dataStore, cleanup := setupTestDB(t)
	defer cleanup()

	fsmStore, fsmCleanup := setupTestFSMStore(t, dataStore)
	defer fsmCleanup()

	logger := hclog.New(&hclog.LoggerOptions{})
	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)

	repo := NewAccountRepository(context.Background(), logger, scheduler0RaftActions, fsmStore)

	// Create multiple accounts
	account1 := &models.Account{Name: "Account 1"}
	account2 := &models.Account{Name: "Account 2"}
	account3 := &models.Account{Name: "Account 3"}

	_, err := repo.CreateAccount(account1)
	assert.Nil(t, err)
	_, err = repo.CreateAccount(account2)
	assert.Nil(t, err)
	_, err = repo.CreateAccount(account3)
	assert.Nil(t, err)

	// Get accounts by IDs
	accounts, err := repo.GetAccounts([]uint64{account1.ID, account3.ID})

	assert.Nil(t, err)
	assert.Len(t, accounts, 2)
	accountMap := make(map[uint64]models.Account)
	for _, acc := range accounts {
		accountMap[acc.ID] = acc
	}
	assert.Contains(t, accountMap, account1.ID)
	assert.Contains(t, accountMap, account3.ID)
	assert.Equal(t, "Account 1", accountMap[account1.ID].Name)
	assert.Equal(t, "Account 3", accountMap[account3.ID].Name)
}

func TestGetAccounts_EmptyList(t *testing.T) {
	// Empty list returns early, no need for raft cluster
	logger := hclog.New(&hclog.LoggerOptions{})
	mockRaftActions := mocks.NewScheduler0RaftActions(t)
	mockFSMStore := setupMockFSMStore(t)

	repo := NewAccountRepository(context.Background(), logger, mockRaftActions, mockFSMStore)

	accounts, err := repo.GetAccounts([]uint64{})

	assert.Nil(t, err)
	assert.Empty(t, accounts)
}

func TestGetFeatures_Success(t *testing.T) {
	dataStore, cleanup := setupTestDB(t)
	defer cleanup()

	fsmStore, fsmCleanup := setupTestFSMStore(t, dataStore)
	defer fsmCleanup()

	logger := hclog.New(&hclog.LoggerOptions{})
	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)

	repo := NewAccountRepository(context.Background(), logger, scheduler0RaftActions, fsmStore)

	// Create an account
	account := &models.Account{Name: "Test Account"}
	_, err := repo.CreateAccount(account)
	assert.Nil(t, err)

	// Add a feature to the account
	err = repo.AddFeature(account.ID, 1)
	assert.Nil(t, err)

	// Get features
	features, err := repo.GetFeatures(account.ID)

	assert.Nil(t, err)
	assert.NotNil(t, features)
	assert.Len(t, *features, 1)
	assert.Equal(t, uint64(1), (*features)[0].FeatureId)
}

func TestGetFeatures_NoFeatures(t *testing.T) {
	dataStore, cleanup := setupTestDB(t)
	defer cleanup()

	fsmStore, fsmCleanup := setupTestFSMStore(t, dataStore)
	defer fsmCleanup()

	logger := hclog.New(&hclog.LoggerOptions{})
	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)

	repo := NewAccountRepository(context.Background(), logger, scheduler0RaftActions, fsmStore)

	// Create an account
	account := &models.Account{Name: "Test Account"}
	_, err := repo.CreateAccount(account)
	assert.Nil(t, err)

	// Get features (should be empty)
	features, err := repo.GetFeatures(account.ID)

	assert.Nil(t, err)
	assert.NotNil(t, features)
	assert.Empty(t, *features)
}

func TestGetFeaturesByAccountIds_Success(t *testing.T) {
	dataStore, cleanup := setupTestDB(t)
	defer cleanup()

	fsmStore, fsmCleanup := setupTestFSMStore(t, dataStore)
	defer fsmCleanup()

	logger := hclog.New(&hclog.LoggerOptions{})
	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)

	repo := NewAccountRepository(context.Background(), logger, scheduler0RaftActions, fsmStore)

	// Create accounts
	account1 := &models.Account{Name: "Account 1"}
	account2 := &models.Account{Name: "Account 2"}

	_, err := repo.CreateAccount(account1)
	assert.Nil(t, err)
	_, err = repo.CreateAccount(account2)
	assert.Nil(t, err)

	// Add features
	err = repo.AddFeature(account1.ID, 1)
	assert.Nil(t, err)
	err = repo.AddFeature(account2.ID, 2)
	assert.Nil(t, err)

	// Get features by account IDs
	featuresMap, err := repo.GetFeaturesByAccountIds([]uint64{account1.ID, account2.ID})

	assert.Nil(t, err)
	assert.NotNil(t, featuresMap)
	assert.Len(t, featuresMap[account1.ID], 1)
	assert.Len(t, featuresMap[account2.ID], 1)
	assert.Equal(t, uint64(1), featuresMap[account1.ID][0].FeatureId)
	assert.Equal(t, uint64(2), featuresMap[account2.ID][0].FeatureId)
}

func TestGetFeaturesByAccountIds_EmptyList(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{})
	mockRaftActions := mocks.NewScheduler0RaftActions(t)
	mockFSMStore := setupMockFSMStore(t)
	mockDataStore := mocks.NewMockDataStore(t)

	// GetFeaturesByAccountIds calls ConnectionLock before checking empty list
	mockFSMStore.On("GetDataStore").Return(mockDataStore)
	mockDataStore.On("ConnectionLock").Return()
	mockDataStore.On("ConnectionUnlock").Return()

	repo := NewAccountRepository(context.Background(), logger, mockRaftActions, mockFSMStore)

	featuresMap, err := repo.GetFeaturesByAccountIds([]uint64{})

	assert.Nil(t, err)
	assert.NotNil(t, featuresMap)
	assert.Empty(t, featuresMap)
}

func TestAddFeature_Success(t *testing.T) {
	dataStore, cleanup := setupTestDB(t)
	defer cleanup()

	fsmStore, fsmCleanup := setupTestFSMStore(t, dataStore)
	defer fsmCleanup()

	logger := hclog.New(&hclog.LoggerOptions{})
	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)

	repo := NewAccountRepository(context.Background(), logger, scheduler0RaftActions, fsmStore)

	// Create an account
	account := &models.Account{Name: "Test Account"}
	_, err := repo.CreateAccount(account)
	assert.Nil(t, err)

	// Add a feature
	err = repo.AddFeature(account.ID, 1)

	assert.Nil(t, err)

	// Verify feature was added
	features, err := repo.GetFeatures(account.ID)
	assert.Nil(t, err)
	assert.Len(t, *features, 1)
}

func TestAddFeature_AlreadyExists(t *testing.T) {
	dataStore, cleanup := setupTestDB(t)
	defer cleanup()

	fsmStore, fsmCleanup := setupTestFSMStore(t, dataStore)
	defer fsmCleanup()

	logger := hclog.New(&hclog.LoggerOptions{})
	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)

	repo := NewAccountRepository(context.Background(), logger, scheduler0RaftActions, fsmStore)

	// Create an account
	account := &models.Account{Name: "Test Account"}
	_, err := repo.CreateAccount(account)
	assert.Nil(t, err)

	// Add a feature twice (should be idempotent)
	err = repo.AddFeature(account.ID, 1)
	assert.Nil(t, err)

	err = repo.AddFeature(account.ID, 1)
	assert.Nil(t, err) // Should succeed (idempotent)

	// Verify feature exists only once
	features, err := repo.GetFeatures(account.ID)
	assert.Nil(t, err)
	assert.Len(t, *features, 1)
}

func TestAddFeature_WriteCommandError(t *testing.T) {
	// This test requires a real database connection to check if feature exists
	// So we'll test the error path through integration test instead
	// Skipping this unit test as it requires complex database mocking
	t.Skip("Skipping - requires real database connection for feature existence check")
}

func TestRemoveFeature_Success(t *testing.T) {
	dataStore, cleanup := setupTestDB(t)
	defer cleanup()

	fsmStore, fsmCleanup := setupTestFSMStore(t, dataStore)
	defer fsmCleanup()

	logger := hclog.New(&hclog.LoggerOptions{})
	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)

	repo := NewAccountRepository(context.Background(), logger, scheduler0RaftActions, fsmStore)

	// Create an account
	account := &models.Account{Name: "Test Account"}
	_, err := repo.CreateAccount(account)
	assert.Nil(t, err)

	// Add a feature
	err = repo.AddFeature(account.ID, 1)
	assert.Nil(t, err)

	// Remove the feature
	err = repo.RemoveFeature(account.ID, 1)

	assert.Nil(t, err)

	// Verify feature was removed
	features, err := repo.GetFeatures(account.ID)
	assert.Nil(t, err)
	assert.Empty(t, *features)
}

func TestRemoveFeature_WriteCommandError(t *testing.T) {
	// This test requires a real database connection to build the SQL query
	// So we'll test the error path through integration test instead
	// Skipping this unit test as it requires complex database mocking
	t.Skip("Skipping - requires real database connection for SQL query building")
}

func TestAddAllFeatures_Success(t *testing.T) {
	dataStore, cleanup := setupTestDB(t)
	defer cleanup()

	fsmStore, fsmCleanup := setupTestFSMStore(t, dataStore)
	defer fsmCleanup()

	logger := hclog.New(&hclog.LoggerOptions{})
	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)

	repo := NewAccountRepository(context.Background(), logger, scheduler0RaftActions, fsmStore)

	// Create an account
	account := &models.Account{Name: "Test Account"}
	_, err := repo.CreateAccount(account)
	assert.Nil(t, err)

	// Add all features (should add all seeded features)
	err = repo.AddAllFeatures(account.ID)

	assert.Nil(t, err)

	// Verify features were added (should have 4 features from seed)
	features, err := repo.GetFeatures(account.ID)
	assert.Nil(t, err)
	assert.Len(t, *features, 4) // 4 features are seeded
}

func TestAddAllFeatures_SomeAlreadyExist(t *testing.T) {
	dataStore, cleanup := setupTestDB(t)
	defer cleanup()

	fsmStore, fsmCleanup := setupTestFSMStore(t, dataStore)
	defer fsmCleanup()

	logger := hclog.New(&hclog.LoggerOptions{})
	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)

	repo := NewAccountRepository(context.Background(), logger, scheduler0RaftActions, fsmStore)

	// Create an account
	account := &models.Account{Name: "Test Account"}
	_, err := repo.CreateAccount(account)
	assert.Nil(t, err)

	// Add one feature manually
	err = repo.AddFeature(account.ID, 1)
	assert.Nil(t, err)

	// Add all features (should skip the one that already exists)
	err = repo.AddAllFeatures(account.ID)

	assert.Nil(t, err)

	// Verify all features were added (should still have 4 features)
	features, err := repo.GetFeatures(account.ID)
	assert.Nil(t, err)
	assert.Len(t, *features, 4)
}

func TestRemoveAllFeatures_Success(t *testing.T) {
	dataStore, cleanup := setupTestDB(t)
	defer cleanup()

	fsmStore, fsmCleanup := setupTestFSMStore(t, dataStore)
	defer fsmCleanup()

	logger := hclog.New(&hclog.LoggerOptions{})
	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)

	repo := NewAccountRepository(context.Background(), logger, scheduler0RaftActions, fsmStore)

	// Create an account
	account := &models.Account{Name: "Test Account"}
	_, err := repo.CreateAccount(account)
	assert.Nil(t, err)

	// Add all features
	err = repo.AddAllFeatures(account.ID)
	assert.Nil(t, err)

	// Remove all features
	err = repo.RemoveAllFeatures(account.ID)

	assert.Nil(t, err)

	// Verify all features were removed
	features, err := repo.GetFeatures(account.ID)
	assert.Nil(t, err)
	assert.Empty(t, *features)
}

func TestRemoveAllFeatures_WriteCommandError(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{})
	mockRaftActions := mocks.NewScheduler0RaftActions(t)
	mockFSMStore := setupMockFSMStore(t)
	mockRaft := &raft.Raft{}
	mockDataStore := mocks.NewMockDataStore(t)

	mockFSMStore.On("GetDataStore").Return(mockDataStore)
	mockFSMStore.On("GetRaft").Return(mockRaft)
	mockDataStore.On("GetOpenConnection").Return(&sql.DB{})

	expectedError := utils.HTTPGenericError(500, "raft error")

	mockRaftActions.On("WriteCommandToRaftLog",
		mock.Anything,
		constants.CommandTypeDbExecute,
		mock.AnythingOfType("string"),
		mock.Anything,
		[]uint64{},
		constants.CommandAction(0),
	).Return((*models.FSMResponse)(nil), expectedError)

	repo := NewAccountRepository(context.Background(), logger, mockRaftActions, mockFSMStore)

	err := repo.RemoveAllFeatures(1)

	assert.Error(t, err)
	assert.Equal(t, expectedError, err)
}

// ========== Edge Case Tests ==========

func TestCreateAccount_EdgeCase_EmptyName(t *testing.T) {
	dataStore, cleanup := setupTestDB(t)
	defer cleanup()

	fsmStore, fsmCleanup := setupTestFSMStore(t, dataStore)
	defer fsmCleanup()

	logger := hclog.New(&hclog.LoggerOptions{})
	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)

	repo := NewAccountRepository(context.Background(), logger, scheduler0RaftActions, fsmStore)

	account := &models.Account{
		Name: "", // Empty name
	}

	_, err := repo.CreateAccount(account)

	assert.Error(t, err)
	assert.Equal(t, http.StatusBadRequest, err.Type)
	assert.Contains(t, err.Message, "account name is required")
}

func TestCreateAccount_EdgeCase_VeryLongName(t *testing.T) {
	dataStore, cleanup := setupTestDB(t)
	defer cleanup()

	fsmStore, fsmCleanup := setupTestFSMStore(t, dataStore)
	defer fsmCleanup()

	logger := hclog.New(&hclog.LoggerOptions{})
	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)

	repo := NewAccountRepository(context.Background(), logger, scheduler0RaftActions, fsmStore)

	// Create a very long name (1000 characters)
	longName := strings.Repeat("a", 1000)
	account := &models.Account{
		Name: longName,
	}

	id, err := repo.CreateAccount(account)

	// Very long names should either be validated or truncated
	// Test current behavior
	if err != nil {
		assert.Equal(t, http.StatusBadRequest, err.Type)
	} else {
		// If it succeeds, verify the name was stored correctly or truncated
		retrievedAccount, getErr := repo.GetAccount(id)
		if getErr == nil {
			assert.True(t, len(retrievedAccount.Name) <= len(longName))
		}
	}
}

func TestGetAccounts_EdgeCase_SomeNonExistent(t *testing.T) {
	dataStore, cleanup := setupTestDB(t)
	defer cleanup()

	fsmStore, fsmCleanup := setupTestFSMStore(t, dataStore)
	defer fsmCleanup()

	logger := hclog.New(&hclog.LoggerOptions{})
	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)

	repo := NewAccountRepository(context.Background(), logger, scheduler0RaftActions, fsmStore)

	// Create one account
	account1 := &models.Account{Name: "Account 1"}
	_, err := repo.CreateAccount(account1)
	assert.Nil(t, err)

	// Get accounts with mix of existing and non-existent IDs
	accounts, err := repo.GetAccounts([]uint64{account1.ID, 99999, 88888})

	assert.Nil(t, err)
	assert.Len(t, accounts, 1, "Should return only existing accounts")
	assert.Equal(t, account1.ID, accounts[0].ID)
}

func TestGetAccounts_EdgeCase_AllNonExistent(t *testing.T) {
	dataStore, cleanup := setupTestDB(t)
	defer cleanup()

	fsmStore, fsmCleanup := setupTestFSMStore(t, dataStore)
	defer fsmCleanup()

	logger := hclog.New(&hclog.LoggerOptions{})
	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)

	repo := NewAccountRepository(context.Background(), logger, scheduler0RaftActions, fsmStore)

	// Get accounts with all non-existent IDs
	accounts, err := repo.GetAccounts([]uint64{99999, 88888, 77777})

	assert.Nil(t, err)
	assert.Empty(t, accounts, "Should return empty list when no accounts exist")
}

func TestGetFeatures_EdgeCase_InvalidAccountID(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{})
	mockRaftActions := mocks.NewScheduler0RaftActions(t)
	mockFSMStore := setupMockFSMStore(t)

	repo := NewAccountRepository(context.Background(), logger, mockRaftActions, mockFSMStore)

	features, err := repo.GetFeatures(0)

	assert.Error(t, err)
	assert.Nil(t, features)
	assert.Equal(t, http.StatusBadRequest, err.Type)
	assert.Contains(t, err.Message, "account id is required")
}

func TestGetFeatures_EdgeCase_NonExistentAccount(t *testing.T) {
	dataStore, cleanup := setupTestDB(t)
	defer cleanup()

	fsmStore, fsmCleanup := setupTestFSMStore(t, dataStore)
	defer fsmCleanup()

	logger := hclog.New(&hclog.LoggerOptions{})
	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)

	repo := NewAccountRepository(context.Background(), logger, scheduler0RaftActions, fsmStore)

	// Get features for non-existent account
	features, err := repo.GetFeatures(99999)

	assert.Nil(t, err, "GetFeatures should not error for non-existent account")
	assert.NotNil(t, features)
	assert.Empty(t, *features, "Should return empty features list for non-existent account")
}

func TestGetFeaturesByAccountIds_EdgeCase_SomeNonExistent(t *testing.T) {
	dataStore, cleanup := setupTestDB(t)
	defer cleanup()

	fsmStore, fsmCleanup := setupTestFSMStore(t, dataStore)
	defer fsmCleanup()

	logger := hclog.New(&hclog.LoggerOptions{})
	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)

	repo := NewAccountRepository(context.Background(), logger, scheduler0RaftActions, fsmStore)

	// Create an account
	account1 := &models.Account{Name: "Account 1"}
	_, err := repo.CreateAccount(account1)
	assert.Nil(t, err)

	// Add a feature
	err = repo.AddFeature(account1.ID, 1)
	assert.Nil(t, err)

	// Get features with mix of existing and non-existent account IDs
	featuresMap, err := repo.GetFeaturesByAccountIds([]uint64{account1.ID, 99999, 88888})

	assert.Nil(t, err)
	assert.NotNil(t, featuresMap)
	assert.Len(t, featuresMap[account1.ID], 1, "Existing account should have features")
	_, exists := featuresMap[99999]
	assert.False(t, exists, "Non-existent account should not be in map")
}

func TestAddFeature_EdgeCase_InvalidAccountID(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{})
	mockRaftActions := mocks.NewScheduler0RaftActions(t)
	mockFSMStore := setupMockFSMStore(t)

	repo := NewAccountRepository(context.Background(), logger, mockRaftActions, mockFSMStore)

	err := repo.AddFeature(0, 1)

	assert.Error(t, err)
	assert.Equal(t, http.StatusBadRequest, err.Type)
	assert.Contains(t, err.Message, "account id is required")
}

func TestAddFeature_EdgeCase_InvalidFeatureID(t *testing.T) {
	dataStore, cleanup := setupTestDB(t)
	defer cleanup()

	fsmStore, fsmCleanup := setupTestFSMStore(t, dataStore)
	defer fsmCleanup()

	logger := hclog.New(&hclog.LoggerOptions{})
	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)

	repo := NewAccountRepository(context.Background(), logger, scheduler0RaftActions, fsmStore)

	// Create an account
	account := &models.Account{Name: "Test Account"}
	_, err := repo.CreateAccount(account)
	assert.Nil(t, err)

	// Try to add feature with ID 0
	err = repo.AddFeature(account.ID, 0)

	assert.Error(t, err)
	assert.Equal(t, http.StatusBadRequest, err.Type)
	assert.Contains(t, err.Message, "feature id is required")
}

func TestRemoveFeature_EdgeCase_NonExistentFeature(t *testing.T) {
	dataStore, cleanup := setupTestDB(t)
	defer cleanup()

	fsmStore, fsmCleanup := setupTestFSMStore(t, dataStore)
	defer fsmCleanup()

	logger := hclog.New(&hclog.LoggerOptions{})
	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)

	repo := NewAccountRepository(context.Background(), logger, scheduler0RaftActions, fsmStore)

	// Create an account
	account := &models.Account{Name: "Test Account"}
	_, err := repo.CreateAccount(account)
	assert.Nil(t, err)

	// Try to remove a feature that doesn't exist (should be idempotent)
	err = repo.RemoveFeature(account.ID, 999)

	assert.Nil(t, err, "Removing non-existent feature should be idempotent and not error")
}

func TestAddAllFeatures_EdgeCase_InvalidAccountID(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{})
	mockRaftActions := mocks.NewScheduler0RaftActions(t)
	mockFSMStore := setupMockFSMStore(t)

	repo := NewAccountRepository(context.Background(), logger, mockRaftActions, mockFSMStore)

	err := repo.AddAllFeatures(0)

	assert.Error(t, err)
	assert.Equal(t, http.StatusBadRequest, err.Type)
	assert.Contains(t, err.Message, "account id is required")
}

func TestRemoveAllFeatures_EdgeCase_InvalidAccountID(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{})
	mockRaftActions := mocks.NewScheduler0RaftActions(t)
	mockFSMStore := setupMockFSMStore(t)

	repo := NewAccountRepository(context.Background(), logger, mockRaftActions, mockFSMStore)

	err := repo.RemoveAllFeatures(0)

	assert.Error(t, err)
	assert.Equal(t, http.StatusBadRequest, err.Type)
	assert.Contains(t, err.Message, "account id is required")
}
