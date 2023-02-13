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
	"scheduler0/utils/batcher"
	"time"
)

// JobRepo job table manager
type jobRepo struct {
	fsmStore *fsm.Store
	logger   hclog.Logger
}

type Job interface {
	GetOneByID(jobModel *models.JobModel) *utils.GenericError
	BatchGetJobsByID(jobIDs []uint64) ([]models.JobModel, *utils.GenericError)
	BatchGetJobsWithIDRange(lowerBound, upperBound uint64) ([]models.JobModel, *utils.GenericError)
	GetJobsPaginated(projectID uint64, offset uint64, limit uint64) ([]models.JobModel, uint64, *utils.GenericError)
	GetJobsTotalCountByProjectID(projectID uint64) (uint64, *utils.GenericError)
	GetJobsTotalCount() (uint64, *utils.GenericError)
	DeleteOneByID(jobModel models.JobModel) (uint64, *utils.GenericError)
	UpdateOneByID(jobModel models.JobModel) (uint64, *utils.GenericError)
	GetAllByProjectID(projectID uint64, offset uint64, limit uint64, orderBy string) ([]models.JobModel, *utils.GenericError)
	BatchInsertJobs(jobRepos []models.JobModel) ([]uint64, *utils.GenericError)
}

const (
	JobsTableName = "jobs"
)

const (
	JobsIdColumn          = "id"
	ProjectIdColumn       = "project_id"
	SpecColumn            = "spec"
	CallbackURLColumn     = "callback_url"
	DataColumn            = "data"
	ExecutionTypeColumn   = "execution_type"
	JobsDateCreatedColumn = "date_created"
)

func NewJobRepo(logger hclog.Logger, store *fsm.Store) Job {
	return &jobRepo{
		fsmStore: store,
		logger:   logger.Named("job-repo"),
	}
}

// GetOneByID returns a single job that matches uuid
func (jobRepo *jobRepo) GetOneByID(jobModel *models.JobModel) *utils.GenericError {
	jobRepo.fsmStore.DataStore.ConnectionLock.Lock()
	defer jobRepo.fsmStore.DataStore.ConnectionLock.Unlock()

	selectBuilder := sq.Select(
		JobsIdColumn,
		ProjectIdColumn,
		SpecColumn,
		CallbackURLColumn,
		ExecutionTypeColumn,
		JobsDateCreatedColumn,
		DataColumn,
	).
		From(JobsTableName).
		Where(fmt.Sprintf("%s = ?", JobsIdColumn), jobModel.ID).
		RunWith(jobRepo.fsmStore.DataStore.Connection)

	rows, err := selectBuilder.Query()
	if err != nil {
		return utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		err = rows.Scan(
			&jobModel.ID,
			&jobModel.ProjectID,
			&jobModel.Spec,
			&jobModel.CallbackUrl,
			&jobModel.ExecutionType,
			&jobModel.DateCreated,
			&jobModel.Data,
		)
		//t, errParse := dateparse.ParseLocal(dataString)
		//if errParse != nil {
		//	return utils.HTTPGenericError(500, errParse.Error())
		//}
		//jobModel.DateCreated = t
		//if err != nil {
		//	return utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
		//}
		count += 1
	}
	if rows.Err() != nil {
		return utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	return nil
}

// BatchGetJobsByID returns jobs where uuid in jobUUIDs
func (jobRepo *jobRepo) BatchGetJobsByID(jobIDs []uint64) ([]models.JobModel, *utils.GenericError) {
	jobRepo.fsmStore.DataStore.ConnectionLock.Lock()
	defer jobRepo.fsmStore.DataStore.ConnectionLock.Unlock()

	jobs := []models.JobModel{}
	batches := batcher.Batch[uint64](jobIDs, 1)

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
			JobsIdColumn,
			ProjectIdColumn,
			SpecColumn,
			CallbackURLColumn,
			ExecutionTypeColumn,
			JobsDateCreatedColumn,
			DataColumn,
		).
			From(JobsTableName).
			Where(fmt.Sprintf("%s IN (%s)", JobsIdColumn, paramsPlaceholder), ids...).
			RunWith(jobRepo.fsmStore.DataStore.Connection)

		rows, err := selectBuilder.Query()
		if err != nil {
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
		}
		for rows.Next() {
			job := models.JobModel{}
			scanErr := rows.Scan(
				&job.ID,
				&job.ProjectID,
				&job.Spec,
				&job.CallbackUrl,
				&job.ExecutionType,
				&job.DateCreated,
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

func (jobRepo *jobRepo) BatchGetJobsWithIDRange(lowerBound, upperBound uint64) ([]models.JobModel, *utils.GenericError) {
	jobRepo.fsmStore.DataStore.ConnectionLock.Lock()
	defer jobRepo.fsmStore.DataStore.ConnectionLock.Unlock()

	selectBuilder := sq.Select(
		JobsIdColumn,
		ProjectIdColumn,
		SpecColumn,
		CallbackURLColumn,
		ExecutionTypeColumn,
		JobsDateCreatedColumn,
		DataColumn,
	).
		From(JobsTableName).
		Where(fmt.Sprintf("%s BETWEEN ? and ?", JobsIdColumn), lowerBound, upperBound).
		RunWith(jobRepo.fsmStore.DataStore.Connection)

	rows, err := selectBuilder.Query()
	if err != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	defer rows.Close()

	jobs := []models.JobModel{}
	for rows.Next() {
		job := models.JobModel{}
		scanErr := rows.Scan(
			&job.ID,
			&job.ProjectID,
			&job.Spec,
			&job.CallbackUrl,
			&job.ExecutionType,
			&job.DateCreated,
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
func (jobRepo *jobRepo) GetAllByProjectID(projectID uint64, offset uint64, limit uint64, orderBy string) ([]models.JobModel, *utils.GenericError) {
	jobRepo.fsmStore.DataStore.ConnectionLock.Lock()
	defer jobRepo.fsmStore.DataStore.ConnectionLock.Unlock()

	jobs := []models.JobModel{}

	selectBuilder := sq.Select(
		JobsIdColumn,
		ProjectIdColumn,
		SpecColumn,
		CallbackURLColumn,
		ExecutionTypeColumn,
		JobsDateCreatedColumn,
		DataColumn,
	).
		From(JobsTableName).
		Offset(offset).
		Limit(limit).
		OrderBy(orderBy).
		Where(fmt.Sprintf("%s = ?", ProjectIdColumn), projectID).
		RunWith(jobRepo.fsmStore.DataStore.Connection)

	rows, err := selectBuilder.Query()
	if err != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	defer rows.Close()
	for rows.Next() {
		job := models.JobModel{}
		err = rows.Scan(
			&job.ID,
			&job.ProjectID,
			&job.Spec,
			&job.CallbackUrl,
			&job.ExecutionType,
			&job.DateCreated,
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
func (jobRepo *jobRepo) UpdateOneByID(jobModel models.JobModel) (uint64, *utils.GenericError) {
	jobPlaceholder := models.JobModel{
		ID: jobModel.ID,
	}

	if jobPlaceholderError := jobRepo.GetOneByID(&jobPlaceholder); jobPlaceholderError != nil {
		return 0, jobPlaceholderError
	}

	if jobPlaceholder.Spec != jobModel.Spec && jobModel.Spec != "" {
		return 0, utils.HTTPGenericError(http.StatusBadRequest, "cannot update cron spec")
	}

	updateQuery := sq.Update(JobsTableName).
		Set(CallbackURLColumn, jobModel.CallbackUrl).
		Set(ExecutionTypeColumn, jobModel.ExecutionType).
		Set(DataColumn, jobModel.Data).
		Where(fmt.Sprintf("%s = ?", JobsIdColumn), jobModel.ID)

	query, params, err := updateQuery.ToSql()
	if err != nil {
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	res, applyErr := fsm.AppApply(jobRepo.fsmStore.Raft, constants.CommandTypeDbExecute, query, params)
	if err != nil {
		return 0, applyErr
	}

	if res == nil {
		return 0, utils.HTTPGenericError(http.StatusServiceUnavailable, "service is unavailable")
	}

	count := res.Data[1].(int64)

	return uint64(count), nil
}

// DeleteOneByID deletes a job with uuid and returns number of affected row
func (jobRepo *jobRepo) DeleteOneByID(jobModel models.JobModel) (uint64, *utils.GenericError) {
	deleteQuery := sq.Delete(JobsTableName).Where(fmt.Sprintf("%s = ?", JobsIdColumn), jobModel.ID)

	query, params, err := deleteQuery.ToSql()
	if err != nil {
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	res, applyErr := fsm.AppApply(jobRepo.fsmStore.Raft, constants.CommandTypeDbExecute, query, params)
	if err != nil {
		return 0, applyErr
	}

	if res == nil {
		return 0, utils.HTTPGenericError(http.StatusServiceUnavailable, "service is unavailable")
	}

	count := res.Data[1].(int64)

	return uint64(count), nil
}

// GetJobsTotalCount returns total number of jobs
func (jobRepo *jobRepo) GetJobsTotalCount() (uint64, *utils.GenericError) {
	jobRepo.fsmStore.DataStore.ConnectionLock.Lock()
	defer jobRepo.fsmStore.DataStore.ConnectionLock.Unlock()

	countQuery := sq.Select("count(*)").From(JobsTableName).RunWith(jobRepo.fsmStore.DataStore.Connection)
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
	jobRepo.fsmStore.DataStore.ConnectionLock.Lock()
	defer jobRepo.fsmStore.DataStore.ConnectionLock.Unlock()

	countQuery := sq.Select("count(*)").
		From(JobsTableName).
		Where(fmt.Sprintf("%s = ?", ProjectIdColumn), projectID).
		RunWith(jobRepo.fsmStore.DataStore.Connection)
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
func (jobRepo *jobRepo) GetJobsPaginated(projectID uint64, offset uint64, limit uint64) ([]models.JobModel, uint64, *utils.GenericError) {
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
func (jobRepo *jobRepo) BatchInsertJobs(jobs []models.JobModel) ([]uint64, *utils.GenericError) {
	batches := batcher.Batch[models.JobModel](jobs, 6)

	returningIds := []uint64{}

	schedulerTime := utils.GetSchedulerTime()
	now := schedulerTime.GetTime(time.Now())

	for _, batch := range batches {
		query := fmt.Sprintf("INSERT INTO jobs (%s, %s, %s, %s, %s, %s) VALUES ",
			ProjectIdColumn,
			SpecColumn,
			CallbackURLColumn,
			ExecutionTypeColumn,
			JobsDateCreatedColumn,
			DataColumn,
		)
		params := []interface{}{}
		ids := []uint64{}

		for i, job := range batch {
			query += fmt.Sprint("(?, ?, ?, ?, ?, ?)")
			job.DateCreated = now
			params = append(params,
				job.ProjectID,
				job.Spec,
				job.CallbackUrl,
				job.ExecutionType,
				job.DateCreated,
				job.Data,
			)

			if i < len(batch)-1 {
				query += ","
			}
		}

		query += ";"

		res, applyErr := fsm.AppApply(jobRepo.fsmStore.Raft, constants.CommandTypeDbExecute, query, params)
		if res == nil {
			return nil, utils.HTTPGenericError(http.StatusServiceUnavailable, "service is unavailable")
		}

		if applyErr != nil {
			return nil, applyErr
		}

		lastInsertedId := uint64(res.Data[0].(int64))

		for i := lastInsertedId - uint64(len(batch)) + 1; i <= lastInsertedId; i++ {
			ids = append(ids, i)
		}

		returningIds = append(returningIds, ids...)
	}

	return returningIds, nil
}
