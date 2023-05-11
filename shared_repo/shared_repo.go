package shared_repo

import (
	"context"
	"fmt"
	"github.com/hashicorp/go-hclog"
	"scheduler0/config"
	"scheduler0/db"
	"scheduler0/models"
	"scheduler0/utils"
)

const (
	JobQueuesTableName        = "job_queues"
	JobQueueNodeIdColumn      = "node_id"
	JobQueueLowerBoundJobId   = "lower_bound_job_id"
	JobQueueUpperBound        = "upper_bound_job_id"
	JobQueueDateCreatedColumn = "date_created"
	JobQueueVersion           = "version"

	ExecutionsUnCommittedTableName    = "job_executions_uncommitted"
	ExecutionsTableName               = "job_executions_committed"
	ExecutionsUniqueIdColumn          = "unique_id"
	ExecutionsStateColumn             = "state"
	ExecutionsNodeIdColumn            = "node_id"
	ExecutionsLastExecutionTimeColumn = "last_execution_time"
	ExecutionsNextExecutionTime       = "next_execution_time"
	ExecutionsJobIdColumn             = "job_id"
	ExecutionsDateCreatedColumn       = "date_created"
	ExecutionsJobQueueVersion         = "job_queue_version"
	ExecutionsVersion                 = "execution_version"
)

const (
	CommittedAsyncTableName   = "async_tasks_committed"
	UnCommittedAsyncTableName = "async_tasks_uncommitted"
)

const (
	AsyncTasksIdColumn          = "id"
	AsyncTasksRequestIdColumn   = "request_id"
	AsyncTasksInputColumn       = "input"
	AsyncTasksOutputColumn      = "output"
	AsyncTasksStateColumn       = "state"
	AsyncTasksServiceColumn     = "service"
	AsyncTasksDateCreatedColumn = "date_created"
)

//go:generate mockery --name SharedRepo --output ../mocks
type SharedRepo interface {
	GetUncommittedExecutionLogs(db db.DataStore) ([]models.JobExecutionLog, error)
	InsertExecutionLogs(db db.DataStore, jobExecutionLogs []models.JobExecutionLog) error
	InsertAsyncTasksLogs(db db.DataStore, asyncTasks []models.AsyncTask, committed bool) error
	DeleteAsyncTasksLogs(db db.DataStore, asyncTasks []models.AsyncTask) error
	DeleteExecutionLogs(db db.DataStore, jobExecutionLogs []models.JobExecutionLog) error
}

type sharedRepo struct {
	logger           hclog.Logger
	scheduler0Config config.Scheduler0Config
}

func NewSharedRepo(logger hclog.Logger, scheduler0Config config.Scheduler0Config) SharedRepo {
	return &sharedRepo{
		logger:           logger,
		scheduler0Config: scheduler0Config,
	}
}

func (repo *sharedRepo) GetUncommittedExecutionLogs(db db.DataStore) ([]models.JobExecutionLog, error) {
	db.ConnectionLock()
	defer db.ConnectionUnlock()

	var executionLogs []models.JobExecutionLog

	configs := repo.scheduler0Config.GetConfigurations()

	rows, err := db.GetOpenConnection().Query(fmt.Sprintf(
		"select count(*) from %s",
		ExecutionsUnCommittedTableName,
	), configs.NodeId, false)
	if err != nil {
		repo.logger.Error("failed to query for the count of uncommitted logs", "error", err.Error())
		return nil, err
	}
	var count int64 = 0
	for rows.Next() {
		scanErr := rows.Scan(&count)
		if scanErr != nil {
			repo.logger.Error("failed to scan count value", "error", scanErr.Error())
			return nil, scanErr
		}
	}
	if rows.Err() != nil {
		repo.logger.Error("rows error", "error", rows.Err())
		return nil, rows.Err()
	}
	err = rows.Close()
	if err != nil {
		repo.logger.Error("failed to close rows", "error", err)
		return nil, err
	}

	rows, err = db.GetOpenConnection().Query(fmt.Sprintf(
		"select max(id) as maxId, min(id) as minId from %s",
		ExecutionsUnCommittedTableName,
	), configs.NodeId, false)
	if err != nil {
		repo.logger.Error("failed to query for max and min id in uncommitted logs", "error", err.Error())
		return nil, err
	}

	repo.logger.Debug(fmt.Sprintf("found %d uncommitted logs", count))

	if count < 1 {
		return executionLogs, nil
	}

	var maxId int64 = 0
	var minId int64 = 0
	for rows.Next() {
		scanErr := rows.Scan(&maxId, &minId)
		if scanErr != nil {
			repo.logger.Error("failed to scan max and min id  on uncommitted logs", "error", scanErr.Error())
			return nil, scanErr
		}
	}
	if rows.Err() != nil {
		repo.logger.Error("rows error", "error", rows.Err())
		return nil, rows.Err()
	}
	err = rows.Close()
	if err != nil {
		repo.logger.Error("failed to close rows", "error", err)
		return nil, err
	}

	repo.logger.Debug(fmt.Sprintf("found max id %d and min id %d in uncommitted jobs \n", maxId, minId))

	ids := []int64{}
	for i := minId; i <= maxId; i++ {
		ids = append(ids, i)
	}

	batches := utils.Batch[int64](ids, 7)

	for _, batch := range batches {
		batchIds := []interface{}{batch[0]}
		params := "?"

		for _, id := range batch[1:] {
			batchIds = append(batchIds, id)
			params += ",?"
		}

		rows, err = db.GetOpenConnection().Query(fmt.Sprintf(
			"select  %s, %s, %s, %s, %s, %s, %s from %s where id in (%s)",
			ExecutionsUniqueIdColumn,
			ExecutionsStateColumn,
			ExecutionsNodeIdColumn,
			ExecutionsLastExecutionTimeColumn,
			ExecutionsNextExecutionTime,
			ExecutionsJobIdColumn,
			ExecutionsDateCreatedColumn,
			ExecutionsUnCommittedTableName,
			params,
		), batchIds...)
		if err != nil {
			repo.logger.Error("failed to query for the uncommitted logs", "error", err.Error())
			return nil, err
		}
		for rows.Next() {
			var jobExecutionLog models.JobExecutionLog
			scanErr := rows.Scan(
				&jobExecutionLog.UniqueId,
				&jobExecutionLog.State,
				&jobExecutionLog.NodeId,
				&jobExecutionLog.LastExecutionDatetime,
				&jobExecutionLog.NextExecutionDatetime,
				&jobExecutionLog.JobId,
				&jobExecutionLog.DataCreated,
			)
			if scanErr != nil {
				repo.logger.Error("failed to scan job execution columns", "error", scanErr.Error())
				return nil, scanErr
			}
			executionLogs = append(executionLogs, jobExecutionLog)
		}
		err = rows.Close()
		if err != nil {
			repo.logger.Error("failed to close rows", "error", err)
			return nil, err
		}
	}

	return executionLogs, nil
}

func (repo *sharedRepo) InsertExecutionLogs(db db.DataStore, jobExecutionLogs []models.JobExecutionLog) error {
	db.ConnectionLock()
	defer db.ConnectionUnlock()

	executionLogsBatches := utils.Batch[models.JobExecutionLog](jobExecutionLogs, 9)

	for _, executionLogsBatch := range executionLogsBatches {
		query := fmt.Sprintf("INSERT INTO %s (%s, %s, %s, %s, %s, %s, %s, %s, %s) VALUES ",
			ExecutionsUnCommittedTableName,
			ExecutionsUniqueIdColumn,
			ExecutionsStateColumn,
			ExecutionsNodeIdColumn,
			ExecutionsLastExecutionTimeColumn,
			ExecutionsNextExecutionTime,
			ExecutionsJobIdColumn,
			ExecutionsDateCreatedColumn,
			ExecutionsJobQueueVersion,
			ExecutionsVersion,
		)

		query += "(?, ?, ?, ?, ?, ?, ?, ?, ?)"
		params := []interface{}{
			executionLogsBatch[0].UniqueId,
			executionLogsBatch[0].State,
			executionLogsBatch[0].NodeId,
			executionLogsBatch[0].LastExecutionDatetime,
			executionLogsBatch[0].NextExecutionDatetime,
			executionLogsBatch[0].JobId,
			executionLogsBatch[0].DataCreated,
			executionLogsBatch[0].JobQueueVersion,
			executionLogsBatch[0].ExecutionVersion,
		}

		for _, executionLog := range executionLogsBatch[1:] {
			params = append(params,
				executionLog.UniqueId,
				executionLog.State,
				executionLog.NodeId,
				executionLog.LastExecutionDatetime,
				executionLog.NextExecutionDatetime,
				executionLog.JobId,
				executionLog.DataCreated,
				executionLog.JobQueueVersion,
				executionLog.ExecutionVersion,
			)
			query += ",(?, ?, ?, ?, ?, ?, ?, ?, ?)"
		}

		query += ";"

		ctx := context.Background()
		tx, err := db.GetOpenConnection().BeginTx(ctx, nil)
		if err != nil {
			repo.logger.Error("failed to create transaction for batch insertion", "error", err)
			return err
		}
		_, err = tx.Exec(query, params...)
		if err != nil {
			trxErr := tx.Rollback()
			if trxErr != nil {
				repo.logger.Error("failed to rollback update transition", "error", trxErr)
			}
			repo.logger.Error("failed to insert un committed executions to recovery db", "error", err)
			return err
		}
		err = tx.Commit()
		if err != nil {
			repo.logger.Error("failed to commit transition", "error", err)
			return err
		}
	}

	return nil
}

func (repo *sharedRepo) InsertAsyncTasksLogs(db db.DataStore, asyncTasks []models.AsyncTask, committed bool) error {
	db.ConnectionLock()
	defer db.ConnectionUnlock()

	batches := utils.Batch[models.AsyncTask](asyncTasks, 6)

	table := CommittedAsyncTableName

	if !committed {
		table = UnCommittedAsyncTableName
	}

	for _, batch := range batches {
		query := fmt.Sprintf("INSERT INTO %s (%s, %s, %s, %s, %s, %s) VALUES (?, ?, ?, ?, ?, ?)",
			table,
			AsyncTasksRequestIdColumn,
			AsyncTasksInputColumn,
			AsyncTasksOutputColumn,
			AsyncTasksStateColumn,
			AsyncTasksServiceColumn,
			AsyncTasksDateCreatedColumn,
		)
		params := []interface{}{
			batch[0].RequestId,
			batch[0].Input,
			batch[0].Output,
			batch[0].State,
			batch[0].Service,
			batch[0].DateCreated,
		}

		for _, row := range batch[1:] {
			query += ",(?, ?, ?, ?, ?, ?)"
			params = append(params, row.RequestId, row.Input, row.Output, row.State, row.Service, row.DateCreated)
		}

		query += ";"

		_, err := db.GetOpenConnection().Exec(query, params...)
		if err != nil {
			repo.logger.Error("failed to insert uncommitted async tasks", "error", err.Error())
			return err
		}
	}

	return nil
}

func (repo *sharedRepo) DeleteAsyncTasksLogs(db db.DataStore, asyncTasks []models.AsyncTask) error {
	db.ConnectionLock()
	defer db.ConnectionUnlock()

	batches := utils.Batch[models.AsyncTask](asyncTasks, 1)

	for _, batch := range batches {
		paramPlaceholder := "?"
		params := []interface{}{
			batch[0].Id,
		}

		for _, asyncTask := range batch[1:] {
			paramPlaceholder += ",?"
			params = append(params, asyncTask.Id)
		}

		ctx := context.Background()
		tx, err := db.GetOpenConnection().BeginTx(ctx, nil)
		if err != nil {
			repo.logger.Error("failed to create transaction for delete", "error", err.Error())
			return err
		}
		query := fmt.Sprintf("DELETE FROM %s WHERE %s IN (%s)", UnCommittedAsyncTableName, AsyncTasksIdColumn, paramPlaceholder)
		_, err = tx.Exec(query, params...)
		if err != nil {
			trxErr := tx.Rollback()
			if trxErr != nil {
				repo.logger.Error("failed to rollback transaction for batch deletion of async tasks", "error", trxErr.Error())
				return trxErr
			}
			repo.logger.Error("batch async tasks delete failed", "error", err.Error())
			return err
		}
		err = tx.Commit()
		if err != nil {
			repo.logger.Error("failed to commit transaction for batch async task deletion", "error", err.Error())
			return err
		}
	}

	return nil
}

func (repo *sharedRepo) DeleteExecutionLogs(db db.DataStore, jobExecutionLogs []models.JobExecutionLog) error {
	db.ConnectionLock()
	defer db.ConnectionUnlock()

	batches := utils.Batch[models.JobExecutionLog](jobExecutionLogs, 9)

	for _, batch := range batches {
		query := fmt.Sprintf("INSERT INTO %s (%s, %s, %s, %s, %s, %s, %s, %s, %s) VALUES ",
			ExecutionsTableName,
			ExecutionsUniqueIdColumn,
			ExecutionsStateColumn,
			ExecutionsNodeIdColumn,
			ExecutionsLastExecutionTimeColumn,
			ExecutionsNextExecutionTime,
			ExecutionsJobIdColumn,
			ExecutionsDateCreatedColumn,
			ExecutionsJobQueueVersion,
			ExecutionsVersion,
		)

		query += "(?, ?, ?, ?, ?, ?, ?, ?, ?)"
		params := []interface{}{
			batch[0].UniqueId,
			batch[0].State,
			batch[0].NodeId,
			batch[0].LastExecutionDatetime,
			batch[0].NextExecutionDatetime,
			batch[0].JobId,
			batch[0].DataCreated,
			batch[0].JobQueueVersion,
			batch[0].ExecutionVersion,
		}

		for _, executionLog := range batch[1:] {
			params = append(params,
				executionLog.UniqueId,
				executionLog.State,
				executionLog.NodeId,
				executionLog.LastExecutionDatetime,
				executionLog.NextExecutionDatetime,
				executionLog.JobId,
				executionLog.DataCreated,
				executionLog.JobQueueVersion,
				executionLog.ExecutionVersion,
			)
			query += ",(?, ?, ?, ?, ?, ?, ?, ?, ?)"
		}

		query += ";"

		ctx := context.Background()
		tx, err := db.GetOpenConnection().BeginTx(ctx, nil)
		if err != nil {
			repo.logger.Error("failed to create transaction for execution logs batch insertion", err.Error())
			return err
		}

		_, err = tx.Exec(query, params...)
		if err != nil {
			trxErr := tx.Rollback()
			if trxErr != nil {
				repo.logger.Error("failed to rollback transaction to insert batch execution logs", "error", trxErr.Error())
				return trxErr
			}
			repo.logger.Error("failed to batch insert execution logs", "error", err.Error())
			return err
		}
		err = tx.Commit()
		if err != nil {
			repo.logger.Error("failed to commit transaction to insert batch execution logs", "error", err.Error())
			return err
		}
	}

	return nil
}
