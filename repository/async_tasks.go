package repository

import (
	"context"
	"fmt"
	sq "github.com/Masterminds/squirrel"
	"github.com/hashicorp/go-hclog"
	"net/http"
	"scheduler0/constants"
	"scheduler0/fsm"
	"scheduler0/models"
	"scheduler0/utils"
	"time"
)

const (
	CommittedAsyncTableName   = "async_tasks_committed"
	UnCommittedAsyncTableName = "async_tasks_uncommitted"
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
	context               context.Context
	fsmStore              fsm.Scheduler0RaftStore
	logger                hclog.Logger
	scheduler0RaftActions fsm.Scheduler0RaftActions
}

type AsyncTasksRepo interface {
	BatchInsert(tasks []models.AsyncTask, committed bool) ([]uint64, *utils.GenericError)
	RaftBatchInsert(tasks []models.AsyncTask) ([]uint64, *utils.GenericError)
	RaftUpdateTaskState(task models.AsyncTask, state models.AsyncTaskState, output string) *utils.GenericError
	UpdateTaskState(task models.AsyncTask, state models.AsyncTaskState, output string) *utils.GenericError
	GetTask(taskId uint64) (*models.AsyncTask, *utils.GenericError)
	GetAllUnCommittedTasks() ([]models.AsyncTask, *utils.GenericError)
}

func NewAsyncTasksRepo(context context.Context, logger hclog.Logger, scheduler0RaftActions fsm.Scheduler0RaftActions, fsmStore fsm.Scheduler0RaftStore) AsyncTasksRepo {
	return &asyncTasksRepo{
		context:               context,
		logger:                logger.Named("async-task-repo"),
		fsmStore:              fsmStore,
		scheduler0RaftActions: scheduler0RaftActions,
	}
}

func (repo *asyncTasksRepo) BatchInsert(tasks []models.AsyncTask, committed bool) ([]uint64, *utils.GenericError) {
	repo.fsmStore.GetDataStore().ConnectionLock()
	defer repo.fsmStore.GetDataStore().ConnectionUnlock()

	batches := utils.Batch[models.AsyncTask](tasks, 5)
	results := make([]uint64, 0, len(tasks))

	schedulerTime := utils.GetSchedulerTime()
	now := schedulerTime.GetTime(time.Now())

	table := CommittedAsyncTableName
	if !committed {
		table = UnCommittedAsyncTableName
	}

	for _, batch := range batches {
		query := fmt.Sprintf("INSERT INTO %s (%s, %s, %s, %s, %s, %s) VALUES (?, ?, ?, ?, ?, ?)", table, RequestIdColumn, InputColumn, OutputColumn, StateColumn, ServiceColumn, DateCreatedColumn)
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
			params = append(params, row.RequestId, row.Input, row.Output, 0, row.Service, now)
		}

		ids := make([]uint64, 0, len(batch))

		query += ";"

		res, err := repo.fsmStore.GetDataStore().GetOpenConnection().Exec(query, params...)
		if err != nil {
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
		}

		if res == nil {
			return nil, utils.HTTPGenericError(http.StatusServiceUnavailable, "service is unavailable")
		}

		lastInsertedId, err := res.LastInsertId()
		if err != nil {
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
		}
		for i := lastInsertedId - int64(len(batch)) + 1; i <= lastInsertedId; i++ {
			ids = append(ids, uint64(i))
		}

		results = append(results, ids...)
	}

	return results, nil
}

func (repo *asyncTasksRepo) RaftBatchInsert(tasks []models.AsyncTask) ([]uint64, *utils.GenericError) {
	batches := utils.Batch[models.AsyncTask](tasks, 5)
	results := make([]uint64, 0, len(tasks))

	schedulerTime := utils.GetSchedulerTime()
	now := schedulerTime.GetTime(time.Now())

	table := CommittedAsyncTableName

	for _, batch := range batches {
		query := fmt.Sprintf("INSERT INTO %s (%s, %s, %s, %s, %s, %s) VALUES (?, ?, ?, ?, ?, ?)", table, RequestIdColumn, InputColumn, OutputColumn, StateColumn, ServiceColumn, DateCreatedColumn)
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

		res, applyErr := repo.scheduler0RaftActions.WriteCommandToRaftLog(repo.fsmStore.GetRaft(), constants.CommandTypeDbExecute, query, 0, params)
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

func (repo *asyncTasksRepo) RaftUpdateTaskState(task models.AsyncTask, state models.AsyncTaskState, output string) *utils.GenericError {
	updateQuery := sq.Update(CommittedAsyncTableName).
		Set(StateColumn, state).
		Set(OutputColumn, output).
		Where(fmt.Sprintf("%s = ?", IdColumn), task.Id)

	query, params, err := updateQuery.ToSql()
	if err != nil {
		return utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	res, applyErr := repo.scheduler0RaftActions.WriteCommandToRaftLog(repo.fsmStore.GetRaft(), constants.CommandTypeDbExecute, query, 0, params)
	if err != nil {
		return applyErr
	}

	if res == nil {
		return utils.HTTPGenericError(http.StatusServiceUnavailable, "service is unavailable")
	}

	return nil
}

func (repo *asyncTasksRepo) UpdateTaskState(task models.AsyncTask, state models.AsyncTaskState, output string) *utils.GenericError {
	repo.fsmStore.GetDataStore().ConnectionLock()
	defer repo.fsmStore.GetDataStore().ConnectionUnlock()

	updateQuery := sq.Update(UnCommittedAsyncTableName).
		Set(StateColumn, state).
		Set(OutputColumn, output).
		Where(fmt.Sprintf("%s = ?", IdColumn), task.Id)

	query, params, err := updateQuery.ToSql()
	if err != nil {
		return utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	res, applyErr := repo.fsmStore.GetDataStore().GetOpenConnection().Exec(query, params...)
	if err != nil {
		return utils.HTTPGenericError(http.StatusInternalServerError, applyErr.Error())
	}

	if res == nil {
		return utils.HTTPGenericError(http.StatusServiceUnavailable, "service is unavailable")
	}

	return nil
}

func (repo *asyncTasksRepo) GetTask(taskId uint64) (*models.AsyncTask, *utils.GenericError) {
	repo.fsmStore.GetDataStore().ConnectionLock()
	defer repo.fsmStore.GetDataStore().ConnectionUnlock()

	query := fmt.Sprintf(
		"select %s, %s, %s, %s, %s, %s, %s from %s union all select %s, %s, %s, %s, %s, %s, %s from %s where %s = ?",
		IdColumn,
		RequestIdColumn,
		InputColumn,
		OutputColumn,
		StateColumn,
		ServiceColumn,
		DateCreatedColumn,
		CommittedAsyncTableName,
		IdColumn,
		RequestIdColumn,
		InputColumn,
		OutputColumn,
		StateColumn,
		ServiceColumn,
		DateCreatedColumn,
		UnCommittedAsyncTableName,
		IdColumn,
	)

	rows, err := repo.fsmStore.GetDataStore().GetOpenConnection().Query(query, taskId)
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

func (repo *asyncTasksRepo) countAsyncTasks(committed bool) uint64 {
	tableName := UnCommittedAsyncTableName

	if committed {
		tableName = CommittedAsyncTableName
	}

	selectBuilder := sq.Select("count(*)").
		From(tableName).
		RunWith(repo.fsmStore.GetDataStore().GetOpenConnection())

	rows, err := selectBuilder.Query()
	if err != nil {
		repo.logger.Error("failed to count async tasks rows", err)
		return 0
	}
	var count uint64 = 0
	for rows.Next() {
		scanErr := rows.Scan(&count)
		if err != nil {
			repo.logger.Error("failed to scan rows ", scanErr)
			return 0
		}
	}
	if rows.Err() != nil {
		repo.logger.Error("failed to count async tasks rows error", rows.Err())
		return 0
	}
	return count
}

func (repo *asyncTasksRepo) getAsyncTasksMinMaxIds(committed bool) (uint64, uint64) {
	tableName := UnCommittedAsyncTableName

	if committed {
		tableName = CommittedAsyncTableName
	}

	selectBuilder := sq.Select("min(id)", "max(id)").
		From(tableName).
		RunWith(repo.fsmStore.GetDataStore().GetOpenConnection())

	rows, err := selectBuilder.Query()
	if err != nil {
		repo.logger.Error("failed to count async tasks rows", err)
		return 0, 0
	}
	var minId uint64 = 0
	var maxId uint64 = 0
	for rows.Next() {
		scanErr := rows.Scan(&minId, &maxId)
		if err != nil {
			repo.logger.Error("failed to scan rows ", scanErr)
			return 0, 0
		}
	}
	if rows.Err() != nil {
		repo.logger.Error("failed to count async tasks rows error", rows.Err())
		return 0, 0
	}
	return minId, maxId
}

func (repo *asyncTasksRepo) GetAllUnCommittedTasks() ([]models.AsyncTask, *utils.GenericError) {
	repo.fsmStore.GetDataStore().ConnectionLock()
	defer repo.fsmStore.GetDataStore().ConnectionUnlock()

	min, max := repo.getAsyncTasksMinMaxIds(false)
	count := repo.countAsyncTasks(false)
	results := make([]models.AsyncTask, 0, count)
	expandedIds := utils.ExpandIdsRange(min, max)

	batches := utils.Batch(expandedIds, 7)

	for _, batch := range batches {
		var params = []interface{}{batch[0]}
		var paramPlaceholders = "?"

		for _, b := range batch[1:] {
			paramPlaceholders += ",?"
			params = append(params, b)
		}

		query := fmt.Sprintf(
			"select %s, %s, %s, %s, %s, %s, %s from %s where id in (%s)",
			IdColumn,
			RequestIdColumn,
			InputColumn,
			OutputColumn,
			StateColumn,
			ServiceColumn,
			DateCreatedColumn,
			UnCommittedAsyncTableName,
			paramPlaceholders,
		)
		rows, err := repo.fsmStore.GetDataStore().GetOpenConnection().Query(query, params...)
		if err != nil {
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
		}
		for rows.Next() {
			var asyncTask models.AsyncTask
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
			results = append(results, asyncTask)
		}
		if rows.Err() != nil {
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, rows.Err().Error())
		}
		closeErr := rows.Close()
		if closeErr != nil {
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, closeErr.Error())
		}
	}

	return results, nil
}
