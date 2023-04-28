package repository

import (
	"fmt"
	sq "github.com/Masterminds/squirrel"
	"github.com/hashicorp/go-hclog"
	"net/http"
	"scheduler0/constants"
	"scheduler0/fsm"
	"scheduler0/models"
	"scheduler0/utils"
	"time"
)

type Credential interface {
	CreateOne(credential models.CredentialModel) (uint64, *utils.GenericError)
	GetOneID(credential *models.CredentialModel) error
	GetByAPIKey(credential *models.CredentialModel) *utils.GenericError
	Count() (uint64, *utils.GenericError)
	List(offset uint64, limit uint64, orderBy string) ([]models.CredentialModel, *utils.GenericError)
	UpdateOneByID(credential models.CredentialModel) (uint64, *utils.GenericError)
	DeleteOneByID(credential models.CredentialModel) (uint64, *utils.GenericError)
}

// CredentialRepo Credential
type credentialRepo struct {
	fsmStore *fsm.Store
	logger   hclog.Logger
}

const (
	CredentialTableName = "credentials"
)

const (
	ArchivedColumn  = "archived"
	ApiKeyColumn    = "api_key"
	ApiSecretColumn = "api_secret"
)

func NewCredentialRepo(logger hclog.Logger, store *fsm.Store) Credential {
	return &credentialRepo{
		fsmStore: store,
		logger:   logger.Named("credential-repo"),
	}
}

// CreateOne creates a single credential and returns the uuid
func (credentialRepo *credentialRepo) CreateOne(credential models.CredentialModel) (uint64, *utils.GenericError) {
	schedulerTime := utils.GetSchedulerTime()
	now := schedulerTime.GetTime(time.Now())

	credential.DateCreated = now
	insertBuilder := sq.Insert(CredentialTableName).
		Columns(
			ArchivedColumn,
			ApiKeyColumn,
			ApiSecretColumn,
			JobsDateCreatedColumn,
		).
		Values(
			credential.Archived,
			credential.ApiKey,
			credential.ApiSecret,
			credential.DateCreated,
		)

	query, params, err := insertBuilder.ToSql()
	if err != nil {
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	res, applyErr := fsm.AppApply(credentialRepo.fsmStore.Raft, constants.CommandTypeDbExecute, query, 0, params)
	if err != nil {
		return 0, applyErr
	}

	if res == nil {
		return 0, utils.HTTPGenericError(http.StatusServiceUnavailable, "service is unavailable")
	}

	credential.ID = uint64(res.Data[0].(int64))

	return credential.ID, nil
}

// GetOneID returns a single credential
func (credentialRepo *credentialRepo) GetOneID(credential *models.CredentialModel) error {
	credentialRepo.fsmStore.DataStore.ConnectionLock.Lock()
	defer credentialRepo.fsmStore.DataStore.ConnectionLock.Unlock()

	sqlr := sq.Expr(fmt.Sprintf(
		"select %s, %s, %s, %s, %s from %s where %s = ?",
		JobsIdColumn,
		ArchivedColumn,
		ApiKeyColumn,
		ApiSecretColumn,
		JobsDateCreatedColumn,
		CredentialTableName,
		JobsIdColumn,
	), credential.ID)

	sqlString, args, err := sqlr.ToSql()
	if err != nil {
		return err
	}

	rows, err := credentialRepo.fsmStore.DataStore.Connection.Query(sqlString, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		err = rows.Scan(
			&credential.ID,
			&credential.Archived,
			&credential.ApiKey,
			&credential.ApiSecret,
			&credential.DateCreated,
		)
	}
	if rows.Err() != nil {
		return err
	}
	return nil
}

// GetByAPIKey returns a credential with the matching api key
func (credentialRepo *credentialRepo) GetByAPIKey(credential *models.CredentialModel) *utils.GenericError {
	credentialRepo.fsmStore.DataStore.ConnectionLock.Lock()
	defer credentialRepo.fsmStore.DataStore.ConnectionLock.Unlock()

	selectBuilder := sq.Select(
		JobsIdColumn,
		ArchivedColumn,
		ApiKeyColumn,
		ApiSecretColumn,
		JobsDateCreatedColumn,
	).
		From(CredentialTableName).
		Where(fmt.Sprintf("%s = ?", ApiKeyColumn), credential.ApiKey).
		RunWith(credentialRepo.fsmStore.DataStore.Connection)

	rows, err := selectBuilder.Query()
	if err != nil {
		return utils.HTTPGenericError(404, err.Error())
	}
	defer rows.Close()
	for rows.Next() {
		err = rows.Scan(
			&credential.ID,
			&credential.Archived,
			&credential.ApiKey,
			&credential.ApiSecret,
			&credential.DateCreated,
		)
		if err != nil {
			return utils.HTTPGenericError(500, err.Error())
		}
	}
	if rows.Err() != nil {
		return utils.HTTPGenericError(500, err.Error())
	}
	return nil
}

// Count returns total number of credential
func (credentialRepo *credentialRepo) Count() (uint64, *utils.GenericError) {
	credentialRepo.fsmStore.DataStore.ConnectionLock.Lock()
	defer credentialRepo.fsmStore.DataStore.ConnectionLock.Unlock()

	countQuery := sq.Select("count(*)").From(CredentialTableName).RunWith(credentialRepo.fsmStore.DataStore.Connection)
	rows, err := countQuery.Query()
	if err != nil {
		return 0, utils.HTTPGenericError(500, err.Error())
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		err = rows.Scan(
			&count,
		)
		if err != nil {
			return 0, utils.HTTPGenericError(500, err.Error())
		}
	}
	if err != nil {
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	return uint64(count), nil
}

// List returns a paginated set of credentials
func (credentialRepo *credentialRepo) List(offset uint64, limit uint64, orderBy string) ([]models.CredentialModel, *utils.GenericError) {
	credentialRepo.fsmStore.DataStore.ConnectionLock.Lock()
	defer credentialRepo.fsmStore.DataStore.ConnectionLock.Unlock()

	selectBuilder := sq.Select(
		JobsIdColumn,
		ArchivedColumn,
		ApiKeyColumn,
		ApiSecretColumn,
		JobsDateCreatedColumn,
	).
		From(CredentialTableName).
		Offset(offset).
		Limit(limit).
		OrderBy(orderBy).
		RunWith(credentialRepo.fsmStore.DataStore.Connection)

	rows, err := selectBuilder.Query()
	if err != nil {
		return nil, utils.HTTPGenericError(404, err.Error())
	}
	credentials := []models.CredentialModel{}
	defer rows.Close()
	for rows.Next() {
		credential := models.CredentialModel{}
		err = rows.Scan(
			&credential.ID,
			&credential.Archived,
			&credential.ApiKey,
			&credential.ApiSecret,
			&credential.DateCreated,
		)
		if err != nil {
			return nil, utils.HTTPGenericError(500, err.Error())
		}
		credentials = append(credentials, credential)
	}
	if rows.Err() != nil {
		return nil, utils.HTTPGenericError(500, err.Error())
	}
	return credentials, nil
}

// UpdateOneByID updates a single credential
func (credentialRepo *credentialRepo) UpdateOneByID(credential models.CredentialModel) (uint64, *utils.GenericError) {
	updateQuery := sq.Update(CredentialTableName).
		Set(ArchivedColumn, credential.Archived).
		Set(ApiKeyColumn, credential.ApiKey).
		Set(ApiSecretColumn, credential.ApiSecret).
		Where(fmt.Sprintf("%s = ?", JobsIdColumn), credential.ID)

	query, params, err := updateQuery.ToSql()
	if err != nil {
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	res, applyErr := fsm.AppApply(credentialRepo.fsmStore.Raft, constants.CommandTypeDbExecute, query, 0, params)
	if err != nil {
		return 0, applyErr
	}

	if res == nil {
		return 0, utils.HTTPGenericError(http.StatusServiceUnavailable, "service is unavailable")
	}

	count := res.Data[1].(uint64)
	return count, nil
}

// DeleteOneByID deletes a single credential
func (credentialRepo *credentialRepo) DeleteOneByID(credential models.CredentialModel) (uint64, *utils.GenericError) {
	deleteQuery := sq.Delete(CredentialTableName).Where(fmt.Sprintf("%s = ?", JobsIdColumn), credential.ID)

	query, params, err := deleteQuery.ToSql()
	if err != nil {
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	res, applyErr := fsm.AppApply(credentialRepo.fsmStore.Raft, constants.CommandTypeDbExecute, query, 0, params)
	if err != nil {
		return 0, applyErr
	}

	if res == nil {
		return 0, utils.HTTPGenericError(http.StatusServiceUnavailable, "service is unavailable")
	}

	count := res.Data[1].(uint64)

	return count, nil
}
