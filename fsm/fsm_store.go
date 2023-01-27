package fsm

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	sq "github.com/Masterminds/squirrel"
	"github.com/hashicorp/raft"
	"google.golang.org/protobuf/proto"
	"io"
	"io/ioutil"
	"log"
	"os"
	"scheduler0/constants"
	"scheduler0/db"
	"scheduler0/models"
	"scheduler0/protobuffs"
	"scheduler0/utils"
	"scheduler0/utils/batcher"
	"sync"
	"time"
)

type Store struct {
	rwMtx                   sync.RWMutex
	DataStore               *db.DataStore
	logger                  *log.Logger
	Raft                    *raft.Raft
	QueueJobsChannel        chan []interface{}
	JobExecutionLogsChannel chan models.CommitJobStateLog
	StopAllJobs             chan bool
	RecoverJobs             chan bool

	raft.BatchingFSM
}

type Response struct {
	Data  []interface{}
	Error string
}

const (
	JobQueuesTableName        = "job_queues"
	JobQueueIdColumn          = "id"
	JobQueueNodeIdColumn      = "node_id"
	JobQueueLowerBoundJobId   = "lower_bound_job_id"
	JobQueueUpperBound        = "upper_bound_job_id"
	JobQueueDateCreatedColumn = "date_created"
	JobQueueVersion           = "version"

	ExecutionsUnCommittedTableName    = "job_executions_uncommitted"
	ExecutionsTableName               = "job_executions_committed"
	ExecutionsIdColumn                = "id"
	ExecutionsUniqueIdColumn          = "unique_id"
	ExecutionsStateColumn             = "state"
	ExecutionsNodeIdColumn            = "node_id"
	ExecutionsLastExecutionTimeColumn = "last_execution_time"
	ExecutionsNextExecutionTime       = "next_execution_time"
	ExecutionsJobIdColumn             = "job_id"
	ExecutionsDateCreatedColumn       = "date_created"
	ExecutionsJobQueueVersion         = "job_queue_version"
	ExecutionsVersion                 = "execution_version"

	JobQueuesVersionTableName     = "job_queue_versions"
	JobNumberOfActiveNodesVersion = "number_of_active_nodes"
)

var _ raft.FSM = &Store{}

func NewFSMStore(db *db.DataStore, logger *log.Logger) *Store {
	return &Store{
		DataStore:               db,
		QueueJobsChannel:        make(chan []interface{}, 1),
		JobExecutionLogsChannel: make(chan models.CommitJobStateLog, 1),
		StopAllJobs:             make(chan bool, 1),
		logger:                  logger,
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
	logger *log.Logger,
	l *raft.Log,
	db *db.DataStore,
	useQueues bool,
	queue chan []interface{},
	stopAllJobsQueue chan bool,
	recoverJobsQueue chan bool) interface{} {

	logPrefix := logger.Prefix()
	logger.SetPrefix(fmt.Sprintf("%s[apply-raft-command] ", logPrefix))
	defer logger.SetPrefix(logPrefix)

	if l.Type == raft.LogConfiguration {
		return nil
	}

	command := &protobuffs.Command{}

	marsherErr := proto.Unmarshal(l.Data, command)
	if marsherErr != nil {
		logger.Fatal("failed to unmarshal command", marsherErr.Error())
	}
	switch command.Type {
	case protobuffs.Command_Type(constants.CommandTypeDbExecute):
		return dbExecute(logger, command, db)
	case protobuffs.Command_Type(constants.CommandTypeJobQueue):
		return insertJobQueue(logger, command, db, useQueues, queue)
	case protobuffs.Command_Type(constants.CommandTypeJobExecutionLogs):
		return executeJobs(logger, command, db)
	case protobuffs.Command_Type(constants.CommandTypeStopJobs):
		if useQueues {
			stopAllJobsQueue <- true
		}
	case protobuffs.Command_Type(constants.CommandTypeRecoverJobs):
		if useQueues {
			recoverJobsQueue <- true
		}
	}

	return nil
}

func (s *Store) Snapshot() (raft.FSMSnapshot, error) {
	logPrefix := s.logger.Prefix()
	s.logger.SetPrefix(fmt.Sprintf("%s[snapshot-fsm] ", logPrefix))
	defer s.logger.SetPrefix(logPrefix)
	fmsSnapshot := NewFSMSnapshot(s.DataStore)
	s.logger.Println("took snapshot")
	return fmsSnapshot, nil
}

func (s *Store) Restore(r io.ReadCloser) error {
	logPrefix := s.logger.Prefix()
	s.logger.SetPrefix(fmt.Sprintf("%s[restoring-snapshot] ", logPrefix))
	defer s.logger.SetPrefix(logPrefix)
	s.logger.Println("restoring snapshot")

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

	db, err := sql.Open("sqlite3", dbFilePath)
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

func dbExecute(logger *log.Logger, command *protobuffs.Command, db *db.DataStore) Response {
	db.ConnectionLock.Lock()
	defer db.ConnectionLock.Unlock()

	params := []interface{}{}
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
		logger.Println("failed to execute sql command", err.Error())
		return Response{
			Data:  nil,
			Error: err.Error(),
		}
	}

	exec, err := tx.Exec(command.Sql, params...)
	if err != nil {
		logger.Println("failed to execute sql command", err.Error())
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
		return Response{
			Data:  nil,
			Error: err.Error(),
		}
	}

	lastInsertedId, err := exec.LastInsertId()
	if err != nil {
		logger.Println("failed to get last ", err.Error())
		rollBackErr := tx.Rollback()
		if rollBackErr != nil {
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
		logger.Println(err.Error())
		rollBackErr := tx.Rollback()
		if rollBackErr != nil {
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

func insertJobQueue(logger *log.Logger, command *protobuffs.Command, db *db.DataStore, useQueues bool, queue chan []interface{}) Response {
	db.ConnectionLock.Lock()
	defer db.ConnectionLock.Unlock()

	jobIds := []interface{}{}
	err := json.Unmarshal(command.Data, &jobIds)
	if err != nil {
		return Response{
			Data:  nil,
			Error: err.Error(),
		}
	}
	lowerBound := jobIds[0].(float64)
	upperBound := jobIds[1].(float64)
	lastVersion := jobIds[2].(float64)

	serverNodeId, err := utils.GetNodeIdWithRaftAddress(logger, raft.ServerAddress(command.ActionTarget))

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
		logger.Fatalln("failed to insert new job queues", err.Error())
	}
	if useQueues {
		queue <- []interface{}{command.Sql, int64(lowerBound), int64(upperBound)}
	}
	return Response{
		Data:  nil,
		Error: "",
	}
}

func batchInsert(logger *log.Logger, db *db.DataStore, jobExecutionLogs []models.JobExecutionLog) {
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
			logger.Fatalln("failed to create transaction for batch insertion", err)
		}

		_, err = tx.Exec(query, params...)
		if err != nil {
			trxErr := tx.Rollback()
			if trxErr != nil {
				logger.Fatalln("failed to rollback update transition", trxErr)
			}
			logger.Fatalln("batch insert failed to update committed status of executions", err)
		}
		err = tx.Commit()
		if err != nil {
			logger.Fatalln("failed to commit transition", err)
		}
	}
}

func batchDelete(logger *log.Logger, db *db.DataStore, jobExecutionLogs []models.JobExecutionLog) {
	batches := batcher.Batch[models.JobExecutionLog](jobExecutionLogs, 9)

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
			logger.Fatalln("failed to create transaction for batch insertion", err)
		}
		query := fmt.Sprintf("DELETE FROM %s WHERE %s IN (%s)", ExecutionsUnCommittedTableName, ExecutionsUniqueIdColumn, paramPlaceholder)
		_, err = tx.Exec(query, params...)
		if err != nil {
			trxErr := tx.Rollback()
			if trxErr != nil {
				logger.Fatalln("failed to rollback update transition", trxErr)
			}
			logger.Fatalln("batch delete failed to update committed status of executions", err)
		}
		err = tx.Commit()
		if err != nil {
			logger.Fatalln("failed to commit transition", err)
		}
	}
}

func executeJobs(logger *log.Logger, command *protobuffs.Command, db *db.DataStore) Response {
	db.ConnectionLock.Lock()
	defer db.ConnectionLock.Unlock()
	jobState := models.CommitJobStateLog{}
	err := json.Unmarshal(command.Data, &jobState)
	if err != nil {
		return Response{
			Data:  nil,
			Error: err.Error(),
		}
	}

	//logger.Println(fmt.Sprintf("received %v jobs execution logs from %s", len(jobState.Logs), jobState.Address))

	if len(jobState.Logs) < 1 {
		return Response{
			Data: nil,
		}
	}

	batchInsert(logger, db, jobState.Logs)
	batchDelete(logger, db, jobState.Logs)

	return Response{
		Data:  nil,
		Error: "",
	}
}
