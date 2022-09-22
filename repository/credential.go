package repository

import (
	"encoding/json"
	"errors"
	"fmt"
	sq "github.com/Masterminds/squirrel"
	"github.com/hashicorp/raft"
	"net/http"
	"scheduler0/constants"
	"scheduler0/fsm"
	"scheduler0/marsher"
	"scheduler0/models"
	"scheduler0/protobuffs"
	"scheduler0/utils"
	"strconv"
	"time"
)

type Credential interface {
	CreateOne(credential models.CredentialModel) (int64, *utils.GenericError)
	GetOneID(credential *models.CredentialModel) error
	GetByAPIKey(credential *models.CredentialModel) *utils.GenericError
	Count() (int, *utils.GenericError)
	List(offset int64, limit int64, orderBy string) ([]models.CredentialModel, *utils.GenericError)
	UpdateOneByID(credential models.CredentialModel) (int64, error)
	DeleteOneByID(credential models.CredentialModel) (int64, error)
}

// CredentialRepo Credential
type credentialRepo struct {
	store *fsm.Store
}

const (
	CredentialTableName = "credentials"
	AndroidPlatform     = "android"
	WebPlatform         = "web"
	IOSPlatform         = "ios"
	ServerPlatform      = "server"
)

const (
	ArchivedColumn                            = "archived"
	PlatformColumn                            = "platform"
	ApiKeyColumn                              = "api_key"
	ApiSecretColumn                           = "api_secret"
	IPRestrictionColumn                       = "ip_restriction"
	HTTPReferrerRestrictionColumn             = "http_referrer_restriction"
	IOSBundleIdReferrerRestrictionColumn      = "ios_bundle_id_restriction"
	AndroidPackageIDReferrerRestrictionColumn = "android_package_name_restriction"
)

func NewCredentialRepo(store *fsm.Store) Credential {
	return &credentialRepo{
		store: store,
	}
}

// CreateOne creates a single credential and returns the uuid
func (credentialRepo *credentialRepo) CreateOne(credential models.CredentialModel) (int64, *utils.GenericError) {
	credential.DateCreated = time.Now().UTC()
	insertBuilder := sq.Insert(CredentialTableName).
		Columns(
			PlatformColumn,
			ArchivedColumn,
			ApiKeyColumn,
			ApiSecretColumn,
			IPRestrictionColumn,
			HTTPReferrerRestrictionColumn,
			IOSBundleIdReferrerRestrictionColumn,
			AndroidPackageIDReferrerRestrictionColumn,
			JobsDateCreatedColumn,
		).
		Values(
			credential.Platform,
			credential.Archived,
			credential.ApiKey,
			credential.ApiSecret,
			credential.IPRestriction,
			credential.HTTPReferrerRestriction,
			credential.IOSBundleIDRestriction,
			credential.AndroidPackageNameRestriction,
			credential.DateCreated.String(),
		)

	sqlString, params, err := insertBuilder.ToSql()

	data, err := json.Marshal(params)
	if err != nil {
		return -1, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	createCommand := &protobuffs.Command{
		Type: protobuffs.Command_Type(constants.COMMAND_TYPE_DB_EXECUTE),
		Sql:  sqlString,
		Data: data,
	}

	if err != nil {
		return -1, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	createCommandData, err := marsher.MarshalCommand(createCommand)
	if err != nil {
		return -1, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	configs := utils.GetScheduler0Configurations()

	timeout, err := strconv.Atoi(configs.RaftApplyTimeout)
	if err != nil {
		return -1, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	af := credentialRepo.store.Raft.Apply(createCommandData, time.Second*time.Duration(timeout)).(raft.ApplyFuture)
	if af.Error() != nil {
		if af.Error() == raft.ErrNotLeader {
			return -1, utils.HTTPGenericError(http.StatusInternalServerError, "raft leader not found")
		}
		return -1, utils.HTTPGenericError(http.StatusInternalServerError, af.Error().Error())
	}

	r := af.Response().(fsm.Response)

	if r.Error != "" {
		return -1, utils.HTTPGenericError(http.StatusInternalServerError, r.Error)
	}

	insertedId := r.Data[0].(int64)
	credential.ID = insertedId

	return insertedId, nil
}

// GetOneID returns a single credential
func (credentialRepo *credentialRepo) GetOneID(credential *models.CredentialModel) error {
	selectBuilder := sq.Select(
		JobsIdColumn,
		ArchivedColumn,
		PlatformColumn,
		ApiKeyColumn,
		ApiSecretColumn,
		IPRestrictionColumn,
		HTTPReferrerRestrictionColumn,
		IOSBundleIdReferrerRestrictionColumn,
		AndroidPackageIDReferrerRestrictionColumn,
		JobsDateCreatedColumn,
	).
		From(CredentialTableName).
		Where(fmt.Sprintf("%s = ?", JobsIdColumn), credential.ID).
		RunWith(credentialRepo.store.SQLDbConnection)

	rows, err := selectBuilder.Query()
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		err = rows.Scan(
			&credential.ID,
			&credential.Archived,
			&credential.Platform,
			&credential.ApiKey,
			&credential.ApiSecret,
			&credential.IPRestriction,
			&credential.HTTPReferrerRestriction,
			&credential.IOSBundleIDRestriction,
			&credential.AndroidPackageNameRestriction,
			&credential.DateCreated,
		)
		if err != nil {
			return err
		}
	}
	if rows.Err() != nil {
		return err
	}
	return nil
}

// GetByAPIKey returns a credential with the matching api key
func (credentialRepo *credentialRepo) GetByAPIKey(credential *models.CredentialModel) *utils.GenericError {
	selectBuilder := sq.Select(
		JobsIdColumn,
		ArchivedColumn,
		PlatformColumn,
		ApiKeyColumn,
		ApiSecretColumn,
		IPRestrictionColumn,
		HTTPReferrerRestrictionColumn,
		IOSBundleIdReferrerRestrictionColumn,
		AndroidPackageIDReferrerRestrictionColumn,
		JobsDateCreatedColumn,
	).
		From(CredentialTableName).
		Where(fmt.Sprintf("%s = ?", ApiKeyColumn), credential.ApiKey).
		RunWith(credentialRepo.store.SQLDbConnection)

	rows, err := selectBuilder.Query()
	if err != nil {
		return utils.HTTPGenericError(404, err.Error())
	}
	defer rows.Close()
	for rows.Next() {
		err = rows.Scan(
			&credential.ID,
			&credential.Archived,
			&credential.Platform,
			&credential.ApiKey,
			&credential.ApiSecret,
			&credential.IPRestriction,
			&credential.HTTPReferrerRestriction,
			&credential.IOSBundleIDRestriction,
			&credential.AndroidPackageNameRestriction,
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
func (credentialRepo *credentialRepo) Count() (int, *utils.GenericError) {
	countQuery := sq.Select("count(*)").From(CredentialTableName).RunWith(credentialRepo.store.SQLDbConnection)
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
	selectBuilder := sq.Select(
		JobsIdColumn,
		ArchivedColumn,
		PlatformColumn,
		ApiKeyColumn,
		ApiSecretColumn,
		IPRestrictionColumn,
		HTTPReferrerRestrictionColumn,
		IOSBundleIdReferrerRestrictionColumn,
		AndroidPackageIDReferrerRestrictionColumn,
		JobsDateCreatedColumn,
	).
		From(CredentialTableName).
		Offset(uint64(offset)).
		Limit(uint64(limit)).
		OrderBy(orderBy).
		RunWith(credentialRepo.store.SQLDbConnection)

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
			&credential.Platform,
			&credential.ApiKey,
			&credential.ApiSecret,
			&credential.IPRestriction,
			&credential.HTTPReferrerRestriction,
			&credential.IOSBundleIDRestriction,
			&credential.AndroidPackageNameRestriction,
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
func (credentialRepo *credentialRepo) UpdateOneByID(credential models.CredentialModel) (int64, error) {
	updateQuery := sq.Update(CredentialTableName).
		Set(ArchivedColumn, credential.Archived).
		Set(PlatformColumn, credential.Platform).
		Set(ApiKeyColumn, credential.ApiKey).
		Set(ApiSecretColumn, credential.ApiSecret).
		Set(IPRestrictionColumn, credential.IPRestriction).
		Set(HTTPReferrerRestrictionColumn, credential.HTTPReferrerRestriction).
		Set(IOSBundleIdReferrerRestrictionColumn, credential.IOSBundleIDRestriction).
		Set(AndroidPackageIDReferrerRestrictionColumn, credential.AndroidPackageNameRestriction).
		Where(fmt.Sprintf("%s = ?", JobsIdColumn), credential.ID).
		RunWith(credentialRepo.store.SQLDbConnection)

	rows, err := updateQuery.Exec()
	if err != nil {
		return -1, err
	}

	count, err := rows.RowsAffected()
	if err != nil {
		return -1, err
	}

	return count, nil
}

// DeleteOneByID deletes a single credential
func (credentialRepo *credentialRepo) DeleteOneByID(credential models.CredentialModel) (int64, error) {
	countQuery := sq.Select(fmt.Sprintf("count(\"%s\")", JobsIdColumn)).From(CredentialTableName).RunWith(credentialRepo.store.SQLDbConnection)
	var count int64

	rows, err := countQuery.Query()
	if err != nil {
		return -1, err
	}
	defer rows.Close()
	for rows.Next() {
		scanErr := rows.Scan(&count)
		if scanErr != nil {
			return -1, scanErr
		}
	}
	if rows.Err() != nil {
		return -1, rows.Err()
	}

	if count == 1 {
		return -1, errors.New("cannot delete all the credentials")
	}

	deleteQuery := sq.Delete(CredentialTableName).Where(fmt.Sprintf("%s = ?", JobsIdColumn), credential.ID).RunWith(credentialRepo.store.SQLDbConnection)

	deletedRows, deleteErr := deleteQuery.Exec()
	if deleteErr != nil {
		return -1, err
	}

	deletedRowsCount, err := deletedRows.RowsAffected()
	if err != nil {
		return -1, err
	}

	return deletedRowsCount, nil
}
