package account_job_executions_count

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"scheduler0/pkg/config"
	"scheduler0/pkg/constants"
	"scheduler0/pkg/db"
	"scheduler0/pkg/fsm"
	"scheduler0/pkg/mocks"
	"scheduler0/pkg/models"
	account_repo "scheduler0/pkg/repository/account"
	"scheduler0/pkg/shared_repo"
	"scheduler0/pkg/utils"
)

// setupMockFSMStore creates a mock FSM store for unit tests
func setupMockFSMStore(t *testing.T) *fsm.MockScheduler0RaftStore {
	return fsm.NewMockScheduler0RaftStore(t)
}

func setupTestDB(t *testing.T) (db.DataStore, func()) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "account-job-executions-count-repo-test",
		Level: hclog.LevelFromString("DEBUG"),
	})

	tempFile, err := os.CreateTemp("", "test-account-job-executions-count-db")
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
		Name:  "account-job-executions-count-repo-test",
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

// createTestAccount creates an account for testing (required for foreign key constraints)
func createTestAccount(t *testing.T, fsmStore fsm.Scheduler0RaftStore, accountId uint64) {
	logger := hclog.New(&hclog.LoggerOptions{})
	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)
	accountRepo := account_repo.NewAccountRepository(context.Background(), logger, scheduler0RaftActions, fsmStore)

	account := &models.Account{
		ID:   accountId,
		Name: fmt.Sprintf("Test Account %d", accountId),
	}
	_, err := accountRepo.CreateAccount(account)
	if err != nil {
		t.Fatalf("Failed to create test account %d: %v", accountId, err)
	}
}

func TestNewAccountJobExecutionsCountRepo(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{})
	mockRaftActions := mocks.NewScheduler0RaftActions(t)
	mockFSMStore := setupMockFSMStore(t)

	repo := NewAccountJobExecutionsCountRepo(context.Background(), logger, mockRaftActions, mockFSMStore)

	assert.NotNil(t, repo)
	accountJobExecutionsCountRepo, ok := repo.(*accountJobExecutionsCountRepo)
	assert.True(t, ok)
	assert.Equal(t, mockRaftActions, accountJobExecutionsCountRepo.scheduler0RaftActions)
	assert.Equal(t, mockFSMStore, accountJobExecutionsCountRepo.fsmStore)
	assert.NotNil(t, accountJobExecutionsCountRepo.logger)
	// Logger is named in constructor, so it won't be equal to the original
	assert.Contains(t, accountJobExecutionsCountRepo.logger.Name(), "account-job-executions-count-repo")
}

func TestCreate_Success(t *testing.T) {
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

	repo := NewAccountJobExecutionsCountRepo(context.Background(), logger, mockRaftActions, mockFSMStore)

	result, err := repo.Create(1, 10)

	assert.Nil(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, uint64(1), result.ID)
	assert.Equal(t, uint64(1), result.AccountId)
	assert.Equal(t, uint64(0), result.ExecutionCount) // Should be 0 initially
}

func TestCreate_WriteCommandError(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{})
	mockRaftActions := mocks.NewScheduler0RaftActions(t)
	mockFSMStore := setupMockFSMStore(t)
	mockRaft := &raft.Raft{}

	mockFSMStore.On("GetRaft").Return(mockRaft)

	expectedError := utils.HTTPGenericError(500, "raft error")

	mockRaftActions.On("WriteCommandToRaftLog",
		mock.Anything,
		constants.CommandTypeDbExecute,
		mock.AnythingOfType("string"),
		mock.Anything,
		[]uint64{},
		constants.CommandAction(0),
	).Return((*models.FSMResponse)(nil), expectedError)

	repo := NewAccountJobExecutionsCountRepo(context.Background(), logger, mockRaftActions, mockFSMStore)

	result, err := repo.Create(1, 10)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, expectedError, err)
}

func TestCreate_NilResponse(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{})
	mockRaftActions := mocks.NewScheduler0RaftActions(t)
	mockFSMStore := setupMockFSMStore(t)
	mockRaft := &raft.Raft{}

	mockFSMStore.On("GetRaft").Return(mockRaft)

	mockRaftActions.On("WriteCommandToRaftLog",
		mock.Anything,
		constants.CommandTypeDbExecute,
		mock.AnythingOfType("string"),
		mock.Anything,
		[]uint64{},
		constants.CommandAction(0),
	).Return((*models.FSMResponse)(nil), (*utils.GenericError)(nil))

	repo := NewAccountJobExecutionsCountRepo(context.Background(), logger, mockRaftActions, mockFSMStore)

	result, err := repo.Create(1, 10)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Message, "service is unavailable")
}

func TestGetByAccountId_Success(t *testing.T) {
	dataStore, cleanup := setupTestDB(t)
	defer cleanup()

	fsmStore, fsmCleanup := setupTestFSMStore(t, dataStore)
	defer fsmCleanup()

	logger := hclog.New(&hclog.LoggerOptions{})
	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)

	// Create account first (required for foreign key)
	createTestAccount(t, fsmStore, 1)

	repo := NewAccountJobExecutionsCountRepo(context.Background(), logger, scheduler0RaftActions, fsmStore)

	// Create an account job executions count
	_, err := repo.Create(1, 0)
	assert.Nil(t, err)

	// Get by account ID
	result, err := repo.GetByAccountId(1)

	assert.Nil(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, uint64(1), result.AccountId)
	assert.Equal(t, uint64(0), result.ExecutionCount)
}

func TestGetByAccountId_NotFound(t *testing.T) {
	dataStore, cleanup := setupTestDB(t)
	defer cleanup()

	fsmStore, fsmCleanup := setupTestFSMStore(t, dataStore)
	defer fsmCleanup()

	logger := hclog.New(&hclog.LoggerOptions{})
	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)

	repo := NewAccountJobExecutionsCountRepo(context.Background(), logger, scheduler0RaftActions, fsmStore)

	// Try to get non-existent account
	result, err := repo.GetByAccountId(999)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, http.StatusNotFound, err.Type)
}

func TestGetByAccountId_InvalidID(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{})
	mockRaftActions := mocks.NewScheduler0RaftActions(t)
	mockFSMStore := setupMockFSMStore(t)
	mockDataStore := mocks.NewMockDataStore(t)

	mockFSMStore.On("GetDataStore").Return(mockDataStore)
	mockDataStore.On("ConnectionLock").Return()
	mockDataStore.On("ConnectionUnlock").Return()

	repo := NewAccountJobExecutionsCountRepo(context.Background(), logger, mockRaftActions, mockFSMStore)

	// Try to get with invalid ID
	result, err := repo.GetByAccountId(0)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, http.StatusBadRequest, err.Type)
}

func TestGetExecutionCountsByAccountIds_Success(t *testing.T) {
	dataStore, cleanup := setupTestDB(t)
	defer cleanup()

	fsmStore, fsmCleanup := setupTestFSMStore(t, dataStore)
	defer fsmCleanup()

	logger := hclog.New(&hclog.LoggerOptions{})
	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)

	// Create accounts first (required for foreign key)
	createTestAccount(t, fsmStore, 1)
	createTestAccount(t, fsmStore, 2)
	createTestAccount(t, fsmStore, 3)

	repo := NewAccountJobExecutionsCountRepo(context.Background(), logger, scheduler0RaftActions, fsmStore)

	// Create multiple account job executions counts
	_, err := repo.Create(1, 0)
	assert.Nil(t, err)
	_, err = repo.Create(2, 0)
	assert.Nil(t, err)
	_, err = repo.Create(3, 0)
	assert.Nil(t, err)

	// Update execution counts
	err = repo.UpdateExecutionCount(1, 10)
	assert.Nil(t, err)
	err = repo.UpdateExecutionCount(2, 20)
	assert.Nil(t, err)

	// Get execution counts by account IDs
	counts, err := repo.GetExecutionCountsByAccountIds([]uint64{1, 3})

	assert.Nil(t, err)
	assert.NotNil(t, counts)
	assert.Len(t, counts, 2)
	assert.Equal(t, uint64(10), counts[1])
	assert.Equal(t, uint64(0), counts[3])
}

func TestGetExecutionCountsByAccountIds_EmptyList(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{})
	mockRaftActions := mocks.NewScheduler0RaftActions(t)
	mockFSMStore := setupMockFSMStore(t)
	mockDataStore := mocks.NewMockDataStore(t)

	// GetExecutionCountsByAccountIds calls ConnectionLock before checking empty list
	mockFSMStore.On("GetDataStore").Return(mockDataStore)
	mockDataStore.On("ConnectionLock").Return()
	mockDataStore.On("ConnectionUnlock").Return()

	repo := NewAccountJobExecutionsCountRepo(context.Background(), logger, mockRaftActions, mockFSMStore)

	counts, err := repo.GetExecutionCountsByAccountIds([]uint64{})

	assert.Nil(t, err)
	assert.NotNil(t, counts)
	assert.Empty(t, counts)
}

func TestUpdateExecutionCount_Success(t *testing.T) {
	dataStore, cleanup := setupTestDB(t)
	defer cleanup()

	fsmStore, fsmCleanup := setupTestFSMStore(t, dataStore)
	defer fsmCleanup()

	logger := hclog.New(&hclog.LoggerOptions{})
	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)

	// Create account first (required for foreign key)
	createTestAccount(t, fsmStore, 1)

	repo := NewAccountJobExecutionsCountRepo(context.Background(), logger, scheduler0RaftActions, fsmStore)

	// Create an account job executions count
	_, err := repo.Create(1, 0)
	assert.Nil(t, err)

	// Update execution count
	err = repo.UpdateExecutionCount(1, 100)

	assert.Nil(t, err)

	// Verify the update
	result, err := repo.GetByAccountId(1)
	assert.Nil(t, err)
	assert.Equal(t, uint64(100), result.ExecutionCount)
}

func TestUpdateExecutionCount_WriteCommandError(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{})
	mockRaftActions := mocks.NewScheduler0RaftActions(t)
	mockFSMStore := setupMockFSMStore(t)
	mockRaft := &raft.Raft{}

	mockFSMStore.On("GetRaft").Return(mockRaft)

	expectedError := utils.HTTPGenericError(500, "raft error")

	mockRaftActions.On("WriteCommandToRaftLog",
		mock.Anything,
		constants.CommandTypeDbExecute,
		mock.AnythingOfType("string"),
		mock.Anything,
		[]uint64{},
		constants.CommandAction(0),
	).Return((*models.FSMResponse)(nil), expectedError)

	repo := NewAccountJobExecutionsCountRepo(context.Background(), logger, mockRaftActions, mockFSMStore)

	err := repo.UpdateExecutionCount(1, 100)

	assert.Error(t, err)
	assert.Equal(t, expectedError, err)
}

func TestResetExecutionCount_Success(t *testing.T) {
	dataStore, cleanup := setupTestDB(t)
	defer cleanup()

	fsmStore, fsmCleanup := setupTestFSMStore(t, dataStore)
	defer fsmCleanup()

	logger := hclog.New(&hclog.LoggerOptions{})
	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)

	// Create account first (required for foreign key)
	createTestAccount(t, fsmStore, 1)

	repo := NewAccountJobExecutionsCountRepo(context.Background(), logger, scheduler0RaftActions, fsmStore)

	// Create an account job executions count
	_, err := repo.Create(1, 0)
	assert.Nil(t, err)

	// Update execution count first
	err = repo.UpdateExecutionCount(1, 100)
	assert.Nil(t, err)

	// Reset execution count
	err = repo.ResetExecutionCount(1, 0)

	assert.Nil(t, err)

	// Verify the reset
	result, err := repo.GetByAccountId(1)
	assert.Nil(t, err)
	assert.Equal(t, uint64(0), result.ExecutionCount)
	// Verify next reset date is set to next month
	assert.True(t, result.NextResetDate.After(result.DateModified))
}

func TestResetExecutionCount_WriteCommandError(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{})
	mockRaftActions := mocks.NewScheduler0RaftActions(t)
	mockFSMStore := setupMockFSMStore(t)
	mockRaft := &raft.Raft{}

	mockFSMStore.On("GetRaft").Return(mockRaft)

	expectedError := utils.HTTPGenericError(500, "raft error")

	mockRaftActions.On("WriteCommandToRaftLog",
		mock.Anything,
		constants.CommandTypeDbExecute,
		mock.AnythingOfType("string"),
		mock.Anything,
		[]uint64{},
		constants.CommandAction(0),
	).Return((*models.FSMResponse)(nil), expectedError)

	repo := NewAccountJobExecutionsCountRepo(context.Background(), logger, mockRaftActions, mockFSMStore)

	err := repo.ResetExecutionCount(1, 0)

	assert.Error(t, err)
	assert.Equal(t, expectedError, err)
}

func TestGetAllExpiredResetDates_Success(t *testing.T) {
	dataStore, cleanup := setupTestDB(t)
	defer cleanup()

	fsmStore, fsmCleanup := setupTestFSMStore(t, dataStore)
	defer fsmCleanup()

	logger := hclog.New(&hclog.LoggerOptions{})
	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)

	// Create accounts first (required for foreign key)
	createTestAccount(t, fsmStore, 1)
	createTestAccount(t, fsmStore, 2)

	repo := NewAccountJobExecutionsCountRepo(context.Background(), logger, scheduler0RaftActions, fsmStore)

	// Create account job executions counts
	_, err := repo.Create(1, 0)
	assert.Nil(t, err)
	_, err = repo.Create(2, 0)
	assert.Nil(t, err)

	// Get all expired reset dates (should be empty since they're set to next month)
	results, err := repo.GetAllExpiredResetDates()

	assert.Nil(t, err)
	// Results can be nil (empty slice) when there are no expired dates
	if results != nil {
		assert.Empty(t, results)
	}
}

func TestGetAllExpiredResetDates_WithExpiredDates(t *testing.T) {
	dataStore, cleanup := setupTestDB(t)
	defer cleanup()

	fsmStore, fsmCleanup := setupTestFSMStore(t, dataStore)
	defer fsmCleanup()

	logger := hclog.New(&hclog.LoggerOptions{})
	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)

	// Create account first (required for foreign key)
	createTestAccount(t, fsmStore, 1)

	repo := NewAccountJobExecutionsCountRepo(context.Background(), logger, scheduler0RaftActions, fsmStore)

	// Create account job executions counts
	_, err := repo.Create(1, 0)
	assert.Nil(t, err)

	// Manually update the next_reset_date to be in the past
	// This would normally be done by the reset operation, but for testing we'll do it directly
	conn := fsmStore.GetDataStore().GetOpenConnection()
	_, execErr := conn.Exec("UPDATE account_job_executions_count SET next_reset_date = datetime('now', '-1 day') WHERE account_id = 1")
	if execErr != nil {
		t.Fatalf("Failed to update next_reset_date: %v", execErr)
	}

	// Get all expired reset dates
	results, err := repo.GetAllExpiredResetDates()

	assert.Nil(t, err)
	assert.NotNil(t, results)
	assert.Len(t, results, 1)
	assert.Equal(t, uint64(1), results[0].AccountId)
}

func TestGetAccountsWithZeroExecutionCount_Success(t *testing.T) {
	dataStore, cleanup := setupTestDB(t)
	defer cleanup()

	fsmStore, fsmCleanup := setupTestFSMStore(t, dataStore)
	defer fsmCleanup()

	logger := hclog.New(&hclog.LoggerOptions{})
	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)

	// Create accounts first (required for foreign key)
	createTestAccount(t, fsmStore, 1)
	createTestAccount(t, fsmStore, 2)
	createTestAccount(t, fsmStore, 3)

	repo := NewAccountJobExecutionsCountRepo(context.Background(), logger, scheduler0RaftActions, fsmStore)

	// Create account job executions counts
	_, err := repo.Create(1, 0)
	assert.Nil(t, err)
	_, err = repo.Create(2, 0)
	assert.Nil(t, err)
	_, err = repo.Create(3, 0)
	assert.Nil(t, err)

	// Update one account's execution count
	err = repo.UpdateExecutionCount(2, 50)
	assert.Nil(t, err)

	// Get accounts with zero execution count
	accountIds, err := repo.GetAccountsWithZeroExecutionCount()

	assert.Nil(t, err)
	assert.NotNil(t, accountIds)
	assert.Len(t, accountIds, 2)
	// Should contain account 1 and 3, but not 2
	assert.Contains(t, accountIds, uint64(1))
	assert.Contains(t, accountIds, uint64(3))
	assert.NotContains(t, accountIds, uint64(2))
}

func TestGetAccountsWithZeroExecutionCount_NoZeroCounts(t *testing.T) {
	dataStore, cleanup := setupTestDB(t)
	defer cleanup()

	fsmStore, fsmCleanup := setupTestFSMStore(t, dataStore)
	defer fsmCleanup()

	logger := hclog.New(&hclog.LoggerOptions{})
	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)

	// Create account first (required for foreign key)
	createTestAccount(t, fsmStore, 1)

	repo := NewAccountJobExecutionsCountRepo(context.Background(), logger, scheduler0RaftActions, fsmStore)

	// Create account job executions counts
	_, err := repo.Create(1, 0)
	assert.Nil(t, err)

	// Update execution count
	err = repo.UpdateExecutionCount(1, 100)
	assert.Nil(t, err)

	// Get accounts with zero execution count
	accountIds, err := repo.GetAccountsWithZeroExecutionCount()

	assert.Nil(t, err)
	// AccountIds can be nil (empty slice) when there are no zero counts
	if accountIds != nil {
		assert.Empty(t, accountIds)
	}
}

func TestDeleteByAccountId_Success(t *testing.T) {
	dataStore, cleanup := setupTestDB(t)
	defer cleanup()

	fsmStore, fsmCleanup := setupTestFSMStore(t, dataStore)
	defer fsmCleanup()

	logger := hclog.New(&hclog.LoggerOptions{})
	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)

	// Create account first (required for foreign key)
	createTestAccount(t, fsmStore, 1)

	repo := NewAccountJobExecutionsCountRepo(context.Background(), logger, scheduler0RaftActions, fsmStore)

	// Create an account job executions count
	_, err := repo.Create(1, 0)
	assert.Nil(t, err)

	// Delete by account ID
	err = repo.DeleteByAccountId(1)

	assert.Nil(t, err)

	// Verify deletion
	result, err := repo.GetByAccountId(1)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, http.StatusNotFound, err.Type)
}

func TestDeleteByAccountId_InvalidID(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{})
	mockRaftActions := mocks.NewScheduler0RaftActions(t)
	mockFSMStore := setupMockFSMStore(t)

	repo := NewAccountJobExecutionsCountRepo(context.Background(), logger, mockRaftActions, mockFSMStore)

	// Try to delete with invalid ID
	err := repo.DeleteByAccountId(0)

	assert.Error(t, err)
	assert.Equal(t, http.StatusBadRequest, err.Type)
}

func TestDeleteByAccountId_WriteCommandError(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{})
	mockRaftActions := mocks.NewScheduler0RaftActions(t)
	mockFSMStore := setupMockFSMStore(t)
	mockRaft := &raft.Raft{}

	mockFSMStore.On("GetRaft").Return(mockRaft)

	expectedError := utils.HTTPGenericError(500, "raft error")

	mockRaftActions.On("WriteCommandToRaftLog",
		mock.Anything,
		constants.CommandTypeDbExecute,
		mock.AnythingOfType("string"),
		mock.Anything,
		[]uint64{},
		constants.CommandAction(0),
	).Return((*models.FSMResponse)(nil), expectedError)

	repo := NewAccountJobExecutionsCountRepo(context.Background(), logger, mockRaftActions, mockFSMStore)

	err := repo.DeleteByAccountId(1)

	assert.Error(t, err)
	assert.Equal(t, expectedError, err)
}
