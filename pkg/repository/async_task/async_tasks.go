package async_task

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"scheduler0/pkg/constants"
	"scheduler0/pkg/fsm"
	"scheduler0/pkg/models"
	"scheduler0/pkg/scheduler0time"
	"scheduler0/pkg/utils"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/hashicorp/go-hclog"
)

type asyncTasksRepo struct {
	context               context.Context
	fsmStore              fsm.Scheduler0RaftStore
	logger                hclog.Logger
	scheduler0RaftActions fsm.Scheduler0RaftActions
}

type AsyncTasksRepo interface {
	BatchInsert(tasks []models.AsyncTask, committed bool) ([]uint64, *utils.GenericError)
	RaftBatchInsert(tasks []models.AsyncTask, fromNodeId uint64) ([]uint64, *utils.GenericError)
	RaftUpdateTaskState(task models.AsyncTask, state models.AsyncTaskState, output string) *utils.GenericError
	UpdateTaskState(task models.AsyncTask, state models.AsyncTaskState, output string) *utils.GenericError
	GetTask(taskId uint64) (*models.AsyncTask, *utils.GenericError)
	GetAllTasks(committed bool) ([]models.AsyncTask, *utils.GenericError)
	GetTaskByRequestIdAndAccountId(requestId string, accountId uint64) (*models.AsyncTask, *utils.GenericError)
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
	startTime := time.Now()
	repo.logger.Info("batch inserting async tasks", "totalTasks", len(tasks), "committed", committed)

	repo.fsmStore.GetDataStore().ConnectionLock()
	defer repo.fsmStore.GetDataStore().ConnectionUnlock()

	batches := utils.Batch[models.AsyncTask](tasks, 7)
	repo.logger.Debug("batched async tasks for insertion", "totalTasks", len(tasks), "batchCount", len(batches), "batchSize", 7)

	results := make([]uint64, 0, len(tasks))

	schedulerTime := scheduler0time.GetSchedulerTime()
	now := schedulerTime.GetTime(time.Now())

	table := constants.CommittedAsyncTableName
	if !committed {
		table = constants.UnCommittedAsyncTableName
	}

	repo.logger.Debug("inserting async tasks into table", "table", table, "batchCount", len(batches))

	for batchIdx, batch := range batches {
		repo.logger.Debug("processing async tasks insertion batch", "table", table, "batchIndex", batchIdx+1, "totalBatches", len(batches), "batchSize", len(batch))

		query := fmt.Sprintf("INSERT INTO %s (%s, %s, %s, %s, %s, %s, %s) VALUES (?, ?, ?, ?, ?, ?, ?)",
			table,
			constants.AsyncTasksRequestIdColumn,
			constants.AsyncTasksInputColumn,
			constants.AsyncTasksOutputColumn,
			constants.AsyncTasksStateColumn,
			constants.AsyncTasksServiceColumn,
			constants.AsyncTasksDateCreatedColumn,
			constants.AsyncTasksAccountIdColumn,
		)
		params := []interface{}{
			batch[0].RequestId,
			batch[0].Input,
			batch[0].Output,
			0,
			batch[0].Service,
			now,
			batch[0].AccountId,
		}

		for _, row := range batch[1:] {
			query += ",(?, ?, ?, ?, ?, ?, ?)"
			params = append(params, row.RequestId, row.Input, row.Output, 0, row.Service, now, row.AccountId)
		}

		ids := make([]uint64, 0, len(batch))

		query += ";"

		res, err := repo.fsmStore.GetDataStore().GetOpenConnection().Exec(query, params...)
		if err != nil {
			repo.logger.Error("BatchInsert: failed to execute insert query", "error", err, "table", table, "batchSize", len(batch))
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
		}

		if res == nil {
			repo.logger.Error("BatchInsert: insert result is nil", "table", table, "batchSize", len(batch))
			return nil, utils.HTTPGenericError(http.StatusServiceUnavailable, "service is unavailable - batch insert insert result is nil")
		}

		lastInsertedId, err := res.LastInsertId()
		if err != nil {
			repo.logger.Error("BatchInsert: failed to get last inserted id", "error", err, "table", table)
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
		}
		for i := lastInsertedId - int64(len(batch)) + 1; i <= lastInsertedId; i++ {
			ids = append(ids, uint64(i))
		}

		results = append(results, ids...)
		repo.logger.Debug("async tasks batch inserted successfully", "table", table, "batchIndex", batchIdx+1, "batchSize", len(batch), "insertedIds", len(ids))
	}

	duration := time.Since(startTime)
	repo.logger.Info("batch insert async tasks completed", "table", table, "totalTasks", len(tasks), "insertedCount", len(results), "batchCount", len(batches), "duration", duration, "durationMs", duration.Milliseconds())
	return results, nil
}

func (repo *asyncTasksRepo) RaftBatchInsert(tasks []models.AsyncTask, fromNodeId uint64) ([]uint64, *utils.GenericError) {
	startTime := time.Now()
	repo.logger.Info("raft batch inserting async tasks", "totalTasks", len(tasks), "fromNodeId", fromNodeId)
	batches := utils.Batch[models.AsyncTask](tasks, 7)
	repo.logger.Debug("batched async tasks for raft insertion", "totalTasks", len(tasks), "batchCount", len(batches), "batchSize", 7)

	results := make([]uint64, 0, len(tasks))
	schedulerTime := scheduler0time.GetSchedulerTime()
	now := schedulerTime.GetTime(time.Now())

	table := constants.CommittedAsyncTableName

	for batchIdx, batch := range batches {
		repo.logger.Debug("processing async tasks raft insertion batch", "table", table, "batchIndex", batchIdx+1, "totalBatches", len(batches), "batchSize", len(batch))

		query := fmt.Sprintf("INSERT INTO %s (%s, %s, %s, %s, %s, %s, %s) VALUES (?, ?, ?, ?, ?, ?, ?)",
			table,
			constants.AsyncTasksRequestIdColumn,
			constants.AsyncTasksInputColumn,
			constants.AsyncTasksOutputColumn,
			constants.AsyncTasksStateColumn,
			constants.AsyncTasksServiceColumn,
			constants.AsyncTasksDateCreatedColumn,
			constants.AsyncTasksAccountIdColumn,
		)
		params := []interface{}{
			batch[0].RequestId,
			batch[0].Input,
			batch[0].Output,
			0,
			batch[0].Service,
			now,
			batch[0].AccountId,
		}
		for _, row := range batch[1:] {
			query += ",(?, ?, ?, ?, ?, ?, ?)"
			params = append(params, row.RequestId, row.Input, row.Output, 0, row.Service, now, row.AccountId)
		}

		ids := make([]uint64, 0, len(batch))

		query += ";"

		res, applyErr := repo.scheduler0RaftActions.WriteCommandToRaftLog(
			repo.fsmStore.GetRaft(),
			constants.CommandTypeDbExecute,
			query,
			params,
			[]uint64{fromNodeId},
			constants.CommandActionQueueJob)
		if applyErr != nil {
			repo.logger.Error("RaftBatchInsert: failed to write command to raft log", "error", applyErr, "fromNodeId", fromNodeId, "batchSize", len(batch))
			return nil, applyErr
		}

		if res == nil {
			repo.logger.Error("RaftBatchInsert: raft log result is nil", "fromNodeId", fromNodeId, "batchSize", len(batch))
			return nil, utils.HTTPGenericError(http.StatusServiceUnavailable, "service is unavailable - raft batch insert raft log result is nil")
		}

		lastInsertedId := uint64(res.Data.LastInsertedId)
		for i := lastInsertedId - uint64(len(batch)) + 1; i <= lastInsertedId; i++ {
			ids = append(ids, i)
		}

		results = append(results, ids...)
		repo.logger.Debug("async tasks raft batch inserted successfully", "batchIndex", batchIdx+1, "totalBatches", len(batches), "batchSize", len(batch), "insertedIds", len(ids))
	}

	duration := time.Since(startTime)
	repo.logger.Info("raft batch insert async tasks completed", "totalTasks", len(tasks), "insertedCount", len(results), "batchCount", len(batches), "fromNodeId", fromNodeId, "duration", duration, "durationMs", duration.Milliseconds())
	return results, nil
}

func (repo *asyncTasksRepo) RaftUpdateTaskState(task models.AsyncTask, state models.AsyncTaskState, output string) *utils.GenericError {
	startTime := time.Now()
	repo.logger.Info("raft updating task state", "taskId", task.Id, "requestId", task.RequestId, "newState", state)
	schedulerTime := scheduler0time.GetSchedulerTime()
	now := schedulerTime.GetTime(time.Now())

	updateQuery := sq.Update(constants.CommittedAsyncTableName).
		Set(constants.AsyncTasksStateColumn, state).
		Set(constants.AsyncTasksOutputColumn, output).
		Set(constants.AsyncTasksDateModifiedColumn, now).
		Where(fmt.Sprintf("%s = ?", constants.AsyncTasksIdColumn), task.Id)

	query, params, err := updateQuery.ToSql()
	if err != nil {
		repo.logger.Error("RaftUpdateTaskState: failed to build update query", "error", err, "taskId", task.Id, "state", state)
		return utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	_, applyErr := repo.scheduler0RaftActions.WriteCommandToRaftLog(repo.fsmStore.GetRaft(), constants.CommandTypeDbExecute, query, params, []uint64{}, 0)
	if applyErr != nil {
		repo.logger.Error("RaftUpdateTaskState: failed to write command to raft log", "error", applyErr, "taskId", task.Id, "state", state)
		return applyErr
	}

	duration := time.Since(startTime)
	repo.logger.Info("raft update task state completed", "taskId", task.Id, "requestId", task.RequestId, "newState", state, "duration", duration, "durationMs", duration.Milliseconds())
	return nil
}

func (repo *asyncTasksRepo) UpdateTaskState(task models.AsyncTask, state models.AsyncTaskState, output string) *utils.GenericError {
	startTime := time.Now()
	repo.logger.Info("updating task state", "taskId", task.Id, "requestId", task.RequestId, "newState", state)

	repo.fsmStore.GetDataStore().ConnectionLock()
	defer repo.fsmStore.GetDataStore().ConnectionUnlock()

	schedulerTime := scheduler0time.GetSchedulerTime()
	now := schedulerTime.GetTime(time.Now())

	updateQuery := sq.Update(constants.UnCommittedAsyncTableName).
		Set(constants.AsyncTasksStateColumn, state).
		Set(constants.AsyncTasksOutputColumn, output).
		Set(constants.AsyncTasksDateModifiedColumn, now).
		Where(fmt.Sprintf("%s = ?", constants.AsyncTasksIdColumn), task.Id)

	query, params, err := updateQuery.ToSql()
	if err != nil {
		repo.logger.Error("UpdateTaskState: failed to build update query", "error", err, "taskId", task.Id, "state", state)
		return utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	_, applyErr := repo.fsmStore.GetDataStore().GetOpenConnection().Exec(query, params...)
	if applyErr != nil {
		repo.logger.Error("UpdateTaskState: failed to execute update query", "error", applyErr, "taskId", task.Id, "state", state)
		return utils.HTTPGenericError(http.StatusInternalServerError, applyErr.Error())
	}

	duration := time.Since(startTime)
	repo.logger.Info("update task state completed", "taskId", task.Id, "requestId", task.RequestId, "newState", state, "duration", duration, "durationMs", duration.Milliseconds())
	return nil
}

func (repo *asyncTasksRepo) GetTask(taskId uint64) (*models.AsyncTask, *utils.GenericError) {
	repo.fsmStore.GetDataStore().ConnectionLock()
	defer repo.fsmStore.GetDataStore().ConnectionUnlock()

	query := fmt.Sprintf(
		"select %s, %s, %s, %s, %s, %s, %s, %s, %s from %s where %s = ? union select %s, %s, %s, %s, %s, %s, %s, %s, %s from %s where %s = ?",
		constants.AsyncTasksIdColumn,
		constants.AsyncTasksRequestIdColumn,
		constants.AsyncTasksInputColumn,
		constants.AsyncTasksOutputColumn,
		constants.AsyncTasksStateColumn,
		constants.AsyncTasksServiceColumn,
		constants.AsyncTasksDateCreatedColumn,
		constants.AsyncTasksAccountIdColumn,
		constants.AsyncTasksDateModifiedColumn,
		constants.CommittedAsyncTableName,
		constants.AsyncTasksIdColumn,
		constants.AsyncTasksIdColumn,
		constants.AsyncTasksRequestIdColumn,
		constants.AsyncTasksInputColumn,
		constants.AsyncTasksOutputColumn,
		constants.AsyncTasksStateColumn,
		constants.AsyncTasksServiceColumn,
		constants.AsyncTasksDateCreatedColumn,
		constants.AsyncTasksAccountIdColumn,
		constants.AsyncTasksDateModifiedColumn,
		constants.UnCommittedAsyncTableName,
		constants.AsyncTasksIdColumn,
	)

	rows, err := repo.fsmStore.GetDataStore().GetOpenConnection().Query(query, taskId, taskId)
	if err != nil {
		repo.logger.Error("GetTask: failed to query task", "error", err, "taskId", taskId)
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	defer rows.Close()
	var asyncTask models.AsyncTask
	for rows.Next() {
		var dateModified sql.NullTime
		scanErr := rows.Scan(
			&asyncTask.Id,
			&asyncTask.RequestId,
			&asyncTask.Input,
			&asyncTask.Output,
			&asyncTask.State,
			&asyncTask.Service,
			&asyncTask.DateCreated,
			&asyncTask.AccountId,
			&dateModified,
		)
		if scanErr != nil {
			repo.logger.Error("GetTask: failed to scan task row", "error", scanErr, "taskId", taskId)
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, scanErr.Error())
		}
		if dateModified.Valid {
			asyncTask.DateModified = dateModified.Time
		} else {
			// If DateModified is NULL, use DateCreated as fallback
			asyncTask.DateModified = asyncTask.DateCreated
			repo.logger.Debug("DateModified is NULL, using DateCreated as fallback", "taskId", taskId)
		}
	}
	if rows.Err() != nil {
		repo.logger.Error("GetTask: row iteration error", "error", rows.Err(), "taskId", taskId)
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, rows.Err().Error())
	}
	return &asyncTask, nil
}

func (repo *asyncTasksRepo) countAsyncTasks(committed bool) uint64 {
	tableName := constants.UnCommittedAsyncTableName

	if committed {
		tableName = constants.CommittedAsyncTableName
	}

	selectBuilder := sq.Select("count(*)").
		From(tableName).
		RunWith(repo.fsmStore.GetDataStore().GetOpenConnection())

	rows, err := selectBuilder.Query()
	if err != nil {
		repo.logger.Error("countAsyncTasks: failed to count async tasks rows", "error", err)
		return 0
	}
	var count uint64 = 0
	for rows.Next() {
		scanErr := rows.Scan(&count)
		if scanErr != nil {
			repo.logger.Error("countAsyncTasks: failed to scan rows", "error", scanErr)
			return 0
		}
	}
	if rows.Err() != nil {
		repo.logger.Error("countAsyncTasks: failed to count async tasks rows error", "error", rows.Err())
		return 0
	}
	return count
}

func (repo *asyncTasksRepo) getAsyncTasksMinMaxIds(committed bool) (uint64, uint64) {
	tableName := constants.UnCommittedAsyncTableName

	if committed {
		tableName = constants.CommittedAsyncTableName
	}

	selectBuilder := sq.Select("min(id)", "max(id)").
		From(tableName).
		RunWith(repo.fsmStore.GetDataStore().GetOpenConnection())

	rows, err := selectBuilder.Query()
	if err != nil {
		repo.logger.Error("getAsyncTasksMinMaxIds: failed to count async tasks rows", "error", err)
		return 0, 0
	}
	defer rows.Close()

	var minID, maxID sql.NullInt64
	for rows.Next() {
		scanErr := rows.Scan(&minID, &maxID)
		if scanErr != nil {
			repo.logger.Error("getAsyncTasksMinMaxIds: failed to scan rows", "error", scanErr)
			return 0, 0
		}
	}
	if rows.Err() != nil {
		repo.logger.Error("getAsyncTasksMinMaxIds: failed to count async tasks rows error", "error", rows.Err())
		return 0, 0
	}

	// No tasks yet
	if !minID.Valid || !maxID.Valid {
		return 0, 0
	}

	return uint64(minID.Int64), uint64(maxID.Int64)
}

func (repo *asyncTasksRepo) GetAllTasks(committed bool) ([]models.AsyncTask, *utils.GenericError) {
	repo.fsmStore.GetDataStore().ConnectionLock()
	defer repo.fsmStore.GetDataStore().ConnectionUnlock()

	table := constants.CommittedAsyncTableName
	if !committed {
		table = constants.UnCommittedAsyncTableName
	}

	min, max := repo.getAsyncTasksMinMaxIds(committed)
	count := repo.countAsyncTasks(committed)

	// No tasks to return
	if count == 0 || (min == 0 && max == 0) {
		return []models.AsyncTask{}, nil
	}

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
			"select %s, %s, %s, %s, %s, %s, %s, %s, %s from %s where id in (%s)",
			constants.AsyncTasksIdColumn,
			constants.AsyncTasksRequestIdColumn,
			constants.AsyncTasksInputColumn,
			constants.AsyncTasksOutputColumn,
			constants.AsyncTasksStateColumn,
			constants.AsyncTasksServiceColumn,
			constants.AsyncTasksDateCreatedColumn,
			constants.AsyncTasksAccountIdColumn,
			constants.AsyncTasksDateModifiedColumn,
			table,
			paramPlaceholders,
		)
		rows, err := repo.fsmStore.GetDataStore().GetOpenConnection().Query(query, params...)
		if err != nil {
			repo.logger.Error("GetAllTasks: failed to query tasks", "error", err, "committed", committed, "batchSize", len(batch))
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
		}
		for rows.Next() {
			var asyncTask models.AsyncTask
			var dateModified sql.NullTime
			scanErr := rows.Scan(
				&asyncTask.Id,
				&asyncTask.RequestId,
				&asyncTask.Input,
				&asyncTask.Output,
				&asyncTask.State,
				&asyncTask.Service,
				&asyncTask.DateCreated,
				&asyncTask.AccountId,
				&dateModified,
			)
			if scanErr != nil {
				repo.logger.Error("GetAllTasks: failed to scan task row", "error", scanErr, "committed", committed)
				return nil, utils.HTTPGenericError(http.StatusInternalServerError, scanErr.Error())
			}
			if dateModified.Valid {
				asyncTask.DateModified = dateModified.Time
			} else {
				// If DateModified is NULL, use DateCreated as fallback
				asyncTask.DateModified = asyncTask.DateCreated
			}
			results = append(results, asyncTask)
		}
		if rows.Err() != nil {
			repo.logger.Error("GetAllTasks: row iteration error", "error", rows.Err(), "committed", committed)
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, rows.Err().Error())
		}
		closeErr := rows.Close()
		if closeErr != nil {
			repo.logger.Error("GetAllTasks: failed to close rows", "error", closeErr, "committed", committed)
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, closeErr.Error())
		}
	}

	return results, nil
}

func (repo *asyncTasksRepo) GetTaskByRequestIdAndAccountId(requestId string, accountId uint64) (*models.AsyncTask, *utils.GenericError) {
	repo.fsmStore.GetDataStore().ConnectionLock()
	defer repo.fsmStore.GetDataStore().ConnectionUnlock()

	query := fmt.Sprintf(
		"select %s, %s, %s, %s, %s, %s, %s, %s, %s from %s where %s = ? and %s = ? union select %s, %s, %s, %s, %s, %s, %s, %s, %s from %s where %s = ? and %s = ?",
		constants.AsyncTasksIdColumn,
		constants.AsyncTasksRequestIdColumn,
		constants.AsyncTasksInputColumn,
		constants.AsyncTasksOutputColumn,
		constants.AsyncTasksStateColumn,
		constants.AsyncTasksServiceColumn,
		constants.AsyncTasksDateCreatedColumn,
		constants.AsyncTasksAccountIdColumn,
		constants.AsyncTasksDateModifiedColumn,
		constants.CommittedAsyncTableName,
		constants.AsyncTasksRequestIdColumn,
		constants.AsyncTasksAccountIdColumn,
		constants.AsyncTasksIdColumn,
		constants.AsyncTasksRequestIdColumn,
		constants.AsyncTasksInputColumn,
		constants.AsyncTasksOutputColumn,
		constants.AsyncTasksStateColumn,
		constants.AsyncTasksServiceColumn,
		constants.AsyncTasksDateCreatedColumn,
		constants.AsyncTasksAccountIdColumn,
		constants.AsyncTasksDateModifiedColumn,
		constants.UnCommittedAsyncTableName,
		constants.AsyncTasksRequestIdColumn,
		constants.AsyncTasksAccountIdColumn,
	)

	repo.logger.Info("GetTaskByRequestIdAndAccountId: getting task by request id and account id", "requestId", requestId, "accountId", accountId)
	rows, err := repo.fsmStore.GetDataStore().GetOpenConnection().Query(query, requestId, accountId, requestId, accountId)
	if err != nil {
		repo.logger.Error("GetTaskByRequestIdAndAccountId: failed to query task", "error", err, "requestId", requestId, "accountId", accountId)
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	defer rows.Close()
	var asyncTask models.AsyncTask
	for rows.Next() {
		var dateModified sql.NullTime
		scanErr := rows.Scan(
			&asyncTask.Id,
			&asyncTask.RequestId,
			&asyncTask.Input,
			&asyncTask.Output,
			&asyncTask.State,
			&asyncTask.Service,
			&asyncTask.DateCreated,
			&asyncTask.AccountId,
			&dateModified,
		)
		if scanErr != nil {
			repo.logger.Error("GetTaskByRequestIdAndAccountId: failed to scan task row", "error", scanErr, "requestId", requestId, "accountId", accountId)
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, scanErr.Error())
		}
		if dateModified.Valid {
			asyncTask.DateModified = dateModified.Time
		} else {
			// If DateModified is NULL, use DateCreated as fallback
			asyncTask.DateModified = asyncTask.DateCreated
			repo.logger.Debug("DateModified is NULL, using DateCreated as fallback", "requestId", requestId, "accountId", accountId)
		}
	}
	if rows.Err() != nil {
		repo.logger.Error("GetTaskByRequestIdAndAccountId: row iteration error", "error", rows.Err(), "requestId", requestId, "accountId", accountId)
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, rows.Err().Error())
	}

	if asyncTask.Id == 0 {
		repo.logger.Warn("GetTaskByRequestIdAndAccountId: task not found", "requestId", requestId, "accountId", accountId)
		return nil, utils.HTTPGenericError(http.StatusNotFound, "task doesn't exist")
	}

	return &asyncTask, nil
}
