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

//go:generate mockery --name CredentialRepo --output ../mocks
type CredentialRepo interface {
	CreateOne(credential models.Credential) (uint64, *utils.GenericError)
	GetOneID(credential *models.Credential) error
	GetByAPIKey(credential *models.Credential) *utils.GenericError
	Count() (uint64, *utils.GenericError)
	List(offset uint64, limit uint64, orderBy string) ([]models.Credential, *utils.GenericError)
	UpdateOneByID(credential models.Credential) (uint64, *utils.GenericError)
	DeleteOneByID(credential models.Credential) (uint64, *utils.GenericError)
}

// CredentialRepo CredentialRepo
type credentialRepo struct {
	fsmStore              fsm.Scheduler0RaftStore
	logger                hclog.Logger
	scheduler0RaftActions fsm.Scheduler0RaftActions
}

func NewCredentialRepo(logger hclog.Logger, scheduler0RaftActions fsm.Scheduler0RaftActions, store fsm.Scheduler0RaftStore) CredentialRepo {
	return &credentialRepo{
		fsmStore:              store,
		logger:                logger.Named("credential-repo"),
		scheduler0RaftActions: scheduler0RaftActions,
	}
}

// CreateOne creates a single credential and returns the uuid
func (credentialRepo *credentialRepo) CreateOne(credential models.Credential) (uint64, *utils.GenericError) {
	schedulerTime := utils.GetSchedulerTime()
	now := schedulerTime.GetTime(time.Now())

	credential.DateCreated = now
	insertBuilder := sq.Insert(constants.CredentialTableName).
		Columns(
			constants.CredentialsArchivedColumn,
			constants.CredentialsApiKeyColumn,
			constants.CredentialsApiSecretColumn,
			constants.CredentialsDateCreatedColumn,
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

	res, applyErr := credentialRepo.scheduler0RaftActions.WriteCommandToRaftLog(credentialRepo.fsmStore.GetRaft(), constants.CommandTypeDbExecute, query, 0, params)
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
func (credentialRepo *credentialRepo) GetOneID(credential *models.Credential) error {
	credentialRepo.fsmStore.GetDataStore().ConnectionLock()
	defer credentialRepo.fsmStore.GetDataStore().ConnectionUnlock()

	sqlr := sq.Expr(fmt.Sprintf(
		"select %s, %s, %s, %s, %s from %s where %s = ?",
		constants.CredentialsIdColumn,
		constants.CredentialsArchivedColumn,
		constants.CredentialsApiKeyColumn,
		constants.CredentialsApiSecretColumn,
		constants.CredentialsDateCreatedColumn,
		constants.CredentialTableName,
		constants.CredentialsIdColumn,
	), credential.ID)

	sqlString, args, err := sqlr.ToSql()
	if err != nil {
		return err
	}

	rows, err := credentialRepo.fsmStore.GetDataStore().GetOpenConnection().Query(sqlString, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	var count = 0
	for rows.Next() {
		scanErr := rows.Scan(
			&credential.ID,
			&credential.Archived,
			&credential.ApiKey,
			&credential.ApiSecret,
			&credential.DateCreated,
		)
		if scanErr != nil {
			return scanErr
		}
		count += 1
	}
	if rows.Err() != nil {
		return err
	}
	if count == 0 {
		return utils.HTTPGenericError(http.StatusNotFound, "credential not found")
	}
	return nil
}

// GetByAPIKey returns a credential with the matching api key
func (credentialRepo *credentialRepo) GetByAPIKey(credential *models.Credential) *utils.GenericError {
	credentialRepo.fsmStore.GetDataStore().ConnectionLock()
	defer credentialRepo.fsmStore.GetDataStore().ConnectionUnlock()

	selectBuilder := sq.Select(
		constants.CredentialsIdColumn,
		constants.CredentialsArchivedColumn,
		constants.CredentialsApiKeyColumn,
		constants.CredentialsApiSecretColumn,
		constants.CredentialsDateCreatedColumn,
	).
		From(constants.CredentialTableName).
		Where(fmt.Sprintf("%s = ?", constants.CredentialsApiKeyColumn), credential.ApiKey).
		RunWith(credentialRepo.fsmStore.GetDataStore().GetOpenConnection())

	rows, err := selectBuilder.Query()
	if err != nil {
		return utils.HTTPGenericError(404, err.Error())
	}
	defer rows.Close()
	var count = 0
	for rows.Next() {
		scanErr := rows.Scan(
			&credential.ID,
			&credential.Archived,
			&credential.ApiKey,
			&credential.ApiSecret,
			&credential.DateCreated,
		)
		if scanErr != nil {
			return utils.HTTPGenericError(500, err.Error())
		}
		count += 1
	}
	if rows.Err() != nil {
		return utils.HTTPGenericError(500, err.Error())
	}
	if count == 0 {
		return utils.HTTPGenericError(http.StatusNotFound, "credential doesn't exist")
	}
	return nil
}

// Count returns total number of credential
func (credentialRepo *credentialRepo) Count() (uint64, *utils.GenericError) {
	credentialRepo.fsmStore.GetDataStore().ConnectionLock()
	defer credentialRepo.fsmStore.GetDataStore().ConnectionUnlock()

	countQuery := sq.Select("count(*)").From(constants.CredentialTableName).
		RunWith(credentialRepo.fsmStore.GetDataStore().GetOpenConnection())
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
func (credentialRepo *credentialRepo) List(offset uint64, limit uint64, orderBy string) ([]models.Credential, *utils.GenericError) {
	credentialRepo.fsmStore.GetDataStore().ConnectionLock()
	defer credentialRepo.fsmStore.GetDataStore().ConnectionUnlock()

	selectBuilder := sq.Select(
		constants.CredentialsIdColumn,
		constants.CredentialsArchivedColumn,
		constants.CredentialsApiKeyColumn,
		constants.CredentialsApiSecretColumn,
		constants.CredentialsDateCreatedColumn,
	).
		From(constants.CredentialTableName).
		Offset(offset).
		Limit(limit).
		OrderBy(orderBy).
		RunWith(credentialRepo.fsmStore.GetDataStore().GetOpenConnection())

	rows, err := selectBuilder.Query()
	if err != nil {
		return nil, utils.HTTPGenericError(404, err.Error())
	}
	credentials := []models.Credential{}
	defer rows.Close()
	for rows.Next() {
		credential := models.Credential{}
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
func (credentialRepo *credentialRepo) UpdateOneByID(credential models.Credential) (uint64, *utils.GenericError) {
	updateQuery := sq.Update(constants.CredentialTableName).
		Set(constants.CredentialsArchivedColumn, credential.Archived).
		Set(constants.CredentialsApiKeyColumn, credential.ApiKey).
		Set(constants.CredentialsApiSecretColumn, credential.ApiSecret).
		Where(fmt.Sprintf("%s = ?", constants.CredentialsIdColumn), credential.ID)

	query, params, err := updateQuery.ToSql()
	if err != nil {
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	res, applyErr := credentialRepo.scheduler0RaftActions.WriteCommandToRaftLog(credentialRepo.fsmStore.GetRaft(), constants.CommandTypeDbExecute, query, 0, params)
	if err != nil {
		return 0, applyErr
	}

	if res == nil {
		return 0, utils.HTTPGenericError(http.StatusServiceUnavailable, "service is unavailable")
	}

	count := res.Data[1].(int64)
	return uint64(count), nil
}

// DeleteOneByID deletes a single credential
func (credentialRepo *credentialRepo) DeleteOneByID(credential models.Credential) (uint64, *utils.GenericError) {
	deleteQuery := sq.Delete(constants.CredentialTableName).Where(fmt.Sprintf("%s = ?", constants.CredentialsIdColumn), credential.ID)

	query, params, err := deleteQuery.ToSql()
	if err != nil {
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	res, applyErr := credentialRepo.scheduler0RaftActions.WriteCommandToRaftLog(credentialRepo.fsmStore.GetRaft(), constants.CommandTypeDbExecute, query, 0, params)
	if err != nil {
		return 0, applyErr
	}

	if res == nil {
		return 0, utils.HTTPGenericError(http.StatusServiceUnavailable, "service is unavailable")
	}

	count := res.Data[1].(int64)

	return uint64(count), nil
}
