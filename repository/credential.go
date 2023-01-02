package repository

import (
	"fmt"
	sq "github.com/Masterminds/squirrel"
	"github.com/araddon/dateparse"
	"log"
	"net/http"
	"scheduler0/constants"
	"scheduler0/fsm"
	"scheduler0/models"
	"scheduler0/utils"
	"time"
)

type Credential interface {
	CreateOne(credential models.CredentialModel) (int64, *utils.GenericError)
	GetOneID(credential *models.CredentialModel) error
	GetByAPIKey(credential *models.CredentialModel) *utils.GenericError
	Count() (int, *utils.GenericError)
	List(offset int64, limit int64, orderBy string) ([]models.CredentialModel, *utils.GenericError)
	UpdateOneByID(credential models.CredentialModel) (int64, *utils.GenericError)
	DeleteOneByID(credential models.CredentialModel) (int64, *utils.GenericError)
}

// CredentialRepo Credential
type credentialRepo struct {
	fsmStore *fsm.Store
	logger   *log.Logger
}

const (
	CredentialTableName = "credentials"
)

const (
	ArchivedColumn  = "archived"
	ApiKeyColumn    = "api_key"
	ApiSecretColumn = "api_secret"
)

func NewCredentialRepo(logger *log.Logger, store *fsm.Store) Credential {
	return &credentialRepo{
		fsmStore: store,
		logger:   logger,
	}
}

// CreateOne creates a single credential and returns the uuid
func (credentialRepo *credentialRepo) CreateOne(credential models.CredentialModel) (int64, *utils.GenericError) {
	credential.DateCreated = time.Now().UTC()
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
			credential.DateCreated.String(),
		)

	query, params, err := insertBuilder.ToSql()
	if err != nil {
		return -1, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	res, applyErr := fsm.AppApply(credentialRepo.logger, credentialRepo.fsmStore.Raft, constants.CommandTypeDbExecute, query, params)
	if err != nil {
		return -1, applyErr
	}

	if res == nil {
		return -1, utils.HTTPGenericError(http.StatusServiceUnavailable, "service is unavailable")
	}

	credential.ID = res.Data[0].(int64)

	return credential.ID, nil
}

// GetOneID returns a single credential
func (credentialRepo *credentialRepo) GetOneID(credential *models.CredentialModel) error {
	credentialRepo.fsmStore.DataStore.ConnectionLock.Lock()
	defer credentialRepo.fsmStore.DataStore.ConnectionLock.Unlock()

	sqlr := sq.Expr(fmt.Sprintf(
		"select %s, %s, %s, %s, cast(\"%s\" as text) from %s where %s = ?",
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
		var dt string
		err = rows.Scan(
			&credential.ID,
			&credential.Archived,
			&credential.ApiKey,
			&credential.ApiSecret,
			&dt,
		)

		t, errParse := dateparse.ParseLocal(dt)
		credential.DateCreated = t
		if errParse != nil {
			return utils.HTTPGenericError(500, errParse.Error())
		}
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
		fmt.Sprintf("cast(\"%s\" as text)", JobsDateCreatedColumn),
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
		dataString := ""
		err = rows.Scan(
			&credential.ID,
			&credential.Archived,
			&credential.ApiKey,
			&credential.ApiSecret,
			&dataString,
		)
		t, errParse := dateparse.ParseLocal(dataString)
		if errParse != nil {
			return utils.HTTPGenericError(500, errParse.Error())
		}
		credential.DateCreated = t
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
func (credentialRepo *credentialRepo) Count() (int, *utils.GenericError) {
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
		return -1, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	return count, nil
}

// List returns a paginated set of credentials
func (credentialRepo *credentialRepo) List(offset int64, limit int64, orderBy string) ([]models.CredentialModel, *utils.GenericError) {
	credentialRepo.fsmStore.DataStore.ConnectionLock.Lock()
	defer credentialRepo.fsmStore.DataStore.ConnectionLock.Unlock()

	selectBuilder := sq.Select(
		JobsIdColumn,
		ArchivedColumn,
		ApiKeyColumn,
		ApiSecretColumn,
		fmt.Sprintf("cast(\"%s\" as text)", JobsDateCreatedColumn),
	).
		From(CredentialTableName).
		Offset(uint64(offset)).
		Limit(uint64(limit)).
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
		var dataString string
		err = rows.Scan(
			&credential.ID,
			&credential.Archived,
			&credential.ApiKey,
			&credential.ApiSecret,
			&dataString,
		)
		t, errParse := dateparse.ParseLocal(dataString)
		if errParse != nil {
			return nil, utils.HTTPGenericError(500, errParse.Error())
		}
		credential.DateCreated = t
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
func (credentialRepo *credentialRepo) UpdateOneByID(credential models.CredentialModel) (int64, *utils.GenericError) {
	updateQuery := sq.Update(CredentialTableName).
		Set(ArchivedColumn, credential.Archived).
		Set(ApiKeyColumn, credential.ApiKey).
		Set(ApiSecretColumn, credential.ApiSecret).
		Where(fmt.Sprintf("%s = ?", JobsIdColumn), credential.ID)

	query, params, err := updateQuery.ToSql()
	if err != nil {
		return -1, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	res, applyErr := fsm.AppApply(credentialRepo.logger, credentialRepo.fsmStore.Raft, constants.CommandTypeDbExecute, query, params)
	if err != nil {
		return -1, applyErr
	}

	if res == nil {
		return -1, utils.HTTPGenericError(http.StatusServiceUnavailable, "service is unavailable")
	}

	count := res.Data[1].(int64)
	return count, nil
}

// DeleteOneByID deletes a single credential
func (credentialRepo *credentialRepo) DeleteOneByID(credential models.CredentialModel) (int64, *utils.GenericError) {
	deleteQuery := sq.Delete(CredentialTableName).Where(fmt.Sprintf("%s = ?", JobsIdColumn), credential.ID)

	query, params, err := deleteQuery.ToSql()
	if err != nil {
		return -1, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	res, applyErr := fsm.AppApply(credentialRepo.logger, credentialRepo.fsmStore.Raft, constants.CommandTypeDbExecute, query, params)
	if err != nil {
		return -1, applyErr
	}

	if res == nil {
		return -1, utils.HTTPGenericError(http.StatusServiceUnavailable, "service is unavailable")
	}

	count := res.Data[1].(int64)

	return count, nil
}
