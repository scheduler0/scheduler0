package account

import (
	"net/http"

	"scheduler0/pkg/constants"
	"scheduler0/pkg/models"
	"scheduler0/pkg/repository/account"
	account_ai_quota_period_repo "scheduler0/pkg/repository/account_ai_quota_period"
	account_job_executions_count_repo "scheduler0/pkg/repository/account_job_executions_count"
	classify_request_repo "scheduler0/pkg/repository/classify_request"
	feature_repo "scheduler0/pkg/repository/feature"
	job_repo "scheduler0/pkg/repository/job"
	prompt_request_repo "scheduler0/pkg/repository/prompt_request"
	"scheduler0/pkg/service/queue"
	"scheduler0/pkg/utils"
)

type AccountService interface {
	CreateAccount(account *models.Account) (uint64, *utils.GenericError)
	GetAccount(id uint64) (*models.Account, *utils.GenericError)
	UpdateAccount(id uint64, name string) *utils.GenericError
	GetFeatures(id uint64) (*[]models.AccountFeature, *utils.GenericError)
	GetFeaturesByAccountIds(accountIds []uint64) (map[uint64][]models.AccountFeature, *utils.GenericError)
	AddFeature(accountId uint64, featureId uint64) *utils.GenericError
	RemoveFeature(accountId uint64, featureId uint64) *utils.GenericError
	AddAllFeatures(accountId uint64) *utils.GenericError
	RemoveAllFeatures(accountId uint64) *utils.GenericError
	UpdateExecutionCount(accountId uint64, count uint64) *utils.GenericError
	GetExecutionCount(accountId uint64) (*models.AccountJobExecutionsCount, *utils.GenericError)
	IncreaseExecutionCount(accountId uint64, count uint64) (uint64, *utils.GenericError)
	GetTokens(accountId uint64) (uint64, *utils.GenericError)
	AddTokens(accountId uint64, amount uint64) (uint64, *utils.GenericError)
	DeductTokens(accountId uint64, amount uint64) (bool, uint64, *utils.GenericError)
	// GetAIUsage returns the account's log-derived AI usage for the current period: the
	// feature-derived monthly limit, the number of successful prompt/classify requests used
	// since the period started, and the remaining allowance. It is the single source of truth
	// the dashboard renders and request handlers enforce against. The period boundary is
	// advanced lazily when the reset date has passed.
	GetAIUsage(accountId uint64) (*models.AIUsage, *utils.GenericError)
}

type accountService struct {
	accountRepository             account.AccountRepository
	accountJobExecutionsCountRepo account_job_executions_count_repo.AccountJobExecutionsCountRepo
	promptRequestRepo             prompt_request_repo.PromptRequestRepo
	classifyRequestRepo           classify_request_repo.ClassifyRequestRepo
	aiQuotaPeriodRepo             account_ai_quota_period_repo.AccountAIQuotaPeriodRepo
	featureRepository             feature_repo.FeatureRepository
	jobRepository                 job_repo.JobRepo
	jobQueueService               queue.JobQueueService
}

func NewAccountService(
	accountRepository account.AccountRepository,
	accountJobExecutionsCountRepo account_job_executions_count_repo.AccountJobExecutionsCountRepo,
	promptRequestRepo prompt_request_repo.PromptRequestRepo,
	classifyRequestRepo classify_request_repo.ClassifyRequestRepo,
	aiQuotaPeriodRepo account_ai_quota_period_repo.AccountAIQuotaPeriodRepo,
	featureRepository feature_repo.FeatureRepository,
	jobRepository job_repo.JobRepo,
	jobQueueService queue.JobQueueService,
) AccountService {
	return &accountService{
		accountRepository:             accountRepository,
		accountJobExecutionsCountRepo: accountJobExecutionsCountRepo,
		promptRequestRepo:             promptRequestRepo,
		classifyRequestRepo:           classifyRequestRepo,
		aiQuotaPeriodRepo:             aiQuotaPeriodRepo,
		featureRepository:             featureRepository,
		jobRepository:                 jobRepository,
		jobQueueService:               jobQueueService,
	}
}

func (s *accountService) CreateAccount(account *models.Account) (uint64, *utils.GenericError) {
	accountID, createErr := s.accountRepository.CreateAccount(account)
	if createErr != nil {
		return 0, createErr
	}
	_, createErr = s.accountJobExecutionsCountRepo.Create(accountID, constants.DefaultNumberOfJobExecutions10KPerMonth)
	if createErr != nil {
		return 0, createErr
	}
	// Anchor the account's AI-quota window at signup. AI usage is log-derived, so no counter
	// is seeded; only the period boundary is stored (and advanced lazily thereafter).
	if _, periodErr := s.aiQuotaPeriodRepo.Create(accountID); periodErr != nil {
		return 0, periodErr
	}
	return accountID, nil
}

func (s *accountService) GetAccount(id uint64) (*models.Account, *utils.GenericError) {
	return s.accountRepository.GetAccount(id)
}

func (s *accountService) UpdateAccount(id uint64, name string) *utils.GenericError {
	return s.accountRepository.UpdateAccount(id, name)
}

func (s *accountService) GetFeatures(id uint64) (*[]models.AccountFeature, *utils.GenericError) {
	return s.accountRepository.GetFeatures(id)
}

func (s *accountService) GetFeaturesByAccountIds(accountIds []uint64) (map[uint64][]models.AccountFeature, *utils.GenericError) {
	return s.accountRepository.GetFeaturesByAccountIds(accountIds)
}

func (s *accountService) RemoveFeature(accountId uint64, featureId uint64) *utils.GenericError {
	// Remove the feature from the account
	removeErr := s.accountRepository.RemoveFeature(accountId, featureId)
	if removeErr != nil {
		return removeErr
	}

	// Note: When removing the IncreasedNumberOfJobExecutions100KPerMonthFeature,
	// we do NOT modify the execution count. The user has already paid for the tokens
	// and should keep them even after unsubscribing.

	return nil
}

// activateAndQueueJobsForAccount activates all jobs for an account and queues them
func (s *accountService) activateAndQueueJobsForAccount(accountId uint64) *utils.GenericError {
	// First, activate all jobs for this account
	updateErr := s.jobRepository.UpdateJobsStatusByAccountId(accountId, models.JobStatusActive)
	if updateErr != nil {
		return updateErr
	}

	// Get all jobs for this account
	jobs, err := s.jobRepository.GetAllByAccountID(accountId)
	if err != nil {
		return err
	}

	// TODO: Ensure jobs are not already queued.
	// Queue the jobs
	if len(jobs) > 0 {
		s.jobQueueService.Queue(jobs)
	}

	return nil
}

func (s *accountService) AddFeature(accountId uint64, featureId uint64) *utils.GenericError {
	// First, get the feature to check if it's the execution count feature
	feature, err := s.featureRepository.GetFeatureByID(featureId)
	if err != nil {
		return err
	}

	// Add the feature to the account
	addErr := s.accountRepository.AddFeature(accountId, featureId)
	if addErr != nil {
		return addErr
	}

	// If this is the IncreasedNumberOfJobExecutions100KPerMonthFeature, add 100K to current execution count
	if feature.Name == constants.IncreasedNumberOfJobExecutions100KPerMonthFeature {
		// Check if account already has an execution count record
		currentRecord, getErr := s.accountJobExecutionsCountRepo.GetByAccountId(accountId)
		if getErr != nil {
			// If no record exists, create one with 100K
			if getErr.Type == http.StatusNotFound {
				_, createErr := s.accountJobExecutionsCountRepo.Create(accountId, constants.DefaultNumberOfJobExecutions100KPerMonth)
				if createErr != nil {
					return createErr
				}
			} else {
				return getErr
			}
		} else {
			// Add 100K to current count (don't reset)
			newCount := currentRecord.ExecutionCount + constants.DefaultNumberOfJobExecutions100KPerMonth
			updateErr := s.accountJobExecutionsCountRepo.UpdateExecutionCount(accountId, newCount)
			if updateErr != nil {
				return updateErr
			}
		}

		// Activate and queue jobs for this account
		activateErr := s.activateAndQueueJobsForAccount(accountId)
		if activateErr != nil {
			return activateErr
		}
	}

	// The prompt/classify request features are pure entitlement tier markers: the monthly
	// limit is derived from feature membership (see promptLimitForAccount /
	// classifyLimitForAccount) and usage is log-derived, so adding the feature needs no
	// counter top-up — the higher limit takes effect immediately.

	return nil
}

func (s *accountService) UpdateExecutionCount(accountId uint64, count uint64) *utils.GenericError {
	return s.accountJobExecutionsCountRepo.UpdateExecutionCount(accountId, count)
}

func (s *accountService) GetExecutionCount(accountId uint64) (*models.AccountJobExecutionsCount, *utils.GenericError) {
	return s.accountJobExecutionsCountRepo.GetByAccountId(accountId)
}

func (s *accountService) AddAllFeatures(accountId uint64) *utils.GenericError {
	// Get all available features
	features, err := s.featureRepository.GetFeatures()
	if err != nil {
		return err
	}

	// Add each feature to the account
	for _, feature := range *features {
		addErr := s.AddFeature(accountId, feature.ID)
		if addErr != nil {
			return addErr
		}
	}

	return nil
}

func (s *accountService) RemoveAllFeatures(accountId uint64) *utils.GenericError {
	// Get all features for this account
	accountFeatures, err := s.accountRepository.GetFeatures(accountId)
	if err != nil {
		return err
	}

	// Remove each feature from the account
	for _, accountFeature := range *accountFeatures {
		removeErr := s.RemoveFeature(accountId, accountFeature.FeatureId)
		if removeErr != nil {
			return removeErr
		}
	}

	return nil
}

func (s *accountService) IncreaseExecutionCount(accountId uint64, count uint64) (uint64, *utils.GenericError) {
	currentExecutionCount, getErr := s.accountJobExecutionsCountRepo.GetByAccountId(accountId)
	if getErr != nil {
		return 0, getErr
	}
	newExecutionCount := currentExecutionCount.ExecutionCount + count
	updateErr := s.accountJobExecutionsCountRepo.UpdateExecutionCount(accountId, newExecutionCount)
	if updateErr != nil {
		return 0, updateErr
	}
	return newExecutionCount, nil
}

func (s *accountService) GetTokens(accountId uint64) (uint64, *utils.GenericError) {
	record, err := s.accountJobExecutionsCountRepo.GetByAccountId(accountId)
	if err != nil {
		return 0, err
	}
	return record.Tokens, nil
}

func (s *accountService) AddTokens(accountId uint64, amount uint64) (uint64, *utils.GenericError) {
	return s.accountJobExecutionsCountRepo.AddTokens(accountId, amount)
}

func (s *accountService) DeductTokens(accountId uint64, amount uint64) (bool, uint64, *utils.GenericError) {
	return s.accountJobExecutionsCountRepo.DeductTokens(accountId, amount)
}

// classifyLimitForAccount returns the monthly classify-request limit for an account:
// the increased cap when the account holds the classify feature, otherwise the free default.
func (s *accountService) classifyLimitForAccount(accountId uint64) (uint64, *utils.GenericError) {
	features, getErr := s.accountRepository.GetFeatures(accountId)
	if getErr != nil {
		return 0, getErr
	}
	limit := uint64(constants.DefaultNumberOfClassifyRequests1KPerMonth)
	if features != nil {
		for _, feature := range *features {
			if feature.Feature == constants.IncreasedNumberOfClassifyRequests100KPerMonthFeature {
				limit = constants.DefaultNumberOfClassifyRequests100KPerMonth
				break
			}
		}
	}
	return limit, nil
}

// promptLimitForAccount returns the monthly prompt-request limit for an account:
// the increased cap when the account holds the prompt feature, otherwise the free default.
func (s *accountService) promptLimitForAccount(accountId uint64) (uint64, *utils.GenericError) {
	features, getErr := s.accountRepository.GetFeatures(accountId)
	if getErr != nil {
		return 0, getErr
	}
	limit := uint64(constants.DefaultNumberOfPromptRequests1KPerMonth)
	if features != nil {
		for _, feature := range *features {
			if feature.Feature == constants.IncreasedNumberOfPromptRequests100KPerMonthFeature {
				limit = constants.DefaultNumberOfPromptRequests100KPerMonth
				break
			}
		}
	}
	return limit, nil
}

// usageDimension builds a usage dimension from a feature-derived limit and the number of
// successful requests used, flooring remaining at zero (a limit downgrade can leave used
// above the new limit for the remainder of the period).
func usageDimension(limit uint64, used uint64) models.AIUsageDimension {
	remaining := uint64(0)
	if limit > used {
		remaining = limit - used
	}
	return models.AIUsageDimension{Limit: limit, Used: used, Remaining: remaining}
}

func (s *accountService) GetAIUsage(accountId uint64) (*models.AIUsage, *utils.GenericError) {
	promptLimit, limitErr := s.promptLimitForAccount(accountId)
	if limitErr != nil {
		return nil, limitErr
	}
	classifyLimit, limitErr := s.classifyLimitForAccount(accountId)
	if limitErr != nil {
		return nil, limitErr
	}

	period, periodErr := s.aiQuotaPeriodRepo.EnsurePeriod(accountId)
	if periodErr != nil {
		return nil, periodErr
	}
	periodStart := period.PeriodStart

	promptUsed, promptCountErr := s.promptRequestRepo.CountPromptRequests(prompt_request_repo.PromptRequestFilter{
		AccountID: accountId,
		Status:    models.PromptRequestStatusSuccess,
		StartDate: &periodStart,
	})
	if promptCountErr != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, promptCountErr.Error())
	}

	classifyUsed, classifyCountErr := s.classifyRequestRepo.CountClassifyRequests(classify_request_repo.ClassifyRequestFilter{
		AccountID: accountId,
		Status:    models.ClassifyRequestStatusSuccess,
		StartDate: &periodStart,
	})
	if classifyCountErr != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, classifyCountErr.Error())
	}

	// Cost sums every status in the period: failed executions can still incur spend, while
	// skipped-intent / unknown-model rows contribute $0 and do not inflate the total.
	estimatedCostUSD, costErr := s.promptRequestRepo.SumEstimatedCostUSD(prompt_request_repo.PromptRequestFilter{
		AccountID: accountId,
		StartDate: &periodStart,
	})
	if costErr != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, costErr.Error())
	}

	return &models.AIUsage{
		AccountId:        accountId,
		PeriodStart:      period.PeriodStart,
		NextResetDate:    period.NextResetDate,
		Prompt:           usageDimension(promptLimit, promptUsed),
		Classify:         usageDimension(classifyLimit, classifyUsed),
		EstimatedCostUSD: estimatedCostUSD,
	}, nil
}
