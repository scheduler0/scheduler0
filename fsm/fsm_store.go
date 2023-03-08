package fsm

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	sq "github.com/Masterminds/squirrel"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	"google.golang.org/protobuf/proto"
	"io"
	"io/ioutil"
	"os"
	"scheduler0/config"
	"scheduler0/constants"
	"scheduler0/db"
	"scheduler0/models"
	"scheduler0/protobuffs"
	"scheduler0/utils"
	"scheduler0/utils/batcher"
	"strconv"
	"sync"
	"time"
)

type Store struct {
	rwMtx            sync.RWMutex
	DataStore        *db.DataStore
	logger           hclog.Logger
	Raft             *raft.Raft
	QueueJobsChannel chan []interface{}
	LocalDataChannel chan models.LocalData
	StopAllJobs      chan bool
	RecoverJobs      chan bool

	raft.BatchingFSM
}

type Response struct {
	Data  []interface{}
	Error string
}

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

var _ raft.FSM = &Store{}

func NewFSMStore(db *db.DataStore, logger hclog.Logger) *Store {
	fsmStoreLogger := logger.Named("fsm-store")

	return &Store{
		DataStore:        db,
		QueueJobsChannel: make(chan []interface{}, 1),
		LocalDataChannel: make(chan models.LocalData, 1),
		StopAllJobs:      make(chan bool, 1),
		RecoverJobs:      make(chan bool, 1),
		logger:           fsmStoreLogger,
	}
}

func (s *Store) Apply(l *raft.Log) interface{} {
	s.rwMtx.Lock()
	defer s.rwMtx.Unlock()

	return ApplyCommand(
		s.logger,
		l,
		s.DataStore,
		true,
		s.QueueJobsChannel,
		s.StopAllJobs,
		s.RecoverJobs,
	)
}

func (s *Store) ApplyBatch(logs []*raft.Log) []interface{} {
	s.rwMtx.Lock()
	defer s.rwMtx.Unlock()

	results := []interface{}{}

	for _, l := range logs {
		result := ApplyCommand(
			s.logger,
			l,
			s.DataStore,
			true,
			s.QueueJobsChannel,
			s.StopAllJobs,
			s.RecoverJobs,
		)
		results = append(results, result)
	}

	return results
}

func ApplyCommand(
	logger hclog.Logger,
	l *raft.Log,
	db *db.DataStore,
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
		configs := config.GetConfigurations()
		if useQueues && command.Sql == strconv.FormatUint(configs.NodeId, 10) {
			recoverJobsQueue <- true
		}
		break
	}

	return nil
}

func (s *Store) Snapshot() (raft.FSMSnapshot, error) {
	s.logger.Info("taking snapshot")
	fmsSnapshot := NewFSMSnapshot(s.DataStore)
	return fmsSnapshot, nil
}

func (s *Store) Restore(r io.ReadCloser) error {
	s.DataStore.FileLock.Lock()
	defer s.DataStore.FileLock.Unlock()

	s.logger.Info("restoring snapshot")
	b, err := utils.BytesFromSnapshot(r)
	if err != nil {
		return fmt.Errorf("restore failed: %s", err.Error())
	}
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("Fatal error getting working dir: %s \n", err)
	}
	dbFilePath := fmt.Sprintf("%v/%v", dir, constants.SqliteDbFileName)
	if err := os.Remove(dbFilePath); err != nil && !os.IsNotExist(err) {
		return err
	}
	if b != nil {
		if err := ioutil.WriteFile(dbFilePath, b, os.ModePerm); err != nil {
			return err
		}
	}

	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?_foreign_keys=1", dbFilePath))
	if err != nil {
		return fmt.Errorf("restore failed to create db: %v", err)
	}

	err = db.Ping()
	if err != nil {
		return fmt.Errorf("ping error: restore failed to create db: %v", err)
	}

	s.DataStore.ConnectionLock.Lock()
	s.DataStore.Connection = db
	s.DataStore.ConnectionLock.Unlock()

	return nil
}

func dbExecute(logger hclog.Logger, command *protobuffs.Command, db *db.DataStore) Response {
	db.ConnectionLock.Lock()
	defer db.ConnectionLock.Unlock()

	var params []interface{}
	err := json.Unmarshal(command.Data, &params)
	if err != nil {
		return Response{
			Data:  nil,
			Error: err.Error(),
		}
	}
	ctx := context.Background()

	tx, err := db.Connection.BeginTx(ctx, nil)
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

func insertJobQueue(logger hclog.Logger, command *protobuffs.Command, db *db.DataStore, useQueues bool, queue chan []interface{}) Response {
	db.ConnectionLock.Lock()
	defer db.ConnectionLock.Unlock()

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

	serverNodeId, err := utils.GetNodeIdWithRaftAddress(raft.ServerAddress(command.ActionTarget))

	schedulerTime := utils.GetSchedulerTime()
	now := schedulerTime.GetTime(time.Now())

	insertBuilder := sq.Insert(JobQueuesTableName).Columns(
		JobQueueNodeIdColumn,
		JobQueueLowerBoundJobId,
		JobQueueUpperBound,
		JobQueueVersion,
		JobQueueDateCreatedColumn,
	).Values(
		serverNodeId,
		lowerBound,
		upperBound,
		lastVersion,
		now,
	).RunWith(db.Connection)

	_, err = insertBuilder.Exec()
	if err != nil {
		logger.Error("failed to insert new job queues", "error", err.Error())
		return Response{
			Data:  nil,
			Error: err.Error(),
		}
	}
	if useQueues {
		queue <- []interface{}{command.Sql, int64(lowerBound), int64(upperBound)}
	}
	return Response{
		Data:  nil,
		Error: "",
	}
}

func batchInsertExecutionLogs(logger hclog.Logger, db *db.DataStore, jobExecutionLogs []models.JobExecutionLog) {
	batches := batcher.Batch[models.JobExecutionLog](jobExecutionLogs, 9)

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
		tx, err := db.Connection.BeginTx(ctx, nil)
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

func batchDeleteDeleteExecutionLogs(logger hclog.Logger, db *db.DataStore, jobExecutionLogs []models.JobExecutionLog) {
	batches := batcher.Batch[models.JobExecutionLog](jobExecutionLogs, 1)

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
		tx, err := db.Connection.BeginTx(ctx, nil)
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

func batchDeleteDeleteAsyncTasks(logger hclog.Logger, db *db.DataStore, tasks []models.AsyncTask) {
	batches := batcher.Batch[models.AsyncTask](tasks, 1)

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
		tx, err := db.Connection.BeginTx(ctx, nil)
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

func batchInsertAsyncTasks(logger hclog.Logger, db *db.DataStore, tasks []models.AsyncTask) {
	batches := batcher.Batch[models.AsyncTask](tasks, 6)

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
		tx, err := db.Connection.BeginTx(ctx, nil)
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

func localDataCommit(logger hclog.Logger, command *protobuffs.Command, db *db.DataStore) Response {
	db.ConnectionLock.Lock()
	defer db.ConnectionLock.Unlock()

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
