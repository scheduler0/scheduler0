package repository

import (
	"encoding/json"
	"fmt"
	sq "github.com/Masterminds/squirrel"
	"github.com/araddon/dateparse"
	"github.com/hashicorp/raft"
	"log"
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
	UpdateOneByID(credential models.CredentialModel) (int64, *utils.GenericError)
	DeleteOneByID(credential models.CredentialModel) (int64, *utils.GenericError)
}

// CredentialRepo Credential
type credentialRepo struct {
	store  *fsm.Store
	logger *log.Logger
}

const (
	CredentialTableName = "credentials"
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

func NewCredentialRepo(logger *log.Logger, store *fsm.Store) Credential {
	return &credentialRepo{
		store:  store,
		logger: logger,
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

	query, params, err := insertBuilder.ToSql()
	if err != nil {
		return -1, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	res, applyErr := credentialRepo.applyToFSM(query, params)
	if err != nil {
		return -1, applyErr
	}

	credential.ID = res.Data[0].(int64)

	return credential.ID, nil
}

// GetOneID returns a single credential
func (credentialRepo *credentialRepo) GetOneID(credential *models.CredentialModel) error {
	sqlr := sq.Expr(fmt.Sprintf(
		"select %s, %s, %s, %s, %s, %s, %s, %s, %s, cast(\"%s\" as text) from %s where %s = ?",
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
		CredentialTableName,
		JobsIdColumn,
	), credential.ID)

	sqlString, args, err := sqlr.ToSql()
	if err != nil {
		return err
	}

	rows, err := credentialRepo.store.SQLDbConnection.Query(sqlString, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var dt string
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
			&dt,
		)

		t, errParse := dateparse.ParseLocal(dt)
		credential.DateCreated = t
		if errParse != nil {
			return utils.HTTPGenericError(500, err.Error())
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
		dataString := ""
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
			&dataString,
		)
		t, errParse := dateparse.ParseLocal(dataString)
		if errParse != nil {
			return utils.HTTPGenericError(500, err.Error())
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
		var dataString string
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
		t, errParse := dateparse.ParseLocal(dataString)
		if errParse != nil {
			return nil, utils.HTTPGenericError(500, err.Error())
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
		Set(PlatformColumn, credential.Platform).
		Set(ApiKeyColumn, credential.ApiKey).
		Set(ApiSecretColumn, credential.ApiSecret).
		Set(IPRestrictionColumn, credential.IPRestriction).
		Set(HTTPReferrerRestrictionColumn, credential.HTTPReferrerRestriction).
		Set(IOSBundleIdReferrerRestrictionColumn, credential.IOSBundleIDRestriction).
		Set(AndroidPackageIDReferrerRestrictionColumn, credential.AndroidPackageNameRestriction).
		Where(fmt.Sprintf("%s = ?", JobsIdColumn), credential.ID).
		RunWith(credentialRepo.store.SQLDbConnection)

	query, params, err := updateQuery.ToSql()
	if err != nil {
		return -1, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	res, applyErr := credentialRepo.applyToFSM(query, params)
	if err != nil {
		return -1, applyErr
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

	res, applyErr := credentialRepo.applyToFSM(query, params)
	if err != nil {
		return -1, applyErr
	}

	count := res.Data[1].(int64)

	return count, nil
}

func (credentialRepo *credentialRepo) applyToFSM(sqlString string, params []interface{}) (*fsm.Response, *utils.GenericError) {
	data, err := json.Marshal(params)
	if err != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	createCommand := &protobuffs.Command{
		Type: protobuffs.Command_Type(constants.CommandTypeDbExecute),
		Sql:  sqlString,
		Data: data,
	}

	if err != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	createCommandData, err := marsher.MarshalCommand(createCommand)
	if err != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	configs := utils.GetScheduler0Configurations(credentialRepo.logger)

	timeout, err := strconv.Atoi(configs.RaftApplyTimeout)
	if err != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	af := credentialRepo.store.Raft.Apply(createCommandData, time.Second*time.Duration(timeout)).(raft.ApplyFuture)
	if af.Error() != nil {
		if af.Error() == raft.ErrNotLeader {
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, "raft leader not found")
		}
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, af.Error().Error())
	}

	r := af.Response().(fsm.Response)

	if r.Error != "" {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, r.Error)
	}

	return &r, nil
}
