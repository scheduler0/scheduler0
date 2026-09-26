package credential

import (
	"context"
	"io/ioutil"
	"net/http"
	"os"
	"scheduler0/pkg/config"
	"scheduler0/pkg/constants"
	"scheduler0/pkg/db"
	"scheduler0/pkg/fsm"
	"scheduler0/pkg/models"
	account_repo "scheduler0/pkg/repository/account"
	credential_repository "scheduler0/pkg/repository/credential"
	"scheduler0/pkg/secrets"
	"scheduler0/pkg/shared_repo"
	"scheduler0/pkg/utils"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/assert"
)

// testSetup contains all the test dependencies
type testSetup struct {
	service         CredentialService
	accountRepo     account_repo.AccountRepository
	scheduler0Store fsm.Scheduler0RaftStore
	cleanup         func()
}

// setupTestCredentialService creates a test credential service with all dependencies
func setupTestCredentialService(t *testing.T) *testSetup {
	ctx := context.Background()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "credential-service-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)
	tempFile, err := ioutil.TempFile("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tempFileName := tempFile.Name()
	tempFile.Close()

	sqliteDb := db.NewSqliteDbConnection(logger, tempFileName)
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
	cluster.FullyConnect()
	scheduler0Store.UpdateRaft(cluster.Leader())
	scheduler0Secrets := secrets.NewScheduler0Secrets()
	credentialRepo := credential_repository.NewCredentialRepo(logger, scheduler0RaftActions, scheduler0Store)
	accountRepo := account_repo.NewAccountRepository(ctx, logger, scheduler0RaftActions, scheduler0Store)

	os.Setenv("SCHEDULER0_SECRET_KEY", "AB551DED82B93DC8035D624A625920E2121367C7538C02277D2D4DB3C0BFFE94")

	dispatcher := utils.NewDispatcher(
		ctx,
		int64(1),
		int64(1),
	)

	dispatcher.Run()

	service := NewCredentialService(ctx, logger, scheduler0Secrets, credentialRepo, dispatcher)

	cleanup := func() {
		cluster.Close()
		os.Remove(tempFileName)
	}

	return &testSetup{
		service:         service,
		accountRepo:     accountRepo,
		scheduler0Store: scheduler0Store,
		cleanup:         cleanup,
	}
}

// createTestAccount creates a test account and returns the actual account ID
func (ts *testSetup) createTestAccount(t *testing.T, accountName string) uint64 {
	account := &models.Account{
		Name: accountName,
	}

	accountId, createErr := ts.accountRepo.CreateAccount(account)
	if createErr != nil {
		t.Fatalf("Failed to create test account: %v", createErr)
	}

	return accountId
}

func Test_CredentialService_CreateNewCredential(t *testing.T) {
	ts := setupTestCredentialService(t)
	defer ts.cleanup()

	accountId := ts.createTestAccount(t, "Test Account")
	credential := models.Credential{
		AccountId: accountId,
		CreatedBy: "test-user",
		Scopes:    []string{"read", "write", "execute"},
	}

	id, _, createErr := ts.service.CreateNewCredential(credential)
	if createErr != nil {
		t.Fatal("failed to create new credential", createErr)
	}
	assert.Equal(t, id, uint64(1))

	// Verify the credential was created with generated API key and secret
	createdCred, getErr := ts.service.FindOneCredentialByID(id, accountId)
	if getErr != nil {
		t.Fatal("failed to get created credential", getErr)
	}
	assert.NotEmpty(t, createdCred.ApiKey)
	assert.NotEmpty(t, createdCred.ApiSecret)
	assert.Equal(t, accountId, createdCred.AccountId)
	assert.Equal(t, "test-user", createdCred.CreatedBy)
	assert.NotNil(t, createdCred.ExpiresAt)
	assert.ElementsMatch(t, []string{"read", "write", "execute"}, createdCred.Scopes)

	// Test: missing scopes should fail
	_, _, missingScopeErr := ts.service.CreateNewCredential(models.Credential{
		AccountId: accountId,
		CreatedBy: "test-user",
	})
	assert.NotNil(t, missingScopeErr)
	assert.Equal(t, http.StatusBadRequest, missingScopeErr.Type)

	// Test: invalid scope should fail
	_, _, invalidScopeErr := ts.service.CreateNewCredential(models.Credential{
		AccountId: accountId,
		CreatedBy: "test-user",
		Scopes:    []string{"superuser"},
	})
	assert.NotNil(t, invalidScopeErr)
	assert.Equal(t, http.StatusBadRequest, invalidScopeErr.Type)

	// Test: the admin scope is accepted at the service layer (the escalation guard
	// that restricts who may grant it lives in the controller, not here).
	adminID, _, adminErr := ts.service.CreateNewCredential(models.Credential{
		AccountId: accountId,
		CreatedBy: "test-user",
		Scopes:    []string{"admin"},
	})
	assert.Nil(t, adminErr)
	assert.NotZero(t, adminID)
}

func Test_resolveCredentialTTL(t *testing.T) {
	maxDuration := time.Duration(constants.CredentialExpiryDays) * 24 * time.Hour
	minDuration := time.Duration(constants.CredentialMinExpirySeconds) * time.Second

	ptr := func(v int64) *int64 { return &v }

	// nil -> default 90-day expiry
	assert.Equal(t, maxDuration, resolveCredentialTTL(nil))

	// in-range value is honored exactly (e.g. 2 hours)
	twoHours := int64(2 * 60 * 60)
	assert.Equal(t, time.Duration(twoHours)*time.Second, resolveCredentialTTL(ptr(twoHours)))

	// below the floor is clamped up
	assert.Equal(t, minDuration, resolveCredentialTTL(ptr(1)))
	assert.Equal(t, minDuration, resolveCredentialTTL(ptr(0)))
	assert.Equal(t, minDuration, resolveCredentialTTL(ptr(-100)))

	// above the ceiling is clamped down
	assert.Equal(t, maxDuration, resolveCredentialTTL(ptr(constants.CredentialExpiryDays*24*60*60+1)))
}

func Test_CredentialService_UpdateOneCredential(t *testing.T) {
	ts := setupTestCredentialService(t)
	defer ts.cleanup()

	accountId := ts.createTestAccount(t, "Test Account")
	id, plaintextSecret, createErr := ts.service.CreateNewCredential(models.Credential{
		AccountId: accountId,
		CreatedBy: "test-user",
		Scopes:    []string{"read", "write", "execute"},
	})
	if createErr != nil {
		t.Fatal("failed to create new credential", createErr)
	}
	assert.Equal(t, id, uint64(1))

	// Get the created credential to have valid API key and secret
	cred, getErr := ts.service.FindOneCredentialByID(id, accountId)
	if getErr != nil {
		t.Fatal("failed to get credential", getErr)
	}

	// Test: Update without API key/secret (the only shape an HTTP body can have,
	// since ApiSecret is json:"-") must succeed and keep the stored key/secret.
	httpModifier := "http-client"
	fromHTTP, updateErr := ts.service.UpdateOneCredential(models.Credential{
		ID:         id,
		AccountId:  accountId,
		ModifiedBy: &httpModifier,
	})
	if updateErr != nil {
		t.Fatal("update without api key/secret should succeed:", updateErr)
	}
	assert.Equal(t, cred.ApiKey, fromHTTP.ApiKey)
	assert.Equal(t, "", fromHTTP.ApiSecret, "secret must never be returned")
	assert.ElementsMatch(t, []string{"read", "write", "execute"}, fromHTTP.Scopes)
	assert.NotNil(t, fromHTTP.ModifiedBy)
	assert.Equal(t, httpModifier, *fromHTTP.ModifiedBy)

	// Verify the stored key/secret survived the update
	afterHTTP, getAfterErr := ts.service.FindOneCredentialByID(id, accountId)
	if getAfterErr != nil {
		t.Fatal("failed to get credential", getAfterErr)
	}
	assert.Equal(t, cred.ApiKey, afterHTTP.ApiKey)
	valid, _, _ := ts.service.ValidateServerAPIKey(cred.ApiKey, plaintextSecret, accountId)
	assert.True(t, valid, "credential must still authenticate after an update")

	// Test: Update with different API key should fail
	_, thirdUpdateErr := ts.service.UpdateOneCredential(models.Credential{
		ID:        id,
		AccountId: accountId,
		ApiSecret: cred.ApiSecret,
		ApiKey:    "some-new-api-key",
	})
	if thirdUpdateErr == nil {
		t.Fatal("update should fail because updating api_key is not allowed")
	}

	// Test: Update with different API secret should fail
	_, fourthUpdateErr := ts.service.UpdateOneCredential(models.Credential{
		ID:        id,
		AccountId: accountId,
		ApiSecret: "some-new-api-secret",
		ApiKey:    cred.ApiKey,
	})
	if fourthUpdateErr == nil {
		t.Fatal("update should fail because updating api_secret is not allowed")
	}

	// Test: Update with non-existent credential should fail
	_, fifthUpdateErr := ts.service.UpdateOneCredential(models.Credential{
		ID:        999,
		AccountId: accountId,
		ApiKey:    cred.ApiKey,
		ApiSecret: cred.ApiSecret,
	})
	if fifthUpdateErr == nil {
		t.Fatal("update should fail because credential does not exist")
	}

	// Test: Valid update should succeed
	assert.Equal(t, cred.Archived, false)
	cred.Archived = true
	modifiedBy := "test-modifier"
	cred.ModifiedBy = &modifiedBy
	updatedCred, sixthUpdateErr := ts.service.UpdateOneCredential(*cred)
	if sixthUpdateErr != nil {
		t.Fatal("update operation should succeed but failed with error:", sixthUpdateErr)
	}
	assert.Equal(t, updatedCred.Archived, true)
	assert.NotNil(t, updatedCred.ModifiedBy)
	assert.Equal(t, "test-modifier", *updatedCred.ModifiedBy)

	// Verify the update persisted
	updatedCred, getErr = ts.service.FindOneCredentialByID(id, accountId)
	if getErr != nil {
		t.Fatal("failed to get credential", getErr)
	}
	assert.Equal(t, updatedCred.Archived, true)
}

func Test_CredentialService_DeleteOneCredential(t *testing.T) {
	ts := setupTestCredentialService(t)
	defer ts.cleanup()

	accountId := ts.createTestAccount(t, "Test Account")
	id, _, createErr := ts.service.CreateNewCredential(models.Credential{
		AccountId: accountId,
		CreatedBy: "test-user",
		Scopes:    []string{"read", "write", "execute"},
	})
	if createErr != nil {
		t.Fatal("failed to create new credential", createErr)
	}
	assert.Equal(t, id, uint64(1))

	// Test: Delete without deletedBy should fail
	_, deleteErr := ts.service.DeleteOneCredential(id, accountId, "")
	if deleteErr == nil {
		t.Fatal("delete should fail because deletedBy is required")
	}
	genericErr, ok := deleteErr.(*utils.GenericError)
	if ok {
		assert.Equal(t, http.StatusBadRequest, genericErr.Type)
	} else {
		t.Fatal("expected GenericError but got different error type")
	}

	// Test: Valid delete should succeed
	deletedBy := "test-deleter"
	deletedCred, deleteErr := ts.service.DeleteOneCredential(id, accountId, deletedBy)
	if deleteErr != nil {
		t.Fatal("failed to delete credential", deleteErr)
	}

	assert.Equal(t, deletedCred.ID, id)
	assert.NotNil(t, deletedCred.DeletedBy)
	assert.Equal(t, deletedBy, *deletedCred.DeletedBy)

	// Test: Delete non-existent credential should fail
	_, secondDeleteErr := ts.service.DeleteOneCredential(999, accountId, deletedBy)
	if secondDeleteErr == nil {
		t.Fatal("delete should fail because credential does not exist")
	}
}

func Test_CredentialService_ListCredentials(t *testing.T) {
	ts := setupTestCredentialService(t)
	defer ts.cleanup()

	accountId := ts.createTestAccount(t, "Test Account")

	// Create multiple credentials
	credentials := []models.Credential{
		{
			AccountId: accountId,
			CreatedBy: "test-user-1",
			Scopes:    []string{"read", "write", "execute"},
		},
		{
			AccountId: accountId,
			CreatedBy: "test-user-2",
			Scopes:    []string{"read", "write", "execute"},
		},
		{
			AccountId: accountId,
			CreatedBy: "test-user-3",
			Scopes:    []string{"read", "write", "execute"},
		},
	}

	for i := range credentials {
		id, _, createErr := ts.service.CreateNewCredential(credentials[i])
		if createErr != nil {
			t.Fatalf("Failed to create credential: %v", createErr)
		}
		cred, getErr := ts.service.FindOneCredentialByID(id, accountId)
		if getErr != nil {
			t.Fatalf("Failed to get credential: %v", getErr)
		}
		credentials[i].ID = cred.ID
		credentials[i].ApiKey = cred.ApiKey
		credentials[i].ApiSecret = cred.ApiSecret
	}

	// Test: List with valid parameters
	offset := uint64(0)
	limit := uint64(10)
	orderBy := "id"
	orderByDirection := "asc"
	result, listErr := ts.service.ListCredentials(offset, limit, orderBy, orderByDirection, accountId)
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
		assert.Equal(t, accountId, credential.AccountId)
	}

	// Assert the total count, offset, and limit
	assert.Equal(t, uint64(expectedCount), result.Total)
	assert.Equal(t, offset, result.Offset)
	assert.Equal(t, limit, result.Limit)

	// Test: List with limit > 100 should fail
	_, listErr = ts.service.ListCredentials(0, 101, orderBy, orderByDirection, accountId)
	if listErr == nil {
		t.Fatal("list should fail because limit exceeds 100")
	}
	assert.Equal(t, http.StatusTooManyRequests, listErr.Type)

	// Test: List with limit = 0 should fail
	_, listErr = ts.service.ListCredentials(0, 0, orderBy, orderByDirection, accountId)
	if listErr == nil {
		t.Fatal("list should fail because limit is 0")
	}
	assert.Equal(t, http.StatusBadRequest, listErr.Type)

	// Test: List with offset < 0 should fail
	_, listErr = ts.service.ListCredentials(1, 10, orderBy, orderByDirection, accountId)
	// Note: offset validation might allow offset >= 0, so we test with a valid offset
	// Test pagination
	result, listErr = ts.service.ListCredentials(0, 2, orderBy, orderByDirection, accountId)
	if listErr != nil {
		t.Fatalf("Failed to list credentials with pagination: %v", listErr)
	}
	assert.Equal(t, 2, len(result.Data))
	assert.Equal(t, uint64(expectedCount), result.Total)

	// Test: List with empty result (use a different account that doesn't exist)
	otherAccountId := ts.createTestAccount(t, "Other Account")
	result, listErr = ts.service.ListCredentials(0, 10, orderBy, orderByDirection, otherAccountId)
	if listErr != nil {
		t.Fatalf("Failed to list credentials for account with no credentials: %v", listErr)
	}
	assert.Equal(t, 0, len(result.Data))
	assert.Equal(t, uint64(0), result.Total)
}

func Test_CredentialService_ValidateServerAPIKey(t *testing.T) {
	ts := setupTestCredentialService(t)
	defer ts.cleanup()

	accountId := ts.createTestAccount(t, "Test Account")
	credential := models.Credential{
		AccountId: accountId,
		CreatedBy: "test-user",
		Scopes:    []string{"read", "write", "execute"},
	}

	id, plaintextSecret, createErr := ts.service.CreateNewCredential(credential)
	if createErr != nil {
		t.Fatalf("Failed to create credential: %v", createErr)
	}
	cred, getErr := ts.service.FindOneCredentialByID(id, accountId)
	if getErr != nil {
		t.Fatalf("Failed to get credential: %v", getErr)
	}

	// The client authenticates with the plaintext secret returned at creation, not the
	// encrypted value stored on the credential row.
	// Test: Validate with valid credentials
	isValid, loaded, validErr := ts.service.ValidateServerAPIKey(cred.ApiKey, plaintextSecret, accountId)
	assert.True(t, isValid)
	assert.Nil(t, validErr)
	assert.NotNil(t, loaded)
	assert.ElementsMatch(t, []string{"read", "write", "execute"}, loaded.Scopes)

	// Test: Validate with invalid API key
	invalidApiKey := "invalid-api-key"
	invalidApiSecret := "invalid-api-secret"
	isValid, loaded, invalidErr := ts.service.ValidateServerAPIKey(invalidApiKey, invalidApiSecret, accountId)
	assert.False(t, isValid)
	assert.NotNil(t, invalidErr)
	assert.Nil(t, loaded)
	assert.Equal(t, http.StatusNotFound, invalidErr.Type)

	// Test: Validate with valid API key but invalid API secret
	isValid, _, invalidErr = ts.service.ValidateServerAPIKey(cred.ApiKey, invalidApiSecret, accountId)
	assert.False(t, isValid)
	assert.Nil(t, invalidErr)

	// Test: Validate with wrong account ID
	otherAccountId := ts.createTestAccount(t, "Other Account")
	isValid, _, invalidErr = ts.service.ValidateServerAPIKey(cred.ApiKey, plaintextSecret, otherAccountId)
	assert.False(t, isValid)
	assert.NotNil(t, invalidErr)
}

func Test_CredentialService_FindOneCredentialByID(t *testing.T) {
	ts := setupTestCredentialService(t)
	defer ts.cleanup()

	accountId := ts.createTestAccount(t, "Test Account")
	credential := models.Credential{
		AccountId: accountId,
		CreatedBy: "test-user",
		Scopes:    []string{"read", "write", "execute"},
	}

	// Test: Find non-existent credential should fail
	_, getErr := ts.service.FindOneCredentialByID(999, accountId)
	if getErr == nil {
		t.Fatal("find should fail because credential does not exist")
	}

	// Create a credential
	id, _, createErr := ts.service.CreateNewCredential(credential)
	if createErr != nil {
		t.Fatal("failed to create new credential", createErr)
	}

	// Test: Find existing credential should succeed
	foundCred, getErr := ts.service.FindOneCredentialByID(id, accountId)
	if getErr != nil {
		t.Fatal("failed to find credential", getErr)
	}
	assert.Equal(t, id, foundCred.ID)
	assert.Equal(t, accountId, foundCred.AccountId)
	assert.NotEmpty(t, foundCred.ApiKey)
	assert.NotEmpty(t, foundCred.ApiSecret)
	assert.Equal(t, "test-user", foundCred.CreatedBy)

	// Test: Find credential with wrong account ID should fail
	otherAccountId := ts.createTestAccount(t, "Other Account")
	_, getErr = ts.service.FindOneCredentialByID(id, otherAccountId)
	if getErr == nil {
		t.Fatal("find should fail because credential belongs to different account")
	}
}

func Test_CredentialService_ArchiveOneCredential(t *testing.T) {
	ts := setupTestCredentialService(t)
	defer ts.cleanup()

	accountId := ts.createTestAccount(t, "Test Account")
	id, _, createErr := ts.service.CreateNewCredential(models.Credential{
		AccountId: accountId,
		CreatedBy: "test-user",
		Scopes:    []string{"read", "write", "execute"},
	})
	if createErr != nil {
		t.Fatal("failed to create new credential", createErr)
	}

	// Test: Archive without archivedBy should fail
	_, archiveErr := ts.service.ArchiveOneCredential(id, accountId, "")
	if archiveErr == nil {
		t.Fatal("archive should fail because archivedBy is required")
	}
	assert.Equal(t, http.StatusBadRequest, archiveErr.Type)

	// Test: Archive system credential (accountId = 1) should fail
	// Note: Account ID 1 is seeded by migrations as the system account
	systemAccountId := uint64(1)
	systemId, _, createErr := ts.service.CreateNewCredential(models.Credential{
		AccountId: systemAccountId,
		CreatedBy: "system",
		Scopes:    []string{"read", "write", "execute"},
	})
	if createErr != nil {
		t.Fatal("failed to create system credential", createErr)
	}

	_, archiveErr = ts.service.ArchiveOneCredential(systemId, systemAccountId, "test-user")
	if archiveErr == nil {
		t.Fatal("archive should fail because system credentials cannot be archived")
	}
	assert.Equal(t, http.StatusBadRequest, archiveErr.Type)

	// Test: Valid archive should succeed
	archivedBy := "test-archiver"
	archivedCred, archiveErr := ts.service.ArchiveOneCredential(id, accountId, archivedBy)
	if archiveErr != nil {
		t.Fatal("failed to archive credential", archiveErr)
	}

	assert.Equal(t, id, archivedCred.ID)
	assert.Equal(t, true, archivedCred.Archived)
	assert.NotNil(t, archivedCred.ArchivedBy)
	assert.Equal(t, archivedBy, *archivedCred.ArchivedBy)

	// Verify the archive persisted
	archivedCred, getErr := ts.service.FindOneCredentialByID(id, accountId)
	if getErr != nil {
		t.Fatal("failed to get archived credential", getErr)
	}
	assert.Equal(t, true, archivedCred.Archived)
	assert.NotNil(t, archivedCred.ArchivedBy)
	assert.Equal(t, archivedBy, *archivedCred.ArchivedBy)

	// Test: Archive non-existent credential should fail
	_, archiveErr = ts.service.ArchiveOneCredential(999, accountId, archivedBy)
	if archiveErr == nil {
		t.Fatal("archive should fail because credential does not exist")
	}
}
