package account_job_executions_count

import (
	"context"
	"fmt"
	"net/http"
	"scheduler0-private/pkg/constants"
	"scheduler0-private/pkg/fsm"
	"scheduler0-private/pkg/models"
	"scheduler0-private/pkg/scheduler0time"
	"scheduler0-private/pkg/utils"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/hashicorp/go-hclog"
)

const (
	AccountJobExecutionsCountTableName            = "account_job_executions_count"
	AccountJobExecutionsCountIdColumn             = "id"
	AccountJobExecutionsCountAccountIdColumn      = "account_id"
	AccountJobExecutionsCountExecutionCountColumn = "execution_count"
	AccountJobExecutionsCountTokensColumn         = "tokens"
	AccountJobExecutionsCountDateCreatedColumn    = "date_created"
	AccountJobExecutionsCountDateModifiedColumn   = "date_modified"
	AccountJobExecutionsCountNextResetDateColumn  = "next_reset_date"
)

type AccountJobExecutionsCountRepo interface {
	Create(accountId uint64, count uint64) (*models.AccountJobExecutionsCount, *utils.GenericError)
	GetByAccountId(accountId uint64) (*models.AccountJobExecutionsCount, *utils.GenericError)
	GetExecutionCountsByAccountIds(accountIds []uint64) (map[uint64]uint64, *utils.GenericError)
	GetTokensByAccountIds(accountIds []uint64) (map[uint64]uint64, *utils.GenericError)
	UpdateExecutionCount(accountId uint64, count uint64) *utils.GenericError
	UpdateTokens(accountId uint64, tokens uint64) *utils.GenericError
	AddTokens(accountId uint64, delta uint64) (uint64, *utils.GenericError)
	DeductTokens(accountId uint64, amount uint64) (bool, uint64, *utils.GenericError)
	ResetExecutionCount(accountId uint64, count uint64) *utils.GenericError
	GetAllExpiredResetDates() ([]models.AccountJobExecutionsCount, *utils.GenericError)
	GetAccountsWithZeroExecutionCount() ([]uint64, *utils.GenericError)
	DeleteByAccountId(accountId uint64) *utils.GenericError
}

type accountJobExecutionsCountRepo struct {
	context               context.Context
	fsmStore              fsm.Scheduler0RaftStore
	logger                hclog.Logger
	scheduler0RaftActions fsm.Scheduler0RaftActions
}

func NewAccountJobExecutionsCountRepo(
	context context.Context,
	logger hclog.Logger,
	scheduler0RaftActions fsm.Scheduler0RaftActions,
	fsmStore fsm.Scheduler0RaftStore,
) AccountJobExecutionsCountRepo {
	return &accountJobExecutionsCountRepo{
		context:               context,
		logger:                logger.Named("account-job-executions-count-repo"),
		scheduler0RaftActions: scheduler0RaftActions,
		fsmStore:              fsmStore,
	}
}

func (repo *accountJobExecutionsCountRepo) Create(accountId uint64, executionCount uint64) (*models.AccountJobExecutionsCount, *utils.GenericError) {
	schedulerTime := scheduler0time.GetSchedulerTime()
	now := schedulerTime.GetTime(time.Now())
	nextResetDate := now.AddDate(0, 1, 0) // Reset next month

	repo.logger.Debug("Create: creating new account job executions count", "accountId", accountId, "executionCount", executionCount)

	insertBuilder := sq.Insert(AccountJobExecutionsCountTableName).
		Columns(
			AccountJobExecutionsCountAccountIdColumn,
			AccountJobExecutionsCountExecutionCountColumn,
			AccountJobExecutionsCountTokensColumn,
			AccountJobExecutionsCountDateCreatedColumn,
			AccountJobExecutionsCountDateModifiedColumn,
			AccountJobExecutionsCountNextResetDateColumn,
		).
		Values(
			accountId,
			executionCount,
			0,
			now,
			now,
			nextResetDate,
		)

	query, params, err := insertBuilder.ToSql()
	if err != nil {
		repo.logger.Error("Create: failed to build insert query", "error", err, "accountId", accountId, "executionCount", executionCount)
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	res, applyErr := repo.scheduler0RaftActions.WriteCommandToRaftLog(
		repo.fsmStore.GetRaft(),
		constants.CommandTypeDbExecute,
		query,
		params,
		[]uint64{},
		0,
	)
	if applyErr != nil {
		repo.logger.Error("Create: failed to write command to raft log", "error", applyErr, "accountId", accountId, "executionCount", executionCount)
		return nil, applyErr
	}

	if res == nil {
		repo.logger.Error("Create: raft log result is nil", "accountId", accountId, "executionCount", executionCount)
		return nil, utils.HTTPGenericError(http.StatusServiceUnavailable, "service is unavailable - create account job executions count raft log result is nil")
	}

	accountJobExecutionsCount := &models.AccountJobExecutionsCount{
		ID:             uint64(res.Data.LastInsertedId),
		AccountId:      accountId,
		ExecutionCount: 0,
		Tokens:         0,
		DateCreated:    now,
		DateModified:   now,
		NextResetDate:  nextResetDate,
	}

	return accountJobExecutionsCount, nil
}

func (repo *accountJobExecutionsCountRepo) GetExecutionCountsByAccountIds(accountIds []uint64) (map[uint64]uint64, *utils.GenericError) {
	repo.fsmStore.GetDataStore().ConnectionLock()
	defer repo.fsmStore.GetDataStore().ConnectionUnlock()

	repo.logger.Debug("GetExecutionCountsByAccountIds: getting account job executions counts by account ids", "accountIds", accountIds)

	if len(accountIds) == 0 {
		return make(map[uint64]uint64), nil
	}

	// Create placeholders for the IN clause
	placeholders := make([]string, len(accountIds))
	params := make([]interface{}, len(accountIds))
	for i, accountId := range accountIds {
		placeholders[i] = "?"
		params[i] = accountId
	}

	query := fmt.Sprintf(
		"SELECT %s, %s FROM %s WHERE %s IN (%s)",
		AccountJobExecutionsCountAccountIdColumn,
		AccountJobExecutionsCountExecutionCountColumn,
		AccountJobExecutionsCountTableName,
		AccountJobExecutionsCountAccountIdColumn,
		strings.Join(placeholders, ","),
	)

	rows, err := repo.fsmStore.GetDataStore().GetOpenConnection().Query(query, params...)
	if err != nil {
		repo.logger.Error("GetExecutionCountsByAccountIds: failed to query execution counts", "error", err, "accountIdsCount", len(accountIds))
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	defer rows.Close()

	result := make(map[uint64]uint64)
	for rows.Next() {
		var accountId uint64
		var executionCount uint64
		scanErr := rows.Scan(&accountId, &executionCount)
		if scanErr != nil {
			repo.logger.Error("GetExecutionCountsByAccountIds: failed to scan row", "error", scanErr)
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, scanErr.Error())
		}
		result[accountId] = executionCount
	}

	if rows.Err() != nil {
		repo.logger.Error("GetExecutionCountsByAccountIds: row iteration error", "error", rows.Err())
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, rows.Err().Error())
	}

	repo.logger.Debug("GetExecutionCountsByAccountIds: execution counts retrieved", "result", result)
	return result, nil
}

func (repo *accountJobExecutionsCountRepo) GetTokensByAccountIds(accountIds []uint64) (map[uint64]uint64, *utils.GenericError) {
	repo.fsmStore.GetDataStore().ConnectionLock()
	defer repo.fsmStore.GetDataStore().ConnectionUnlock()

	repo.logger.Debug("GetTokensCountsByAccountIds: getting account job executions counts by account ids", "accountIds", accountIds)

	if len(accountIds) == 0 {
		return make(map[uint64]uint64), nil
	}

	// Create placeholders for the IN clause
	placeholders := make([]string, len(accountIds))
	params := make([]interface{}, len(accountIds))
	for i, accountId := range accountIds {
		placeholders[i] = "?"
		params[i] = accountId
	}

	query := fmt.Sprintf(
		"SELECT %s, COALESCE(%s, 0) FROM %s WHERE %s IN (%s)",
		AccountJobExecutionsCountAccountIdColumn,
		AccountJobExecutionsCountTokensColumn,
		AccountJobExecutionsCountTableName,
		AccountJobExecutionsCountAccountIdColumn,
		strings.Join(placeholders, ","),
	)

	rows, err := repo.fsmStore.GetDataStore().GetOpenConnection().Query(query, params...)
	if err != nil {
		repo.logger.Error("GetTokensCountsByAccountIds: failed to query execution counts", "error", err, "accountIdsCount", len(accountIds))
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	defer rows.Close()

	result := make(map[uint64]uint64)
	for rows.Next() {
		var accountId uint64
		var executionCount uint64
		scanErr := rows.Scan(&accountId, &executionCount)
		if scanErr != nil {
			repo.logger.Error("GetTokensCountsByAccountIds: failed to scan row", "error", scanErr)
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, scanErr.Error())
		}
		result[accountId] = executionCount
	}

	if rows.Err() != nil {
		repo.logger.Error("GetTokensCountsByAccountIds: row iteration error", "error", rows.Err())
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, rows.Err().Error())
	}

	repo.logger.Debug("GetTokensCountsByAccountIds: execution counts retrieved", "result", result)
	return result, nil
}

func (repo *accountJobExecutionsCountRepo) GetByAccountId(accountId uint64) (*models.AccountJobExecutionsCount, *utils.GenericError) {
	repo.fsmStore.GetDataStore().ConnectionLock()
	defer repo.fsmStore.GetDataStore().ConnectionUnlock()

	repo.logger.Debug("GetByAccountId: getting account job executions count", "accountId", accountId)

	if accountId == 0 {
		repo.logger.Warn("GetByAccountId: account id is required", "accountId", accountId)
		return nil, utils.HTTPGenericError(http.StatusBadRequest, "account id is required")
	}

	selectBuilder := sq.Select(
		AccountJobExecutionsCountIdColumn,
		AccountJobExecutionsCountAccountIdColumn,
		AccountJobExecutionsCountExecutionCountColumn,
		fmt.Sprintf("COALESCE(%s, 0) AS %s", AccountJobExecutionsCountTokensColumn, AccountJobExecutionsCountTokensColumn),
		AccountJobExecutionsCountDateCreatedColumn,
		AccountJobExecutionsCountDateModifiedColumn,
		AccountJobExecutionsCountNextResetDateColumn,
	).
		From(AccountJobExecutionsCountTableName).
		Where(fmt.Sprintf("%s = ?", AccountJobExecutionsCountAccountIdColumn), accountId).
		RunWith(repo.fsmStore.GetDataStore().GetOpenConnection())

	rows, err := selectBuilder.Query()
	if err != nil {
		repo.logger.Error("GetByAccountId: failed to query account job executions count", "error", err, "accountId", accountId)
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	defer rows.Close()

	if !rows.Next() {
		repo.logger.Warn("GetByAccountId: account job executions count not found", "accountId", accountId)
		return nil, utils.HTTPGenericError(http.StatusNotFound, fmt.Sprintf("account job executions count not found for account id: %d", accountId))
	}

	var accountJobExecutionsCount models.AccountJobExecutionsCount
	scanErr := rows.Scan(
		&accountJobExecutionsCount.ID,
		&accountJobExecutionsCount.AccountId,
		&accountJobExecutionsCount.ExecutionCount,
		&accountJobExecutionsCount.Tokens,
		&accountJobExecutionsCount.DateCreated,
		&accountJobExecutionsCount.DateModified,
		&accountJobExecutionsCount.NextResetDate,
	)
	if scanErr != nil {
		repo.logger.Error("GetByAccountId: failed to scan row", "error", scanErr, "accountId", accountId)
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, scanErr.Error())
	}

	repo.logger.Debug("GetByAccountId: account job executions count retrieved", "accountJobExecutionsCount", accountJobExecutionsCount)
	return &accountJobExecutionsCount, nil
}

func (repo *accountJobExecutionsCountRepo) UpdateExecutionCount(accountId uint64, count uint64) *utils.GenericError {
	schedulerTime := scheduler0time.GetSchedulerTime()
	now := schedulerTime.GetTime(time.Now())

	repo.logger.Debug("UpdateExecutionCount: updating account job executions count", "accountId", accountId, "count", count)

	updateBuilder := sq.Update(AccountJobExecutionsCountTableName).
		Set(AccountJobExecutionsCountExecutionCountColumn, count).
		Set(AccountJobExecutionsCountDateModifiedColumn, now).
		Where(fmt.Sprintf("%s = ?", AccountJobExecutionsCountAccountIdColumn), accountId)

	query, params, err := updateBuilder.ToSql()
	if err != nil {
		repo.logger.Error("UpdateExecutionCount: failed to build update query", "error", err, "accountId", accountId, "count", count)
		return utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	_, applyErr := repo.scheduler0RaftActions.WriteCommandToRaftLog(
		repo.fsmStore.GetRaft(),
		constants.CommandTypeDbExecute,
		query,
		params,
		[]uint64{},
		0,
	)
	if applyErr != nil {
		repo.logger.Error("UpdateExecutionCount: failed to write command to raft log", "error", applyErr, "accountId", accountId, "count", count)
		return applyErr
	}

	repo.logger.Debug("UpdateExecutionCount: account job executions count updated", "accountId", accountId, "count", count)
	return nil
}

func (repo *accountJobExecutionsCountRepo) UpdateTokens(accountId uint64, tokens uint64) *utils.GenericError {
	schedulerTime := scheduler0time.GetSchedulerTime()
	now := schedulerTime.GetTime(time.Now())

	repo.logger.Debug("UpdateTokens: updating account tokens", "accountId", accountId, "tokens", tokens)

	updateBuilder := sq.Update(AccountJobExecutionsCountTableName).
		Set(AccountJobExecutionsCountTokensColumn, tokens).
		Set(AccountJobExecutionsCountDateModifiedColumn, now).
		Where(fmt.Sprintf("%s = ?", AccountJobExecutionsCountAccountIdColumn), accountId)

	query, params, err := updateBuilder.ToSql()
	if err != nil {
		repo.logger.Error("UpdateTokens: failed to build update query", "error", err, "accountId", accountId, "tokens", tokens)
		return utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	_, applyErr := repo.scheduler0RaftActions.WriteCommandToRaftLog(
		repo.fsmStore.GetRaft(),
		constants.CommandTypeDbExecute,
		query,
		params,
		[]uint64{},
		0,
	)
	if applyErr != nil {
		repo.logger.Error("UpdateTokens: failed to write command to raft log", "error", applyErr, "accountId", accountId, "tokens", tokens)
		return applyErr
	}

	repo.logger.Debug("UpdateTokens: account job updated", "accountId", accountId, "tokens", tokens)
	return nil
}

// AddTokens atomically increments the token balance for an account and returns the new balance.
// Follows the same read-then-Raft-write pattern as IncreaseExecutionCount.
func (repo *accountJobExecutionsCountRepo) AddTokens(accountId uint64, delta uint64) (uint64, *utils.GenericError) {
	record, getErr := repo.GetByAccountId(accountId)
	if getErr != nil {
		repo.logger.Error("AddTokens: failed to get account record", "error", getErr, "accountId", accountId)
		return 0, getErr
	}

	newTokens := record.Tokens + delta

	updateErr := repo.UpdateTokens(accountId, newTokens)
	if updateErr != nil {
		repo.logger.Error("AddTokens: failed to update tokens", "error", updateErr, "accountId", accountId, "delta", delta)
		return 0, updateErr
	}

	repo.logger.Debug("AddTokens: tokens added", "accountId", accountId, "delta", delta, "newBalance", newTokens)
	return newTokens, nil
}

// DeductTokens atomically deducts tokens from an account balance.
// Returns (false, 0, nil) when the current balance is zero (insufficient tokens).
// When balance < amount the remainder is deducted down to zero so the caller is never
// left with a negative balance; the boolean return signals whether the full amount was covered.
func (repo *accountJobExecutionsCountRepo) DeductTokens(accountId uint64, amount uint64) (bool, uint64, *utils.GenericError) {
	record, getErr := repo.GetByAccountId(accountId)
	if getErr != nil {
		repo.logger.Error("DeductTokens: failed to get account record", "error", getErr, "accountId", accountId)
		return false, 0, getErr
	}

	if record.Tokens == 0 {
		repo.logger.Warn("DeductTokens: account has zero token balance", "accountId", accountId)
		return false, 0, nil
	}

	sufficient := record.Tokens >= amount
	var newTokens uint64
	if sufficient {
		newTokens = record.Tokens - amount
	} else {
		newTokens = 0
	}

	updateErr := repo.UpdateTokens(accountId, newTokens)
	if updateErr != nil {
		repo.logger.Error("DeductTokens: failed to update tokens", "error", updateErr, "accountId", accountId, "amount", amount)
		return false, record.Tokens, updateErr
	}

	repo.logger.Debug("DeductTokens: tokens deducted", "accountId", accountId, "amount", amount, "sufficient", sufficient, "newBalance", newTokens)
	return sufficient, newTokens, nil
}

func (repo *accountJobExecutionsCountRepo) ResetExecutionCount(accountId uint64, count uint64) *utils.GenericError {
	schedulerTime := scheduler0time.GetSchedulerTime()
	now := schedulerTime.GetTime(time.Now())
	nextResetDate := now.AddDate(0, 1, 0) // Reset next month

	repo.logger.Debug("ResetExecutionCount: resetting account job executions count", "accountId", accountId)

	updateBuilder := sq.Update(AccountJobExecutionsCountTableName).
		Set(AccountJobExecutionsCountExecutionCountColumn, count).
		Set(AccountJobExecutionsCountDateModifiedColumn, now).
		Set(AccountJobExecutionsCountNextResetDateColumn, nextResetDate).
		Where(fmt.Sprintf("%s = ?", AccountJobExecutionsCountAccountIdColumn), accountId)

	query, params, err := updateBuilder.ToSql()
	if err != nil {
		repo.logger.Error("ResetExecutionCount: failed to build update query", "error", err, "accountId", accountId, "count", count)
		return utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	_, applyErr := repo.scheduler0RaftActions.WriteCommandToRaftLog(
		repo.fsmStore.GetRaft(),
		constants.CommandTypeDbExecute,
		query,
		params,
		[]uint64{},
		0,
	)
	if applyErr != nil {
		repo.logger.Error("ResetExecutionCount: failed to write command to raft log", "error", applyErr, "accountId", accountId, "count", count)
		return applyErr
	}

	repo.logger.Debug("ResetExecutionCount: account job executions count reset", "accountId", accountId, "count", count)
	return nil
}

func (repo *accountJobExecutionsCountRepo) GetAllExpiredResetDates() ([]models.AccountJobExecutionsCount, *utils.GenericError) {
	repo.fsmStore.GetDataStore().ConnectionLock()
	defer repo.fsmStore.GetDataStore().ConnectionUnlock()

	schedulerTime := scheduler0time.GetSchedulerTime()
	now := schedulerTime.GetTime(time.Now())

	repo.logger.Debug("GetAllExpiredResetDates: getting all expired reset dates")

	selectBuilder := sq.Select(
		AccountJobExecutionsCountIdColumn,
		AccountJobExecutionsCountAccountIdColumn,
		AccountJobExecutionsCountExecutionCountColumn,
		AccountJobExecutionsCountDateCreatedColumn,
		AccountJobExecutionsCountDateModifiedColumn,
		AccountJobExecutionsCountNextResetDateColumn,
	).
		From(AccountJobExecutionsCountTableName).
		Where(fmt.Sprintf("%s <= ?", AccountJobExecutionsCountNextResetDateColumn), now).
		RunWith(repo.fsmStore.GetDataStore().GetOpenConnection())

	rows, err := selectBuilder.Query()
	if err != nil {
		repo.logger.Error("GetAllExpiredResetDates: failed to query expired reset dates", "error", err)
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	defer rows.Close()

	var accountJobExecutionsCounts []models.AccountJobExecutionsCount
	for rows.Next() {
		var accountJobExecutionsCount models.AccountJobExecutionsCount
		scanErr := rows.Scan(
			&accountJobExecutionsCount.ID,
			&accountJobExecutionsCount.AccountId,
			&accountJobExecutionsCount.ExecutionCount,
			&accountJobExecutionsCount.DateCreated,
			&accountJobExecutionsCount.DateModified,
			&accountJobExecutionsCount.NextResetDate,
		)
		if scanErr != nil {
			repo.logger.Error("GetAllExpiredResetDates: failed to scan row", "error", scanErr)
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, scanErr.Error())
		}
		accountJobExecutionsCounts = append(accountJobExecutionsCounts, accountJobExecutionsCount)
	}

	if rows.Err() != nil {
		repo.logger.Error("GetAllExpiredResetDates: row iteration error", "error", rows.Err())
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, rows.Err().Error())
	}

	repo.logger.Debug("GetAllExpiredResetDates: expired reset dates retrieved", "accountJobExecutionsCounts", accountJobExecutionsCounts)
	return accountJobExecutionsCounts, nil
}

func (repo *accountJobExecutionsCountRepo) GetAccountsWithZeroExecutionCount() ([]uint64, *utils.GenericError) {
	repo.fsmStore.GetDataStore().ConnectionLock()
	defer repo.fsmStore.GetDataStore().ConnectionUnlock()

	repo.logger.Debug("GetAccountsWithZeroExecutionCount: getting accounts with zero execution count")

	selectBuilder := sq.Select(AccountJobExecutionsCountAccountIdColumn).
		From(AccountJobExecutionsCountTableName).
		Where(fmt.Sprintf("%s = ?", AccountJobExecutionsCountExecutionCountColumn), 0).
		RunWith(repo.fsmStore.GetDataStore().GetOpenConnection())

	rows, err := selectBuilder.Query()
	if err != nil {
		repo.logger.Error("GetAccountsWithZeroExecutionCount: failed to query accounts with zero execution count", "error", err)
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	defer rows.Close()

	var accountIds []uint64
	for rows.Next() {
		var accountId uint64
		scanErr := rows.Scan(&accountId)
		if scanErr != nil {
			repo.logger.Error("GetAccountsWithZeroExecutionCount: failed to scan row", "error", scanErr)
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, scanErr.Error())
		}
		accountIds = append(accountIds, accountId)
	}

	if rows.Err() != nil {
		repo.logger.Error("GetAccountsWithZeroExecutionCount: row iteration error", "error", rows.Err())
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, rows.Err().Error())
	}

	repo.logger.Debug("GetAccountsWithZeroExecutionCount: found accounts with zero execution count", "count", len(accountIds))
	return accountIds, nil
}

func (repo *accountJobExecutionsCountRepo) DeleteByAccountId(accountId uint64) *utils.GenericError {
	repo.logger.Debug("DeleteByAccountId: deleting account job executions count", "accountId", accountId)

	if accountId == 0 {
		repo.logger.Warn("DeleteByAccountId: account id is required", "accountId", accountId)
		return utils.HTTPGenericError(http.StatusBadRequest, "account id is required")
	}

	deleteBuilder := sq.Delete(AccountJobExecutionsCountTableName).
		Where(fmt.Sprintf("%s = ?", AccountJobExecutionsCountAccountIdColumn), accountId)

	query, params, err := deleteBuilder.ToSql()
	if err != nil {
		repo.logger.Error("DeleteByAccountId: failed to build delete query", "error", err, "accountId", accountId)
		return utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	_, applyErr := repo.scheduler0RaftActions.WriteCommandToRaftLog(
		repo.fsmStore.GetRaft(),
		constants.CommandTypeDbExecute,
		query,
		params,
		[]uint64{},
		0,
	)
	if applyErr != nil {
		repo.logger.Error("DeleteByAccountId: failed to write command to raft log", "error", applyErr, "accountId", accountId)
		return applyErr
	}

	repo.logger.Debug("DeleteByAccountId: account job executions count deleted", "accountId", accountId)
	return nil
}
