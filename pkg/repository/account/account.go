package account

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

type AccountRepository interface {
	GetAccounts(accountIds []uint64) ([]models.Account, *utils.GenericError)
	CreateAccount(account *models.Account) (uint64, *utils.GenericError)
	GetAccount(id uint64) (*models.Account, *utils.GenericError)
	UpdateAccount(id uint64, name string) *utils.GenericError
	GetFeatures(id uint64) (*[]models.AccountFeature, *utils.GenericError)
	GetFeaturesByAccountIds(accountIds []uint64) (map[uint64][]models.AccountFeature, *utils.GenericError)
	AddFeature(accountId uint64, featureId uint64) *utils.GenericError
	RemoveFeature(accountId uint64, featureId uint64) *utils.GenericError
	AddAllFeatures(accountId uint64) *utils.GenericError
	RemoveAllFeatures(accountId uint64) *utils.GenericError
	GetAllAccountIds() ([]uint64, *utils.GenericError)
}

type accountRepository struct {
	context               context.Context
	fsmStore              fsm.Scheduler0RaftStore
	logger                hclog.Logger
	scheduler0RaftActions fsm.Scheduler0RaftActions
}

func NewAccountRepository(context context.Context, logger hclog.Logger, scheduler0RaftActions fsm.Scheduler0RaftActions, fsmStore fsm.Scheduler0RaftStore) AccountRepository {
	return &accountRepository{
		context:               context,
		logger:                logger,
		scheduler0RaftActions: scheduler0RaftActions,
		fsmStore:              fsmStore,
	}
}

func (repo *accountRepository) CreateAccount(account *models.Account) (uint64, *utils.GenericError) {
	if account == nil {
		repo.logger.Warn("CreateAccount: account is nil")
		return 0, utils.HTTPGenericError(http.StatusBadRequest, "account is required")
	}

	accountName := strings.TrimSpace(account.Name)
	if accountName == "" {
		repo.logger.Warn("CreateAccount: account name is required", "accountName", account.Name)
		return 0, utils.HTTPGenericError(http.StatusBadRequest, "account name is required")
	}

	schedulerTime := scheduler0time.GetSchedulerTime()
	now := schedulerTime.GetTime(time.Now())

	repo.logger.Debug("CreateAccount: creating new account", "account", accountName)
	insertBuilder := sq.Insert(constants.AccountsTableName).
		Columns(
			constants.AccountsNameColumn,
			constants.AccountsDateCreatedColumn,
		).
		Values(
			accountName,
			now,
		)

	query, params, err := insertBuilder.ToSql()
	if err != nil {
		repo.logger.Error("CreateAccount: failed to build insert query", "error", err, "accountName", accountName)
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	res, applyErr := repo.scheduler0RaftActions.WriteCommandToRaftLog(repo.fsmStore.GetRaft(), constants.CommandTypeDbExecute, query, params, []uint64{}, 0)
	if applyErr != nil {
		repo.logger.Error("CreateAccount: failed to write command to raft log", "error", applyErr, "accountName", accountName)
		return 0, applyErr
	}

	if res == nil {
		repo.logger.Error("CreateAccount: raft log result is nil", "accountName", accountName)
		return 0, utils.HTTPGenericError(http.StatusServiceUnavailable, "service is unavailable - create account raft log result is nil")
	}

	account.ID = uint64(res.Data.LastInsertedId)

	return account.ID, nil
}

func (repo *accountRepository) GetAccount(id uint64) (*models.Account, *utils.GenericError) {
	repo.fsmStore.GetDataStore().ConnectionLock()
	defer repo.fsmStore.GetDataStore().ConnectionUnlock()

	repo.logger.Debug("GetAccount: getting account", "account", id)

	if id == 0 {
		repo.logger.Warn("GetAccount: account id is required", "accountId", id)
		return nil, utils.HTTPGenericError(http.StatusBadRequest, "account id is required")
	}

	selectBuilder := sq.Select(
		constants.AccountsIdColumn,
		constants.AccountsNameColumn,
		constants.AccountsDateCreatedColumn,
		constants.AccountsDateModifiedColumn,
	).
		From(constants.AccountsTableName).
		Where(fmt.Sprintf("%s = ?", constants.AccountsIdColumn), id).
		RunWith(repo.fsmStore.GetDataStore().GetOpenConnection())

	rows, err := selectBuilder.Query()
	if err != nil {
		repo.logger.Error("GetAccount: failed to query account", "error", err, "accountId", id)
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	defer rows.Close()
	var account models.Account
	for rows.Next() {
		scanErr := rows.Scan(
			&account.ID,
			&account.Name,
			&account.CreatedAt,
			&account.UpdatedAt,
		)
		if scanErr != nil {
			repo.logger.Error("GetAccount: failed to scan account row", "error", scanErr, "accountId", id)
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, scanErr.Error())
		}
	}
	if rows.Err() != nil {
		repo.logger.Error("GetAccount: row iteration error", "error", rows.Err(), "accountId", id)
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, rows.Err().Error())
	}
	if account.ID == 0 {
		repo.logger.Warn("GetAccount: account not found", "accountId", id)
		return nil, utils.HTTPGenericError(http.StatusNotFound, "account doesn't exist")
	}

	return &account, nil
}

func (repo *accountRepository) UpdateAccount(id uint64, name string) *utils.GenericError {
	if id == 0 {
		repo.logger.Warn("UpdateAccount: account id is required", "accountId", id)
		return utils.HTTPGenericError(http.StatusBadRequest, "account id is required")
	}

	name = strings.TrimSpace(name)
	if name == "" {
		repo.logger.Warn("UpdateAccount: account name is required", "accountId", id)
		return utils.HTTPGenericError(http.StatusBadRequest, "account name is required")
	}

	schedulerTime := scheduler0time.GetSchedulerTime()
	now := schedulerTime.GetTime(time.Now())

	repo.logger.Debug("UpdateAccount: updating account name", "accountId", id, "name", name)

	updateBuilder := sq.Update(constants.AccountsTableName).
		Set(constants.AccountsNameColumn, name).
		Set(constants.AccountsDateModifiedColumn, now).
		Where(fmt.Sprintf("%s = ?", constants.AccountsIdColumn), id)

	query, params, err := updateBuilder.ToSql()
	if err != nil {
		repo.logger.Error("UpdateAccount: failed to build update query", "error", err, "accountId", id)
		return utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	res, applyErr := repo.scheduler0RaftActions.WriteCommandToRaftLog(repo.fsmStore.GetRaft(), constants.CommandTypeDbExecute, query, params, []uint64{}, 0)
	if applyErr != nil {
		repo.logger.Error("UpdateAccount: failed to write command to raft log", "error", applyErr, "accountId", id)
		return applyErr
	}

	if res == nil {
		repo.logger.Error("UpdateAccount: raft log result is nil", "accountId", id)
		return utils.HTTPGenericError(http.StatusServiceUnavailable, "service is unavailable - update account raft log result is nil")
	}

	return nil
}

func (repo *accountRepository) GetFeatures(id uint64) (*[]models.AccountFeature, *utils.GenericError) {
	if id == 0 {
		repo.logger.Warn("GetFeatures: account id is required", "accountId", id)
		return nil, utils.HTTPGenericError(http.StatusBadRequest, "account id is required")
	}

	repo.fsmStore.GetDataStore().ConnectionLock()
	defer repo.fsmStore.GetDataStore().ConnectionUnlock()

	repo.logger.Debug("GetFeatures: getting features", "account", id)

	selectBuilder := sq.Select(
		constants.AccountFeaturesAccountIdColumn,
		constants.AccountFeaturesFeatureIdColumn,
		constants.FeaturesNameColumn,
	).
		From(constants.AccountFeaturesTableName).
		LeftJoin(fmt.Sprintf("%s ON %s = %s", constants.FeaturesTableName, constants.AccountFeaturesFeatureIdColumn, constants.FeaturesIdColumn)).
		Where(fmt.Sprintf("%s = ?", constants.AccountFeaturesAccountIdColumn), id).
		RunWith(repo.fsmStore.GetDataStore().GetOpenConnection())

	rows, err := selectBuilder.Query()
	if err != nil {
		repo.logger.Error("GetFeatures: failed to query features", "error", err, "accountId", id)
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	defer rows.Close()
	var features []models.AccountFeature
	for rows.Next() {
		var feature models.AccountFeature
		scanErr := rows.Scan(
			&feature.AccountId,
			&feature.FeatureId,
			&feature.Feature,
		)
		if scanErr != nil {
			repo.logger.Error("GetFeatures: failed to scan feature row", "error", scanErr, "accountId", id)
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, scanErr.Error())
		}
		features = append(features, feature)
	}
	if rows.Err() != nil {
		repo.logger.Error("GetFeatures: row iteration error", "error", rows.Err(), "accountId", id)
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, rows.Err().Error())
	}

	return &features, nil
}

func (repo *accountRepository) GetFeaturesByAccountIds(accountIds []uint64) (map[uint64][]models.AccountFeature, *utils.GenericError) {
	repo.fsmStore.GetDataStore().ConnectionLock()
	defer repo.fsmStore.GetDataStore().ConnectionUnlock()

	repo.logger.Debug("GetFeaturesByAccountIds: getting features by account ids", "accountIds", accountIds)

	if len(accountIds) == 0 {
		return make(map[uint64][]models.AccountFeature), nil
	}

	// Create placeholders for the IN clause
	placeholders := make([]string, len(accountIds))
	params := make([]interface{}, len(accountIds))
	for i, accountId := range accountIds {
		placeholders[i] = "?"
		params[i] = accountId
	}

	query := fmt.Sprintf(
		"SELECT %s, %s, %s FROM %s LEFT JOIN %s ON %s = %s WHERE %s IN (%s)",
		constants.AccountFeaturesAccountIdColumn,
		constants.AccountFeaturesFeatureIdColumn,
		constants.FeaturesNameColumn,
		constants.AccountFeaturesTableName,
		constants.FeaturesTableName,
		constants.AccountFeaturesFeatureIdColumn,
		constants.FeaturesIdColumn,
		constants.AccountFeaturesAccountIdColumn,
		strings.Join(placeholders, ","),
	)

	rows, err := repo.fsmStore.GetDataStore().GetOpenConnection().Query(query, params...)
	if err != nil {
		repo.logger.Error("GetFeaturesByAccountIds: failed to query features", "error", err, "accountIdsCount", len(accountIds))
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	defer rows.Close()

	result := make(map[uint64][]models.AccountFeature)
	for rows.Next() {
		var feature models.AccountFeature
		scanErr := rows.Scan(
			&feature.AccountId,
			&feature.FeatureId,
			&feature.Feature,
		)
		if scanErr != nil {
			repo.logger.Error("GetFeaturesByAccountIds: failed to scan feature row", "error", scanErr)
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, scanErr.Error())
		}
		result[feature.AccountId] = append(result[feature.AccountId], feature)
	}

	if rows.Err() != nil {
		repo.logger.Error("GetFeaturesByAccountIds: row iteration error", "error", rows.Err())
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, rows.Err().Error())
	}

	return result, nil
}

func (repo *accountRepository) AddFeature(accountId uint64, featureId uint64) *utils.GenericError {
	if accountId == 0 {
		repo.logger.Warn("AddFeature: account id is required", "accountId", accountId)
		return utils.HTTPGenericError(http.StatusBadRequest, "account id is required")
	}

	if featureId == 0 {
		repo.logger.Warn("AddFeature: feature id is required", "featureId", featureId)
		return utils.HTTPGenericError(http.StatusBadRequest, "feature id is required")
	}

	repo.logger.Debug("AddFeature: adding feature", "account", accountId, "feature", featureId)

	// Check if the feature already exists for this account
	selectBuilder := sq.Select("1").
		From(constants.AccountFeaturesTableName).
		Where(fmt.Sprintf("%s = ?", constants.AccountFeaturesAccountIdColumn), accountId).
		Where(fmt.Sprintf("%s = ?", constants.AccountFeaturesFeatureIdColumn), featureId).
		Limit(1).
		RunWith(repo.fsmStore.GetDataStore().GetOpenConnection())

	row := selectBuilder.QueryRow()
	var exists int
	err := row.Scan(&exists)
	if err == nil {
		// Row exists, so the feature is already added; idempotent success
		return nil
	} else if err.Error() != "sql: no rows in result set" {
		// Some other error occurred
		repo.logger.Error("AddFeature: failed to check if feature exists", "error", err, "accountId", accountId, "featureId", featureId)
		return utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	schedulerTime := scheduler0time.GetSchedulerTime()
	now := schedulerTime.GetTime(time.Now())

	insertBuilder := sq.Insert(constants.AccountFeaturesTableName).
		Columns(
			constants.AccountFeaturesAccountIdColumn,
			constants.AccountFeaturesFeatureIdColumn,
			constants.AccountFeaturesDateCreatedColumn,
		).
		Values(
			accountId,
			featureId,
			now,
		)

	query, params, err := insertBuilder.ToSql()
	if err != nil {
		repo.logger.Error("AddFeature: failed to build insert query", "error", err, "accountId", accountId, "featureId", featureId)
		return utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
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
		repo.logger.Error("AddFeature: failed to write command to raft log", "error", applyErr, "accountId", accountId, "featureId", featureId)
		return applyErr
	}

	if res == nil {
		repo.logger.Error("AddFeature: raft log result is nil", "accountId", accountId, "featureId", featureId)
		return utils.HTTPGenericError(http.StatusServiceUnavailable, "service is unavailable - add feature raft log result is nil")
	}

	return nil
}

func (repo *accountRepository) RemoveFeature(accountId uint64, featureId uint64) *utils.GenericError {
	if accountId == 0 {
		repo.logger.Warn("RemoveFeature: account id is required", "accountId", accountId)
		return utils.HTTPGenericError(http.StatusBadRequest, "account id is required")
	}

	if featureId == 0 {
		repo.logger.Warn("RemoveFeature: feature id is required", "featureId", featureId)
		return utils.HTTPGenericError(http.StatusBadRequest, "feature id is required")
	}

	repo.logger.Debug("RemoveFeature: removing feature", "account", accountId, "feature", featureId)

	deleteBuilder := sq.Delete(constants.AccountFeaturesTableName).
		Where(fmt.Sprintf("%s = ?", constants.AccountFeaturesAccountIdColumn), accountId).
		Where(fmt.Sprintf("%s = ?", constants.AccountFeaturesFeatureIdColumn), featureId).
		RunWith(repo.fsmStore.GetDataStore().GetOpenConnection())

	query, params, err := deleteBuilder.ToSql()
	if err != nil {
		repo.logger.Error("RemoveFeature: failed to build delete query", "error", err, "accountId", accountId, "featureId", featureId)
		return utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
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
		repo.logger.Error("RemoveFeature: failed to write command to raft log", "error", applyErr, "accountId", accountId, "featureId", featureId)
		return applyErr
	}

	if res == nil {
		repo.logger.Error("RemoveFeature: raft log result is nil", "accountId", accountId, "featureId", featureId)
		return utils.HTTPGenericError(http.StatusServiceUnavailable, "service is unavailable - remove feature raft log result is nil")
	}

	return nil
}

func (repo *accountRepository) AddAllFeatures(accountId uint64) *utils.GenericError {
	if accountId == 0 {
		repo.logger.Warn("AddAllFeatures: account id is required", "accountId", accountId)
		return utils.HTTPGenericError(http.StatusBadRequest, "account id is required")
	}

	repo.logger.Debug("AddAllFeatures: adding all features to account", "account", accountId)

	// First, get all available features
	selectBuilder := sq.Select(constants.FeaturesIdColumn).
		From(constants.FeaturesTableName).
		RunWith(repo.fsmStore.GetDataStore().GetOpenConnection())

	rows, err := selectBuilder.Query()
	if err != nil {
		repo.logger.Error("AddAllFeatures: failed to query available features", "error", err, "accountId", accountId)
		return utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	defer rows.Close()

	var featureIds []uint64
	for rows.Next() {
		var featureId uint64
		scanErr := rows.Scan(&featureId)
		if scanErr != nil {
			repo.logger.Error("AddAllFeatures: failed to scan feature id", "error", scanErr, "accountId", accountId)
			return utils.HTTPGenericError(http.StatusInternalServerError, scanErr.Error())
		}
		featureIds = append(featureIds, featureId)
	}
	if rows.Err() != nil {
		repo.logger.Error("AddAllFeatures: row iteration error when getting features", "error", rows.Err(), "accountId", accountId)
		return utils.HTTPGenericError(http.StatusInternalServerError, rows.Err().Error())
	}

	// Add each feature to the account
	schedulerTime := scheduler0time.GetSchedulerTime()
	now := schedulerTime.GetTime(time.Now())

	for _, featureId := range featureIds {
		// Check if the feature already exists for this account
		selectBuilder := sq.Select("1").
			From(constants.AccountFeaturesTableName).
			Where(fmt.Sprintf("%s = ?", constants.AccountFeaturesAccountIdColumn), accountId).
			Where(fmt.Sprintf("%s = ?", constants.AccountFeaturesFeatureIdColumn), featureId).
			Limit(1).
			RunWith(repo.fsmStore.GetDataStore().GetOpenConnection())

		row := selectBuilder.QueryRow()
		var exists int
		err := row.Scan(&exists)
		if err == nil {
			// Feature already exists, skip
			continue
		} else if err.Error() != "sql: no rows in result set" {
			// Some other error occurred
			repo.logger.Error("AddAllFeatures: failed to check if feature exists", "error", err, "accountId", accountId, "featureId", featureId)
			return utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
		}

		// Add the feature
		insertBuilder := sq.Insert(constants.AccountFeaturesTableName).
			Columns(
				constants.AccountFeaturesAccountIdColumn,
				constants.AccountFeaturesFeatureIdColumn,
				constants.AccountFeaturesDateCreatedColumn,
			).
			Values(
				accountId,
				featureId,
				now,
			)

		query, params, err := insertBuilder.ToSql()
		if err != nil {
			repo.logger.Error("AddAllFeatures: failed to build insert query", "error", err, "accountId", accountId, "featureId", featureId)
			return utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
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
			repo.logger.Error("AddAllFeatures: failed to write command to raft log", "error", applyErr, "accountId", accountId, "featureId", featureId)
			return applyErr
		}

		if res == nil {
			repo.logger.Error("AddAllFeatures: raft log result is nil", "accountId", accountId, "featureId", featureId)
			return utils.HTTPGenericError(http.StatusServiceUnavailable, "service is unavailable - add all features raft log result is nil")
		}
	}

	return nil
}

func (repo *accountRepository) RemoveAllFeatures(accountId uint64) *utils.GenericError {
	if accountId == 0 {
		repo.logger.Warn("RemoveAllFeatures: account id is required", "accountId", accountId)
		return utils.HTTPGenericError(http.StatusBadRequest, "account id is required")
	}

	repo.logger.Debug("RemoveAllFeatures: removing all features from account", "account", accountId)

	deleteBuilder := sq.Delete(constants.AccountFeaturesTableName).
		Where(fmt.Sprintf("%s = ?", constants.AccountFeaturesAccountIdColumn), accountId).
		RunWith(repo.fsmStore.GetDataStore().GetOpenConnection())

	query, params, err := deleteBuilder.ToSql()
	if err != nil {
		repo.logger.Error("RemoveAllFeatures: failed to build delete query", "error", err, "accountId", accountId)
		return utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
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
		repo.logger.Error("RemoveAllFeatures: failed to write command to raft log", "error", applyErr, "accountId", accountId)
		return applyErr
	}

	if res == nil {
		repo.logger.Error("RemoveAllFeatures: raft log result is nil", "accountId", accountId)
		return utils.HTTPGenericError(http.StatusServiceUnavailable, "service is unavailable - remove all features raft log result is nil")
	}

	return nil
}

func (repo *accountRepository) GetAccounts(accountIds []uint64) ([]models.Account, *utils.GenericError) {
	repo.logger.Debug("GetAccounts: getting accounts", "accountIds", accountIds)

	if len(accountIds) == 0 {
		return []models.Account{}, nil
	}

	// Build placeholders and params for IN clause
	params := make([]interface{}, 0, len(accountIds))
	var paramPlaceholders string

	if len(accountIds) > 0 {
		params = append(params, accountIds[0])
		paramPlaceholders = "?"
		for _, accountId := range accountIds[1:] {
			paramPlaceholders += ",?"
			params = append(params, accountId)
		}
	}

	selectBuilder := sq.Select(
		constants.AccountsIdColumn,
		constants.AccountsNameColumn,
		constants.AccountsDateCreatedColumn,
		constants.AccountsDateModifiedColumn,
	).
		From(constants.AccountsTableName).
		Where(fmt.Sprintf("%s IN (%s)", constants.AccountsIdColumn, paramPlaceholders), params...).
		RunWith(repo.fsmStore.GetDataStore().GetOpenConnection())

	rows, err := selectBuilder.Query()
	if err != nil {
		repo.logger.Error("GetAccounts: failed to query accounts", "error", err, "accountIds", accountIds)
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	defer rows.Close()
	var accounts []models.Account
	for rows.Next() {
		var account models.Account
		scanErr := rows.Scan(
			&account.ID,
			&account.Name,
			&account.CreatedAt,
			&account.UpdatedAt,
		)
		if scanErr != nil {
			repo.logger.Error("GetAccounts: failed to scan account row", "error", scanErr, "accountId", account.ID)
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, scanErr.Error())
		}
		accounts = append(accounts, account)
	}
	if rows.Err() != nil {
		repo.logger.Error("GetAccounts: row iteration error", "error", rows.Err(), "accountIds", accountIds)
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, rows.Err().Error())
	}

	repo.logger.Debug("GetAccounts: accounts retrieved", "accounts", accounts)

	return accounts, nil
}

func (repo *accountRepository) GetAllAccountIds() ([]uint64, *utils.GenericError) {
	repo.fsmStore.GetDataStore().ConnectionLock()
	defer repo.fsmStore.GetDataStore().ConnectionUnlock()

	var accountIds []uint64

	selectBuilder := sq.Select(constants.AccountsIdColumn).
		From(constants.AccountsTableName).
		RunWith(repo.fsmStore.GetDataStore().GetOpenConnection())

	rows, err := selectBuilder.Query()
	if err != nil {
		repo.logger.Error("GetAllAccountIds: failed to query all account IDs", "error", err)
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	defer rows.Close()

	for rows.Next() {
		var accountId uint64
		scanErr := rows.Scan(&accountId)
		if scanErr != nil {
			repo.logger.Error("GetAllAccountIds: failed to scan account ID", "error", scanErr)
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, scanErr.Error())
		}
		accountIds = append(accountIds, accountId)
	}

	if rows.Err() != nil {
		repo.logger.Error("GetAllAccountIds: row iteration error", "error", rows.Err())
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, rows.Err().Error())
	}

	repo.logger.Debug("GetAllAccountIds: retrieved account IDs", "accountCount", len(accountIds))

	return accountIds, nil
}
