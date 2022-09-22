package repository

import (
	"fmt"
	sq "github.com/Masterminds/squirrel"
	"log"
	"net/http"
	"scheduler0/fsm"
	"scheduler0/models"
	"scheduler0/utils"
	"strings"
	"time"
)

type Execution interface {
	GetOneByID(execution *models.ExecutionModel) (int64, *utils.GenericError)
	ListByJobID(jobID int64, offset int64, limit int64, orderBy string) ([]models.ExecutionModel, *utils.GenericError)
	CountByJobID(jobID int64) (int64, *utils.GenericError)
	DeleteOneID(executionId string) (int64, *utils.GenericError)
	FindJobExecutionPlaceholderByID(jobID int64) (int64, *utils.GenericError, []models.ExecutionModel)
	BatchGetExecutions(executionIDs []int64) ([]models.ExecutionModel, *utils.GenericError)
	BatchGetExecutionPlaceholders(executionIDs []int64) ([]models.ExecutionModel, *utils.GenericError)
	BatchInsertExecutions(repos []models.ExecutionModel) ([]int64, *utils.GenericError)
	BatchUpdateExecutions(repos []models.ExecutionModel) *utils.GenericError
}

// ExecutionRepo this repo handles interacting with the database for all execution entity type
type executionRepo struct {
	store *fsm.Store
}

const (
	executionsTableName = "executions"
)

const (
	ExecutionsIdColumn          = "id"
	JobIdColumn                 = "job_id"
	TimeAddedColumn             = "time_added"
	TimeExecutedColumn          = "time_executed"
	ExecutionTimeColumn         = "execution_time"
	StatusCodeColumn            = "status_code"
	ExecutionsDateCreatedColumn = "date_created"
)

func NewExecutionRepo(store *fsm.Store) Execution {
	return &executionRepo{
		store: store,
	}
}

// GetOneByID returns a executions with the UUID
func (executionRepo *executionRepo) GetOneByID(execution *models.ExecutionModel) (int64, *utils.GenericError) {
	selectBuilder := sq.Select(
		ExecutionsIdColumn,
		JobIdColumn,
		TimeAddedColumn,
		TimeExecutedColumn,
		ExecutionTimeColumn,
		StatusCodeColumn,
		ExecutionsDateCreatedColumn,
	).
		From(executionsTableName).
		Where(fmt.Sprintf("%s = ?", ExecutionsIdColumn), execution.ID).
		RunWith(executionRepo.store.SQLDbConnection)

	rows, err := selectBuilder.Query()
	if err != nil {
		return -1, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		err = rows.Scan(
			&execution.ID,
			&execution.JobID,
			&execution.TimeAdded,
			&execution.TimeExecuted,
			&execution.ExecutionTime,
			&execution.StatusCode,
			&execution.DateCreated,
		)
		if err != nil {
			return -1, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
		}
		count += 1
	}
	if rows.Err() != nil {
		return int64(count), utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	return int64(count), nil
}

// ListByJobID returns a list of executions
func (executionRepo *executionRepo) ListByJobID(jobID int64, offset int64, limit int64, orderBy string) ([]models.ExecutionModel, *utils.GenericError) {
	selectBuilder := sq.Select(
		ExecutionsIdColumn,
		JobIdColumn,
		TimeAddedColumn,
		TimeExecutedColumn,
		ExecutionTimeColumn,
		StatusCodeColumn,
		ExecutionsDateCreatedColumn,
	).
		From(executionsTableName).
		Offset(uint64(offset)).
		Limit(uint64(limit)).
		OrderBy(orderBy).
		Where(fmt.Sprintf("%s = ?", JobIdColumn), jobID).
		RunWith(executionRepo.store.SQLDbConnection)

	rows, err := selectBuilder.Query()
	if err != nil {
		return nil, utils.HTTPGenericError(http.StatusNotFound, err.Error())
	}
	executions := []models.ExecutionModel{}
	defer rows.Close()
	for rows.Next() {
		execution := models.ExecutionModel{}
		err = rows.Scan(
			&execution.ID,
			&execution.JobID,
			&execution.TimeAdded,
			&execution.TimeExecuted,
			&execution.ExecutionTime,
			&execution.StatusCode,
			&execution.DateCreated,
		)
		if err != nil {
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
		}
		executions = append(executions, execution)
	}
	if rows.Err() != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	return executions, nil
}

// CountByJobID returns the total number of executions with JobID matching jobID
func (executionRepo *executionRepo) CountByJobID(jobID int64) (int64, *utils.GenericError) {
	countQuery := sq.Select("count(*)").
		From(executionsTableName).
		Where(fmt.Sprintf("%s = ?", JobIdColumn), jobID).
		RunWith(executionRepo.store.SQLDbConnection)
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

// DeleteOneID deletes a single execution entity from the database
func (executionRepo *executionRepo) DeleteOneID(executionId string) (int64, *utils.GenericError) {
	deleteQuery := sq.Delete(executionsTableName).Where(fmt.Sprintf("%s = ?", ExecutionsIdColumn), executionId).RunWith(executionRepo.store.SQLDbConnection)

	deletedRows, deleteErr := deleteQuery.Exec()
	if deleteErr != nil {
		return -1, utils.HTTPGenericError(http.StatusInternalServerError, deleteErr.Error())
	}

	deletedRowsCount, rowsAErr := deletedRows.RowsAffected()
	if rowsAErr != nil {
		return -1, utils.HTTPGenericError(http.StatusInternalServerError, rowsAErr.Error())
	}

	return deletedRowsCount, nil
}

// FindJobExecutionPlaceholderByID returns an execution placeholder for a job that's not been executed
func (executionRepo *executionRepo) FindJobExecutionPlaceholderByID(jobID int64) (int64, *utils.GenericError, []models.ExecutionModel) {
	selectBuilder := sq.Select(
		ExecutionsIdColumn,
		JobIdColumn,
		TimeAddedColumn,
		TimeExecutedColumn,
		ExecutionTimeColumn,
		StatusCodeColumn,
		ExecutionsDateCreatedColumn,
	).
		From(executionsTableName).
		Where(fmt.Sprintf("%s = ? AND %s is NULL", JobIdColumn, TimeExecutedColumn), jobID).
		RunWith(executionRepo.store.SQLDbConnection)

	rows, selectQueryErr := selectBuilder.Query()
	if selectQueryErr != nil {
		return -1, utils.HTTPGenericError(http.StatusInternalServerError, selectQueryErr.Error()), nil
	}
	defer rows.Close()
	executions := []models.ExecutionModel{}
	for rows.Next() {
		var execution models.ExecutionModel
		var timeExecuted *time.Time
		var executionTime *int64
		var statusCode *string
		rowsErr := rows.Scan(
			&execution.ID,
			&execution.JobID,
			&execution.TimeAdded,
			&timeExecuted,
			&executionTime,
			&statusCode,
			&execution.DateCreated,
		)
		if rowsErr != nil {
			return -1, utils.HTTPGenericError(http.StatusInternalServerError, rowsErr.Error()), nil
		}

		if timeExecuted != nil {
			execution.TimeExecuted = *timeExecuted
		}

		if executionTime != nil {
			execution.ExecutionTime = *executionTime
		}

		if statusCode != nil {
			execution.StatusCode = *statusCode
		}

		executions = append(executions, execution)
	}
	if rows.Err() != nil {
		return -1, utils.HTTPGenericError(http.StatusInternalServerError, rows.Err().Error()), nil
	}
	return int64(len(executions)), nil, executions
}

// BatchGetExecutions returns executions where job_uuid in jobUUIDs
func (executionRepo *executionRepo) BatchGetExecutions(executionIDs []int64) ([]models.ExecutionModel, *utils.GenericError) {
	paramPlaceholder := ""
	ids := []interface{}{}

	for i, id := range executionIDs {
		paramPlaceholder += "?"

		if i < len(executionIDs)-1 {
			paramPlaceholder += ","
		}

		ids = append(ids, id)
	}

	selectBuilder := sq.Select(
		ExecutionsIdColumn,
		JobIdColumn,
		TimeAddedColumn,
		TimeExecutedColumn,
		ExecutionTimeColumn,
		StatusCodeColumn,
		ExecutionsDateCreatedColumn,
	).
		From(executionsTableName).
		Where(fmt.Sprintf("%s IN (%s)", ExecutionsIdColumn, paramPlaceholder), ids...).
		RunWith(executionRepo.store.SQLDbConnection)

	rows, selectErr := selectBuilder.Query()
	if selectErr != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, selectErr.Error())
	}
	defer rows.Close()
	executions := []models.ExecutionModel{}
	for rows.Next() {
		var execution models.ExecutionModel
		var timeExecuted *time.Time
		var executionTime *int64
		var statusCode *string
		scanErr := rows.Scan(
			&execution.ID,
			&execution.JobID,
			&execution.TimeAdded,
			&timeExecuted,
			&executionTime,
			&statusCode,
			&execution.DateCreated,
		)

		if timeExecuted != nil {
			execution.TimeExecuted = *timeExecuted
		}

		if executionTime != nil {
			execution.ExecutionTime = *executionTime
		}

		if statusCode != nil {
			execution.StatusCode = *statusCode
		}

		if scanErr != nil {
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, scanErr.Error())
		}
		executions = append(executions, execution)
	}
	if rows.Err() != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, rows.Err().Error())
	}

	return executions, nil
}

// BatchGetExecutionPlaceholders returns execution placeholders where job_uuid in jobUUIDs
func (executionRepo *executionRepo) BatchGetExecutionPlaceholders(jobIDs []int64) ([]models.ExecutionModel, *utils.GenericError) {
	jobs := []models.ExecutionModel{}

	paramPlaceholder := ""
	ids := []interface{}{}

	for i, id := range jobIDs {
		paramPlaceholder += "?"

		if i < len(jobIDs)-1 {
			paramPlaceholder += ","
		}

		ids = append(ids, id)
	}

	selectBuilder := sq.Select(
		ExecutionsIdColumn,
		JobIdColumn,
		TimeAddedColumn,
		TimeExecutedColumn,
		ExecutionTimeColumn,
		StatusCodeColumn,
		ExecutionsDateCreatedColumn,
	).
		From(executionsTableName).
		Where(fmt.Sprintf("%s IN (%s) AND  %s is NULL", JobIdColumn, paramPlaceholder, TimeExecutedColumn), jobIDs).
		RunWith(executionRepo.store.SQLDbConnection)

	rows, selectErr := selectBuilder.Query()
	if selectErr != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, selectErr.Error())
	}
	defer rows.Close()
	executions := []models.ExecutionModel{}
	for rows.Next() {
		var execution models.ExecutionModel
		var timeExecuted *time.Time
		var executionTime *int64
		var statusCode *string
		scanErr := rows.Scan(
			&execution.ID,
			&execution.JobID,
			&execution.TimeAdded,
			&timeExecuted,
			&executionTime,
			&statusCode,
			&execution.DateCreated,
		)
		if scanErr != nil {
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, scanErr.Error())
		}
		if timeExecuted != nil {
			execution.TimeExecuted = *timeExecuted
		}

		if executionTime != nil {
			execution.ExecutionTime = *executionTime
		}

		if statusCode != nil {
			execution.StatusCode = *statusCode
		}
		executions = append(executions, execution)
	}
	if rows.Err() != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, rows.Err().Error())
	}

	return jobs, nil
}

// BatchInsertExecutions inserts executions in batch
func (executionRepo *executionRepo) BatchInsertExecutions(repos []models.ExecutionModel) ([]int64, *utils.GenericError) {
	batches := [][]models.ExecutionModel{}

	if len(repos) > 100 {
		temp := []models.ExecutionModel{}
		count := 0
		for count < len(repos) {
			temp = append(temp, repos[count])
			if len(temp) == 100 {
				batches = append(batches, temp)
				temp = []models.ExecutionModel{}
			}
			count += 1
		}
		if len(temp) > 0 {
			batches = append(batches, temp)
			temp = []models.ExecutionModel{}
		}
	} else {
		batches = append(batches, repos)
	}

	returningIds := []int64{}

	for _, repos := range batches {
		query := "INSERT INTO executions (job_id, time_added, date_created) VALUES "
		params := []interface{}{}
		ids := []int64{}

		for i, repo := range repos {
			query += fmt.Sprint("(?, ?, ?)")
			params = append(params,
				repo.JobID,
				repo.TimeAdded.Format(time.RFC3339),
				repo.DateCreated.Format(time.RFC3339),
			)

			if i < len(repos)-1 {
				query += ","
			}
		}

		query += ";"

		tx, beginError := executionRepo.store.SQLDbConnection.Begin()
		if beginError != nil {
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, beginError.Error())
		}

		res, err := tx.Exec(query, params...)
		if err != nil {
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
		}
		lastInsertedId, lastInsertedIdErr := res.LastInsertId()
		if lastInsertedIdErr != nil {
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, lastInsertedIdErr.Error())
		}
		commitErr := tx.Commit()
		if commitErr != nil {
			err := tx.Rollback()
			if err != nil {
				log.Fatalln("failed to rollback batch insert operation", err)
				return nil, nil
			}
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, commitErr.Error())
		}
		for i := lastInsertedId - int64(len(repos)) + 1; i <= lastInsertedId; i++ {
			ids = append(ids, i)
		}

		returningIds = append(returningIds, ids...)
	}

	return returningIds, nil
}

// BatchUpdateExecutions update execution placeholder for jobs in batch
func (executionRepo *executionRepo) BatchUpdateExecutions(repos []models.ExecutionModel) *utils.GenericError {
	batches := [][]models.ExecutionModel{}

	if len(repos) > 100 {
		temp := []models.ExecutionModel{}
		count := 0
		for count < len(repos) {
			temp = append(temp, repos[count])
			if len(temp) == 100 {
				batches = append(batches, temp)
				temp = []models.ExecutionModel{}
			}
			count += 1
		}
		if len(temp) > 0 {
			batches = append(batches, temp)
			temp = []models.ExecutionModel{}
		}
	} else {
		batches = append(batches, repos)
	}

	for _, repos := range batches {
		query := "UPDATE executions SET"

		params := []interface{}{}
		uuids_p := []string{}
		uuids := []int64{}

		for i, repo := range repos {
			query += " time_executed = CASE job_id WHEN ? THEN ? END,"
			query += " execution_time = CASE job_id WHEN ? THEN ? END,"
			query += " status_code = CASE job_id WHEN ? THEN ? END"

			params = append(params,
				repo.JobID,
				repo.TimeExecuted,
				repo.JobID,
				repo.ExecutionTime,
				repo.JobID,
				repo.StatusCode,
			)

			uuids_p = append(uuids_p, "?")
			uuids = append(uuids, repo.JobID)

			if i < len(repos)-1 {
				query += ","
			}
		}

		for _, uuid := range uuids {
			params = append(params, uuid)
		}

		transaction, transactionErr := executionRepo.store.SQLDbConnection.Begin()
		if transactionErr != nil {
			return utils.HTTPGenericError(http.StatusInternalServerError, transactionErr.Error())
		}

		query += " WHERE job_id IN (" + strings.Join(uuids_p, ",") + ") AND time_executed is NULL;"
		_, err := transaction.Exec(query, params...)
		if err != nil {
			return utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
		}
		transactionCommitErr := transaction.Commit()
		if transactionCommitErr != nil {
			return utils.HTTPGenericError(http.StatusInternalServerError, transactionCommitErr.Error())
		}
	}

	return nil
}
