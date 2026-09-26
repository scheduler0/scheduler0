package job

import (
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

// JobRepo job table manager
type jobRepo struct {
	fsmStore              fsm.Scheduler0RaftStore
	logger                hclog.Logger
	scheduler0RaftActions fsm.Scheduler0RaftActions
}

type JobRepo interface {
	GetOneByID(jobModel *models.Job) *utils.GenericError
	BatchGetJobsByID(jobIDs []uint64) ([]models.Job, *utils.GenericError)
	BatchGetJobsWithIDRange(lowerBound, upperBound int64) ([]models.Job, *utils.GenericError)
	GetJobsPaginated(accountID uint64, projectID uint64, offset uint64, limit uint64, orderByColumn string, orderByDirection string) ([]models.Job, uint64, *utils.GenericError)
	GetJobsTotalCountByProjectID(projectID uint64) (uint64, *utils.GenericError)
	GetJobsTotalCountByAccountID(accountID uint64, projectID *uint64) (uint64, *utils.GenericError)
	GetJobsTotalCount() (uint64, *utils.GenericError)
	DeleteOneByID(jobModel models.Job) (uint64, *utils.GenericError)
	UpdateOneByID(jobModel models.Job) (uint64, *utils.GenericError)
	GetAllByProjectID(projectID uint64, offset uint64, limit uint64, orderByColumn string, orderByDirection string) ([]models.Job, *utils.GenericError)
	GetAllByAccountID(accountID uint64) ([]models.Job, *utils.GenericError)
	GetActiveJobsByExecutorID(executorID uint64, accountID uint64) ([]models.Job, *utils.GenericError)
	BatchInsertJobs(jobRepos []models.Job) ([]uint64, *utils.GenericError)
	UpdateJobsStatusByAccountId(accountId uint64, status string) *utils.GenericError
	DeleteJobsByProjectID(projectID uint64, accountId uint64, deletedBy string) (uint64, *utils.GenericError)
}

func NewJobRepo(logger hclog.Logger, scheduler0RaftActions fsm.Scheduler0RaftActions, store fsm.Scheduler0RaftStore) JobRepo {
	return &jobRepo{
		fsmStore:              store,
		scheduler0RaftActions: scheduler0RaftActions,
		logger:                logger.Named("job-repo"),
	}
}

// GetOneByID returns a single job that matches uuid
func (jobRepo *jobRepo) GetOneByID(jobModel *models.Job) *utils.GenericError {
	startTime := time.Now()
	jobRepo.logger.Debug("getting job by ID", "jobId", jobModel.ID)

	jobRepo.fsmStore.GetDataStore().ConnectionLock()
	defer jobRepo.fsmStore.GetDataStore().ConnectionUnlock()

	selectBuilder := sq.Select(
		constants.JobsIdColumn,
		constants.JobsProjectIdColumn,
		constants.JobsSpecColumn,
		constants.JobsDateCreatedColumn,
		constants.JobsTimezoneColumn,
		constants.JobsTimezoneOffsetColumn,
		constants.JobsDataColumn,
		constants.JobsAccountIdColumn,
		constants.JobsCreatedByColumn,
		constants.JobsDateModifiedColumn,
		constants.JobsModifiedByColumn,
		constants.JobsDeletedByColumn,
		constants.JobsExecutorIdColumn,
		constants.JobsStartDateColumn,
		constants.JobsEndDateColumn,
		constants.JobsRetryMaxColumn,
		constants.JobsStatusColumn,
	).
		From(constants.JobsTableName).
		Where(fmt.Sprintf("%s = ?", constants.JobsIdColumn), jobModel.ID)
	// When AccountId is supplied, scope the lookup to that tenant so callers cannot
	// read another account's job by guessing/enumerating IDs.
	if jobModel.AccountId != 0 {
		selectBuilder = selectBuilder.Where(fmt.Sprintf("%s = ?", constants.JobsAccountIdColumn), jobModel.AccountId)
	}
	selectBuilder = selectBuilder.RunWith(jobRepo.fsmStore.GetDataStore().GetOpenConnection())

	rows, err := selectBuilder.Query()
	if err != nil {
		jobRepo.logger.Error("GetOneByID: failed to query job", "error", err, "jobId", jobModel.ID)
		return utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		scanErr := rows.Scan(
			&jobModel.ID,
			&jobModel.ProjectID,
			&jobModel.Spec,
			&jobModel.DateCreated,
			&jobModel.Timezone,
			&jobModel.TimezoneOffset,
			&jobModel.Data,
			&jobModel.AccountId,
			&jobModel.CreatedBy,
			&jobModel.DateModified,
			&jobModel.ModifiedBy,
			&jobModel.DeletedBy,
			&jobModel.ExecutorId,
			&jobModel.StartDate,
			&jobModel.EndDate,
			&jobModel.RetryMax,
			&jobModel.Status,
		)
		if scanErr != nil {
			jobRepo.logger.Error("GetOneByID: failed to scan job row", "error", scanErr, "jobId", jobModel.ID)
			return utils.HTTPGenericError(http.StatusInternalServerError, scanErr.Error())
		}
		count += 1
	}
	if rows.Err() != nil {
		jobRepo.logger.Error("GetOneByID: row iteration error", "error", rows.Err(), "jobId", jobModel.ID)
		return utils.HTTPGenericError(http.StatusInternalServerError, rows.Err().Error())
	}
	if count == 0 {
		duration := time.Since(startTime)
		jobRepo.logger.Warn("GetOneByID: job not found", "jobId", jobModel.ID, "duration", duration, "durationMs", duration.Milliseconds())
		return utils.HTTPGenericError(http.StatusNotFound, "job cannot be found")
	}
	duration := time.Since(startTime)
	jobRepo.logger.Debug("GetOneByID: job retrieved successfully", "jobId", jobModel.ID, "projectId", jobModel.ProjectID, "accountId", jobModel.AccountId, "duration", duration, "durationMs", duration.Milliseconds())
	return nil
}

// BatchGetJobsByID returns jobs where uuid in jobUUIDs
func (jobRepo *jobRepo) BatchGetJobsByID(jobIDs []uint64) ([]models.Job, *utils.GenericError) {
	startTime := time.Now()
	jobRepo.logger.Info("batch getting jobs by ID", "jobCount", len(jobIDs))

	jobRepo.fsmStore.GetDataStore().ConnectionLock()
	defer jobRepo.fsmStore.GetDataStore().ConnectionUnlock()

	jobs := []models.Job{}
	batches := utils.Batch[uint64](jobIDs, 1)
	jobRepo.logger.Debug("batched job IDs for query", "totalJobIds", len(jobIDs), "batchCount", len(batches))

	for batchIdx, batch := range batches {
		jobRepo.logger.Debug("processing job batch", "batchIndex", batchIdx+1, "totalBatches", len(batches), "batchSize", len(batch))

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
			constants.JobsIdColumn,
			constants.JobsProjectIdColumn,
			constants.JobsSpecColumn,
			constants.JobsDateCreatedColumn,
			constants.JobsTimezoneColumn,
			constants.JobsTimezoneOffsetColumn,
			constants.JobsDataColumn,
			constants.JobsAccountIdColumn,
			constants.JobsCreatedByColumn,
			constants.JobsDateModifiedColumn,
			constants.JobsModifiedByColumn,
			constants.JobsDeletedByColumn,
			constants.JobsExecutorIdColumn,
			constants.JobsStartDateColumn,
			constants.JobsEndDateColumn,
			constants.JobsRetryMaxColumn,
			constants.JobsStatusColumn,
		).
			From(constants.JobsTableName).
			Where(fmt.Sprintf("%s IN (%s)", constants.JobsIdColumn, paramsPlaceholder), ids...).
			Where(fmt.Sprintf("(%s IS NULL OR %s = '')", constants.JobsDeletedByColumn, constants.JobsDeletedByColumn)).
			RunWith(jobRepo.fsmStore.GetDataStore().GetOpenConnection())

		rows, err := selectBuilder.Query()
		if err != nil {
			jobRepo.logger.Error("BatchGetJobsByID: failed to query jobs", "error", err, "batchSize", len(batch))
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
		}
		for rows.Next() {
			job := models.Job{}
			scanErr := rows.Scan(
				&job.ID,
				&job.ProjectID,
				&job.Spec,
				&job.DateCreated,
				&job.Timezone,
				&job.TimezoneOffset,
				&job.Data,
				&job.AccountId,
				&job.CreatedBy,
				&job.DateModified,
				&job.ModifiedBy,
				&job.DeletedBy,
				&job.ExecutorId,
				&job.StartDate,
				&job.EndDate,
				&job.RetryMax,
				&job.Status,
			)
			if scanErr != nil {
				jobRepo.logger.Error("BatchGetJobsByID: failed to scan job row", "error", scanErr, "batchSize", len(batch))
				return nil, utils.HTTPGenericError(http.StatusInternalServerError, scanErr.Error())
			}
			jobs = append(jobs, job)
		}
		if rows.Err() != nil {
			jobRepo.logger.Error("BatchGetJobsByID: row iteration error", "error", rows.Err(), "batchSize", len(batch))
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, rows.Err().Error())
		}
		rows.Close()
		jobRepo.logger.Debug("job batch query completed", "batchIndex", batchIdx+1, "jobsInBatch", len(jobs))
	}

	duration := time.Since(startTime)
	jobRepo.logger.Info("batch get jobs by ID completed", "requestedCount", len(jobIDs), "returnedCount", len(jobs), "duration", duration, "durationMs", duration.Milliseconds())
	return jobs, nil
}

func (jobRepo *jobRepo) BatchGetJobsWithIDRange(lowerBound, upperBound int64) ([]models.Job, *utils.GenericError) {
	startTime := time.Now()
	jobRepo.logger.Info("batch getting jobs with ID range", "lowerBound", lowerBound, "upperBound", upperBound, "rangeSize", upperBound-lowerBound+1)

	jobRepo.fsmStore.GetDataStore().ConnectionLock()
	defer jobRepo.fsmStore.GetDataStore().ConnectionUnlock()

	selectBuilder := sq.Select(
		constants.JobsIdColumn,
		constants.JobsProjectIdColumn,
		constants.JobsSpecColumn,
		constants.JobsDateCreatedColumn,
		constants.JobsTimezoneColumn,
		constants.JobsTimezoneOffsetColumn,
		constants.JobsDataColumn,
		constants.JobsAccountIdColumn,
		constants.JobsCreatedByColumn,
		constants.JobsDateModifiedColumn,
		constants.JobsModifiedByColumn,
		constants.JobsDeletedByColumn,
		constants.JobsExecutorIdColumn,
		constants.JobsStartDateColumn,
		constants.JobsEndDateColumn,
		constants.JobsRetryMaxColumn,
		constants.JobsStatusColumn,
	).
		From(constants.JobsTableName).
		Where(fmt.Sprintf("%s BETWEEN ? and ?", constants.JobsIdColumn), lowerBound, upperBound).
		Where(fmt.Sprintf("(%s IS NULL OR %s = '')", constants.JobsDeletedByColumn, constants.JobsDeletedByColumn)).
		RunWith(jobRepo.fsmStore.GetDataStore().GetOpenConnection())

	rows, err := selectBuilder.Query()
	if err != nil {
		jobRepo.logger.Error("BatchGetJobsWithIDRange: failed to query jobs", "error", err, "lowerBound", lowerBound, "upperBound", upperBound)
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	defer rows.Close()

	jobs := []models.Job{}
	for rows.Next() {
		job := models.Job{}
		scanErr := rows.Scan(
			&job.ID,
			&job.ProjectID,
			&job.Spec,
			&job.DateCreated,
			&job.Timezone,
			&job.TimezoneOffset,
			&job.Data,
			&job.AccountId,
			&job.CreatedBy,
			&job.DateModified,
			&job.ModifiedBy,
			&job.DeletedBy,
			&job.ExecutorId,
			&job.StartDate,
			&job.EndDate,
			&job.RetryMax,
			&job.Status,
		)
		if scanErr != nil {
			jobRepo.logger.Error("BatchGetJobsWithIDRange: failed to scan job row", "error", scanErr, "lowerBound", lowerBound, "upperBound", upperBound)
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, scanErr.Error())
		}
		jobs = append(jobs, job)
	}
	if rows.Err() != nil {
		jobRepo.logger.Error("BatchGetJobsWithIDRange: row iteration error", "error", rows.Err(), "lowerBound", lowerBound, "upperBound", upperBound)
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, rows.Err().Error())
	}

	duration := time.Since(startTime)
	jobRepo.logger.Info("batch get jobs with ID range completed", "lowerBound", lowerBound, "upperBound", upperBound, "returnedCount", len(jobs), "duration", duration, "durationMs", duration.Milliseconds())
	return jobs, nil
}

// GetAllByProjectID returns paginated set of jobs that are not archived
func (jobRepo *jobRepo) GetAllByProjectID(projectID uint64, offset uint64, limit uint64, orderByColumn string, orderByDirection string) ([]models.Job, *utils.GenericError) {
	startTime := time.Now()
	jobRepo.logger.Info("getting all jobs by project ID", "projectId", projectID, "offset", offset, "limit", limit, "orderBy", orderByColumn, "orderDirection", orderByDirection)

	jobRepo.fsmStore.GetDataStore().ConnectionLock()
	defer jobRepo.fsmStore.GetDataStore().ConnectionUnlock()

	// Validate orderByColumn to prevent SQL injection
	validColumns := map[string]bool{
		"id":            true,
		"project_id":    true,
		"spec":          true,
		"date_created":  true,
		"timezone":      true,
		"account_id":    true,
		"date_modified": true,
		"modified_by":   true,
		"deleted_by":    true,
		"executor_id":   true,
		"start_date":    true,
		"end_date":      true,
		"retry_max":     true,
	}

	if !validColumns[orderByColumn] {
		jobRepo.logger.Warn("GetAllByProjectID: invalid order by column", "orderByColumn", orderByColumn, "projectId", projectID)
		return nil, utils.HTTPGenericError(http.StatusBadRequest, "invalid order by column")
	}

	// Validate orderByDirection
	if orderByDirection != "" {
		orderByDirection = strings.ToLower(orderByDirection)
		if orderByDirection != "asc" && orderByDirection != "desc" {
			jobRepo.logger.Warn("GetAllByProjectID: invalid order by direction", "orderByDirection", orderByDirection, "projectId", projectID)
			return nil, utils.HTTPGenericError(http.StatusBadRequest, "invalid order by direction. Must be ASC or DESC")
		}
		orderByDirection = strings.ToUpper(orderByDirection)
	}

	jobs := []models.Job{}

	selectBuilder := sq.Select(
		constants.JobsIdColumn,
		constants.JobsProjectIdColumn,
		constants.JobsSpecColumn,
		constants.JobsDateCreatedColumn,
		constants.JobsTimezoneColumn,
		constants.JobsTimezoneOffsetColumn,
		constants.JobsDataColumn,
		constants.JobsAccountIdColumn,
		constants.JobsCreatedByColumn,
		constants.JobsDateModifiedColumn,
		constants.JobsModifiedByColumn,
		constants.JobsDeletedByColumn,
		constants.JobsExecutorIdColumn,
		constants.JobsStartDateColumn,
		constants.JobsEndDateColumn,
		constants.JobsRetryMaxColumn,
		constants.JobsStatusColumn,
	).
		From(constants.JobsTableName).
		Offset(offset).
		Limit(limit).
		OrderBy(fmt.Sprintf("%s %s", orderByColumn, orderByDirection)).
		Where(fmt.Sprintf("%s = ?", constants.JobsProjectIdColumn), projectID).
		Where(fmt.Sprintf("(%s IS NULL OR %s = '')", constants.JobsDeletedByColumn, constants.JobsDeletedByColumn)).
		RunWith(jobRepo.fsmStore.GetDataStore().GetOpenConnection())

	rows, err := selectBuilder.Query()
	if err != nil {
		jobRepo.logger.Error("GetAllByProjectID: failed to query jobs", "error", err, "projectId", projectID, "offset", offset, "limit", limit)
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	defer rows.Close()
	for rows.Next() {
		job := models.Job{}
		err = rows.Scan(
			&job.ID,
			&job.ProjectID,
			&job.Spec,
			&job.DateCreated,
			&job.Timezone,
			&job.TimezoneOffset,
			&job.Data,
			&job.AccountId,
			&job.CreatedBy,
			&job.DateModified,
			&job.ModifiedBy,
			&job.DeletedBy,
			&job.ExecutorId,
			&job.StartDate,
			&job.EndDate,
			&job.RetryMax,
			&job.Status,
		)
		if err != nil {
			jobRepo.logger.Error("GetAllByProjectID: failed to scan job row", "error", err, "projectId", projectID)
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
		}
		jobs = append(jobs, job)
	}
	if rows.Err() != nil {
		jobRepo.logger.Error("GetAllByProjectID: row iteration error", "error", rows.Err(), "projectId", projectID)
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, rows.Err().Error())
	}

	duration := time.Since(startTime)
	jobRepo.logger.Info("get all jobs by project ID completed", "projectId", projectID, "returnedCount", len(jobs), "offset", offset, "limit", limit, "duration", duration, "durationMs", duration.Milliseconds())
	return jobs, nil
}

// UpdateOneByID updates a job and returns number of affected rows
func (jobRepo *jobRepo) UpdateOneByID(jobModel models.Job) (uint64, *utils.GenericError) {
	jobPlaceholder := models.Job{
		ID:        jobModel.ID,
		AccountId: jobModel.AccountId,
	}

	if jobPlaceholderError := jobRepo.GetOneByID(&jobPlaceholder); jobPlaceholderError != nil {
		jobRepo.logger.Error("UpdateOneByID: failed to get existing job", "error", jobPlaceholderError, "jobId", jobModel.ID)
		return 0, jobPlaceholderError
	}

	// Prevent any changes to the cron spec (including clearing it)
	if jobPlaceholder.Spec != jobModel.Spec {
		jobRepo.logger.Warn("UpdateOneByID: cannot update cron spec", "jobId", jobModel.ID, "accountId", jobModel.AccountId, "existingSpec", jobPlaceholder.Spec, "newSpec", jobModel.Spec)
		return 0, utils.HTTPGenericError(http.StatusBadRequest, "cannot update cron spec")
	}

	schedulerTime := scheduler0time.GetSchedulerTime()
	now := schedulerTime.GetTime(time.Now())

	if jobModel.AccountId == 0 {
		jobRepo.logger.Warn("UpdateOneByID: account id is required", "jobId", jobModel.ID)
		return 0, utils.HTTPGenericError(http.StatusBadRequest, "account id is required")
	}

	if jobModel.DateModified == nil || jobModel.DateModified.IsZero() {
		jobModel.DateModified = &now
	}

	updateQuery := sq.Update(constants.JobsTableName).
		Set(constants.JobsTimezoneColumn, jobModel.Timezone).
		Set(constants.JobsTimezoneOffsetColumn, jobModel.TimezoneOffset).
		Set(constants.JobsDataColumn, jobModel.Data).
		Set(constants.JobsAccountIdColumn, jobModel.AccountId).
		Set(constants.JobsCreatedByColumn, jobModel.CreatedBy).
		Set(constants.JobsStartDateColumn, jobModel.StartDate).
		Set(constants.JobsEndDateColumn, jobModel.EndDate).
		Set(constants.JobsDateModifiedColumn, jobModel.DateModified).
		Set(constants.JobsModifiedByColumn, jobModel.ModifiedBy).
		Set(constants.JobsDeletedByColumn, jobModel.DeletedBy).
		Set(constants.JobsExecutorIdColumn, jobModel.ExecutorId).
		Set(constants.JobsRetryMaxColumn, jobModel.RetryMax).
		Set(constants.JobsStatusColumn, jobModel.Status).
		Where(fmt.Sprintf("%s = ?", constants.JobsIdColumn), jobModel.ID).
		Where(fmt.Sprintf("%s = ?", constants.JobsAccountIdColumn), jobModel.AccountId)

	query, params, err := updateQuery.ToSql()
	if err != nil {
		jobRepo.logger.Error("UpdateOneByID: failed to build update query", "error", err, "jobId", jobModel.ID, "accountId", jobModel.AccountId)
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	res, applyErr := jobRepo.scheduler0RaftActions.WriteCommandToRaftLog(jobRepo.fsmStore.GetRaft(), constants.CommandTypeDbExecute, query, params, []uint64{}, 0)
	if applyErr != nil {
		jobRepo.logger.Error("UpdateOneByID: failed to write command to raft log", "error", applyErr, "jobId", jobModel.ID, "accountId", jobModel.AccountId)
		return 0, applyErr
	}

	if res == nil {
		jobRepo.logger.Error("UpdateOneByID: raft log result is nil", "jobId", jobModel.ID, "accountId", jobModel.AccountId)
		return 0, utils.HTTPGenericError(http.StatusServiceUnavailable, "service is unavailable - update one by id raft log result is nil")
	}

	count := res.Data.RowsAffected

	return uint64(count), nil
}

// DeleteOneByID marks a job as deleted by setting the DeletedBy field and returns number of affected row
func (jobRepo *jobRepo) DeleteOneByID(jobModel models.Job) (uint64, *utils.GenericError) {
	schedulerTime := scheduler0time.GetSchedulerTime()
	now := schedulerTime.GetTime(time.Now())

	if jobModel.DeletedBy == nil {
		deletedBy := constants.SystemActorName
		jobModel.DeletedBy = &deletedBy
	}

	if jobModel.AccountId == 0 {
		jobRepo.logger.Warn("DeleteOneByID: account id is required", "jobId", jobModel.ID)
		return 0, utils.HTTPGenericError(http.StatusBadRequest, "account id is required")
	}

	if jobModel.DateModified == nil {
		jobModel.DateModified = &now
	}

	updateQuery := sq.Update(constants.JobsTableName).
		Set(constants.JobsDeletedByColumn, jobModel.DeletedBy).
		Set(constants.JobsDateModifiedColumn, now).
		Set(constants.JobsStatusColumn, jobModel.Status).
		Where(fmt.Sprintf("%s = ?", constants.JobsIdColumn), jobModel.ID).
		Where(fmt.Sprintf("%s = ?", constants.JobsAccountIdColumn), jobModel.AccountId)

	query, params, err := updateQuery.ToSql()
	if err != nil {
		jobRepo.logger.Error("DeleteOneByID: failed to build delete query", "error", err, "jobId", jobModel.ID, "accountId", jobModel.AccountId)
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	res, applyErr := jobRepo.scheduler0RaftActions.WriteCommandToRaftLog(jobRepo.fsmStore.GetRaft(), constants.CommandTypeDbExecute, query, params, []uint64{}, 0)
	if applyErr != nil {
		jobRepo.logger.Error("DeleteOneByID: failed to write command to raft log", "error", applyErr, "jobId", jobModel.ID, "accountId", jobModel.AccountId)
		return 0, applyErr
	}

	if res == nil {
		jobRepo.logger.Error("DeleteOneByID: raft log result is nil", "jobId", jobModel.ID, "accountId", jobModel.AccountId)
		return 0, utils.HTTPGenericError(http.StatusServiceUnavailable, "service is unavailable - delete one by id raft log result is nil")
	}

	count := res.Data.RowsAffected

	return uint64(count), nil
}

// DeleteJobsByProjectID marks all jobs for a project as deleted by setting the DeletedBy field.
// accountId is required so a delete cannot cascade across another tenant's jobs.
func (jobRepo *jobRepo) DeleteJobsByProjectID(projectID uint64, accountId uint64, deletedBy string) (uint64, *utils.GenericError) {
	if accountId == 0 {
		return 0, utils.HTTPGenericError(http.StatusBadRequest, "account id is required")
	}

	schedulerTime := scheduler0time.GetSchedulerTime()
	now := schedulerTime.GetTime(time.Now())

	updateQuery := sq.Update(constants.JobsTableName).
		Set(constants.JobsDeletedByColumn, deletedBy).
		Set(constants.JobsDateModifiedColumn, now).
		Set(constants.JobsStatusColumn, "deleted").
		Where(fmt.Sprintf("%s = ?", constants.JobsProjectIdColumn), projectID).
		Where(fmt.Sprintf("%s = ?", constants.JobsAccountIdColumn), accountId).
		Where(fmt.Sprintf("(%s IS NULL OR %s = '')", constants.JobsDeletedByColumn, constants.JobsDeletedByColumn))

	query, params, err := updateQuery.ToSql()
	if err != nil {
		jobRepo.logger.Error("DeleteJobsByProjectID: failed to build delete query", "error", err, "projectId", projectID, "accountId", accountId, "deletedBy", deletedBy)
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	res, applyErr := jobRepo.scheduler0RaftActions.WriteCommandToRaftLog(jobRepo.fsmStore.GetRaft(), constants.CommandTypeDbExecute, query, params, []uint64{}, 0)
	if applyErr != nil {
		jobRepo.logger.Error("DeleteJobsByProjectID: failed to write command to raft log", "error", applyErr, "projectId", projectID, "accountId", accountId, "deletedBy", deletedBy)
		return 0, applyErr
	}

	if res == nil {
		jobRepo.logger.Error("DeleteJobsByProjectID: raft log result is nil", "projectId", projectID, "accountId", accountId, "deletedBy", deletedBy)
		return 0, utils.HTTPGenericError(http.StatusServiceUnavailable, "service is unavailable - delete jobs by project id raft log result is nil")
	}

	count := res.Data.RowsAffected

	return uint64(count), nil
}

// GetJobsTotalCount returns total number of jobs
func (jobRepo *jobRepo) GetJobsTotalCount() (uint64, *utils.GenericError) {
	startTime := time.Now()
	jobRepo.logger.Debug("getting total jobs count")

	jobRepo.fsmStore.GetDataStore().ConnectionLock()
	defer jobRepo.fsmStore.GetDataStore().ConnectionUnlock()

	countQuery := sq.Select("count(*)").
		From(constants.JobsTableName).
		Where(fmt.Sprintf("(%s IS NULL OR %s = '')", constants.JobsDeletedByColumn, constants.JobsDeletedByColumn)).
		RunWith(jobRepo.fsmStore.GetDataStore().GetOpenConnection())
	rows, err := countQuery.Query()
	if err != nil {
		jobRepo.logger.Error("GetJobsTotalCount: failed to query job count", "error", err)
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		err = rows.Scan(
			&count,
		)
		if err != nil {
			jobRepo.logger.Error("GetJobsTotalCount: failed to scan count", "error", err)
			return 0, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
		}
	}
	if rows.Err() != nil {
		jobRepo.logger.Error("GetJobsTotalCount: row iteration error", "error", rows.Err())
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, rows.Err().Error())
	}

	duration := time.Since(startTime)
	jobRepo.logger.Debug("get total jobs count completed", "count", count, "duration", duration, "durationMs", duration.Milliseconds())
	return uint64(count), nil
}

// GetJobsTotalCountByProjectID returns the number of jobs for project with uuid
func (jobRepo *jobRepo) GetJobsTotalCountByProjectID(projectID uint64) (uint64, *utils.GenericError) {
	startTime := time.Now()
	jobRepo.logger.Debug("getting jobs total count by project ID", "projectId", projectID)

	jobRepo.fsmStore.GetDataStore().ConnectionLock()
	defer jobRepo.fsmStore.GetDataStore().ConnectionUnlock()

	if projectID == 0 {
		jobRepo.logger.Warn("GetJobsTotalCountByProjectID: project id is required", "projectId", projectID)
		return 0, utils.HTTPGenericError(http.StatusBadRequest, "project id is required")
	}

	countQuery := sq.Select("count(*)").
		From(constants.JobsTableName).
		Where(fmt.Sprintf("%s = ?", constants.JobsProjectIdColumn), projectID).
		Where(fmt.Sprintf("(%s IS NULL OR %s = '')", constants.JobsDeletedByColumn, constants.JobsDeletedByColumn)).
		RunWith(jobRepo.fsmStore.GetDataStore().GetOpenConnection())
	rows, queryErr := countQuery.Query()
	if queryErr != nil {
		jobRepo.logger.Error("GetJobsTotalCountByProjectID: failed to query job count", "error", queryErr, "projectId", projectID)
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, queryErr.Error())
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		scanErr := rows.Scan(
			&count,
		)
		if scanErr != nil {
			jobRepo.logger.Error("GetJobsTotalCountByProjectID: failed to scan count", "error", scanErr, "projectId", projectID)
			return 0, utils.HTTPGenericError(http.StatusInternalServerError, scanErr.Error())
		}
	}
	if rows.Err() != nil {
		jobRepo.logger.Error("GetJobsTotalCountByProjectID: row iteration error", "error", rows.Err(), "projectId", projectID)
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, rows.Err().Error())
	}

	duration := time.Since(startTime)
	jobRepo.logger.Debug("get jobs total count by project ID completed", "projectId", projectID, "count", count, "duration", duration, "durationMs", duration.Milliseconds())
	return uint64(count), nil
}

// GetJobsPaginated returns a set of jobs starting at offset with the limit
// Filters by accountID always, and optionally by projectID when projectID > 0
func (jobRepo *jobRepo) GetJobsPaginated(accountID uint64, projectID uint64, offset uint64, limit uint64, orderByColumn string, orderByDirection string) ([]models.Job, uint64, *utils.GenericError) {
	startTime := time.Now()
	jobRepo.logger.Info("getting jobs paginated", "accountId", accountID, "projectId", projectID, "offset", offset, "limit", limit, "orderBy", orderByColumn, "orderDirection", orderByDirection)

	jobRepo.fsmStore.GetDataStore().ConnectionLock()
	defer jobRepo.fsmStore.GetDataStore().ConnectionUnlock()

	// Validate orderByColumn to prevent SQL injection
	validColumns := map[string]bool{
		"id":            true,
		"project_id":    true,
		"spec":          true,
		"date_created":  true,
		"timezone":      true,
		"account_id":    true,
		"date_modified": true,
		"modified_by":   true,
		"deleted_by":    true,
		"executor_id":   true,
		"start_date":    true,
		"end_date":      true,
		"retry_max":     true,
	}
	if !validColumns[orderByColumn] {
		jobRepo.logger.Warn("GetJobsPaginated: invalid order by column", "orderByColumn", orderByColumn, "accountId", accountID, "projectId", projectID)
		return nil, 0, utils.HTTPGenericError(http.StatusBadRequest, "invalid order by column")
	}

	if orderByDirection != "" {
		orderByDirection = strings.ToLower(orderByDirection)
		if orderByDirection != "asc" && orderByDirection != "desc" {
			jobRepo.logger.Warn("GetJobsPaginated: invalid order by direction", "orderByDirection", orderByDirection, "accountId", accountID, "projectId", projectID)
			return nil, 0, utils.HTTPGenericError(http.StatusBadRequest, "invalid order by direction. Must be ASC or DESC")
		}
		orderByDirection = strings.ToUpper(orderByDirection)
	}

	// Count
	countQuery := sq.Select("count(*)").
		From(constants.JobsTableName).
		Where(fmt.Sprintf("%s = ?", constants.JobsAccountIdColumn), accountID).
		Where(fmt.Sprintf("(%s IS NULL OR %s = '')", constants.JobsDeletedByColumn, constants.JobsDeletedByColumn)).
		RunWith(jobRepo.fsmStore.GetDataStore().GetOpenConnection())
	if projectID > 0 {
		countQuery = countQuery.Where(fmt.Sprintf("%s = ?", constants.JobsProjectIdColumn), projectID)
	}
	rows, err := countQuery.Query()
	if err != nil {
		jobRepo.logger.Error("GetJobsPaginated: failed to query job count", "error", err, "accountId", accountID, "projectId", projectID)
		return nil, 0, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	defer rows.Close()
	totalInt := 0
	for rows.Next() {
		if err := rows.Scan(&totalInt); err != nil {
			jobRepo.logger.Error("GetJobsPaginated: failed to scan count", "error", err, "accountId", accountID, "projectId", projectID)
			return nil, 0, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
		}
	}
	if rows.Err() != nil {
		jobRepo.logger.Error("GetJobsPaginated: row iteration error when counting", "error", rows.Err(), "accountId", accountID, "projectId", projectID)
		return nil, 0, utils.HTTPGenericError(http.StatusInternalServerError, rows.Err().Error())
	}
	total := uint64(totalInt)

	// Page query
	selectBuilder := sq.Select(
		constants.JobsIdColumn,
		constants.JobsProjectIdColumn,
		constants.JobsSpecColumn,
		constants.JobsDateCreatedColumn,
		constants.JobsTimezoneColumn,
		constants.JobsTimezoneOffsetColumn,
		constants.JobsDataColumn,
		constants.JobsAccountIdColumn,
		constants.JobsCreatedByColumn,
		constants.JobsDateModifiedColumn,
		constants.JobsModifiedByColumn,
		constants.JobsDeletedByColumn,
		constants.JobsExecutorIdColumn,
		constants.JobsStartDateColumn,
		constants.JobsEndDateColumn,
		constants.JobsRetryMaxColumn,
		constants.JobsStatusColumn,
	).
		From(constants.JobsTableName).
		Offset(offset).
		Limit(limit).
		OrderBy(fmt.Sprintf("%s %s", orderByColumn, orderByDirection)).
		Where(fmt.Sprintf("%s = ?", constants.JobsAccountIdColumn), accountID).
		Where(fmt.Sprintf("(%s IS NULL OR %s = '')", constants.JobsDeletedByColumn, constants.JobsDeletedByColumn)).
		RunWith(jobRepo.fsmStore.GetDataStore().GetOpenConnection())
	if projectID > 0 {
		selectBuilder = selectBuilder.Where(fmt.Sprintf("%s = ?", constants.JobsProjectIdColumn), projectID)
	}

	dataRows, derr := selectBuilder.Query()
	if derr != nil {
		jobRepo.logger.Error("GetJobsPaginated: failed to query jobs", "error", derr, "accountId", accountID, "projectId", projectID, "offset", offset, "limit", limit)
		return nil, total, utils.HTTPGenericError(http.StatusInternalServerError, derr.Error())
	}
	defer dataRows.Close()
	jobs := []models.Job{}
	for dataRows.Next() {
		job := models.Job{}
		if err := dataRows.Scan(
			&job.ID,
			&job.ProjectID,
			&job.Spec,
			&job.DateCreated,
			&job.Timezone,
			&job.TimezoneOffset,
			&job.Data,
			&job.AccountId,
			&job.CreatedBy,
			&job.DateModified,
			&job.ModifiedBy,
			&job.DeletedBy,
			&job.ExecutorId,
			&job.StartDate,
			&job.EndDate,
			&job.RetryMax,
			&job.Status,
		); err != nil {
			jobRepo.logger.Error("GetJobsPaginated: failed to scan job row", "error", err, "accountId", accountID, "projectId", projectID)
			return nil, total, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
		}
		jobs = append(jobs, job)
	}
	if dataRows.Err() != nil {
		jobRepo.logger.Error("GetJobsPaginated: row iteration error", "error", dataRows.Err(), "accountId", accountID, "projectId", projectID)
		return nil, total, utils.HTTPGenericError(http.StatusInternalServerError, dataRows.Err().Error())
	}

	duration := time.Since(startTime)
	jobRepo.logger.Info("get jobs paginated completed", "accountId", accountID, "projectId", projectID, "returnedCount", len(jobs), "total", total, "offset", offset, "limit", limit, "duration", duration, "durationMs", duration.Milliseconds())
	return jobs, total, nil
}

// GetJobsTotalCountByAccountID returns the number of jobs for an account, optionally filtered by project
func (jobRepo *jobRepo) GetJobsTotalCountByAccountID(accountID uint64, projectID *uint64) (uint64, *utils.GenericError) {
	startTime := time.Now()
	pid := uint64(0)
	if projectID != nil {
		pid = *projectID
	}
	jobRepo.logger.Debug("getting jobs total count by account ID", "accountId", accountID, "projectId", pid)

	jobRepo.fsmStore.GetDataStore().ConnectionLock()
	defer jobRepo.fsmStore.GetDataStore().ConnectionUnlock()

	countQuery := sq.Select("count(*)").
		From(constants.JobsTableName).
		Where(fmt.Sprintf("%s = ?", constants.JobsAccountIdColumn), accountID).
		Where(fmt.Sprintf("(%s IS NULL OR %s = '')", constants.JobsDeletedByColumn, constants.JobsDeletedByColumn)).
		RunWith(jobRepo.fsmStore.GetDataStore().GetOpenConnection())
	if projectID != nil && *projectID > 0 {
		countQuery = countQuery.Where(fmt.Sprintf("%s = ?", constants.JobsProjectIdColumn), *projectID)
	}
	rows, err := countQuery.Query()
	if err != nil {
		jobRepo.logger.Error("GetJobsTotalCountByAccountID: failed to query job count", "error", err, "accountId", accountID, "projectId", projectID)
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	defer rows.Close()
	totalInt := 0
	for rows.Next() {
		if err := rows.Scan(&totalInt); err != nil {
			jobRepo.logger.Error("GetJobsTotalCountByAccountID: failed to scan count", "error", err, "accountId", accountID, "projectId", projectID)
			return 0, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
		}
	}
	if rows.Err() != nil {
		jobRepo.logger.Error("GetJobsTotalCountByAccountID: row iteration error", "error", rows.Err(), "accountId", accountID, "projectId", projectID)
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, rows.Err().Error())
	}

	duration := time.Since(startTime)
	jobRepo.logger.Debug("get jobs total count by account ID completed", "accountId", accountID, "projectId", pid, "count", totalInt, "duration", duration, "durationMs", duration.Milliseconds())
	return uint64(totalInt), nil
}

// BatchInsertJobs inserts n number of jobs
func (jobRepo *jobRepo) BatchInsertJobs(jobs []models.Job) ([]uint64, *utils.GenericError) {
	startTime := time.Now()
	jobRepo.logger.Info("batch inserting jobs", "jobCount", len(jobs))

	if len(jobs) == 0 {
		jobRepo.logger.Debug("BatchInsertJobs: empty jobs list, returning early")
		return []uint64{}, nil
	}

	batches := utils.Batch[models.Job](jobs, 11)
	jobRepo.logger.Debug("batched jobs for insertion", "totalJobs", len(jobs), "batchCount", len(batches), "batchSize", 11)

	returningIds := []uint64{}

	schedulerTime := scheduler0time.GetSchedulerTime()
	now := schedulerTime.GetTime(time.Now())

	for batchIdx, batch := range batches {
		jobRepo.logger.Debug("processing job insertion batch", "batchIndex", batchIdx+1, "totalBatches", len(batches), "batchSize", len(batch))

		query := fmt.Sprintf("INSERT INTO jobs (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s) VALUES ",
			constants.JobsProjectIdColumn,
			constants.JobsSpecColumn,
			constants.JobsDateCreatedColumn,
			constants.JobsTimezoneColumn,
			constants.JobsTimezoneOffsetColumn,
			constants.JobsDataColumn,
			constants.JobsAccountIdColumn,
			constants.JobsCreatedByColumn,
			constants.JobsExecutorIdColumn,
			constants.JobsStartDateColumn,
			constants.JobsEndDateColumn,
			constants.JobsRetryMaxColumn,
			constants.JobsStatusColumn,
		)
		params := []interface{}{}
		ids := []uint64{}

		for i, job := range batch {
			query += "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"

			job.DateCreated = now

			if job.StartDate.IsZero() {
				job.StartDate = now
			}

			params = append(params,
				job.ProjectID,
				job.Spec,
				job.DateCreated,
				job.Timezone,
				job.TimezoneOffset,
				job.Data,
				job.AccountId,
				job.CreatedBy,
				job.ExecutorId,
				job.StartDate,
				job.EndDate,
				job.RetryMax,
				job.Status,
			)

			if i < len(batch)-1 {
				query += ","
			}
		}

		query += ";"

		res, applyErr := jobRepo.scheduler0RaftActions.WriteCommandToRaftLog(jobRepo.fsmStore.GetRaft(), constants.CommandTypeDbExecute, query, params, []uint64{}, 0)
		if applyErr != nil {
			jobRepo.logger.Error("BatchInsertJobs: failed to write command to raft log", "error", applyErr, "batchSize", len(batch))
			return nil, applyErr
		}

		lastInsertedId := uint64(res.Data.LastInsertedId)

		for i := lastInsertedId - uint64(len(batch)) + 1; i <= lastInsertedId; i++ {
			ids = append(ids, i)
		}

		returningIds = append(returningIds, ids...)
		jobRepo.logger.Debug("job batch inserted successfully", "batchIndex", batchIdx+1, "totalBatches", len(batches), "batchSize", len(batch), "insertedIds", len(ids))
	}

	duration := time.Since(startTime)
	jobRepo.logger.Info("batch insert jobs completed", "totalJobs", len(jobs), "insertedCount", len(returningIds), "batchCount", len(batches), "duration", duration, "durationMs", duration.Milliseconds())
	return returningIds, nil
}

// UpdateJobsStatusByAccountId updates the status of all jobs for a specific account
func (jobRepo *jobRepo) UpdateJobsStatusByAccountId(accountId uint64, status string) *utils.GenericError {
	startTime := time.Now()
	jobRepo.logger.Info("updating jobs status by account ID", "accountId", accountId, "newStatus", status)

	schedulerTime := scheduler0time.GetSchedulerTime()
	now := schedulerTime.GetTime(time.Now())

	updateQuery := sq.Update(constants.JobsTableName).
		Set(constants.JobsStatusColumn, status).
		Set(constants.JobsDateModifiedColumn, now).
		Set(constants.JobsModifiedByColumn, constants.SystemActorName).
		Where(fmt.Sprintf("%s = ?", constants.JobsAccountIdColumn), accountId).
		Where(fmt.Sprintf("(%s IS NULL OR %s = '')", constants.JobsDeletedByColumn, constants.JobsDeletedByColumn))

	query, params, err := updateQuery.ToSql()
	if err != nil {
		jobRepo.logger.Error("UpdateJobsStatusByAccountId: failed to build update query", "error", err, "accountId", accountId, "status", status)
		return utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	res, applyErr := jobRepo.scheduler0RaftActions.WriteCommandToRaftLog(jobRepo.fsmStore.GetRaft(), constants.CommandTypeDbExecute, query, params, []uint64{}, 0)
	if applyErr != nil {
		jobRepo.logger.Error("UpdateJobsStatusByAccountId: failed to write command to raft log", "error", applyErr, "accountId", accountId, "status", status)
		return applyErr
	}

	if res == nil {
		jobRepo.logger.Error("UpdateJobsStatusByAccountId: raft log result is nil", "accountId", accountId, "status", status)
		return utils.HTTPGenericError(http.StatusServiceUnavailable, "service is unavailable - update jobs status by account id raft log result is nil")
	}

	duration := time.Since(startTime)
	rowsAffected := res.Data.RowsAffected
	jobRepo.logger.Info("update jobs status by account ID completed", "accountId", accountId, "newStatus", status, "rowsAffected", rowsAffected, "duration", duration, "durationMs", duration.Milliseconds())
	return nil
}

// GetAllByAccountID returns all jobs for a specific account
func (jobRepo *jobRepo) GetAllByAccountID(accountId uint64) ([]models.Job, *utils.GenericError) {
	startTime := time.Now()
	jobRepo.logger.Info("getting all jobs by account ID", "accountId", accountId)

	jobRepo.fsmStore.GetDataStore().ConnectionLock()
	defer jobRepo.fsmStore.GetDataStore().ConnectionUnlock()

	selectBuilder := sq.Select(
		constants.JobsIdColumn,
		constants.JobsProjectIdColumn,
		constants.JobsSpecColumn,
		constants.JobsDateCreatedColumn,
		constants.JobsTimezoneColumn,
		constants.JobsTimezoneOffsetColumn,
		constants.JobsDataColumn,
		constants.JobsAccountIdColumn,
		constants.JobsCreatedByColumn,
		constants.JobsDateModifiedColumn,
		constants.JobsModifiedByColumn,
		constants.JobsDeletedByColumn,
		constants.JobsExecutorIdColumn,
		constants.JobsStartDateColumn,
		constants.JobsEndDateColumn,
		constants.JobsRetryMaxColumn,
		constants.JobsStatusColumn,
	).
		From(constants.JobsTableName).
		Where(fmt.Sprintf("%s = ?", constants.JobsAccountIdColumn), accountId).
		Where(fmt.Sprintf("(%s IS NULL OR %s = '')", constants.JobsDeletedByColumn, constants.JobsDeletedByColumn)).
		RunWith(jobRepo.fsmStore.GetDataStore().GetOpenConnection())

	rows, err := selectBuilder.Query()
	if err != nil {
		jobRepo.logger.Error("GetAllByAccountID: failed to query jobs", "error", err, "accountId", accountId)
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	defer rows.Close()

	var jobs []models.Job
	for rows.Next() {
		job := models.Job{}
		scanErr := rows.Scan(
			&job.ID,
			&job.ProjectID,
			&job.Spec,
			&job.DateCreated,
			&job.Timezone,
			&job.TimezoneOffset,
			&job.Data,
			&job.AccountId,
			&job.CreatedBy,
			&job.DateModified,
			&job.ModifiedBy,
			&job.DeletedBy,
			&job.ExecutorId,
			&job.StartDate,
			&job.EndDate,
			&job.RetryMax,
			&job.Status,
		)
		if scanErr != nil {
			jobRepo.logger.Error("GetAllByAccountID: failed to scan job row", "error", scanErr, "accountId", accountId)
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, scanErr.Error())
		}
		jobs = append(jobs, job)
	}

	if rows.Err() != nil {
		jobRepo.logger.Error("GetAllByAccountID: row iteration error", "error", rows.Err(), "accountId", accountId)
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, rows.Err().Error())
	}

	duration := time.Since(startTime)
	jobRepo.logger.Info("get all jobs by account ID completed", "accountId", accountId, "returnedCount", len(jobs), "duration", duration, "durationMs", duration.Milliseconds())
	return jobs, nil
}

// GetActiveJobsByExecutorID returns all non-deleted jobs assigned to the given executor
// for the given account. Local executors poll this to discover the jobs they should run.
func (jobRepo *jobRepo) GetActiveJobsByExecutorID(executorID uint64, accountID uint64) ([]models.Job, *utils.GenericError) {
	if accountID == 0 {
		return nil, utils.HTTPGenericError(http.StatusBadRequest, "account id is required")
	}
	if executorID == 0 {
		return nil, utils.HTTPGenericError(http.StatusBadRequest, "executor id is required")
	}

	jobRepo.fsmStore.GetDataStore().ConnectionLock()
	defer jobRepo.fsmStore.GetDataStore().ConnectionUnlock()

	selectBuilder := sq.Select(
		constants.JobsIdColumn,
		constants.JobsProjectIdColumn,
		constants.JobsSpecColumn,
		constants.JobsDateCreatedColumn,
		constants.JobsTimezoneColumn,
		constants.JobsTimezoneOffsetColumn,
		constants.JobsDataColumn,
		constants.JobsAccountIdColumn,
		constants.JobsCreatedByColumn,
		constants.JobsDateModifiedColumn,
		constants.JobsModifiedByColumn,
		constants.JobsDeletedByColumn,
		constants.JobsExecutorIdColumn,
		constants.JobsStartDateColumn,
		constants.JobsEndDateColumn,
		constants.JobsRetryMaxColumn,
		constants.JobsStatusColumn,
	).
		From(constants.JobsTableName).
		Where(fmt.Sprintf("%s = ?", constants.JobsExecutorIdColumn), executorID).
		Where(fmt.Sprintf("%s = ?", constants.JobsAccountIdColumn), accountID).
		Where(fmt.Sprintf("(%s IS NULL OR %s = '')", constants.JobsDeletedByColumn, constants.JobsDeletedByColumn)).
		RunWith(jobRepo.fsmStore.GetDataStore().GetOpenConnection())

	rows, err := selectBuilder.Query()
	if err != nil {
		jobRepo.logger.Error("GetActiveJobsByExecutorID: failed to query jobs", "error", err, "executorId", executorID, "accountId", accountID)
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	defer rows.Close()

	var jobs []models.Job
	for rows.Next() {
		job := models.Job{}
		scanErr := rows.Scan(
			&job.ID,
			&job.ProjectID,
			&job.Spec,
			&job.DateCreated,
			&job.Timezone,
			&job.TimezoneOffset,
			&job.Data,
			&job.AccountId,
			&job.CreatedBy,
			&job.DateModified,
			&job.ModifiedBy,
			&job.DeletedBy,
			&job.ExecutorId,
			&job.StartDate,
			&job.EndDate,
			&job.RetryMax,
			&job.Status,
		)
		if scanErr != nil {
			jobRepo.logger.Error("GetActiveJobsByExecutorID: failed to scan job row", "error", scanErr, "executorId", executorID, "accountId", accountID)
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, scanErr.Error())
		}
		jobs = append(jobs, job)
	}

	if rows.Err() != nil {
		jobRepo.logger.Error("GetActiveJobsByExecutorID: row iteration error", "error", rows.Err(), "executorId", executorID, "accountId", accountID)
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, rows.Err().Error())
	}

	return jobs, nil
}
