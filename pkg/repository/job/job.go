package job

import (
	"fmt"
	sq "github.com/Masterminds/squirrel"
	"github.com/hashicorp/go-hclog"
	"net/http"
	"scheduler0/pkg/constants"
	"scheduler0/pkg/fsm"
	"scheduler0/pkg/models"
	"scheduler0/pkg/scheduler0time"
	"scheduler0/pkg/utils"
	"time"
)

// JobRepo job table manager
type jobRepo struct {
	fsmStore              fsm.Scheduler0RaftStore
	logger                hclog.Logger
	scheduler0RaftActions fsm.Scheduler0RaftActions
}

//go:generate mockery --name JobRepo --output ../mocks
type JobRepo interface {
	GetOneByID(jobModel *models.Job) *utils.GenericError
	BatchGetJobsByID(jobIDs []uint64) ([]models.Job, *utils.GenericError)
	BatchGetJobsWithIDRange(lowerBound, upperBound int64) ([]models.Job, *utils.GenericError)
	GetJobsPaginated(projectID uint64, offset uint64, limit uint64) ([]models.Job, uint64, *utils.GenericError)
	GetJobsTotalCountByProjectID(projectID uint64) (uint64, *utils.GenericError)
	GetJobsTotalCount() (uint64, *utils.GenericError)
	DeleteOneByID(jobModel models.Job) (uint64, *utils.GenericError)
	UpdateOneByID(jobModel models.Job) (uint64, *utils.GenericError)
	GetAllByProjectID(projectID uint64, offset uint64, limit uint64, orderBy string) ([]models.Job, *utils.GenericError)
	BatchInsertJobs(jobRepos []models.Job) ([]uint64, *utils.GenericError)
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
	jobRepo.fsmStore.GetDataStore().ConnectionLock()
	defer jobRepo.fsmStore.GetDataStore().ConnectionUnlock()

	selectBuilder := sq.Select(
		constants.JobsIdColumn,
		constants.JobsProjectIdColumn,
		constants.JobsSpecColumn,
		constants.JobsCallbackURLColumn,
		constants.JobsExecutionTypeColumn,
		constants.JobsDateCreatedColumn,
		constants.JobsTimezoneColumn,
		constants.JobsTimezoneOffsetColumn,
		constants.JobsDataColumn,
	).
		From(constants.JobsTableName).
		Where(fmt.Sprintf("%s = ?", constants.JobsIdColumn), jobModel.ID).
		RunWith(jobRepo.fsmStore.GetDataStore().GetOpenConnection())

	rows, err := selectBuilder.Query()
	if err != nil {
		return utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		scanErr := rows.Scan(
			&jobModel.ID,
			&jobModel.ProjectID,
			&jobModel.Spec,
			&jobModel.CallbackUrl,
			&jobModel.ExecutionType,
			&jobModel.DateCreated,
			&jobModel.Timezone,
			&jobModel.TimezoneOffset,
			&jobModel.Data,
		)
		if scanErr != nil {
			return utils.HTTPGenericError(http.StatusInternalServerError, scanErr.Error())
		}
		count += 1
	}
	if rows.Err() != nil {
		return utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	if count == 0 {
		return utils.HTTPGenericError(http.StatusNotFound, "job cannot be found")
	}
	return nil
}

// BatchGetJobsByID returns jobs where uuid in jobUUIDs
func (jobRepo *jobRepo) BatchGetJobsByID(jobIDs []uint64) ([]models.Job, *utils.GenericError) {
	jobRepo.fsmStore.GetDataStore().ConnectionLock()
	defer jobRepo.fsmStore.GetDataStore().ConnectionUnlock()

	jobs := []models.Job{}
	batches := utils.Batch[uint64](jobIDs, 1)

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
			constants.JobsIdColumn,
			constants.JobsProjectIdColumn,
			constants.JobsSpecColumn,
			constants.JobsCallbackURLColumn,
			constants.JobsExecutionTypeColumn,
			constants.JobsDateCreatedColumn,
			constants.JobsTimezoneColumn,
			constants.JobsTimezoneOffsetColumn,
			constants.JobsDataColumn,
		).
			From(constants.JobsTableName).
			Where(fmt.Sprintf("%s IN (%s)", constants.JobsIdColumn, paramsPlaceholder), ids...).
			RunWith(jobRepo.fsmStore.GetDataStore().GetOpenConnection())

		rows, err := selectBuilder.Query()
		if err != nil {
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
		}
		for rows.Next() {
			job := models.Job{}
			scanErr := rows.Scan(
				&job.ID,
				&job.ProjectID,
				&job.Spec,
				&job.CallbackUrl,
				&job.ExecutionType,
				&job.DateCreated,
				&job.Timezone,
				&job.TimezoneOffset,
				&job.Data,
			)
			if scanErr != nil {
				return nil, utils.HTTPGenericError(http.StatusInternalServerError, scanErr.Error())
			}
			jobs = append(jobs, job)
		}
		if rows.Err() != nil {
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, rows.Err().Error())
		}
		rows.Close()
	}

	return jobs, nil
}

func (jobRepo *jobRepo) BatchGetJobsWithIDRange(lowerBound, upperBound int64) ([]models.Job, *utils.GenericError) {
	jobRepo.fsmStore.GetDataStore().ConnectionLock()
	defer jobRepo.fsmStore.GetDataStore().ConnectionUnlock()

	selectBuilder := sq.Select(
		constants.JobsIdColumn,
		constants.JobsProjectIdColumn,
		constants.JobsSpecColumn,
		constants.JobsCallbackURLColumn,
		constants.JobsExecutionTypeColumn,
		constants.JobsDateCreatedColumn,
		constants.JobsTimezoneColumn,
		constants.JobsTimezoneOffsetColumn,
		constants.JobsDataColumn,
	).
		From(constants.JobsTableName).
		Where(fmt.Sprintf("%s BETWEEN ? and ?", constants.JobsIdColumn), lowerBound, upperBound).
		RunWith(jobRepo.fsmStore.GetDataStore().GetOpenConnection())

	rows, err := selectBuilder.Query()
	if err != nil {
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
			&job.CallbackUrl,
			&job.ExecutionType,
			&job.DateCreated,
			&job.Timezone,
			&job.TimezoneOffset,
			&job.Data,
		)
		if scanErr != nil {
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, scanErr.Error())
		}
		jobs = append(jobs, job)
	}
	if rows.Err() != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, rows.Err().Error())
	}

	return jobs, nil
}

// GetAllByProjectID returns paginated set of jobs that are not archived
func (jobRepo *jobRepo) GetAllByProjectID(projectID uint64, offset uint64, limit uint64, orderBy string) ([]models.Job, *utils.GenericError) {
	jobRepo.fsmStore.GetDataStore().ConnectionLock()
	defer jobRepo.fsmStore.GetDataStore().ConnectionUnlock()

	jobs := []models.Job{}

	selectBuilder := sq.Select(
		constants.JobsIdColumn,
		constants.JobsProjectIdColumn,
		constants.JobsSpecColumn,
		constants.JobsCallbackURLColumn,
		constants.JobsExecutionTypeColumn,
		constants.JobsDateCreatedColumn,
		constants.JobsTimezoneColumn,
		constants.JobsTimezoneOffsetColumn,
		constants.JobsDataColumn,
	).
		From(constants.JobsTableName).
		Offset(offset).
		Limit(limit).
		OrderBy(orderBy).
		Where(fmt.Sprintf("%s = ?", constants.JobsProjectIdColumn), projectID).
		RunWith(jobRepo.fsmStore.GetDataStore().GetOpenConnection())

	rows, err := selectBuilder.Query()
	if err != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	defer rows.Close()
	for rows.Next() {
		job := models.Job{}
		err = rows.Scan(
			&job.ID,
			&job.ProjectID,
			&job.Spec,
			&job.CallbackUrl,
			&job.ExecutionType,
			&job.DateCreated,
			&job.Timezone,
			&job.TimezoneOffset,
			&job.Data,
		)
		if err != nil {
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
		}
		jobs = append(jobs, job)
	}
	if rows.Err() != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	return jobs, nil
}

// UpdateOneByID updates a job and returns number of affected rows
func (jobRepo *jobRepo) UpdateOneByID(jobModel models.Job) (uint64, *utils.GenericError) {
	jobPlaceholder := models.Job{
		ID: jobModel.ID,
	}

	if jobPlaceholderError := jobRepo.GetOneByID(&jobPlaceholder); jobPlaceholderError != nil {
		return 0, jobPlaceholderError
	}

	if jobPlaceholder.Spec != jobModel.Spec && jobModel.Spec != "" {
		return 0, utils.HTTPGenericError(http.StatusBadRequest, "cannot update cron spec")
	}

	updateQuery := sq.Update(constants.JobsTableName).
		Set(constants.JobsCallbackURLColumn, jobModel.CallbackUrl).
		Set(constants.JobsExecutionTypeColumn, jobModel.ExecutionType).
		Set(constants.JobsTimezoneColumn, jobModel.Timezone).
		Set(constants.JobsTimezoneOffsetColumn, jobModel.TimezoneOffset).
		Set(constants.JobsDataColumn, jobModel.Data).
		Where(fmt.Sprintf("%s = ?", constants.JobsIdColumn), jobModel.ID)

	query, params, err := updateQuery.ToSql()
	if err != nil {
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	res, applyErr := jobRepo.scheduler0RaftActions.WriteCommandToRaftLog(jobRepo.fsmStore.GetRaft(), constants.CommandTypeDbExecute, query, params, []uint64{}, 0)
	if err != nil {
		return 0, applyErr
	}

	if res == nil {
		return 0, utils.HTTPGenericError(http.StatusServiceUnavailable, "service is unavailable")
	}

	count := res.Data.RowsAffected

	return uint64(count), nil
}

// DeleteOneByID deletes a job with uuid and returns number of affected row
func (jobRepo *jobRepo) DeleteOneByID(jobModel models.Job) (uint64, *utils.GenericError) {
	deleteQuery := sq.Delete(constants.JobsTableName).Where(fmt.Sprintf("%s = ?", constants.JobsIdColumn), jobModel.ID)

	query, params, err := deleteQuery.ToSql()
	if err != nil {
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	res, applyErr := jobRepo.scheduler0RaftActions.WriteCommandToRaftLog(jobRepo.fsmStore.GetRaft(), constants.CommandTypeDbExecute, query, params, []uint64{}, 0)
	if err != nil {
		return 0, applyErr
	}

	if res == nil {
		return 0, utils.HTTPGenericError(http.StatusServiceUnavailable, "service is unavailable")
	}

	count := res.Data.RowsAffected

	return uint64(count), nil
}

// GetJobsTotalCount returns total number of jobs
func (jobRepo *jobRepo) GetJobsTotalCount() (uint64, *utils.GenericError) {
	jobRepo.fsmStore.GetDataStore().ConnectionLock()
	defer jobRepo.fsmStore.GetDataStore().ConnectionUnlock()

	countQuery := sq.Select("count(*)").From(constants.JobsTableName).RunWith(jobRepo.fsmStore.GetDataStore().GetOpenConnection())
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

// GetJobsTotalCountByProjectID returns the number of jobs for project with uuid
func (jobRepo *jobRepo) GetJobsTotalCountByProjectID(projectID uint64) (uint64, *utils.GenericError) {
	jobRepo.fsmStore.GetDataStore().ConnectionLock()
	defer jobRepo.fsmStore.GetDataStore().ConnectionUnlock()

	countQuery := sq.Select("count(*)").
		From(constants.JobsTableName).
		Where(fmt.Sprintf("%s = ?", constants.JobsProjectIdColumn), projectID).
		RunWith(jobRepo.fsmStore.GetDataStore().GetOpenConnection())
	rows, queryErr := countQuery.Query()
	if queryErr != nil {
		return 0, utils.HTTPGenericError(500, queryErr.Error())
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		scanErr := rows.Scan(
			&count,
		)
		if scanErr != nil {
			return 0, utils.HTTPGenericError(500, scanErr.Error())
		}
	}
	if rows.Err() != nil {
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, rows.Err().Error())
	}

	return uint64(count), nil
}

// GetJobsPaginated returns a set of jobs starting at offset with the limit
func (jobRepo *jobRepo) GetJobsPaginated(projectID uint64, offset uint64, limit uint64) ([]models.Job, uint64, *utils.GenericError) {
	total, err := jobRepo.GetJobsTotalCountByProjectID(projectID)

	if err != nil {
		return nil, 0, err
	}

	jobRepos, err := jobRepo.GetAllByProjectID(projectID, offset, limit, "date_created")
	if err != nil {
		return nil, total, err
	}

	return jobRepos, uint64(len(jobRepos)), nil
}

// BatchInsertJobs inserts n number of jobs
func (jobRepo *jobRepo) BatchInsertJobs(jobs []models.Job) ([]uint64, *utils.GenericError) {
	batches := utils.Batch[models.Job](jobs, 8)

	returningIds := []uint64{}

	schedulerTime := scheduler0time.GetSchedulerTime()
	now := schedulerTime.GetTime(time.Now())

	for _, batch := range batches {
		query := fmt.Sprintf("INSERT INTO jobs (%s, %s, %s, %s, %s, %s, %s, %s) VALUES ",
			constants.JobsProjectIdColumn,
			constants.JobsSpecColumn,
			constants.JobsCallbackURLColumn,
			constants.JobsExecutionTypeColumn,
			constants.JobsDateCreatedColumn,
			constants.JobsTimezoneColumn,
			constants.JobsTimezoneOffsetColumn,
			constants.JobsDataColumn,
		)
		params := []interface{}{}
		ids := []uint64{}

		for i, job := range batch {
			query += fmt.Sprint("(?, ?, ?, ?, ?, ?, ?, ?)")
			job.DateCreated = now
			params = append(params,
				job.ProjectID,
				job.Spec,
				job.CallbackUrl,
				job.ExecutionType,
				job.DateCreated,
				job.Timezone,
				job.TimezoneOffset,
				job.Data,
			)

			if i < len(batch)-1 {
				query += ","
			}
		}

		query += ";"

		res, applyErr := jobRepo.scheduler0RaftActions.WriteCommandToRaftLog(jobRepo.fsmStore.GetRaft(), constants.CommandTypeDbExecute, query, params, []uint64{}, 0)
		if applyErr != nil {
			return nil, applyErr
		}

		lastInsertedId := uint64(res.Data.LastInsertedId)

		for i := lastInsertedId - uint64(len(batch)) + 1; i <= lastInsertedId; i++ {
			ids = append(ids, i)
		}

		returningIds = append(returningIds, ids...)
	}

	return returningIds, nil
}
