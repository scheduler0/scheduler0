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

// JobRepo job table manager
type jobRepo struct {
	store  *fsm.Store
	logger *log.Logger
}

type Job interface {
	GetOneByID(jobModel *models.JobModel) *utils.GenericError
	BatchGetJobsByID(jobIDs []int64) ([]models.JobModel, *utils.GenericError)
	GetJobsPaginated(projectID int64, offset int64, limit int64) ([]models.JobModel, int64, *utils.GenericError)
	GetJobsTotalCountByProjectID(projectID int64) (int64, *utils.GenericError)
	GetJobsTotalCount() (int64, *utils.GenericError)
	DeleteOneByID(jobModel models.JobModel) (int64, *utils.GenericError)
	UpdateOneByID(jobModel models.JobModel) (int64, *utils.GenericError)
	GetAllByProjectID(projectID int64, offset int64, limit int64, orderBy string) ([]models.JobModel, *utils.GenericError)
	BatchInsertJobs(jobRepos []models.JobModel) ([]int64, *utils.GenericError)
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

func NewJobRepo(logger *log.Logger, store *fsm.Store) Job {
	return &jobRepo{
		store:  store,
		logger: logger,
	}
}

// GetOneByID returns a single job that matches uuid
func (jobRepo *jobRepo) GetOneByID(jobModel *models.JobModel) *utils.GenericError {
	selectBuilder := sq.Select(
		JobsIdColumn,
		ProjectIdColumn,
		SpecColumn,
		CallbackURLColumn,
		ExecutionTypeColumn,
		fmt.Sprintf("cast(\"%s\" as text)", JobsDateCreatedColumn),
		DataColumn,
	).
		From(JobsTableName).
		Where(fmt.Sprintf("%s = ?", JobsIdColumn), jobModel.ID).
		RunWith(jobRepo.store.SQLDbConnection)

	rows, err := selectBuilder.Query()
	if err != nil {
		return utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var dataString string
		err = rows.Scan(
			&jobModel.ID,
			&jobModel.ProjectID,
			&jobModel.Spec,
			&jobModel.CallbackUrl,
			&jobModel.ExecutionType,
			&dataString,
			&jobModel.Data,
		)
		t, errParse := dateparse.ParseLocal(dataString)
		if errParse != nil {
			return utils.HTTPGenericError(500, errParse.Error())
		}
		jobModel.DateCreated = t
		if err != nil {
			return utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
		}
		count += 1
	}
	if rows.Err() != nil {
		return utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	return nil
}

// BatchGetJobsByID returns jobs where uuid in jobUUIDs
func (jobRepo *jobRepo) BatchGetJobsByID(jobIDs []int64) ([]models.JobModel, *utils.GenericError) {
	jobs := []models.JobModel{}
	batches := [][]int64{}

	if len(jobIDs) > 100 {
		temp := []int64{}
		count := 0

		for count < len(jobIDs) {
			temp = append(temp, jobIDs[count])
			if len(temp) == 100 {
				batches = append(batches, temp)
				temp = []int64{}
			}
			count += 1
		}

		if len(temp) > 0 {
			batches = append(batches, temp)
			temp = []int64{}
		}
	} else {
		batches = append(batches, jobIDs)
	}

	for _, jobIDs := range batches {
		paramsPlaceholder := ""
		ids := []interface{}{}

		for i, id := range jobIDs {
			paramsPlaceholder += "?"

			if i < len(jobIDs)-1 {
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
			fmt.Sprintf("cast(\"%s\" as text)", JobsDateCreatedColumn),
			DataColumn,
		).
			From(JobsTableName).
			Where(fmt.Sprintf("%s IN (%s)", JobsIdColumn, paramsPlaceholder), ids...).
			RunWith(jobRepo.store.SQLDbConnection)

		rows, err := selectBuilder.Query()
		if err != nil {
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
		}
		defer rows.Close()
		for rows.Next() {
			job := models.JobModel{}
			var dataString string
			scanErr := rows.Scan(
				&job.ID,
				&job.ProjectID,
				&job.Spec,
				&job.CallbackUrl,
				&job.ExecutionType,
				&dataString,
				&job.Data,
			)
			t, errParse := dateparse.ParseLocal(dataString)
			if errParse != nil {
				return nil, utils.HTTPGenericError(500, errParse.Error())
			}
			job.DateCreated = t
			if scanErr != nil {
				return nil, utils.HTTPGenericError(http.StatusInternalServerError, scanErr.Error())
			}
			jobs = append(jobs, job)
		}
		if rows.Err() != nil {
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, rows.Err().Error())
		}
	}

	return jobs, nil
}

// GetAllByProjectID returns paginated set of jobs that are not archived
func (jobRepo *jobRepo) GetAllByProjectID(projectID int64, offset int64, limit int64, orderBy string) ([]models.JobModel, *utils.GenericError) {
	jobs := []models.JobModel{}

	selectBuilder := sq.Select(
		JobsIdColumn,
		ProjectIdColumn,
		SpecColumn,
		CallbackURLColumn,
		ExecutionTypeColumn,
		fmt.Sprintf("cast(\"%s\" as text)", JobsDateCreatedColumn),
		DataColumn,
	).
		From(JobsTableName).
		Offset(uint64(offset)).
		Limit(uint64(limit)).
		OrderBy(orderBy).
		Where(fmt.Sprintf("%s = ?", ProjectIdColumn), projectID).
		RunWith(jobRepo.store.SQLDbConnection)

	rows, err := selectBuilder.Query()
	if err != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	defer rows.Close()
	for rows.Next() {
		job := models.JobModel{}
		var dateString string
		err = rows.Scan(
			&job.ID,
			&job.ProjectID,
			&job.Spec,
			&job.CallbackUrl,
			&job.ExecutionType,
			&dateString,
			&job.Data,
		)
		t, errParse := dateparse.ParseLocal(dateString)
		if errParse != nil {
			return nil, utils.HTTPGenericError(500, errParse.Error())
		}
		job.DateCreated = t
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
func (jobRepo *jobRepo) UpdateOneByID(jobModel models.JobModel) (int64, *utils.GenericError) {
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
		return -1, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	res, applyErr := fsm.AppApply(jobRepo.logger, jobRepo.store.Raft, constants.CommandTypeDbExecute, query, params)
	if err != nil {
		return -1, applyErr
	}

	if res == nil {
		return -1, utils.HTTPGenericError(http.StatusServiceUnavailable, "service is unavailable")
	}

	count := res.Data[1].(int64)

	return count, nil
}

// DeleteOneByID deletes a job with uuid and returns number of affected row
func (jobRepo *jobRepo) DeleteOneByID(jobModel models.JobModel) (int64, *utils.GenericError) {
	deleteQuery := sq.Delete(JobsTableName).Where(fmt.Sprintf("%s = ?", JobsIdColumn), jobModel.ID)

	query, params, err := deleteQuery.ToSql()
	if err != nil {
		return -1, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	res, applyErr := fsm.AppApply(jobRepo.logger, jobRepo.store.Raft, constants.CommandTypeDbExecute, query, params)
	if err != nil {
		return -1, applyErr
	}

	if res == nil {
		return -1, utils.HTTPGenericError(http.StatusServiceUnavailable, "service is unavailable")
	}

	count := res.Data[1].(int64)

	return count, nil
}

// GetJobsTotalCount returns total number of jobs
func (jobRepo *jobRepo) GetJobsTotalCount() (int64, *utils.GenericError) {
	countQuery := sq.Select("count(*)").From(JobsTableName).RunWith(jobRepo.store.SQLDbConnection)
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

	return int64(count), nil
}

// GetJobsTotalCountByProjectID returns the number of jobs for project with uuid
func (jobRepo *jobRepo) GetJobsTotalCountByProjectID(projectID int64) (int64, *utils.GenericError) {
	countQuery := sq.Select("count(*)").
		From(JobsTableName).
		Where(fmt.Sprintf("%s = ?", ProjectIdColumn), projectID).
		RunWith(jobRepo.store.SQLDbConnection)
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
		return -1, utils.HTTPGenericError(http.StatusInternalServerError, rows.Err().Error())
	}

	return int64(count), nil
}

// GetJobsPaginated returns a set of jobs starting at offset with the limit
func (jobRepo *jobRepo) GetJobsPaginated(projectID int64, offset int64, limit int64) ([]models.JobModel, int64, *utils.GenericError) {
	total, err := jobRepo.GetJobsTotalCountByProjectID(projectID)

	if err != nil {
		return nil, 0, err
	}

	jobRepos, err := jobRepo.GetAllByProjectID(projectID, offset, limit, "date_created")
	if err != nil {
		return nil, total, err
	}

	return jobRepos, int64(len(jobRepos)), nil
}

// BatchInsertJobs inserts n number of jobs
func (jobRepo *jobRepo) BatchInsertJobs(jobRepos []models.JobModel) ([]int64, *utils.GenericError) {
	batches := make([][]models.JobModel, 0)

	if len(jobRepos) > 100 {
		temp := make([]models.JobModel, 0)
		count := 0
		for count < len(jobRepos) {
			temp = append(temp, jobRepos[count])
			if len(temp) == 100 {
				batches = append(batches, temp)
				temp = make([]models.JobModel, 0)
			}
			count += 1
		}
		if len(temp) > 0 {
			batches = append(batches, temp)
			temp = make([]models.JobModel, 0)
		}
	} else {
		batches = append(batches, jobRepos)
	}

	returningIds := []int64{}

	for _, batchRepo := range batches {
		query := fmt.Sprintf("INSERT INTO jobs (%s, %s, %s, %s, %s, %s) VALUES ",
			ProjectIdColumn,
			SpecColumn,
			CallbackURLColumn,
			ExecutionTypeColumn,
			JobsDateCreatedColumn,
			DataColumn,
		)
		params := []interface{}{}
		ids := []int64{}

		for i, job := range batchRepo {
			query += fmt.Sprint("(?, ?, ?, ?, ?, ?)")
			job.DateCreated = time.Now().UTC()
			params = append(params,
				job.ProjectID,
				job.Spec,
				job.CallbackUrl,
				job.ExecutionType,
				job.DateCreated,
				job.Data,
			)

			if i < len(batchRepo)-1 {
				query += ","
			}
		}

		query += ";"

		res, applyErr := fsm.AppApply(jobRepo.logger, jobRepo.store.Raft, constants.CommandTypeDbExecute, query, params)
		if res == nil {
			return nil, utils.HTTPGenericError(http.StatusServiceUnavailable, "service is unavailable")
		}

		if applyErr != nil {
			return nil, applyErr
		}

		lastInsertedId := res.Data[0].(int64)

		for i := lastInsertedId - int64(len(batchRepo)) + 1; i <= lastInsertedId; i++ {
			ids = append(ids, i)
		}

		returningIds = append(returningIds, ids...)
	}

	return returningIds, nil
}
