package credential

import (
	"database/sql"
	"fmt"
	"net/http"
	"scheduler0/pkg/constants"
	"scheduler0/pkg/fsm"
	"scheduler0/pkg/models"
	"scheduler0/pkg/scheduler0time"
	"scheduler0/pkg/utils"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/hashicorp/go-hclog"
)

type CredentialRepo interface {
	CreateOne(credential models.Credential) (uint64, *utils.GenericError)
	GetOneID(credential *models.Credential) error
	GetByAPIKey(credential *models.Credential) *utils.GenericError
	Count(accountId uint64) (uint64, *utils.GenericError)
	List(offset uint64, limit uint64, orderByColumn string, orderByDirection string, accountId uint64) ([]models.Credential, *utils.GenericError)
	ListExpired(now time.Time, limit uint64) ([]models.Credential, *utils.GenericError)
	UpdateOneByID(credential models.Credential) (uint64, *utils.GenericError)
	DeleteOneByID(credential models.Credential) (uint64, *utils.GenericError)
	ArchiveOneByID(credential models.Credential) (uint64, *utils.GenericError)
	ArchiveExpired(now time.Time) (uint64, *utils.GenericError)
	ReEncryptSecrets(oldKey, newKey string) (uint64, *utils.GenericError)
}

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

func scopesToStorage(scopes []string) string {
	cleaned := make([]string, 0, len(scopes))
	for _, s := range scopes {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		cleaned = append(cleaned, s)
	}
	return strings.Join(cleaned, ",")
}

func scopesFromStorage(stored string) []string {
	stored = strings.TrimSpace(stored)
	if stored == "" {
		return nil
	}
	parts := strings.Split(stored, ",")
	scopes := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		scopes = append(scopes, p)
	}
	return scopes
}

func scanCredential(rows *sql.Rows, credential *models.Credential) error {
	var scopesStr string
	if err := rows.Scan(
		&credential.ID,
		&credential.Archived,
		&credential.ApiKey,
		&credential.ApiSecret,
		&credential.DateCreated,
		&credential.AccountId,
		&credential.CreatedBy,
		&credential.DateModified,
		&credential.ModifiedBy,
		&credential.DeletedBy,
		&credential.ArchivedBy,
		&credential.ExpiresAt,
		&scopesStr,
	); err != nil {
		return err
	}
	credential.Scopes = scopesFromStorage(scopesStr)
	return nil
}

var credentialColumns = []string{
	constants.CredentialsIdColumn,
	constants.CredentialsArchivedColumn,
	constants.CredentialsApiKeyColumn,
	constants.CredentialsApiSecretColumn,
	constants.CredentialsDateCreatedColumn,
	constants.CredentialsAccountIdColumn,
	constants.CredentialsCreatedByColumn,
	constants.CredentialsDateModifiedColumn,
	constants.CredentialsModifiedByColumn,
	constants.CredentialsDeletedByColumn,
	constants.CredentialsArchivedByColumn,
	constants.CredentialsExpiresAtColumn,
	constants.CredentialsScopesColumn,
}

func (credentialRepo *credentialRepo) CreateOne(credential models.Credential) (uint64, *utils.GenericError) {
	if credential.AccountId == 0 {
		credentialRepo.logger.Warn("CreateOne: account id is required", "accountId", credential.AccountId)
		return 0, utils.HTTPGenericError(http.StatusBadRequest, "account id is required")
	}

	apiKey := strings.TrimSpace(credential.ApiKey)
	if apiKey == "" {
		credentialRepo.logger.Warn("CreateOne: api key is required", "accountId", credential.AccountId)
		return 0, utils.HTTPGenericError(http.StatusBadRequest, "api key is required")
	}

	apiSecret := strings.TrimSpace(credential.ApiSecret)
	if apiSecret == "" {
		credentialRepo.logger.Warn("CreateOne: api secret is required", "accountId", credential.AccountId)
		return 0, utils.HTTPGenericError(http.StatusBadRequest, "api secret is required")
	}

	if len(credential.Scopes) == 0 {
		credentialRepo.logger.Warn("CreateOne: at least one scope is required", "accountId", credential.AccountId)
		return 0, utils.HTTPGenericError(http.StatusBadRequest, "at least one scope is required")
	}

	scopesStored := scopesToStorage(credential.Scopes)
	if scopesStored == "" {
		return 0, utils.HTTPGenericError(http.StatusBadRequest, "at least one scope is required")
	}

	schedulerTime := scheduler0time.GetSchedulerTime()
	now := schedulerTime.GetTime(time.Now())

	if credential.CreatedBy == "" {
		credential.CreatedBy = constants.SystemActorName
	}

	if credential.DateCreated.IsZero() {
		credential.DateCreated = now
	}

	credentialRepo.logger.Debug("CreateOne: creating new credential", "credential.accountId", credential.AccountId)
	insertBuilder := sq.Insert(constants.CredentialTableName).
		Columns(
			constants.CredentialsArchivedColumn,
			constants.CredentialsApiKeyColumn,
			constants.CredentialsApiSecretColumn,
			constants.CredentialsDateCreatedColumn,
			constants.CredentialsAccountIdColumn,
			constants.CredentialsCreatedByColumn,
			constants.CredentialsExpiresAtColumn,
			constants.CredentialsScopesColumn,
		).
		Values(
			credential.Archived,
			apiKey,
			apiSecret,
			credential.DateCreated,
			credential.AccountId,
			credential.CreatedBy,
			credential.ExpiresAt,
			scopesStored,
		)

	query, params, err := insertBuilder.ToSql()
	if err != nil {
		credentialRepo.logger.Error("CreateOne: failed to build insert query", "error", err, "accountId", credential.AccountId)
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	res, applyErr := credentialRepo.scheduler0RaftActions.WriteCommandToRaftLog(credentialRepo.fsmStore.GetRaft(), constants.CommandTypeDbExecute, query, params, []uint64{}, 0)
	if applyErr != nil {
		credentialRepo.logger.Error("CreateOne: failed to write command to raft log", "error", applyErr, "accountId", credential.AccountId)
		return 0, applyErr
	}

	if res == nil {
		credentialRepo.logger.Error("CreateOne: raft log result is nil", "accountId", credential.AccountId)
		return 0, utils.HTTPGenericError(http.StatusServiceUnavailable, "service is unavailable - create one raft log result is nil")
	}

	credential.ID = uint64(res.Data.LastInsertedId)

	return credential.ID, nil
}

func (credentialRepo *credentialRepo) GetOneID(credential *models.Credential) error {
	credentialRepo.fsmStore.GetDataStore().ConnectionLock()
	defer credentialRepo.fsmStore.GetDataStore().ConnectionUnlock()

	if credential.AccountId == 0 {
		credentialRepo.logger.Warn("GetOneID: account id is required", "accountId", credential.AccountId, "credentialId", credential.ID)
		return utils.HTTPGenericError(http.StatusBadRequest, "account id is required")
	}

	selectBuilder := sq.Select(credentialColumns...).
		From(constants.CredentialTableName).
		Where(fmt.Sprintf("%s = ?", constants.CredentialsIdColumn), credential.ID).
		Where(fmt.Sprintf("%s = ?", constants.CredentialsAccountIdColumn), credential.AccountId).
		Where(fmt.Sprintf("(%s IS NULL OR %s = '')", constants.CredentialsDeletedByColumn, constants.CredentialsDeletedByColumn)).
		RunWith(credentialRepo.fsmStore.GetDataStore().GetOpenConnection())

	rows, err := selectBuilder.Query()
	if err != nil {
		credentialRepo.logger.Error("GetOneID: failed to query credential", "error", err, "accountId", credential.AccountId, "credentialId", credential.ID)
		return err
	}
	defer rows.Close()
	var count = 0
	for rows.Next() {
		if scanErr := scanCredential(rows, credential); scanErr != nil {
			credentialRepo.logger.Error("GetOneID: failed to scan credential row", "error", scanErr, "accountId", credential.AccountId, "credentialId", credential.ID)
			return scanErr
		}
		count += 1
	}
	if rows.Err() != nil {
		credentialRepo.logger.Error("GetOneID: row iteration error", "error", rows.Err(), "accountId", credential.AccountId, "credentialId", credential.ID)
		return rows.Err()
	}
	if count == 0 {
		credentialRepo.logger.Warn("GetOneID: credential not found", "accountId", credential.AccountId, "credentialId", credential.ID)
		return utils.HTTPGenericError(http.StatusNotFound, "credential not found")
	}
	return nil
}

func (credentialRepo *credentialRepo) GetByAPIKey(credential *models.Credential) *utils.GenericError {
	credentialRepo.fsmStore.GetDataStore().ConnectionLock()
	defer credentialRepo.fsmStore.GetDataStore().ConnectionUnlock()
	if credential.AccountId == 0 {
		credentialRepo.logger.Warn("GetByAPIKey: account id is required", "accountId", credential.AccountId)
		return utils.HTTPGenericError(http.StatusBadRequest, "account id is required")
	}

	selectBuilder := sq.Select(credentialColumns...).
		From(constants.CredentialTableName).
		Where(fmt.Sprintf("%s = ?", constants.CredentialsApiKeyColumn), credential.ApiKey).
		Where(fmt.Sprintf("%s = ?", constants.CredentialsAccountIdColumn), credential.AccountId).
		Where(fmt.Sprintf("(%s IS NULL OR %s = '')", constants.CredentialsDeletedByColumn, constants.CredentialsDeletedByColumn)).
		RunWith(credentialRepo.fsmStore.GetDataStore().GetOpenConnection())

	rows, err := selectBuilder.Query()
	if err != nil {
		credentialRepo.logger.Error("GetByAPIKey: failed to query credential", "error", err, "accountId", credential.AccountId)
		return utils.HTTPGenericError(http.StatusNotFound, err.Error())
	}
	defer rows.Close()
	var count = 0
	for rows.Next() {
		if scanErr := scanCredential(rows, credential); scanErr != nil {
			credentialRepo.logger.Error("GetByAPIKey: failed to scan credential row", "error", scanErr, "accountId", credential.AccountId)
			return utils.HTTPGenericError(http.StatusInternalServerError, scanErr.Error())
		}
		count += 1
	}
	if rows.Err() != nil {
		credentialRepo.logger.Error("GetByAPIKey: row iteration error", "error", rows.Err(), "accountId", credential.AccountId)
		return utils.HTTPGenericError(http.StatusInternalServerError, rows.Err().Error())
	}
	if count == 0 {
		credentialRepo.logger.Warn("GetByAPIKey: credential doesn't exist", "accountId", credential.AccountId)
		return utils.HTTPGenericError(http.StatusNotFound, "credential doesn't exist")
	}
	return nil
}

func (credentialRepo *credentialRepo) Count(accountId uint64) (uint64, *utils.GenericError) {
	credentialRepo.fsmStore.GetDataStore().ConnectionLock()
	defer credentialRepo.fsmStore.GetDataStore().ConnectionUnlock()

	countQuery := sq.Select("count(*)").From(constants.CredentialTableName).
		Where(fmt.Sprintf("%s = ?", constants.CredentialsAccountIdColumn), accountId).
		Where(fmt.Sprintf("(%s IS NULL OR %s = '')", constants.CredentialsDeletedByColumn, constants.CredentialsDeletedByColumn)).
		Where(fmt.Sprintf("(%s = ? OR %s IS NULL)", constants.CredentialsArchivedColumn, constants.CredentialsArchivedColumn), false).
		RunWith(credentialRepo.fsmStore.GetDataStore().GetOpenConnection())
	rows, err := countQuery.Query()
	if err != nil {
		credentialRepo.logger.Error("Count: failed to query credential count", "error", err, "accountId", accountId)
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		scanErr := rows.Scan(&count)
		if scanErr != nil {
			credentialRepo.logger.Error("Count: failed to scan count", "error", scanErr, "accountId", accountId)
			return 0, utils.HTTPGenericError(http.StatusInternalServerError, scanErr.Error())
		}
	}
	if rows.Err() != nil {
		credentialRepo.logger.Error("Count: row iteration error", "error", rows.Err(), "accountId", accountId)
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, rows.Err().Error())
	}

	return uint64(count), nil
}

func (credentialRepo *credentialRepo) List(offset uint64, limit uint64, orderByColumn string, orderByDirection string, accountId uint64) ([]models.Credential, *utils.GenericError) {
	credentialRepo.fsmStore.GetDataStore().ConnectionLock()
	defer credentialRepo.fsmStore.GetDataStore().ConnectionUnlock()

	if accountId == 0 {
		credentialRepo.logger.Warn("List: account id is required", "accountId", accountId)
		return nil, utils.HTTPGenericError(http.StatusBadRequest, "account id is required")
	}

	validColumns := map[string]bool{
		"id":            true,
		"date_created":  true,
		"date_modified": true,
		"created_by":    true,
		"modified_by":   true,
		"deleted_by":    true,
		"expires_at":    true,
	}

	if !validColumns[orderByColumn] {
		credentialRepo.logger.Warn("List: invalid order by column", "orderByColumn", orderByColumn, "accountId", accountId)
		return nil, utils.HTTPGenericError(http.StatusBadRequest, "invalid order by column")
	}

	if orderByDirection != "" {
		orderByDirection = strings.ToLower(orderByDirection)
		if orderByDirection != "asc" && orderByDirection != "desc" {
			credentialRepo.logger.Warn("List: invalid order by direction", "orderByDirection", orderByDirection, "accountId", accountId)
			return nil, utils.HTTPGenericError(http.StatusBadRequest, "invalid order by direction. Must be ASC or DESC")
		}
		orderByDirection = strings.ToUpper(orderByDirection)
	}

	selectBuilder := sq.Select(credentialColumns...).
		From(constants.CredentialTableName).
		Offset(offset).
		Limit(limit).
		OrderBy(fmt.Sprintf("%s %s", orderByColumn, orderByDirection)).
		Where(fmt.Sprintf("%s = ?", constants.CredentialsAccountIdColumn), accountId).
		Where(fmt.Sprintf("(%s IS NULL OR %s = '')", constants.CredentialsDeletedByColumn, constants.CredentialsDeletedByColumn)).
		Where(fmt.Sprintf("(%s = ? OR %s IS NULL)", constants.CredentialsArchivedColumn, constants.CredentialsArchivedColumn), false).
		RunWith(credentialRepo.fsmStore.GetDataStore().GetOpenConnection())

	rows, err := selectBuilder.Query()
	if err != nil {
		credentialRepo.logger.Error("List: failed to query credentials", "error", err, "accountId", accountId, "offset", offset, "limit", limit)
		return nil, utils.HTTPGenericError(http.StatusNotFound, err.Error())
	}
	credentials := []models.Credential{}
	defer rows.Close()
	for rows.Next() {
		credential := models.Credential{}
		if scanErr := scanCredential(rows, &credential); scanErr != nil {
			credentialRepo.logger.Error("List: failed to scan credential row", "error", scanErr, "accountId", accountId)
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, scanErr.Error())
		}
		credentials = append(credentials, credential)
	}
	if rows.Err() != nil {
		credentialRepo.logger.Error("List: row iteration error", "error", rows.Err(), "accountId", accountId)
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, rows.Err().Error())
	}
	return credentials, nil
}

func (credentialRepo *credentialRepo) ListExpired(now time.Time, limit uint64) ([]models.Credential, *utils.GenericError) {
	credentialRepo.fsmStore.GetDataStore().ConnectionLock()
	defer credentialRepo.fsmStore.GetDataStore().ConnectionUnlock()

	selectBuilder := sq.Select(credentialColumns...).
		From(constants.CredentialTableName).
		Where(fmt.Sprintf("%s IS NOT NULL", constants.CredentialsExpiresAtColumn)).
		Where(fmt.Sprintf("%s < ?", constants.CredentialsExpiresAtColumn), now).
		Where(fmt.Sprintf("(%s IS NULL OR %s = '')", constants.CredentialsDeletedByColumn, constants.CredentialsDeletedByColumn)).
		Where(fmt.Sprintf("(%s = ? OR %s IS NULL)", constants.CredentialsArchivedColumn, constants.CredentialsArchivedColumn), false).
		Limit(limit).
		RunWith(credentialRepo.fsmStore.GetDataStore().GetOpenConnection())

	rows, err := selectBuilder.Query()
	if err != nil {
		credentialRepo.logger.Error("ListExpired: failed to query expired credentials", "error", err)
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	credentials := []models.Credential{}
	defer rows.Close()
	for rows.Next() {
		credential := models.Credential{}
		if scanErr := scanCredential(rows, &credential); scanErr != nil {
			credentialRepo.logger.Error("ListExpired: failed to scan credential row", "error", scanErr)
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, scanErr.Error())
		}
		credentials = append(credentials, credential)
	}
	if rows.Err() != nil {
		credentialRepo.logger.Error("ListExpired: row iteration error", "error", rows.Err())
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, rows.Err().Error())
	}
	return credentials, nil
}

func (credentialRepo *credentialRepo) UpdateOneByID(credential models.Credential) (uint64, *utils.GenericError) {
	schedulerTime := scheduler0time.GetSchedulerTime()
	now := schedulerTime.GetTime(time.Now())

	if credential.DateModified == nil {
		credential.DateModified = &now
	}

	if credential.AccountId == 0 {
		credentialRepo.logger.Warn("UpdateOneByID: account id is required", "accountId", credential.AccountId, "credentialId", credential.ID)
		return 0, utils.HTTPGenericError(http.StatusBadRequest, "account id is required")
	}

	updateQuery := sq.Update(constants.CredentialTableName).
		Set(constants.CredentialsArchivedColumn, credential.Archived).
		Set(constants.CredentialsApiKeyColumn, credential.ApiKey).
		Set(constants.CredentialsApiSecretColumn, credential.ApiSecret).
		Set(constants.CredentialsDateModifiedColumn, now).
		Set(constants.CredentialsModifiedByColumn, credential.ModifiedBy).
		Where(fmt.Sprintf("%s = ?", constants.CredentialsIdColumn), credential.ID).
		Where(fmt.Sprintf("%s = ?", constants.CredentialsAccountIdColumn), credential.AccountId)

	query, params, err := updateQuery.ToSql()
	if err != nil {
		credentialRepo.logger.Error("UpdateOneByID: failed to build update query", "error", err, "accountId", credential.AccountId, "credentialId", credential.ID)
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	res, applyErr := credentialRepo.scheduler0RaftActions.WriteCommandToRaftLog(credentialRepo.fsmStore.GetRaft(), constants.CommandTypeDbExecute, query, params, []uint64{}, 0)
	if applyErr != nil {
		credentialRepo.logger.Error("UpdateOneByID: failed to write command to raft log", "error", applyErr, "accountId", credential.AccountId, "credentialId", credential.ID)
		return 0, applyErr
	}

	if res == nil {
		credentialRepo.logger.Error("UpdateOneByID: raft log result is nil", "accountId", credential.AccountId, "credentialId", credential.ID)
		return 0, utils.HTTPGenericError(http.StatusServiceUnavailable, "service is unavailable - update one by id raft log result is nil")
	}

	count := res.Data.RowsAffected
	return uint64(count), nil
}

func (credentialRepo *credentialRepo) DeleteOneByID(credential models.Credential) (uint64, *utils.GenericError) {
	schedulerTime := scheduler0time.GetSchedulerTime()
	now := schedulerTime.GetTime(time.Now())

	if credential.DeletedBy == nil {
		deletedBy := constants.SystemActorName
		credential.DeletedBy = &deletedBy
	}

	if credential.DateModified == nil {
		credential.DateModified = &now
	}

	if credential.AccountId == 0 {
		credentialRepo.logger.Warn("DeleteOneByID: account id is required", "accountId", credential.AccountId, "credentialId", credential.ID)
		return 0, utils.HTTPGenericError(http.StatusBadRequest, "account id is required")
	}

	deleteQuery := sq.Update(constants.CredentialTableName).
		Set(constants.CredentialsArchivedColumn, true).
		Set(constants.CredentialsDateModifiedColumn, now).
		Set(constants.CredentialsDeletedByColumn, credential.DeletedBy).
		Where(fmt.Sprintf("%s = ?", constants.CredentialsIdColumn), credential.ID).
		Where(fmt.Sprintf("%s = ?", constants.CredentialsAccountIdColumn), credential.AccountId)

	query, params, err := deleteQuery.ToSql()
	if err != nil {
		credentialRepo.logger.Error("DeleteOneByID: failed to build delete query", "error", err, "accountId", credential.AccountId, "credentialId", credential.ID)
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	res, applyErr := credentialRepo.scheduler0RaftActions.WriteCommandToRaftLog(credentialRepo.fsmStore.GetRaft(), constants.CommandTypeDbExecute, query, params, []uint64{}, 0)
	if applyErr != nil {
		credentialRepo.logger.Error("DeleteOneByID: failed to write command to raft log", "error", applyErr, "accountId", credential.AccountId, "credentialId", credential.ID)
		return 0, applyErr
	}

	if res == nil {
		credentialRepo.logger.Error("DeleteOneByID: raft log result is nil", "accountId", credential.AccountId, "credentialId", credential.ID)
		return 0, utils.HTTPGenericError(http.StatusServiceUnavailable, "service is unavailable")
	}

	count := res.Data.RowsAffected

	return uint64(count), nil
}

func (credentialRepo *credentialRepo) ArchiveOneByID(credential models.Credential) (uint64, *utils.GenericError) {
	schedulerTime := scheduler0time.GetSchedulerTime()
	now := schedulerTime.GetTime(time.Now())

	updateQuery := sq.Update(constants.CredentialTableName).
		Set(constants.CredentialsArchivedColumn, true).
		Set(constants.CredentialsArchivedByColumn, *credential.ArchivedBy).
		Set(constants.CredentialsDateModifiedColumn, now).
		Where(fmt.Sprintf("%s = ?", constants.CredentialsIdColumn), credential.ID).
		Where(fmt.Sprintf("%s = ?", constants.CredentialsAccountIdColumn), credential.AccountId)

	query, params, err := updateQuery.ToSql()
	if err != nil {
		credentialRepo.logger.Error("ArchiveOneByID: failed to build update query", "error", err, "accountId", credential.AccountId, "credentialId", credential.ID)
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	res, applyErr := credentialRepo.scheduler0RaftActions.WriteCommandToRaftLog(credentialRepo.fsmStore.GetRaft(), constants.CommandTypeDbExecute, query, params, []uint64{}, 0)
	if applyErr != nil {
		credentialRepo.logger.Error("ArchiveOneByID: failed to write command to raft log", "error", applyErr, "accountId", credential.AccountId, "credentialId", credential.ID)
		return 0, applyErr
	}

	if res == nil {
		credentialRepo.logger.Error("ArchiveOneByID: raft log result is nil", "accountId", credential.AccountId, "credentialId", credential.ID)
		return 0, utils.HTTPGenericError(http.StatusServiceUnavailable, "service is unavailable - archive one by id raft log result is nil")
	}

	count := res.Data.RowsAffected

	return uint64(count), nil
}

func (credentialRepo *credentialRepo) ReEncryptSecrets(oldKey, newKey string) (uint64, *utils.GenericError) {
	type row struct {
		id        uint64
		apiSecret string
	}

	credentialRepo.fsmStore.GetDataStore().ConnectionLock()
	selectBuilder := sq.Select(
		constants.CredentialsIdColumn,
		constants.CredentialsApiSecretColumn,
	).From(constants.CredentialTableName).
		RunWith(credentialRepo.fsmStore.GetDataStore().GetOpenConnection())

	rows, err := selectBuilder.Query()
	if err != nil {
		credentialRepo.fsmStore.GetDataStore().ConnectionUnlock()
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	pending := []row{}
	for rows.Next() {
		var r row
		if scanErr := rows.Scan(&r.id, &r.apiSecret); scanErr != nil {
			rows.Close()
			credentialRepo.fsmStore.GetDataStore().ConnectionUnlock()
			return 0, utils.HTTPGenericError(http.StatusInternalServerError, scanErr.Error())
		}
		pending = append(pending, r)
	}
	rowsErr := rows.Err()
	rows.Close()
	credentialRepo.fsmStore.GetDataStore().ConnectionUnlock()
	if rowsErr != nil {
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, rowsErr.Error())
	}

	var rotated uint64
	for _, r := range pending {
		newCipher, ok := utils.ReEncrypt(r.apiSecret, oldKey, newKey)
		if !ok {
			continue
		}

		query, params, buildErr := sq.Update(constants.CredentialTableName).
			Set(constants.CredentialsApiSecretColumn, newCipher).
			Where(fmt.Sprintf("%s = ?", constants.CredentialsIdColumn), r.id).
			ToSql()
		if buildErr != nil {
			return rotated, utils.HTTPGenericError(http.StatusInternalServerError, buildErr.Error())
		}

		res, applyErr := credentialRepo.scheduler0RaftActions.WriteCommandToRaftLog(credentialRepo.fsmStore.GetRaft(), constants.CommandTypeDbExecute, query, params, []uint64{}, 0)
		if applyErr != nil {
			return rotated, applyErr
		}
		if res == nil {
			return rotated, utils.HTTPGenericError(http.StatusServiceUnavailable, "service is unavailable - re-encrypt credential secrets raft log result is nil")
		}
		rotated++
	}

	return rotated, nil
}

func (credentialRepo *credentialRepo) ArchiveExpired(now time.Time) (uint64, *utils.GenericError) {
	archivedBy := "system:expiry-sweeper"

	updateQuery := sq.Update(constants.CredentialTableName).
		Set(constants.CredentialsArchivedColumn, true).
		Set(constants.CredentialsArchivedByColumn, archivedBy).
		Set(constants.CredentialsDateModifiedColumn, now).
		Where(fmt.Sprintf("%s IS NOT NULL", constants.CredentialsExpiresAtColumn)).
		Where(fmt.Sprintf("%s < ?", constants.CredentialsExpiresAtColumn), now).
		Where(fmt.Sprintf("(%s IS NULL OR %s = '')", constants.CredentialsDeletedByColumn, constants.CredentialsDeletedByColumn)).
		Where(fmt.Sprintf("(%s = ? OR %s IS NULL)", constants.CredentialsArchivedColumn, constants.CredentialsArchivedColumn), false)

	query, params, err := updateQuery.ToSql()
	if err != nil {
		credentialRepo.logger.Error("ArchiveExpired: failed to build update query", "error", err)
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	res, applyErr := credentialRepo.scheduler0RaftActions.WriteCommandToRaftLog(credentialRepo.fsmStore.GetRaft(), constants.CommandTypeDbExecute, query, params, []uint64{}, 0)
	if applyErr != nil {
		credentialRepo.logger.Error("ArchiveExpired: failed to write command to raft log", "error", applyErr)
		return 0, applyErr
	}
	if res == nil {
		credentialRepo.logger.Error("ArchiveExpired: raft log result is nil")
		return 0, utils.HTTPGenericError(http.StatusServiceUnavailable, "service is unavailable - archive expired raft log result is nil")
	}

	return uint64(res.Data.RowsAffected), nil
}
