package fsm

import (
	"context"
	"encoding/json"
	"fmt"
	sq "github.com/Masterminds/squirrel"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	"google.golang.org/protobuf/proto"
	"net/http"
	"scheduler0/config"
	"scheduler0/constants"
	"scheduler0/db"
	"scheduler0/models"
	"scheduler0/protobuffs"
	"scheduler0/utils"
	"time"
)

type Scheduler0RaftActions interface {
	WriteCommandToRaftLog(
		rft *raft.Raft,
		commandType constants.Command,
		sqlString string,
		nodeId uint64,
		params []interface{}) (*Response, *utils.GenericError)
	ApplyRaftLog(
		logger hclog.Logger,
		l *raft.Log,
		db db.DataStore,
		useQueues bool,
		queue chan []interface{},
		stopAllJobsQueue chan bool,
		recoverJobsQueue chan bool) interface{}
}

type scheduler0RaftActions struct{}

func NewScheduler0RaftActions() Scheduler0RaftActions {
	return &scheduler0RaftActions{}
}

func (_ *scheduler0RaftActions) WriteCommandToRaftLog(
	rft *raft.Raft,
	commandType constants.Command,
	sqlString string,
	nodeId uint64,
	params []interface{}) (*Response, *utils.GenericError) {
	data, err := json.Marshal(params)
	if err != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	fmt.Println("WriteCommandToRaftLog")

	createCommand := &protobuffs.Command{
		Type:       protobuffs.Command_Type(commandType),
		Sql:        sqlString,
		Data:       data,
		TargetNode: nodeId,
	}

	if err != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	createCommandData, err := proto.Marshal(createCommand)
	if err != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	configs := config.NewScheduler0Config().GetConfigurations()
	fmt.Println("rft.Apply(createCommandData, time.Second*time.Duration(configs.RaftApplyTimeout)).(raft.ApplyFuture)", rft)

	af := rft.Apply(createCommandData, time.Second*time.Duration(configs.RaftApplyTimeout)).(raft.ApplyFuture)
	if af.Error() != nil {
		if af.Error() == raft.ErrNotLeader {
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, "server not raft leader")
		}
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, af.Error().Error())
	}
	fmt.Println("ffff)")

	if af.Response() != nil {
		r := af.Response().(Response)
		if r.Error != "" {
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, r.Error)
		}
		return &r, nil
	}
	fmt.Println("ffseff)")

	return nil, nil
}

func (_ *scheduler0RaftActions) ApplyRaftLog(
	logger hclog.Logger,
	l *raft.Log,
	db db.DataStore,
	useQueues bool,
	queue chan []interface{},
	stopAllJobsQueue chan bool,
	recoverJobsQueue chan bool) interface{} {

	if l.Type == raft.LogConfiguration {
		return nil
	}

	command := &protobuffs.Command{}

	marshalErr := proto.Unmarshal(l.Data, command)
	if marshalErr != nil {
		logger.Error("failed to unmarshal command", marshalErr.Error())
		return Response{
			Data:  nil,
			Error: marshalErr.Error(),
		}
	}
	switch command.Type {
	case protobuffs.Command_Type(constants.CommandTypeDbExecute):
		return dbExecute(logger, command, db)
	case protobuffs.Command_Type(constants.CommandTypeJobQueue):
		return insertJobQueue(logger, command, db, useQueues, queue)
	case protobuffs.Command_Type(constants.CommandTypeLocalData):
		return localDataCommit(logger, command, db)
	case protobuffs.Command_Type(constants.CommandTypeStopJobs):
		if useQueues {
			stopAllJobsQueue <- true
		}
		break
	case protobuffs.Command_Type(constants.CommandTypeRecoverJobs):
		configs := config.NewScheduler0Config().GetConfigurations()
		if useQueues && command.TargetNode == configs.NodeId {
			recoverJobsQueue <- true
		}
		break
	}

	return nil
}

func dbExecute(logger hclog.Logger, command *protobuffs.Command, db db.DataStore) Response {
	db.ConnectionLock()
	defer db.ConnectionUnlock()

	var params []interface{}
	err := json.Unmarshal(command.Data, &params)
	if err != nil {
		return Response{
			Data:  nil,
			Error: err.Error(),
		}
	}
	ctx := context.Background()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		logger.Error("failed to execute sql command", "error", err.Error())
		return Response{
			Data:  nil,
			Error: err.Error(),
		}
	}

	exec, err := tx.Exec(command.Sql, params...)
	if err != nil {
		logger.Error("failed to execute sql command", "error", err.Error())
		rollBackErr := tx.Rollback()
		if rollBackErr != nil {
			return Response{
				Data:  nil,
				Error: err.Error(),
			}
		}
	}

	err = tx.Commit()
	if err != nil {
		logger.Error("failed to commit transaction", "error", err.Error())
		return Response{
			Data:  nil,
			Error: err.Error(),
		}
	}

	lastInsertedId, err := exec.LastInsertId()
	if err != nil {
		logger.Error("failed to get last inserted id", "error", err.Error())
		rollBackErr := tx.Rollback()
		if rollBackErr != nil {
			logger.Error("failed to roll back transaction", "error", rollBackErr.Error())
			return Response{
				Data:  nil,
				Error: rollBackErr.Error(),
			}
		}
		return Response{
			Data:  nil,
			Error: err.Error(),
		}
	}
	rowsAffected, err := exec.RowsAffected()
	if err != nil {
		logger.Error("failed to get number of rows affected", "error", err.Error())
		rollBackErr := tx.Rollback()
		if rollBackErr != nil {
			logger.Error("failed to roll back transaction", "error", rollBackErr.Error())
			return Response{
				Data:  nil,
				Error: rollBackErr.Error(),
			}
		}
		return Response{
			Data:  nil,
			Error: err.Error(),
		}
	}
	data := []interface{}{lastInsertedId, rowsAffected}

	return Response{
		Data:  data,
		Error: "",
	}
}

func insertJobQueue(logger hclog.Logger, command *protobuffs.Command, db db.DataStore, useQueues bool, queue chan []interface{}) Response {
	db.ConnectionLock()
	defer db.ConnectionUnlock()

	var jobIds []interface{}
	err := json.Unmarshal(command.Data, &jobIds)
	if err != nil {
		logger.Error("failed unmarshal json bytes to jobs", "error", err.Error())
		return Response{
			Data:  nil,
			Error: err.Error(),
		}
	}
	lowerBound := jobIds[0].(float64)
	upperBound := jobIds[1].(float64)
	lastVersion := jobIds[2].(float64)

	schedulerTime := utils.GetSchedulerTime()
	now := schedulerTime.GetTime(time.Now())

	insertBuilder := sq.Insert(JobQueuesTableName).Columns(
		JobQueueNodeIdColumn,
		JobQueueLowerBoundJobId,
		JobQueueUpperBound,
		JobQueueVersion,
		JobQueueDateCreatedColumn,
	).Values(
		command.TargetNode,
		lowerBound,
		upperBound,
		lastVersion,
		now,
	).RunWith(db.GetOpenConnection())

	_, err = insertBuilder.Exec()
	if err != nil {
		logger.Error("failed to insert new job queues", "error", err.Error())
		return Response{
			Data:  nil,
			Error: err.Error(),
		}
	}
	if useQueues {
		queue <- []interface{}{command.TargetNode, int64(lowerBound), int64(upperBound)}
	}
	return Response{
		Data:  nil,
		Error: "",
	}
}

func batchInsertExecutionLogs(logger hclog.Logger, db db.DataStore, jobExecutionLogs []models.JobExecutionLog) {
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
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			logger.Error("failed to create transaction for execution logs batch insertion", err.Error())
			return
		}

		_, err = tx.Exec(query, params...)
		if err != nil {
			trxErr := tx.Rollback()
			if trxErr != nil {
				logger.Error("failed to rollback transaction to insert batch execution logs", "error", trxErr.Error())
				return
			}
			logger.Error("failed to batch insert execution logs", "error", err.Error())
			return
		}
		err = tx.Commit()
		if err != nil {
			logger.Error("failed to commit transaction to insert batch execution logs", "error", err.Error())
			return
		}
	}
}

func batchDeleteDeleteExecutionLogs(logger hclog.Logger, db db.DataStore, jobExecutionLogs []models.JobExecutionLog) {
	batches := utils.Batch[models.JobExecutionLog](jobExecutionLogs, 1)

	for _, batch := range batches {
		paramPlaceholder := "?"
		params := []interface{}{
			batch[0].UniqueId,
		}

		for _, jobExecutionLog := range batch[1:] {
			paramPlaceholder += ",?"
			params = append(params, jobExecutionLog.UniqueId)
		}

		ctx := context.Background()
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			logger.Error("failed to create transaction for batch execution log deletion", err)
			return
		}
		query := fmt.Sprintf("DELETE FROM %s WHERE %s IN (%s)", ExecutionsUnCommittedTableName, ExecutionsUniqueIdColumn, paramPlaceholder)
		_, err = tx.Exec(query, params...)
		if err != nil {
			trxErr := tx.Rollback()
			if trxErr != nil {
				logger.Error("failed to rollback batch execution log deletion transaction", "error", trxErr.Error())
				return
			}
			logger.Error("failed to batch delete execution logs", "error", err.Error())
			return
		}
		err = tx.Commit()
		if err != nil {
			logger.Error("failed to commit transaction for batch execution log deletion", "error", err.Error())
			return
		}
	}
}

func batchDeleteDeleteAsyncTasks(logger hclog.Logger, db db.DataStore, tasks []models.AsyncTask) {
	batches := utils.Batch[models.AsyncTask](tasks, 1)

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
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			logger.Error("failed to create transaction for delete", "error", err.Error())
			return
		}
		query := fmt.Sprintf("DELETE FROM %s WHERE %s IN (%s)", UnCommittedAsyncTableName, AsyncTasksIdColumn, paramPlaceholder)
		_, err = tx.Exec(query, params...)
		if err != nil {
			trxErr := tx.Rollback()
			if trxErr != nil {
				logger.Error("failed to rollback transaction for batch deletion of async tasks", "error", trxErr.Error())
				return
			}
			logger.Error("batch async tasks delete failed", "error", err.Error())
			return
		}
		err = tx.Commit()
		if err != nil {
			logger.Error("failed to commit transaction for batch async task deletion", "error", err.Error())
			return
		}
	}
}

func batchInsertAsyncTasks(logger hclog.Logger, db db.DataStore, tasks []models.AsyncTask) {
	batches := utils.Batch[models.AsyncTask](tasks, 6)

	for _, batch := range batches {
		query := fmt.Sprintf("INSERT INTO %s (%s, %s, %s, %s, %s, %s) VALUES ",
			CommittedAsyncTableName,
			AsyncTasksRequestIdColumn,
			AsyncTasksInputColumn,
			AsyncTasksOutputColumn,
			AsyncTasksStateColumn,
			AsyncTasksServiceColumn,
			AsyncTasksDateCreatedColumn,
		)

		query += "(?, ?, ?, ?, ?, ?)"
		params := []interface{}{
			batch[0].RequestId,
			batch[0].Input,
			batch[0].Output,
			batch[0].State,
			batch[0].Service,
			batch[0].DateCreated,
		}

		for _, asyncTask := range batch[1:] {
			params = append(params,
				asyncTask.RequestId,
				asyncTask.Input,
				asyncTask.Output,
				asyncTask.State,
				asyncTask.Service,
				asyncTask.DateCreated,
			)
			query += ",(?, ?, ?, ?, ?, ?)"
		}

		query += ";"

		ctx := context.Background()
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			logger.Error("failed to create transaction for batch async tasks insertion", "error", err.Error())
			return
		}

		_, err = tx.Exec(query, params...)
		if err != nil {
			trxErr := tx.Rollback()
			if trxErr != nil {
				logger.Error("failed to rollback transaction for batch async tasks insertion", "error", trxErr.Error())
				return
			}
			logger.Error("batch insert failed for async tasks insertion", "error", err.Error())
			return
		}
		err = tx.Commit()
		if err != nil {
			logger.Error("failed to commit insert delete transaction for async tasks", "error", err.Error())
			return
		}
	}
}

func localDataCommit(logger hclog.Logger, command *protobuffs.Command, db db.DataStore) Response {
	db.ConnectionLock()
	defer db.ConnectionUnlock()

	localData := models.CommitLocalData{}
	err := json.Unmarshal(command.Data, &localData)
	if err != nil {
		logger.Error("failed to unmarshal local data to commit", err.Error())
		return Response{
			Data:  nil,
			Error: err.Error(),
		}
	}

	logger.Debug(fmt.Sprintf("received %d local execution logs to commit", len(localData.Data.ExecutionLogs)))

	if len(localData.Data.ExecutionLogs) > 0 {
		batchInsertExecutionLogs(logger, db, localData.Data.ExecutionLogs)
		batchDeleteDeleteExecutionLogs(logger, db, localData.Data.ExecutionLogs)
	}

	logger.Debug(fmt.Sprintf("received %d local async tasks to commit", len(localData.Data.AsyncTasks)))

	if len(localData.Data.AsyncTasks) > 0 {
		batchInsertAsyncTasks(logger, db, localData.Data.AsyncTasks)
		batchDeleteDeleteAsyncTasks(logger, db, localData.Data.AsyncTasks)
	}

	return Response{
		Data:  nil,
		Error: "",
	}
}
