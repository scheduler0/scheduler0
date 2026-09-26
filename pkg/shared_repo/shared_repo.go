package shared_repo

import (
	"context"
	"fmt"
	"scheduler0/pkg/config"
	"scheduler0/pkg/constants"
	"scheduler0/pkg/db"
	"scheduler0/pkg/models"
	"scheduler0/pkg/utils"
	"time"

	"github.com/hashicorp/go-hclog"
)

type SharedRepo interface {
	GetExecutionLogs(db db.DataStore, committed bool) ([]models.JobExecutionLog, error)
	GetAsyncTasksLogs(db db.DataStore, committed bool) ([]models.AsyncTask, error)
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
	repo.logger.Debug("getting execution logs", "committed", committed)
	startTime := time.Now()
	
	db.ConnectionLock()
	defer db.ConnectionUnlock()

	var executionLogs []models.JobExecutionLog

	configs := repo.scheduler0Config.GetConfigurations()
	table := constants.ExecutionsUnCommittedTableName
	if committed {
		table = constants.ExecutionsCommittedTableName
	}
	
	repo.logger.Debug("querying execution logs from table", "table", table, "nodeId", configs.NodeId)

	rows, err := db.GetOpenConnection().Query(fmt.Sprintf(
		"select count(*) from %s",
		table,
	))
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
	))
	if err != nil {
		repo.logger.Error("failed to query for max and min id in uncommitted logs", "error", err.Error())
		return nil, err
	}

	repo.logger.Info("execution logs count query completed", "table", table, "count", count, "nodeId", configs.NodeId)

	if count < 1 {
		repo.logger.Debug("no execution logs found", "table", table, "nodeId", configs.NodeId)
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

	repo.logger.Info("execution logs id range query completed", "table", table, "minId", minId, "maxId", maxId, "nodeId", configs.NodeId)

	ids := []int64{}
	for i := minId; i <= maxId; i++ {
		ids = append(ids, i)
	}

	batches := utils.Batch[int64](ids, 11)
	repo.logger.Debug("batched execution log ids for query", "table", table, "totalIds", len(ids), "batchCount", len(batches), "batchSize", 11)

	for batchIdx, batch := range batches {
		repo.logger.Debug("processing execution logs batch", "table", table, "batchIndex", batchIdx+1, "totalBatches", len(batches), "batchSize", len(batch))
		batchIds := []interface{}{batch[0]}
		params := "?"

		for _, id := range batch[1:] {
			batchIds = append(batchIds, id)
			params += ",?"
		}

		rows, err = db.GetOpenConnection().Query(fmt.Sprintf(
			"select  %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s from %s where id in (%s)",
			constants.ExecutionsUniqueIdColumn,
			constants.ExecutionsStateColumn,
			constants.ExecutionsNodeIdColumn,
			constants.ExecutionsLastExecutionTimeColumn,
			constants.ExecutionsNextExecutionTime,
			constants.ExecutionsJobIdColumn,
			constants.ExecutionsJobQueueVersion,
			constants.ExecutionsVersion,
			constants.ExecutionsDateCreatedColumn,
			constants.ExecutionsDateModifiedColumn,
			constants.ExecutionsAccountIdColumn,
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
				&jobExecutionLog.DateCreated,
				&jobExecutionLog.DateModified,
				&jobExecutionLog.AccountId,
			)
			if scanErr != nil {
				repo.logger.Error("failed to scan job execution columns", "error", scanErr.Error())
				return nil, scanErr
			}
			executionLogs = append(executionLogs, jobExecutionLog)
		}
		err = rows.Close()
		if err != nil {
			repo.logger.Error("failed to close rows", "error", err, "table", table, "batchIndex", batchIdx+1)
			return nil, err
		}
		repo.logger.Debug("execution logs batch query completed", "table", table, "batchIndex", batchIdx+1, "batchSize", len(batch), "logsInBatch", len(executionLogs))
	}

	duration := time.Since(startTime)
	repo.logger.Info("get execution logs completed", "table", table, "totalLogs", len(executionLogs), "nodeId", configs.NodeId, "duration", duration, "durationMs", duration.Milliseconds())
	return executionLogs, nil
}

func (repo *sharedRepo) GetAsyncTasksLogs(db db.DataStore, committed bool) ([]models.AsyncTask, error) {
	repo.logger.Debug("getting async tasks logs", "committed", committed)
	// TODO: Implement async tasks logs retrieval
	repo.logger.Warn("GetAsyncTasksLogs not implemented, returning empty result", "committed", committed)
	return nil, nil
}

func (repo *sharedRepo) InsertExecutionLogs(db db.DataStore, committed bool, jobExecutionLogs []models.JobExecutionLog) error {
	startTime := time.Now()
	repo.logger.Info("inserting execution logs", "committed", committed, "totalLogs", len(jobExecutionLogs))
	
	db.ConnectionLock()
	defer db.ConnectionUnlock()

	executionLogsBatches := utils.Batch[models.JobExecutionLog](jobExecutionLogs, 11)
	repo.logger.Debug("batched execution logs for insertion", "totalLogs", len(jobExecutionLogs), "batchCount", len(executionLogsBatches), "batchSize", 11)

	table := constants.ExecutionsUnCommittedTableName
	if committed {
		table = constants.ExecutionsCommittedTableName
	}
	
	repo.logger.Debug("inserting execution logs into table", "table", table, "batchCount", len(executionLogsBatches))

	for batchIdx, executionLogsBatch := range executionLogsBatches {
		repo.logger.Debug("processing execution logs insertion batch", "table", table, "batchIndex", batchIdx+1, "totalBatches", len(executionLogsBatches), "batchSize", len(executionLogsBatch))
		
		query := fmt.Sprintf("INSERT INTO %s (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s) VALUES ",
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
			constants.ExecutionsAccountIdColumn,
			constants.ExecutionsDateModifiedColumn,
		)

		query += "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
		params := []interface{}{
			executionLogsBatch[0].UniqueId,
			executionLogsBatch[0].State,
			executionLogsBatch[0].NodeId,
			executionLogsBatch[0].LastExecutionDatetime,
			executionLogsBatch[0].NextExecutionDatetime,
			executionLogsBatch[0].JobId,
			executionLogsBatch[0].DateCreated,
			executionLogsBatch[0].JobQueueVersion,
			executionLogsBatch[0].ExecutionVersion,
			executionLogsBatch[0].AccountId,
			executionLogsBatch[0].DateModified,
		}

		for _, executionLog := range executionLogsBatch[1:] {
			params = append(params,
				executionLog.UniqueId,
				executionLog.State,
				executionLog.NodeId,
				executionLog.LastExecutionDatetime,
				executionLog.NextExecutionDatetime,
				executionLog.JobId,
				executionLog.DateCreated,
				executionLog.JobQueueVersion,
				executionLog.ExecutionVersion,
				executionLog.AccountId,
				executionLog.DateModified,
			)
			query += ",(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
		}

		query += ";"

		ctx := context.Background()
		repo.logger.Debug("starting transaction for execution logs batch insertion", "table", table, "batchIndex", batchIdx+1, "batchSize", len(executionLogsBatch))
		
		tx, err := db.GetOpenConnection().BeginTx(ctx, nil)
		if err != nil {
			repo.logger.Error("failed to create transaction for batch insertion", "error", err, "table", table, "batchIndex", batchIdx+1)
			return err
		}
		
		_, err = tx.Exec(query, params...)
		if err != nil {
			trxErr := tx.Rollback()
			if trxErr != nil {
				repo.logger.Error("failed to rollback transaction", "error", trxErr, "table", table, "batchIndex", batchIdx+1)
			}
			repo.logger.Error("failed to insert execution logs", "error", err, "table", table, "batchIndex", batchIdx+1, "batchSize", len(executionLogsBatch))
			return err
		}
		
		commitErr := tx.Commit()
		if commitErr != nil {
			repo.logger.Error("failed to commit transaction", "error", commitErr, "table", table, "batchIndex", batchIdx+1)
			return commitErr
		}
		
		repo.logger.Debug("execution logs batch inserted successfully", "table", table, "batchIndex", batchIdx+1, "batchSize", len(executionLogsBatch))
	}

	duration := time.Since(startTime)
	repo.logger.Info("insert execution logs completed", "table", table, "totalLogs", len(jobExecutionLogs), "batchCount", len(executionLogsBatches), "duration", duration, "durationMs", duration.Milliseconds())
	return nil
}

func (repo *sharedRepo) DeleteExecutionLogs(db db.DataStore, committed bool, jobExecutionLogs []models.JobExecutionLog) error {
	startTime := time.Now()
	repo.logger.Info("deleting execution logs", "committed", committed, "totalLogs", len(jobExecutionLogs))
	
	db.ConnectionLock()
	defer db.ConnectionUnlock()

	batches := utils.Batch[models.JobExecutionLog](jobExecutionLogs, 1)
	repo.logger.Debug("batched execution logs for deletion", "totalLogs", len(jobExecutionLogs), "batchCount", len(batches))

	table := constants.ExecutionsUnCommittedTableName
	if committed {
		table = constants.ExecutionsCommittedTableName
	}
	
	repo.logger.Debug("deleting execution logs from table", "table", table, "batchCount", len(batches))

	for batchIdx, batch := range batches {
		repo.logger.Debug("processing execution logs deletion batch", "table", table, "batchIndex", batchIdx+1, "totalBatches", len(batches), "batchSize", len(batch))
		
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
		repo.logger.Debug("starting transaction for execution logs batch deletion", "table", table, "batchIndex", batchIdx+1, "batchSize", len(batch))
		
		tx, err := db.GetOpenConnection().BeginTx(ctx, nil)
		if err != nil {
			repo.logger.Error("failed to create transaction for execution logs batch deletion", "error", err, "table", table, "batchIndex", batchIdx+1)
			return err
		}

		_, err = tx.Exec(query, params...)
		if err != nil {
			trxErr := tx.Rollback()
			if trxErr != nil {
				repo.logger.Error("failed to rollback transaction", "error", trxErr, "table", table, "batchIndex", batchIdx+1)
				return trxErr
			}
			repo.logger.Error("failed to delete execution logs", "error", err, "table", table, "batchIndex", batchIdx+1, "batchSize", len(batch))
			return err
		}
		
		err = tx.Commit()
		if err != nil {
			repo.logger.Error("failed to commit transaction", "error", err, "table", table, "batchIndex", batchIdx+1)
			return err
		}
		
		repo.logger.Debug("execution logs batch deleted successfully", "table", table, "batchIndex", batchIdx+1, "batchSize", len(batch))
	}

	duration := time.Since(startTime)
	repo.logger.Info("delete execution logs completed", "table", table, "totalLogs", len(jobExecutionLogs), "batchCount", len(batches), "duration", duration, "durationMs", duration.Milliseconds())
	return nil
}

func (repo *sharedRepo) InsertAsyncTasksLogs(db db.DataStore, committed bool, asyncTasks []models.AsyncTask) error {
	startTime := time.Now()
	repo.logger.Info("inserting async tasks logs", "committed", committed, "totalTasks", len(asyncTasks))
	
	db.ConnectionLock()
	defer db.ConnectionUnlock()

	batches := utils.Batch[models.AsyncTask](asyncTasks, 6)
	repo.logger.Debug("batched async tasks for insertion", "totalTasks", len(asyncTasks), "batchCount", len(batches), "batchSize", 6)

	table := constants.CommittedAsyncTableName
	if !committed {
		table = constants.UnCommittedAsyncTableName
	}
	
	repo.logger.Debug("inserting async tasks into table", "table", table, "batchCount", len(batches))

	for batchIdx, batch := range batches {
		repo.logger.Debug("processing async tasks insertion batch", "table", table, "batchIndex", batchIdx+1, "totalBatches", len(batches), "batchSize", len(batch))
		
		query := fmt.Sprintf("INSERT INTO %s (%s, %s, %s, %s, %s, %s, %s, %s) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
			table,
			constants.AsyncTasksRequestIdColumn,
			constants.AsyncTasksInputColumn,
			constants.AsyncTasksOutputColumn,
			constants.AsyncTasksStateColumn,
			constants.AsyncTasksServiceColumn,
			constants.AsyncTasksDateCreatedColumn,
			constants.AsyncTasksAccountIdColumn,
			constants.AsyncTasksDateModifiedColumn,
		)
		params := []interface{}{
			batch[0].RequestId,
			batch[0].Input,
			batch[0].Output,
			batch[0].State,
			batch[0].Service,
			batch[0].DateCreated,
			batch[0].AccountId,
			batch[0].DateModified,
		}

		for _, row := range batch[1:] {
			query += ",(?, ?, ?, ?, ?, ?, ?, ?)"
			params = append(params, row.RequestId, row.Input, row.Output, row.State, row.Service, row.DateCreated, row.AccountId, row.DateModified)
		}

		query += ";"

		_, err := db.GetOpenConnection().Exec(query, params...)
		if err != nil {
			repo.logger.Error("failed to insert async tasks", "error", err, "table", table, "batchIndex", batchIdx+1, "batchSize", len(batch))
			return err
		}
		
		repo.logger.Debug("async tasks batch inserted successfully", "table", table, "batchIndex", batchIdx+1, "batchSize", len(batch))
	}

	duration := time.Since(startTime)
	repo.logger.Info("insert async tasks logs completed", "table", table, "totalTasks", len(asyncTasks), "batchCount", len(batches), "duration", duration, "durationMs", duration.Milliseconds())
	return nil
}

func (repo *sharedRepo) DeleteAsyncTasksLogs(db db.DataStore, committed bool, asyncTasks []models.AsyncTask) error {
	startTime := time.Now()
	repo.logger.Info("deleting async tasks logs", "committed", committed, "totalTasks", len(asyncTasks))
	
	db.ConnectionLock()
	defer db.ConnectionUnlock()

	batches := utils.Batch[models.AsyncTask](asyncTasks, 1)
	repo.logger.Debug("batched async tasks for deletion", "totalTasks", len(asyncTasks), "batchCount", len(batches))

	table := constants.CommittedAsyncTableName
	if !committed {
		table = constants.UnCommittedAsyncTableName
	}
	
	repo.logger.Debug("deleting async tasks from table", "table", table, "batchCount", len(batches))

	for batchIdx, batch := range batches {
		repo.logger.Debug("processing async tasks deletion batch", "table", table, "batchIndex", batchIdx+1, "totalBatches", len(batches), "batchSize", len(batch))
		
		paramPlaceholder := "?"
		params := []interface{}{
			batch[0].RequestId,
		}

		for _, asyncTask := range batch[1:] {
			paramPlaceholder += ",?"
			params = append(params, asyncTask.RequestId)
		}

		ctx := context.Background()
		repo.logger.Debug("starting transaction for async tasks batch deletion", "table", table, "batchIndex", batchIdx+1, "batchSize", len(batch))
		
		tx, err := db.GetOpenConnection().BeginTx(ctx, nil)
		if err != nil {
			repo.logger.Error("failed to create transaction for async tasks deletion", "error", err, "table", table, "batchIndex", batchIdx+1)
			return err
		}
		
		query := fmt.Sprintf("DELETE FROM %s WHERE %s IN (%s)", table, constants.AsyncTasksRequestIdColumn, paramPlaceholder)
		_, err = tx.Exec(query, params...)
		if err != nil {
			trxErr := tx.Rollback()
			if trxErr != nil {
				repo.logger.Error("failed to rollback transaction", "error", trxErr, "table", table, "batchIndex", batchIdx+1)
				return trxErr
			}
			repo.logger.Error("failed to delete async tasks", "error", err, "table", table, "batchIndex", batchIdx+1, "batchSize", len(batch))
			return err
		}
		
		err = tx.Commit()
		if err != nil {
			repo.logger.Error("failed to commit transaction", "error", err, "table", table, "batchIndex", batchIdx+1)
			return err
		}
		
		repo.logger.Debug("async tasks batch deleted successfully", "table", table, "batchIndex", batchIdx+1, "batchSize", len(batch))
	}

	duration := time.Since(startTime)
	repo.logger.Info("delete async tasks logs completed", "table", table, "totalTasks", len(asyncTasks), "batchCount", len(batches), "duration", duration, "durationMs", duration.Milliseconds())
	return nil
}
