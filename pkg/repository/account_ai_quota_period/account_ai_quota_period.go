package account_ai_quota_period

import (
	"fmt"
	"net/http"
	"scheduler0/pkg/constants"
	"scheduler0/pkg/fsm"
	"scheduler0/pkg/models"
	"scheduler0/pkg/scheduler0time"
	"scheduler0/pkg/utils"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/hashicorp/go-hclog"
)

const (
	TableName        = "account_ai_quota_period"
	ColAccountID     = "account_id"
	ColPeriodStart   = "period_start"
	ColNextResetDate = "next_reset_date"
	ColDateCreated   = "date_created"
	ColDateModified  = "date_modified"
)

// AccountAIQuotaPeriodRepo persists the per-account monthly AI-quota window. AI usage is
// log-derived, so this boundary is the only quota state stored per account. All writes go
// through the Raft log, mirroring the other account repositories.
type AccountAIQuotaPeriodRepo interface {
	// EnsurePeriod returns the account's current AI-quota window, creating it (anchored at
	// now) when missing and advancing it lazily when the reset date has passed. The returned
	// PeriodStart is the start of the current window that usage should be counted from.
	EnsurePeriod(accountId uint64) (*models.AIQuotaPeriod, *utils.GenericError)
	// Create initializes an account's window anchored at now, resetting one month out. It is
	// idempotent-friendly for account creation; callers may ignore an already-exists error.
	Create(accountId uint64) (*models.AIQuotaPeriod, *utils.GenericError)
	GetByAccountId(accountId uint64) (*models.AIQuotaPeriod, *utils.GenericError)
	DeleteByAccountId(accountId uint64) *utils.GenericError
}

type accountAIQuotaPeriodRepo struct {
	fsmStore              fsm.Scheduler0RaftStore
	scheduler0RaftActions fsm.Scheduler0RaftActions
	logger                hclog.Logger
}

func NewAccountAIQuotaPeriodRepo(
	logger hclog.Logger,
	scheduler0RaftActions fsm.Scheduler0RaftActions,
	fsmStore fsm.Scheduler0RaftStore,
) AccountAIQuotaPeriodRepo {
	return &accountAIQuotaPeriodRepo{
		fsmStore:              fsmStore,
		scheduler0RaftActions: scheduler0RaftActions,
		logger:                logger.Named("account-ai-quota-period-repo"),
	}
}

func (repo *accountAIQuotaPeriodRepo) Create(accountId uint64) (*models.AIQuotaPeriod, *utils.GenericError) {
	if accountId == 0 {
		return nil, utils.HTTPGenericError(http.StatusBadRequest, "account id is required")
	}
	now := scheduler0time.GetSchedulerTime().GetTime(time.Now())
	nextResetDate := now.AddDate(0, 1, 0)

	query, params, err := sq.Insert(TableName).
		Columns(ColAccountID, ColPeriodStart, ColNextResetDate, ColDateCreated, ColDateModified).
		Values(accountId, now, nextResetDate, now, now).
		ToSql()
	if err != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	_, applyErr := repo.scheduler0RaftActions.WriteCommandToRaftLog(
		repo.fsmStore.GetRaft(), constants.CommandTypeDbExecute, query, params, []uint64{}, 0,
	)
	if applyErr != nil {
		repo.logger.Error("Create: failed to write command to raft log", "error", applyErr, "accountId", accountId)
		return nil, applyErr
	}

	return &models.AIQuotaPeriod{
		AccountId:     accountId,
		PeriodStart:   now,
		NextResetDate: nextResetDate,
		DateCreated:   now,
		DateModified:  now,
	}, nil
}

func (repo *accountAIQuotaPeriodRepo) GetByAccountId(accountId uint64) (*models.AIQuotaPeriod, *utils.GenericError) {
	repo.fsmStore.GetDataStore().ConnectionLock()
	defer repo.fsmStore.GetDataStore().ConnectionUnlock()

	if accountId == 0 {
		return nil, utils.HTTPGenericError(http.StatusBadRequest, "account id is required")
	}

	selectBuilder := sq.Select(ColAccountID, ColPeriodStart, ColNextResetDate, ColDateCreated, ColDateModified).
		From(TableName).
		Where(fmt.Sprintf("%s = ?", ColAccountID), accountId).
		RunWith(repo.fsmStore.GetDataStore().GetOpenConnection())

	rows, err := selectBuilder.Query()
	if err != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, utils.HTTPGenericError(http.StatusNotFound, fmt.Sprintf("ai quota period not found for account id: %d", accountId))
	}

	var record models.AIQuotaPeriod
	if scanErr := rows.Scan(&record.AccountId, &record.PeriodStart, &record.NextResetDate, &record.DateCreated, &record.DateModified); scanErr != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, scanErr.Error())
	}
	return &record, nil
}

// EnsurePeriod returns the current window, creating or advancing it as needed.
func (repo *accountAIQuotaPeriodRepo) EnsurePeriod(accountId uint64) (*models.AIQuotaPeriod, *utils.GenericError) {
	record, getErr := repo.GetByAccountId(accountId)
	if getErr != nil {
		if getErr.Type == http.StatusNotFound {
			return repo.Create(accountId)
		}
		return nil, getErr
	}

	now := scheduler0time.GetSchedulerTime().GetTime(time.Now())
	if now.Before(record.NextResetDate) {
		return record, nil
	}

	// The window has rolled over (possibly by more than one month). Advance to the latest
	// boundary at or before now so usage is counted from the current window's start.
	periodStart := record.NextResetDate
	nextReset := record.NextResetDate.AddDate(0, 1, 0)
	for !now.Before(nextReset) {
		periodStart = nextReset
		nextReset = nextReset.AddDate(0, 1, 0)
	}

	if advErr := repo.advance(accountId, periodStart, nextReset); advErr != nil {
		return nil, advErr
	}
	record.PeriodStart = periodStart
	record.NextResetDate = nextReset
	return record, nil
}

func (repo *accountAIQuotaPeriodRepo) advance(accountId uint64, periodStart, nextReset time.Time) *utils.GenericError {
	now := scheduler0time.GetSchedulerTime().GetTime(time.Now())
	query, params, err := sq.Update(TableName).
		Set(ColPeriodStart, periodStart).
		Set(ColNextResetDate, nextReset).
		Set(ColDateModified, now).
		Where(fmt.Sprintf("%s = ?", ColAccountID), accountId).
		ToSql()
	if err != nil {
		return utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	_, applyErr := repo.scheduler0RaftActions.WriteCommandToRaftLog(
		repo.fsmStore.GetRaft(), constants.CommandTypeDbExecute, query, params, []uint64{}, 0,
	)
	if applyErr != nil {
		repo.logger.Error("advance: failed to write command to raft log", "error", applyErr, "accountId", accountId)
		return applyErr
	}
	return nil
}

func (repo *accountAIQuotaPeriodRepo) DeleteByAccountId(accountId uint64) *utils.GenericError {
	if accountId == 0 {
		return utils.HTTPGenericError(http.StatusBadRequest, "account id is required")
	}
	query, params, err := sq.Delete(TableName).
		Where(fmt.Sprintf("%s = ?", ColAccountID), accountId).
		ToSql()
	if err != nil {
		return utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	_, applyErr := repo.scheduler0RaftActions.WriteCommandToRaftLog(
		repo.fsmStore.GetRaft(), constants.CommandTypeDbExecute, query, params, []uint64{}, 0,
	)
	if applyErr != nil {
		return applyErr
	}
	return nil
}
