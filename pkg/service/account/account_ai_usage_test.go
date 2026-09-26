package account

import (
	"context"
	"os"
	"testing"
	"time"

	"scheduler0-private/pkg/config"
	"scheduler0-private/pkg/constants"
	"scheduler0-private/pkg/db"
	"scheduler0-private/pkg/fsm"
	"scheduler0-private/pkg/models"
	account_repo "scheduler0-private/pkg/repository/account"
	account_ai_quota_period_repo "scheduler0-private/pkg/repository/account_ai_quota_period"
	account_job_executions_count_repo "scheduler0-private/pkg/repository/account_job_executions_count"
	classify_request_repo "scheduler0-private/pkg/repository/classify_request"
	feature_repo "scheduler0-private/pkg/repository/feature"
	job_repo "scheduler0-private/pkg/repository/job"
	prompt_request_repo "scheduler0-private/pkg/repository/prompt_request"
	"scheduler0-private/pkg/service/queue"
	"scheduler0-private/pkg/shared_repo"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// aiUsageDeps bundles the repositories a GetAIUsage test needs to seed the request logs.
type aiUsageDeps struct {
	service         AccountService
	promptRepo      prompt_request_repo.PromptRequestRepo
	classifyRepo    classify_request_repo.ClassifyRequestRepo
	quotaPeriodRepo account_ai_quota_period_repo.AccountAIQuotaPeriodRepo
}

func setupAIUsageTest(t *testing.T) (aiUsageDeps, func()) {
	logger := hclog.New(&hclog.LoggerOptions{Name: "account-ai-usage-test", Level: hclog.LevelFromString("ERROR")})

	tempFile, err := os.CreateTemp("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tempFile.Close()
	tempFileName := tempFile.Name()

	sqliteDb := db.NewSqliteDbConnection(logger, tempFileName)
	sqliteDb.RunMigration(logger)
	sqliteDb.OpenConnectionToExistingDB()

	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)
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
		MakeFSMFunc:    func() raft.FSM { return scheduler0Store.GetFSM() },
	})
	cluster.FullyConnect()
	scheduler0Store.UpdateRaft(cluster.Leader())

	ctx := context.TODO()
	accountRepo := account_repo.NewAccountRepository(ctx, logger, scheduler0RaftActions, scheduler0Store)
	accountJobExecutionsCountRepo := account_job_executions_count_repo.NewAccountJobExecutionsCountRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)
	promptRepo := prompt_request_repo.NewPromptRequestRepo(logger, scheduler0RaftActions, scheduler0Store)
	classifyRepo := classify_request_repo.NewClassifyRequestRepo(logger, scheduler0RaftActions, scheduler0Store)
	quotaPeriodRepo := account_ai_quota_period_repo.NewAccountAIQuotaPeriodRepo(logger, scheduler0RaftActions, scheduler0Store)
	featureRepo := feature_repo.NewFeatureRepository(ctx, scheduler0Store)
	jobRepo := job_repo.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)

	mockJobQueueService := queue.NewMockJobQueueService(t)
	mockJobQueueService.On("GetJobAllocations").Return(make(map[uint64]uint64)).Maybe()
	mockJobQueueService.On("GetSingleNodeMode").Return(false).Maybe()
	mockJobQueueService.On("Queue", mock.Anything).Return().Maybe()

	svc := NewAccountService(accountRepo, accountJobExecutionsCountRepo, promptRepo, classifyRepo, quotaPeriodRepo, featureRepo, jobRepo, mockJobQueueService)

	cleanup := func() {
		cluster.Close()
		os.Remove(tempFileName)
	}
	return aiUsageDeps{service: svc, promptRepo: promptRepo, classifyRepo: classifyRepo, quotaPeriodRepo: quotaPeriodRepo}, cleanup
}

// Test_GetAIUsage_CountsOnlyInPeriodSuccessRows verifies usage is COUNT(success rows in the
// current period): failed/skipped rows and rows before period_start are ignored, and the
// feature-derived free-tier limit is surfaced with the correct remaining allowance.
func Test_GetAIUsage_CountsOnlyInPeriodSuccessRows(t *testing.T) {
	deps, cleanup := setupAIUsageTest(t)
	defer cleanup()

	accountId, createErr := deps.service.CreateAccount(&models.Account{Name: "Usage Test"})
	assert.Nil(t, createErr)

	now := time.Now().UTC()
	before := now.AddDate(0, 0, -40) // before the current period start (anchored at signup ~now)

	// Prompt: 3 in-period successes (counted), 1 failure (ignored), 1 pre-period success (ignored).
	for i := 0; i < 3; i++ {
		assert.NoError(t, deps.promptRepo.Record(models.AccountPromptRequest{AccountID: accountId, Status: models.PromptRequestStatusSuccess, DateCreated: now}))
	}
	assert.NoError(t, deps.promptRepo.Record(models.AccountPromptRequest{AccountID: accountId, Status: models.PromptRequestStatusFailed, DateCreated: now}))
	assert.NoError(t, deps.promptRepo.Record(models.AccountPromptRequest{AccountID: accountId, Status: models.PromptRequestStatusSuccess, DateCreated: before}))

	// Classify: 2 in-period successes (counted), 1 failure (ignored).
	for i := 0; i < 2; i++ {
		assert.NoError(t, deps.classifyRepo.Record(models.AccountClassifyRequest{AccountID: accountId, Kind: models.ClassifyRequestKindClassify, Status: models.ClassifyRequestStatusSuccess, DateCreated: now}))
	}
	assert.NoError(t, deps.classifyRepo.Record(models.AccountClassifyRequest{AccountID: accountId, Kind: models.ClassifyRequestKindClassify, Status: models.ClassifyRequestStatusFailed, DateCreated: now}))

	usage, usageErr := deps.service.GetAIUsage(accountId)
	assert.Nil(t, usageErr)

	// Free-tier feature-derived limits.
	assert.Equal(t, uint64(constants.DefaultNumberOfPromptRequests1KPerMonth), usage.Prompt.Limit)
	assert.Equal(t, uint64(3), usage.Prompt.Used)
	assert.Equal(t, usage.Prompt.Limit-3, usage.Prompt.Remaining)

	assert.Equal(t, uint64(constants.DefaultNumberOfClassifyRequests1KPerMonth), usage.Classify.Limit)
	assert.Equal(t, uint64(2), usage.Classify.Used)
	assert.Equal(t, usage.Classify.Limit-2, usage.Classify.Remaining)

	// No costs were recorded on the seed rows.
	assert.InDelta(t, 0.0, usage.EstimatedCostUSD, 1e-9)
}

// Test_GetAIUsage_SumsEstimatedCostInPeriod verifies EstimatedCostUSD is the sum of
// estimated_cost_usd for all prompt-request rows in the current period (any status), and
// excludes pre-period rows.
func Test_GetAIUsage_SumsEstimatedCostInPeriod(t *testing.T) {
	deps, cleanup := setupAIUsageTest(t)
	defer cleanup()

	accountId, createErr := deps.service.CreateAccount(&models.Account{Name: "Cost Usage Test"})
	assert.Nil(t, createErr)

	now := time.Now().UTC()
	before := now.AddDate(0, 0, -40)

	assert.NoError(t, deps.promptRepo.Record(models.AccountPromptRequest{
		AccountID: accountId, Status: models.PromptRequestStatusSuccess, EstimatedCostUSD: 0.01, DateCreated: now,
	}))
	assert.NoError(t, deps.promptRepo.Record(models.AccountPromptRequest{
		AccountID: accountId, Status: models.PromptRequestStatusFailed, EstimatedCostUSD: 0.0025, DateCreated: now,
	}))
	assert.NoError(t, deps.promptRepo.Record(models.AccountPromptRequest{
		AccountID: accountId, Status: models.PromptRequestStatusSkippedIntent, EstimatedCostUSD: 0, DateCreated: now,
	}))
	// Pre-period spend must not count toward the current period total.
	assert.NoError(t, deps.promptRepo.Record(models.AccountPromptRequest{
		AccountID: accountId, Status: models.PromptRequestStatusSuccess, EstimatedCostUSD: 9.99, DateCreated: before,
	}))

	usage, usageErr := deps.service.GetAIUsage(accountId)
	assert.Nil(t, usageErr)
	assert.InDelta(t, 0.0125, usage.EstimatedCostUSD, 1e-9)
}
