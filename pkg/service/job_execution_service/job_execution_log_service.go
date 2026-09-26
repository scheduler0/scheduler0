package job_execution_service

import (
	"fmt"
	"scheduler0/pkg/constants"
	"scheduler0/pkg/models"
	account_repo "scheduler0/pkg/repository/account"
	"scheduler0/pkg/repository/account_job_executions_count"
	job_repo "scheduler0/pkg/repository/job"
	"scheduler0/pkg/repository/job_execution"
	job_queue_repo "scheduler0/pkg/repository/job_queue"
	"time"

	"github.com/hashicorp/go-hclog"
)

// JobExecutionLogService provides access to job execution logs
// (now in its own file)
type JobExecutionLogService interface {
	GetExecutionLogsFiltered(accountId uint64, startDate, endDate *time.Time, projectId *uint64, jobId *uint64, state *models.JobExecutionLogState, orderBy string, orderDirection string) ([]models.JobExecutionLog, error)
	GetDateRangeAnalytics(accountId uint64, startDate, startTime time.Time) (*models.DateRangeAnalyticsResponse, error)
	GetExecutionTotals(accountId uint64) (*models.ExecutionTotalsResponse, error)
	CleanupOldExecutionLogsForAccount(accountId uint64, retentionDays int) error
}

type jobExecutionLogService struct {
	repo                  job_execution.JobExecutionsRepo
	accountsExecutionRepo account_job_executions_count.AccountJobExecutionsCountRepo
	jobsQueueRepo         job_queue_repo.JobQueuesRepo
	accountRepo           account_repo.AccountRepository
	jobRepo               job_repo.JobRepo
	logger                hclog.Logger
}

func NewJobExecutionLogService(
	repo job_execution.JobExecutionsRepo,
	logger hclog.Logger,
	accountsExecutionRepo account_job_executions_count.AccountJobExecutionsCountRepo,
	jobsQueueRepo job_queue_repo.JobQueuesRepo,
	accountRepo account_repo.AccountRepository,
	jobRepo job_repo.JobRepo,
) JobExecutionLogService {
	return &jobExecutionLogService{
		repo:                  repo,
		accountsExecutionRepo: accountsExecutionRepo,
		jobsQueueRepo:         jobsQueueRepo,
		accountRepo:           accountRepo,
		jobRepo:               jobRepo,
		logger:                logger,
	}
}

func (s *jobExecutionLogService) GetExecutionLogsFiltered(accountId uint64, startDate, endDate *time.Time, projectId *uint64, jobId *uint64, state *models.JobExecutionLogState, orderBy string, orderDirection string) ([]models.JobExecutionLog, error) {
	s.logger.Info("GetExecutionLogsFiltered entry",
		"accountId", accountId,
		"startDate", startDate,
		"endDate", endDate,
		"projectId", projectId,
		"jobId", jobId,
		"state", state,
		"orderBy", orderBy,
		"orderDirection", orderDirection)

	logs, err := s.repo.GetExecutionLogsFiltered(accountId, startDate, endDate, projectId, jobId, state, orderBy, orderDirection)

	if err != nil {
		s.logger.Error("GetExecutionLogsFiltered repository error", "error", err, "accountId", accountId)
		return nil, err
	}

	s.logger.Info("GetExecutionLogsFiltered success", "accountId", accountId, "count", len(logs))
	return logs, nil
}

func (s *jobExecutionLogService) GetDateRangeAnalytics(accountId uint64, startDate, startTime time.Time) (*models.DateRangeAnalyticsResponse, error) {
	return s.repo.GetDateRangeAnalytics(accountId, startDate, startTime)
}

func (s *jobExecutionLogService) GetExecutionTotals(accountId uint64) (*models.ExecutionTotalsResponse, error) {
	return s.repo.GetExecutionTotals(accountId)
}

func (s *jobExecutionLogService) CleanupOldExecutionLogsForAccount(accountId uint64, retentionDays int) error {
	s.logger.Info("CleanupOldExecutionLogsForAccount entry",
		"accountId", accountId,
		"retentionDays", retentionDays)

	counts, err := s.accountsExecutionRepo.GetExecutionCountsByAccountIds([]uint64{accountId})
	if err != nil {
		s.logger.Error("failed to get execution counts for account id: ", accountId)
		return err
	}

	mostRecentQueueDate, queueDateErr := s.jobsQueueRepo.GetMostRecentJobQueueDate()
	if queueDateErr != nil {
		s.logger.Error("failed to get most recent job queue date", "error", queueDateErr)
		return fmt.Errorf("failed to get most recent job queue date: %w", queueDateErr)
	}
	s.logger.Debug("most recent job queue date", "date", mostRecentQueueDate, "isZero", mostRecentQueueDate.IsZero())

	var usageByAccount map[uint64]uint64
	if !mostRecentQueueDate.IsZero() {
		var usageErr error
		usageByAccount, usageErr = s.repo.GetExecutionUsageByAccountIds([]uint64{accountId}, mostRecentQueueDate)
		if usageErr != nil {
			s.logger.Error("failed to get execution usage by account ids", "error", usageErr, "startDate", mostRecentQueueDate)
			return fmt.Errorf("failed to get execution usage by account ids: %w", usageErr)
		}
		s.logger.Debug("retrieved execution usage from committed logs", "usageByAccount", usageByAccount)
	} else {
		usageByAccount = make(map[uint64]uint64)
		s.logger.Debug("no previous queue date, starting with zero usage")
	}

	accountFeatures, err := s.accountRepo.GetFeaturesByAccountIds([]uint64{accountId})
	if err != nil {
		s.logger.Error("failed to get account features", "error", err)
		return err
	}
	s.logger.Debug("retrieved account features", "accountFeaturesCount", len(accountFeatures))

	executionTotal := usageByAccount[accountId]

	currentExecutionCount := counts[accountId] + executionTotal

	executionLimit := uint64(constants.DefaultNumberOfJobExecutions10KPerMonth)
	if features, ok := accountFeatures[accountId]; ok {
		for _, feature := range features {
			if feature.Feature == constants.IncreasedNumberOfJobExecutions100KPerMonthFeature {
				executionLimit = constants.DefaultNumberOfJobExecutions100KPerMonth
				break
			}
		}
	}

	if currentExecutionCount > executionLimit && accountId != 1 {
		err := s.jobRepo.UpdateJobsStatusByAccountId(accountId, models.JobStatusInactive)
		if err != nil {
			s.logger.Error("failed to update job status to inactive", "error", err)
			return err
		}
	}

	if err := s.repo.DeleteOldExecutionLogsForAccount(accountId, retentionDays); err != nil {
		s.logger.Error("failed to cleanup old execution logs for account", "error", err, "accountId", accountId, "retentionDays", retentionDays)
		return err
	}

	s.logger.Info("successfully cleaned up old execution logs for account", "accountId", accountId, "retentionDays", retentionDays)
	return nil
}
