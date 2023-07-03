package shared_repo

import (
	"context"
	"fmt"
	"github.com/hashicorp/go-hclog"
	"scheduler0/config"
	"scheduler0/constants"
	"scheduler0/db"
	"scheduler0/models"
	"scheduler0/utils"
)

//go:generate mockery --name SharedRepo --output ../mocks
type SharedRepo interface {
	GetExecutionLogs(db db.DataStore, committed bool) ([]models.JobExecutionLog, error)
	InsertExecutionLogs(db db.DataStore, committed bool, jobExecutionLogs []models.JobExecutionLog) error
	DeleteExecutionLogs(db db.DataStore, committed bool, jobExecutionLogs []models.JobExecutionLog) error
	InsertAsyncTasksLogs(db db.DataStore, committed bool, asyncTasks []models.AsyncTask) error
	DeleteAsyncTasksLogs(db db.DataStore, committed bool, asyncTasks []models.AsyncTask) error
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

func (repo *sharedRepo) GetExecutionLogs(db db.DataStore, committed bool) ([]models.JobExecutionLog, error) {
	db.ConnectionLock()
	defer db.ConnectionUnlock()

	var executionLogs []models.JobExecutionLog

	configs := repo.scheduler0Config.GetConfigurations()
	table := constants.ExecutionsUnCommittedTableName
	if committed {
		table = constants.ExecutionsCommittedTableName
	}

	rows, err := db.GetOpenConnection().Query(fmt.Sprintf(
		"select count(*) from %s",
		table,
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
		table,
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
			"select  %s, %s, %s, %s, %s, %s, %s, %s, %s from %s where id in (%s)",
			constants.ExecutionsUniqueIdColumn,
			constants.ExecutionsStateColumn,
			constants.ExecutionsNodeIdColumn,
			constants.ExecutionsLastExecutionTimeColumn,
			constants.ExecutionsNextExecutionTime,
			constants.ExecutionsJobIdColumn,
			constants.ExecutionsJobQueueVersion,
			constants.ExecutionsVersion,
			constants.ExecutionsDateCreatedColumn,
			table,
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
				&jobExecutionLog.JobQueueVersion,
				&jobExecutionLog.ExecutionVersion,
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

func (repo *sharedRepo) InsertExecutionLogs(db db.DataStore, committed bool, jobExecutionLogs []models.JobExecutionLog) error {
	db.ConnectionLock()
	defer db.ConnectionUnlock()

	executionLogsBatches := utils.Batch[models.JobExecutionLog](jobExecutionLogs, 9)

	table := constants.ExecutionsUnCommittedTableName
	if committed {
		table = constants.ExecutionsCommittedTableName
	}

	for _, executionLogsBatch := range executionLogsBatches {
		query := fmt.Sprintf("INSERT INTO %s (%s, %s, %s, %s, %s, %s, %s, %s, %s) VALUES ",
			table,
			constants.ExecutionsUniqueIdColumn,
			constants.ExecutionsStateColumn,
			constants.ExecutionsNodeIdColumn,
			constants.ExecutionsLastExecutionTimeColumn,
			constants.ExecutionsNextExecutionTime,
			constants.ExecutionsJobIdColumn,
			constants.ExecutionsDateCreatedColumn,
			constants.ExecutionsJobQueueVersion,
			constants.ExecutionsVersion,
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

func (repo *sharedRepo) DeleteExecutionLogs(db db.DataStore, committed bool, jobExecutionLogs []models.JobExecutionLog) error {
	db.ConnectionLock()
	defer db.ConnectionUnlock()

	batches := utils.Batch[models.JobExecutionLog](jobExecutionLogs, 1)

	table := constants.ExecutionsUnCommittedTableName
	if committed {
		table = constants.ExecutionsCommittedTableName
	}

	for _, batch := range batches {
		query := fmt.Sprintf("DELETE FROM %s WHERE %s IN ", table, constants.ExecutionsUniqueIdColumn)

		query += "(?"
		params := []interface{}{
			batch[0].UniqueId,
		}

		for _, executionLog := range batch[1:] {
			params = append(params,
				executionLog.UniqueId,
			)
			query += ",?"
		}

		query += ");"

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

func (repo *sharedRepo) InsertAsyncTasksLogs(db db.DataStore, committed bool, asyncTasks []models.AsyncTask) error {
	db.ConnectionLock()
	defer db.ConnectionUnlock()

	batches := utils.Batch[models.AsyncTask](asyncTasks, 6)

	table := constants.CommittedAsyncTableName
	if !committed {
		table = constants.UnCommittedAsyncTableName
	}

	for _, batch := range batches {
		query := fmt.Sprintf("INSERT INTO %s (%s, %s, %s, %s, %s, %s) VALUES (?, ?, ?, ?, ?, ?)",
			table,
			constants.AsyncTasksRequestIdColumn,
			constants.AsyncTasksInputColumn,
			constants.AsyncTasksOutputColumn,
			constants.AsyncTasksStateColumn,
			constants.AsyncTasksServiceColumn,
			constants.AsyncTasksDateCreatedColumn,
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

func (repo *sharedRepo) DeleteAsyncTasksLogs(db db.DataStore, committed bool, asyncTasks []models.AsyncTask) error {
	db.ConnectionLock()
	defer db.ConnectionUnlock()

	batches := utils.Batch[models.AsyncTask](asyncTasks, 1)

	table := constants.CommittedAsyncTableName
	if !committed {
		table = constants.UnCommittedAsyncTableName
	}

	for _, batch := range batches {
		paramPlaceholder := "?"
		params := []interface{}{
			batch[0].RequestId,
		}

		for _, asyncTask := range batch[1:] {
			paramPlaceholder += ",?"
			params = append(params, asyncTask.RequestId)
		}

		ctx := context.Background()
		tx, err := db.GetOpenConnection().BeginTx(ctx, nil)
		if err != nil {
			repo.logger.Error("failed to create transaction for delete", "error", err.Error())
			return err
		}
		query := fmt.Sprintf("DELETE FROM %s WHERE %s IN (%s)", table, constants.AsyncTasksRequestIdColumn, paramPlaceholder)
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
