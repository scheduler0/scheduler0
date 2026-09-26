package executor

import (
	"encoding/json"
	"fmt"
	"net/http"
	"scheduler0/pkg/constants"
	"scheduler0/pkg/fsm"
	"scheduler0/pkg/models"
	"scheduler0/pkg/scheduler0time"
	"scheduler0/pkg/secrets"
	"scheduler0/pkg/utils"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/hashicorp/go-hclog"
)

func encodeExecutorTags(tags []string) string {
	if len(tags) == 0 {
		return ""
	}
	encoded, err := json.Marshal(tags)
	if err != nil {
		return ""
	}
	return string(encoded)
}

func decodeExecutorTags(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var tags []string
	if err := json.Unmarshal([]byte(raw), &tags); err != nil {
		return nil
	}
	return tags
}

type JobExecutorRepo interface {
	CreateOne(executor models.JobExecutor) (uint64, *utils.GenericError)
	GetOneByID(id, accountId uint64) (*models.JobExecutor, *utils.GenericError)
	UpdateOneByID(executor models.JobExecutor) (uint64, *utils.GenericError)
	DeleteOneByID(executor models.JobExecutor) (uint64, *utils.GenericError)
	List(offset uint64, limit uint64, orderByColumn string, orderByDirection string, accountId uint64) ([]models.JobExecutor, *utils.GenericError)
	BatchGetByIds(executorIds []uint64) ([]models.JobExecutor, *utils.GenericError)
	Count(accountId uint64) (uint64, *utils.GenericError)
	ReEncryptSecrets(oldKey, newKey string) (uint64, *utils.GenericError)
}

type executorRepo struct {
	fsmStore              fsm.Scheduler0RaftStore
	logger                hclog.Logger
	scheduler0RaftActions fsm.Scheduler0RaftActions
	scheduler0Secret      secrets.Scheduler0Secrets
}

func NewExecutorRepo(logger hclog.Logger, scheduler0RaftActions fsm.Scheduler0RaftActions, fsmStore fsm.Scheduler0RaftStore, scheduler0Secret secrets.Scheduler0Secrets) JobExecutorRepo {
	return &executorRepo{
		fsmStore:              fsmStore,
		logger:                logger,
		scheduler0RaftActions: scheduler0RaftActions,
		scheduler0Secret:      scheduler0Secret,
	}
}

func (repo *executorRepo) encryptExecutorSecret(plaintext string) (string, *utils.GenericError) {
	if plaintext == "" {
		return "", nil
	}
	if repo.scheduler0Secret == nil {
		return "", utils.HTTPGenericError(http.StatusInternalServerError, "scheduler0 secrets not configured; cannot encrypt executor secrets")
	}
	creds := repo.scheduler0Secret.GetSecrets()
	if creds == nil || creds.SecretKey == "" {
		return "", utils.HTTPGenericError(http.StatusInternalServerError, "scheduler0 secret key is not set; cannot encrypt executor secrets")
	}
	return utils.Encrypt(plaintext, creds.SecretKey), nil
}

func (repo *executorRepo) decryptExecutorSecret(ciphertext string) (plaintext string) {
	if ciphertext == "" {
		return ""
	}
	if repo.scheduler0Secret == nil {
		repo.logger.Error("cannot decrypt executor secret: scheduler0 secrets not configured")
		return ""
	}
	creds := repo.scheduler0Secret.GetSecrets()
	if creds == nil || creds.SecretKey == "" {
		repo.logger.Error("cannot decrypt executor secret: scheduler0 secret key is not set")
		return ""
	}

	defer func() {
		if r := recover(); r != nil {
			repo.logger.Error("failed to decrypt executor secret; treating it as unset", "error", r)
			plaintext = ""
		}
	}()
	return utils.Decrypt(ciphertext, creds.SecretKey)
}

func (repo *executorRepo) storedExecutorSecrets(executorId uint64, accountId uint64) (cloudApiKey, cloudApiSecret, webhookSecret string, err *utils.GenericError) {
	repo.fsmStore.GetDataStore().ConnectionLock()
	defer repo.fsmStore.GetDataStore().ConnectionUnlock()

	rows, queryErr := sq.Select(
		constants.JobExecutorCloudApiKey,
		constants.JobExecutorCloudApiSecret,
		constants.JobExecutorWebhookSecretColumn,
	).From(constants.JobExecutorTableName).
		Where(sq.Eq{
			constants.JobExecutorIdColumn:        executorId,
			constants.JobExecutorAccountIdColumn: accountId,
		}).RunWith(repo.fsmStore.GetDataStore().GetOpenConnection()).Query()
	if queryErr != nil {
		return "", "", "", utils.HTTPGenericError(http.StatusInternalServerError, queryErr.Error())
	}
	defer rows.Close()

	if !rows.Next() {
		return "", "", "", nil
	}
	if scanErr := rows.Scan(&cloudApiKey, &cloudApiSecret, &webhookSecret); scanErr != nil {
		return "", "", "", utils.HTTPGenericError(http.StatusInternalServerError, scanErr.Error())
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return "", "", "", utils.HTTPGenericError(http.StatusInternalServerError, rowsErr.Error())
	}
	return cloudApiKey, cloudApiSecret, webhookSecret, nil
}

func (repo *executorRepo) CreateOne(executor models.JobExecutor) (uint64, *utils.GenericError) {
	if executor.CreatedBy == "" {
		executor.CreatedBy = constants.SystemActorName
	}

	schedulerTime := scheduler0time.GetSchedulerTime()
	now := schedulerTime.GetTime(time.Now())

	if executor.DateCreated.IsZero() {
		executor.DateCreated = now
	}

	if executor.AccountId == 0 {
		return 0, utils.HTTPGenericError(http.StatusBadRequest, "account id is required")
	}

	executorName := strings.TrimSpace(executor.Name)
	if executorName == "" {
		return 0, utils.HTTPGenericError(http.StatusBadRequest, "executor name is required")
	}

	if !strings.Contains(executor.Type, string(models.ExecutorTypeCloudFunction)) &&
		!strings.Contains(executor.Type, string(models.ExecutorTypeWebhookUrl)) &&
		!strings.Contains(executor.Type, string(models.ExecutorTypeLocal)) {
		return 0, utils.HTTPGenericError(http.StatusBadRequest, "invalid job executor type")
	}

	if models.ExecutorType(executor.Type) == models.ExecutorTypeLocal && strings.TrimSpace(executor.Command) == "" {
		return 0, utils.HTTPGenericError(http.StatusBadRequest, "command is required for local executors")
	}

	webhookUrl := strings.TrimSpace(executor.WebhookUrl)
	if models.ExecutorType(executor.Type) == models.ExecutorTypeWebhookUrl && webhookUrl == "" {
		return 0, utils.HTTPGenericError(http.StatusBadRequest, "webhook url is required")
	}

	if models.ExecutorType(executor.Type) == models.ExecutorTypeWebhookUrl && executor.WebhookMethod == "" {
		return 0, utils.HTTPGenericError(http.StatusBadRequest, "webhook method is required")
	}

	if models.ExecutorType(executor.Type) == models.ExecutorTypeCloudFunction {
		if executor.CloudProvider == "" {
			return 0, utils.HTTPGenericError(http.StatusBadRequest, "cloud provider is required for cloud function executors")
		}
		if executor.Region == "" {
			return 0, utils.HTTPGenericError(http.StatusBadRequest, "region is required for cloud function executors")
		}
		if executor.CloudResourceUrl == "" {
			return 0, utils.HTTPGenericError(http.StatusBadRequest, "cloud resource URL is required for cloud function executors")
		}
		if executor.CloudApiKey == "" {
			return 0, utils.HTTPGenericError(http.StatusBadRequest, "cloud API key is required for cloud function executors")
		}
		if executor.CloudApiSecret == "" {
			return 0, utils.HTTPGenericError(http.StatusBadRequest, "cloud API secret is required for cloud function executors")
		}
	}

	encryptedCloudApiKey, encErr := repo.encryptExecutorSecret(executor.CloudApiKey)
	if encErr != nil {
		return 0, encErr
	}
	encryptedCloudApiSecret, encErr := repo.encryptExecutorSecret(executor.CloudApiSecret)
	if encErr != nil {
		return 0, encErr
	}
	encryptedWebhookSecret, encErr := repo.encryptExecutorSecret(executor.WebhookSecret)
	if encErr != nil {
		return 0, encErr
	}

	insertBuilder := sq.Insert(constants.JobExecutorTableName).
		Columns(
			constants.JobExecutorAccountIdColumn,
			constants.JobExecutorNameColumn,
			constants.JobExecutorDescriptionColumn,
			constants.JobExecutorTagsColumn,
			constants.JobExecutorTypeColumn,
			constants.JobExecutorCloudProviderColumn,
			constants.JobExecutorRegionColumn,
			constants.JobExecutorCloudResourceUrlColumn,
			constants.JobExecutorCloudApiKey,
			constants.JobExecutorCloudApiSecret,
			constants.JobExecutorDateCreatedColumn,
			constants.JobExecutorWebhookUrlColumn,
			constants.JobExecutorWebhookSecretColumn,
			constants.JobExecutorWebhookMethodColumn,
			constants.JobExecutorCreatedByColumn,
			constants.JobExecutorCommandColumn,
			constants.JobExecutorWorkingDirColumn,
			constants.JobExecutorPayloadAggregationColumn,
		).Values(
		executor.AccountId,
		executorName,
		executor.Description,
		encodeExecutorTags(executor.Tags),
		executor.Type,
		executor.CloudProvider,
		executor.Region,
		executor.CloudResourceUrl,
		encryptedCloudApiKey,
		encryptedCloudApiSecret,
		executor.DateCreated,
		webhookUrl,
		encryptedWebhookSecret,
		executor.WebhookMethod,
		executor.CreatedBy,
		executor.Command,
		executor.WorkingDir,
		executor.PayloadAggregation,
	)
	query, params, err := insertBuilder.ToSql()
	if err != nil {
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	res, applyErr := repo.scheduler0RaftActions.WriteCommandToRaftLog(repo.fsmStore.GetRaft(), constants.CommandTypeDbExecute, query, params, []uint64{}, 0)
	if applyErr != nil {
		return 0, applyErr
	}

	if res == nil {
		return 0, utils.HTTPGenericError(http.StatusServiceUnavailable, "service is unavailable - create one raft log result is nil")
	}

	executor.ID = uint64(res.Data.LastInsertedId)

	return executor.ID, nil
}

func (repo *executorRepo) GetOneByID(executorId uint64, accountId uint64) (*models.JobExecutor, *utils.GenericError) {
	repo.fsmStore.GetDataStore().ConnectionLock()
	defer repo.fsmStore.GetDataStore().ConnectionUnlock()

	jobExecutor := &models.JobExecutor{}

	if accountId == 0 {
		return nil, utils.HTTPGenericError(http.StatusBadRequest, "account id is required")
	}

	selectBuilder := sq.Select(
		constants.JobExecutorIdColumn,
		constants.JobExecutorAccountIdColumn,
		constants.JobExecutorNameColumn,
		constants.JobExecutorDescriptionColumn,
		constants.JobExecutorTagsColumn,
		constants.JobExecutorTypeColumn,
		constants.JobExecutorCloudProviderColumn,
		constants.JobExecutorRegionColumn,
		constants.JobExecutorCloudResourceUrlColumn,
		constants.JobExecutorCloudApiKey,
		constants.JobExecutorCloudApiSecret,
		constants.JobExecutorWebhookUrlColumn,
		constants.JobExecutorWebhookSecretColumn,
		constants.JobExecutorWebhookMethodColumn,
		constants.JobExecutorDateCreatedColumn,
		constants.JobExecutorDateModifiedColumn,
		constants.JobExecutorCreatedByColumn,
		constants.JobExecutorModifiedByColumn,
		constants.JobExecutorDeletedByColumn,
		constants.JobExecutorCommandColumn,
		constants.JobExecutorWorkingDirColumn,
		constants.JobExecutorPayloadAggregationColumn,
	).From(constants.JobExecutorTableName).
		Where(sq.Eq{
			constants.JobExecutorIdColumn:        executorId,
			constants.JobExecutorAccountIdColumn: accountId,
		}).RunWith(repo.fsmStore.GetDataStore().GetOpenConnection())

	rows, err := selectBuilder.Query()
	if err != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	defer rows.Close()
	for rows.Next() {
		var tagsRaw string
		var payloadAggregation bool
		scanErr := rows.Scan(
			&jobExecutor.ID,
			&jobExecutor.AccountId,
			&jobExecutor.Name,
			&jobExecutor.Description,
			&tagsRaw,
			&jobExecutor.Type,
			&jobExecutor.CloudProvider,
			&jobExecutor.Region,
			&jobExecutor.CloudResourceUrl,
			&jobExecutor.CloudApiKey,
			&jobExecutor.CloudApiSecret,
			&jobExecutor.WebhookUrl,
			&jobExecutor.WebhookSecret,
			&jobExecutor.WebhookMethod,
			&jobExecutor.DateCreated,
			&jobExecutor.DateModified,
			&jobExecutor.CreatedBy,
			&jobExecutor.ModifiedBy,
			&jobExecutor.DeletedBy,
			&jobExecutor.Command,
			&jobExecutor.WorkingDir,
			&payloadAggregation,
		)
		if scanErr != nil {
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, scanErr.Error())
		}
		jobExecutor.Tags = decodeExecutorTags(tagsRaw)
		jobExecutor.PayloadAggregation = payloadAggregation
		jobExecutor.CloudApiKey = repo.decryptExecutorSecret(jobExecutor.CloudApiKey)
		jobExecutor.CloudApiSecret = repo.decryptExecutorSecret(jobExecutor.CloudApiSecret)
		jobExecutor.WebhookSecret = repo.decryptExecutorSecret(jobExecutor.WebhookSecret)
	}

	return jobExecutor, nil
}

func (repo *executorRepo) UpdateOneByID(executor models.JobExecutor) (uint64, *utils.GenericError) {
	if executor.ID == 0 {
		return 0, utils.HTTPGenericError(http.StatusBadRequest, "job executor id is required")
	}

	if executor.ModifiedBy == nil {
		systemActor := constants.SystemActorName
		executor.ModifiedBy = &systemActor
	}

	schedulerTime := scheduler0time.GetSchedulerTime()
	now := schedulerTime.GetTime(time.Now())

	if executor.DateModified == nil || executor.DateModified.IsZero() {
		executor.DateModified = &now
	}

	if executor.AccountId == 0 {
		return 0, utils.HTTPGenericError(http.StatusBadRequest, "account id is required")
	}

	if !strings.Contains(executor.Type, string(models.ExecutorTypeCloudFunction)) &&
		!strings.Contains(executor.Type, string(models.ExecutorTypeWebhookUrl)) &&
		!strings.Contains(executor.Type, string(models.ExecutorTypeLocal)) {
		return 0, utils.HTTPGenericError(http.StatusBadRequest, "invalid job executor type")
	}

	if models.ExecutorType(executor.Type) == models.ExecutorTypeLocal && strings.TrimSpace(executor.Command) == "" {
		return 0, utils.HTTPGenericError(http.StatusBadRequest, "command is required for local executors")
	}

	if models.ExecutorType(executor.Type) == models.ExecutorTypeCloudFunction {
		if executor.CloudProvider == "" {
			return 0, utils.HTTPGenericError(http.StatusBadRequest, "cloud provider is required for cloud function executors")
		}
		if executor.Region == "" {
			return 0, utils.HTTPGenericError(http.StatusBadRequest, "region is required for cloud function executors")
		}
		if executor.CloudResourceUrl == "" {
			return 0, utils.HTTPGenericError(http.StatusBadRequest, "cloud resource URL is required for cloud function executors")
		}
	}

	encryptedCloudApiKey, encErr := repo.encryptExecutorSecret(executor.CloudApiKey)
	if encErr != nil {
		return 0, encErr
	}
	encryptedCloudApiSecret, encErr := repo.encryptExecutorSecret(executor.CloudApiSecret)
	if encErr != nil {
		return 0, encErr
	}
	encryptedWebhookSecret, encErr := repo.encryptExecutorSecret(executor.WebhookSecret)
	if encErr != nil {
		return 0, encErr
	}

	if encryptedCloudApiKey == "" || encryptedCloudApiSecret == "" || encryptedWebhookSecret == "" {
		storedCloudApiKey, storedCloudApiSecret, storedWebhookSecret, storedErr := repo.storedExecutorSecrets(executor.ID, executor.AccountId)
		if storedErr != nil {
			return 0, storedErr
		}
		if encryptedCloudApiKey == "" {
			encryptedCloudApiKey = storedCloudApiKey
		}
		if encryptedCloudApiSecret == "" {
			encryptedCloudApiSecret = storedCloudApiSecret
		}
		if encryptedWebhookSecret == "" {
			encryptedWebhookSecret = storedWebhookSecret
		}
	}

	updateBuilder := sq.Update(constants.JobExecutorTableName).
		Set(constants.JobExecutorNameColumn, executor.Name).
		Set(constants.JobExecutorDescriptionColumn, executor.Description).
		Set(constants.JobExecutorTagsColumn, encodeExecutorTags(executor.Tags)).
		Set(constants.JobExecutorTypeColumn, executor.Type).
		Set(constants.JobExecutorCloudProviderColumn, executor.CloudProvider).
		Set(constants.JobExecutorCloudApiKey, encryptedCloudApiKey).
		Set(constants.JobExecutorCloudApiSecret, encryptedCloudApiSecret).
		Set(constants.JobExecutorRegionColumn, executor.Region).
		Set(constants.JobExecutorCloudResourceUrlColumn, executor.CloudResourceUrl).
		Set(constants.JobExecutorWebhookUrlColumn, executor.WebhookUrl).
		Set(constants.JobExecutorWebhookSecretColumn, encryptedWebhookSecret).
		Set(constants.JobExecutorWebhookMethodColumn, executor.WebhookMethod).
		Set(constants.JobExecutorCommandColumn, executor.Command).
		Set(constants.JobExecutorWorkingDirColumn, executor.WorkingDir).
		Set(constants.JobExecutorPayloadAggregationColumn, executor.PayloadAggregation).
		Set(constants.JobExecutorModifiedByColumn, executor.ModifiedBy).
		Set(constants.JobExecutorDateModifiedColumn, executor.DateModified).
		Where(fmt.Sprintf("%s = ?", constants.JobExecutorIdColumn), executor.ID).
		Where(fmt.Sprintf("%s = ?", constants.JobExecutorAccountIdColumn), executor.AccountId)

	query, params, err := updateBuilder.ToSql()
	if err != nil {
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	res, applyErr := repo.scheduler0RaftActions.WriteCommandToRaftLog(repo.fsmStore.GetRaft(), constants.CommandTypeDbExecute, query, params, []uint64{}, 0)
	if applyErr != nil {
		return 0, applyErr
	}

	if res == nil {
		return 0, utils.HTTPGenericError(http.StatusServiceUnavailable, "service is unavailable - update one by id raft log result is nil")
	}

	count := res.Data.RowsAffected

	return uint64(count), nil
}

func (repo *executorRepo) DeleteOneByID(executor models.JobExecutor) (uint64, *utils.GenericError) {
	schedulerTime := scheduler0time.GetSchedulerTime()
	now := schedulerTime.GetTime(time.Now())

	if executor.DeletedBy == nil {
		deletedBy := constants.SystemActorName
		executor.DeletedBy = &deletedBy
	}

	if executor.DateModified == nil {
		executor.DateModified = &now
	}

	if executor.AccountId == 0 {
		return 0, utils.HTTPGenericError(http.StatusBadRequest, "account id is required")
	}

	deleteQuery := sq.Update(constants.JobExecutorTableName).
		Set(constants.JobExecutorDateModifiedColumn, executor.DateModified).
		Set(constants.JobExecutorDeletedByColumn, executor.DeletedBy).
		Where(fmt.Sprintf("%s = ?", constants.JobExecutorIdColumn), executor.ID).
		Where(fmt.Sprintf("%s = ?", constants.JobExecutorAccountIdColumn), executor.AccountId)

	query, params, err := deleteQuery.ToSql()
	if err != nil {
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	res, applyErr := repo.scheduler0RaftActions.WriteCommandToRaftLog(repo.fsmStore.GetRaft(), constants.CommandTypeDbExecute, query, params, []uint64{}, 0)
	if applyErr != nil {
		return 0, applyErr
	}

	if res == nil {
		return 0, utils.HTTPGenericError(http.StatusServiceUnavailable, "service is unavailable - delete one by id raft log result is nil")
	}

	count := res.Data.RowsAffected

	return uint64(count), nil
}

func (repo *executorRepo) List(offset uint64, limit uint64, orderByColumn string, orderByDirection string, accountId uint64) ([]models.JobExecutor, *utils.GenericError) {
	repo.fsmStore.GetDataStore().ConnectionLock()
	defer repo.fsmStore.GetDataStore().ConnectionUnlock()

	if accountId == 0 {
		return nil, utils.HTTPGenericError(http.StatusBadRequest, "account id is required")
	}

	validColumns := map[string]bool{
		"id":            true,
		"date_created":  true,
		"date_modified": true,
		"created_by":    true,
		"modified_by":   true,
		"deleted_by":    true,
	}

	if !validColumns[orderByColumn] {
		return nil, utils.HTTPGenericError(http.StatusBadRequest, "invalid order by column")
	}

	if orderByDirection != "" {
		orderByDirection = strings.ToLower(orderByDirection)
		if orderByDirection != "asc" && orderByDirection != "desc" {
			return nil, utils.HTTPGenericError(http.StatusBadRequest, "invalid order by direction. Must be ASC or DESC")
		}
		orderByDirection = strings.ToUpper(orderByDirection)
	}

	selectBuilder := sq.Select(
		constants.JobExecutorIdColumn,
		constants.JobExecutorAccountIdColumn,
		constants.JobExecutorNameColumn,
		constants.JobExecutorDescriptionColumn,
		constants.JobExecutorTagsColumn,
		constants.JobExecutorTypeColumn,
		constants.JobExecutorCloudProviderColumn,
		constants.JobExecutorRegionColumn,
		constants.JobExecutorCloudResourceUrlColumn,
		constants.JobExecutorCloudApiKey,
		constants.JobExecutorCloudApiSecret,
		constants.JobExecutorWebhookUrlColumn,
		constants.JobExecutorWebhookSecretColumn,
		constants.JobExecutorWebhookMethodColumn,
		constants.JobExecutorDateCreatedColumn,
		constants.JobExecutorDateModifiedColumn,
		constants.JobExecutorCreatedByColumn,
		constants.JobExecutorModifiedByColumn,
		constants.JobExecutorDeletedByColumn,
		constants.JobExecutorCommandColumn,
		constants.JobExecutorWorkingDirColumn,
		constants.JobExecutorPayloadAggregationColumn,
	).
		From(constants.JobExecutorTableName).
		Offset(offset).
		Limit(limit).
		OrderBy(fmt.Sprintf("%s %s", orderByColumn, orderByDirection)).
		Where(fmt.Sprintf("%s = ?", constants.JobExecutorAccountIdColumn), accountId).
		Where(fmt.Sprintf("(%s IS NULL OR %s = '')", constants.JobExecutorDeletedByColumn, constants.JobExecutorDeletedByColumn)).
		RunWith(repo.fsmStore.GetDataStore().GetOpenConnection())

	rows, err := selectBuilder.Query()
	if err != nil {
		return nil, utils.HTTPGenericError(404, err.Error())
	}
	executors := []models.JobExecutor{}
	defer rows.Close()
	for rows.Next() {
		executor := models.JobExecutor{}
		var tagsRaw string
		var payloadAggregation bool
		err = rows.Scan(
			&executor.ID,
			&executor.AccountId,
			&executor.Name,
			&executor.Description,
			&tagsRaw,
			&executor.Type,
			&executor.CloudProvider,
			&executor.Region,
			&executor.CloudResourceUrl,
			&executor.CloudApiKey,
			&executor.CloudApiSecret,
			&executor.WebhookUrl,
			&executor.WebhookSecret,
			&executor.WebhookMethod,
			&executor.DateCreated,
			&executor.DateModified,
			&executor.CreatedBy,
			&executor.ModifiedBy,
			&executor.DeletedBy,
			&executor.Command,
			&executor.WorkingDir,
			&payloadAggregation,
		)
		if err != nil {
			return nil, utils.HTTPGenericError(500, err.Error())
		}

		executor.Tags = decodeExecutorTags(tagsRaw)
		executor.PayloadAggregation = payloadAggregation
		executor.CloudApiKey = repo.decryptExecutorSecret(executor.CloudApiKey)
		executor.CloudApiSecret = repo.decryptExecutorSecret(executor.CloudApiSecret)
		executor.WebhookSecret = repo.decryptExecutorSecret(executor.WebhookSecret)
		executors = append(executors, executor)
	}
	if rows.Err() != nil {
		return nil, utils.HTTPGenericError(500, err.Error())
	}
	return executors, nil
}

func (repo *executorRepo) BatchGetByIds(executorIds []uint64) ([]models.JobExecutor, *utils.GenericError) {
	repo.fsmStore.GetDataStore().ConnectionLock()
	defer repo.fsmStore.GetDataStore().ConnectionUnlock()

	if len(executorIds) == 0 {
		return []models.JobExecutor{}, nil
	}

	batches := utils.Batch[uint64](executorIds, 17)

	executors := []models.JobExecutor{}

	for _, batch := range batches {
		paramsPlaceholder := ""
		ids := []interface{}{}

		for i, id := range batch {
			paramsPlaceholder += "?"

			if i < len(batch)-1 {
				paramsPlaceholder += ","
			}

			ids = append(ids, id)
		}

		selectBuilder := sq.Select(
			constants.JobExecutorIdColumn,
			constants.JobExecutorAccountIdColumn,
			constants.JobExecutorNameColumn,
			constants.JobExecutorDescriptionColumn,
			constants.JobExecutorTagsColumn,
			constants.JobExecutorTypeColumn,
			constants.JobExecutorCloudProviderColumn,
			constants.JobExecutorRegionColumn,
			constants.JobExecutorCloudResourceUrlColumn,
			constants.JobExecutorCloudApiKey,
			constants.JobExecutorCloudApiSecret,
			constants.JobExecutorWebhookUrlColumn,
			constants.JobExecutorWebhookSecretColumn,
			constants.JobExecutorWebhookMethodColumn,
			constants.JobExecutorDateCreatedColumn,
			constants.JobExecutorDateModifiedColumn,
			constants.JobExecutorCreatedByColumn,
			constants.JobExecutorModifiedByColumn,
			constants.JobExecutorDeletedByColumn,
			constants.JobExecutorCommandColumn,
			constants.JobExecutorWorkingDirColumn,
			constants.JobExecutorPayloadAggregationColumn,
		).
			From(constants.JobExecutorTableName).
			Where(fmt.Sprintf("%s IN (%s)", constants.JobExecutorIdColumn, paramsPlaceholder), ids...).
			RunWith(repo.fsmStore.GetDataStore().GetOpenConnection())

		query, _, err := selectBuilder.ToSql()
		if err != nil {
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
		}

		rows, err := repo.fsmStore.GetDataStore().GetOpenConnection().Query(query, ids...)
		if err != nil {
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
		}
		for rows.Next() {
			executor := models.JobExecutor{}
			var tagsRaw string
			var payloadAggregation bool
			scanErr := rows.Scan(
				&executor.ID,
				&executor.AccountId,
				&executor.Name,
				&executor.Description,
				&tagsRaw,
				&executor.Type,
				&executor.CloudProvider,
				&executor.Region,
				&executor.CloudResourceUrl,
				&executor.CloudApiKey,
				&executor.CloudApiSecret,
				&executor.WebhookUrl,
				&executor.WebhookSecret,
				&executor.WebhookMethod,
				&executor.DateCreated,
				&executor.DateModified,
				&executor.CreatedBy,
				&executor.ModifiedBy,
				&executor.DeletedBy,
				&executor.Command,
				&executor.WorkingDir,
				&payloadAggregation,
			)
			if scanErr != nil {
				return nil, utils.HTTPGenericError(http.StatusInternalServerError, scanErr.Error())
			}
			executor.Tags = decodeExecutorTags(tagsRaw)
			executor.PayloadAggregation = payloadAggregation
			executor.CloudApiKey = repo.decryptExecutorSecret(executor.CloudApiKey)
			executor.CloudApiSecret = repo.decryptExecutorSecret(executor.CloudApiSecret)
			executor.WebhookSecret = repo.decryptExecutorSecret(executor.WebhookSecret)
			executors = append(executors, executor)
		}
		if rows.Err() != nil {
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, rows.Err().Error())
		}
		rows.Close()
	}

	return executors, nil
}

func (repo *executorRepo) ReEncryptSecrets(oldKey, newKey string) (uint64, *utils.GenericError) {
	type row struct {
		id             uint64
		cloudApiKey    string
		cloudApiSecret string
		webhookSecret  string
	}

	repo.fsmStore.GetDataStore().ConnectionLock()
	selectBuilder := sq.Select(
		constants.JobExecutorIdColumn,
		constants.JobExecutorCloudApiKey,
		constants.JobExecutorCloudApiSecret,
		constants.JobExecutorWebhookSecretColumn,
	).From(constants.JobExecutorTableName).
		RunWith(repo.fsmStore.GetDataStore().GetOpenConnection())

	rows, err := selectBuilder.Query()
	if err != nil {
		repo.fsmStore.GetDataStore().ConnectionUnlock()
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	pending := []row{}
	for rows.Next() {
		var r row
		if scanErr := rows.Scan(&r.id, &r.cloudApiKey, &r.cloudApiSecret, &r.webhookSecret); scanErr != nil {
			rows.Close()
			repo.fsmStore.GetDataStore().ConnectionUnlock()
			return 0, utils.HTTPGenericError(http.StatusInternalServerError, scanErr.Error())
		}
		pending = append(pending, r)
	}
	rowsErr := rows.Err()
	rows.Close()
	repo.fsmStore.GetDataStore().ConnectionUnlock()
	if rowsErr != nil {
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, rowsErr.Error())
	}

	var rotated uint64
	for _, r := range pending {
		updateBuilder := sq.Update(constants.JobExecutorTableName)
		changed := false

		if newCipher, ok := utils.ReEncrypt(r.cloudApiKey, oldKey, newKey); ok {
			updateBuilder = updateBuilder.Set(constants.JobExecutorCloudApiKey, newCipher)
			changed = true
		}
		if newCipher, ok := utils.ReEncrypt(r.cloudApiSecret, oldKey, newKey); ok {
			updateBuilder = updateBuilder.Set(constants.JobExecutorCloudApiSecret, newCipher)
			changed = true
		}
		if newCipher, ok := utils.ReEncrypt(r.webhookSecret, oldKey, newKey); ok {
			updateBuilder = updateBuilder.Set(constants.JobExecutorWebhookSecretColumn, newCipher)
			changed = true
		}
		if !changed {
			continue
		}

		query, params, buildErr := updateBuilder.
			Where(fmt.Sprintf("%s = ?", constants.JobExecutorIdColumn), r.id).
			ToSql()
		if buildErr != nil {
			return rotated, utils.HTTPGenericError(http.StatusInternalServerError, buildErr.Error())
		}

		res, applyErr := repo.scheduler0RaftActions.WriteCommandToRaftLog(repo.fsmStore.GetRaft(), constants.CommandTypeDbExecute, query, params, []uint64{}, 0)
		if applyErr != nil {
			return rotated, applyErr
		}
		if res == nil {
			return rotated, utils.HTTPGenericError(http.StatusServiceUnavailable, "service is unavailable - re-encrypt executor secrets raft log result is nil")
		}
		rotated++
	}

	return rotated, nil
}

func (repo *executorRepo) Count(accountId uint64) (uint64, *utils.GenericError) {
	if accountId == 0 {
		return 0, utils.HTTPGenericError(http.StatusBadRequest, "account id is required")
	}

	repo.fsmStore.GetDataStore().ConnectionLock()
	defer repo.fsmStore.GetDataStore().ConnectionUnlock()

	countQuery := sq.Select("count(*)").From(constants.JobExecutorTableName).
		Where(fmt.Sprintf("%s = ?", constants.JobExecutorAccountIdColumn), accountId).
		Where(fmt.Sprintf("(%s IS NULL OR %s = '')", constants.JobExecutorDeletedByColumn, constants.JobExecutorDeletedByColumn)).
		RunWith(repo.fsmStore.GetDataStore().GetOpenConnection())
	rows, err := countQuery.Query()

	if err != nil {
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		err = rows.Scan(
			&count,
		)
		if err != nil {
			return 0, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
		}
	}
	if rows.Err() != nil {
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, rows.Err().Error())
	}

	return uint64(count), nil
}
