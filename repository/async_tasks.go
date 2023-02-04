package repository

import (
	"context"
	"fmt"
	sq "github.com/Masterminds/squirrel"
	"log"
	"net/http"
	"scheduler0/constants"
	"scheduler0/fsm"
	"scheduler0/models"
	"scheduler0/utils"
	"scheduler0/utils/batcher"
	"time"
)

const (
	AsyncTableName = "async_tasks"
)

const (
	IdColumn          = "id"
	RequestIdColumn   = "request_id"
	InputColumn       = "input"
	OutputColumn      = "output"
	StateColumn       = "state"
	ServiceColumn     = "service"
	DateCreatedColumn = "date_created"
)

type asyncTasksRepo struct {
	context  context.Context
	fsmStore *fsm.Store
	logger   *log.Logger
}

type AsyncTasksRepo interface {
	BatchInsert(tasks []models.AsyncTask) ([]uint64, *utils.GenericError)
	UpdateTaskState(task models.AsyncTask, state models.AsyncTaskState, output string) *utils.GenericError
	GetTask(taskId uint64) (*models.AsyncTask, *utils.GenericError)
}

func NewAsyncTasksRepo(context context.Context, logger *log.Logger, fsmStore *fsm.Store) AsyncTasksRepo {
	return &asyncTasksRepo{
		context:  context,
		logger:   logger,
		fsmStore: fsmStore,
	}
}

func (repo *asyncTasksRepo) BatchInsert(tasks []models.AsyncTask) ([]uint64, *utils.GenericError) {
	batches := batcher.Batch[models.AsyncTask](tasks, 5)
	results := make([]uint64, 0, len(tasks))

	schedulerTime := utils.GetSchedulerTime()
	now := schedulerTime.GetTime(time.Now())

	for _, batch := range batches {
		query := fmt.Sprintf("INSERT INTO %s (%s, %s, %s, %s, %s, %s) VALUES (?, ?, ?, ?, ?, ?)", AsyncTableName, RequestIdColumn, InputColumn, OutputColumn, StateColumn, ServiceColumn, DateCreatedColumn)
		params := []interface{}{
			batch[0].RequestId,
			batch[0].Input,
			batch[0].Output,
			0,
			batch[0].Service,
			now,
		}

		for _, row := range batch[1:] {
			query += ",(?, ?, ?, ?, ?, ?)"
			params = append(params, row.RequestId, row.Input, row.Output, 0, now)
		}

		ids := make([]uint64, 0, len(batch))

		query += ";"

		res, applyErr := fsm.AppApply(repo.logger, repo.fsmStore.Raft, constants.CommandTypeDbExecute, query, params)
		if applyErr != nil {
			return nil, applyErr
		}

		if res == nil {
			return nil, utils.HTTPGenericError(http.StatusServiceUnavailable, "service is unavailable")
		}

		lastInsertedId := uint64(res.Data[0].(int64))
		for i := lastInsertedId - uint64(len(batch)) + 1; i <= lastInsertedId; i++ {
			ids = append(ids, i)
		}

		results = append(results, ids...)
	}

	return results, nil
}

func (repo *asyncTasksRepo) UpdateTaskState(task models.AsyncTask, state models.AsyncTaskState, output string) *utils.GenericError {
	updateQuery := sq.Update(AsyncTableName).
		Set(StateColumn, state).
		Set(OutputColumn, output).
		Where(fmt.Sprintf("%s = ?", IdColumn), task.Id)

	query, params, err := updateQuery.ToSql()
	if err != nil {
		return utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	res, applyErr := fsm.AppApply(repo.logger, repo.fsmStore.Raft, constants.CommandTypeDbExecute, query, params)
	if err != nil {
		return applyErr
	}

	if res == nil {
		return utils.HTTPGenericError(http.StatusServiceUnavailable, "service is unavailable")
	}

	return nil
}

func (repo *asyncTasksRepo) GetTask(taskId uint64) (*models.AsyncTask, *utils.GenericError) {
	repo.fsmStore.DataStore.ConnectionLock.Lock()
	defer repo.fsmStore.DataStore.ConnectionLock.Unlock()

	selectQuery := sq.Select(
		IdColumn,
		RequestIdColumn,
		InputColumn,
		OutputColumn,
		StateColumn,
		ServiceColumn,
		DateCreatedColumn,
	).
		From(AsyncTableName).
		Where(IdColumn, taskId).
		RunWith(repo.fsmStore.DataStore.Connection)

	rows, err := selectQuery.Query()
	if err != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	defer rows.Close()
	var asyncTask models.AsyncTask
	for rows.Next() {
		scanErr := rows.Scan(
			&asyncTask.Id,
			&asyncTask.RequestId,
			&asyncTask.Input,
			&asyncTask.Output,
			&asyncTask.State,
			&asyncTask.Service,
			&asyncTask.DateCreated,
		)
		if scanErr != nil {
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, scanErr.Error())
		}
	}
	if rows.Err() != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, rows.Err().Error())
	}
	return &asyncTask, nil
}
