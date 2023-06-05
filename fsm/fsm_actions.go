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
	"scheduler0/shared_repo"
	"scheduler0/utils"
	"time"
)

//go:generate mockery --name Scheduler0RaftActions --output ../mocks
type Scheduler0RaftActions interface {
	WriteCommandToRaftLog(
		rft *raft.Raft,
		commandType constants.Command,
		sqlString string,
		nodeId uint64,
		params []interface{}) (*models.Response, *utils.GenericError)
	ApplyRaftLog(
		logger hclog.Logger,
		l *raft.Log,
		db db.DataStore,
		useQueues bool,
		queue chan []interface{},
		stopAllJobsQueue chan bool,
		recoverJobsQueue chan bool) interface{}
}

type scheduler0RaftActions struct {
	sharedRepo shared_repo.SharedRepo
}

func NewScheduler0RaftActions(sharedRepo shared_repo.SharedRepo) Scheduler0RaftActions {
	return &scheduler0RaftActions{
		sharedRepo: sharedRepo,
	}
}

func (_ *scheduler0RaftActions) WriteCommandToRaftLog(
	rft *raft.Raft,
	commandType constants.Command,
	sqlString string,
	nodeId uint64,
	params []interface{}) (*models.Response, *utils.GenericError) {
	data, err := json.Marshal(params)
	if err != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

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

	af := rft.Apply(createCommandData, time.Second*time.Duration(configs.RaftApplyTimeout)).(raft.ApplyFuture)
	if af.Error() != nil {
		if af.Error() == raft.ErrNotLeader {
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, "server not raft leader")
		}
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, af.Error().Error())
	}

	if af.Response() != nil {
		r, ok := af.Response().(models.Response)
		if !ok {
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, "unknown raft response type")
		}
		if r.Error != "" {
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, r.Error)
		}
		return &r, nil
	}

	return nil, nil
}

func (raftActions *scheduler0RaftActions) ApplyRaftLog(
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
		return models.Response{
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
		return localDataCommit(logger, command, db, raftActions.sharedRepo)
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

func dbExecute(logger hclog.Logger, command *protobuffs.Command, db db.DataStore) models.Response {
	db.ConnectionLock()
	defer db.ConnectionUnlock()

	var params []interface{}
	err := json.Unmarshal(command.Data, &params)
	if err != nil {
		return models.Response{
			Data:  nil,
			Error: err.Error(),
		}
	}
	ctx := context.Background()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		logger.Error("failed to execute sql command", "error", err.Error())
		return models.Response{
			Data:  nil,
			Error: err.Error(),
		}
	}

	exec, err := tx.Exec(command.Sql, params...)
	if err != nil {
		logger.Error("failed to execute sql command", "error", err.Error())
		rollBackErr := tx.Rollback()
		if rollBackErr != nil {
			return models.Response{
				Data:  nil,
				Error: err.Error(),
			}
		}
		return models.Response{
			Data:  nil,
			Error: err.Error(),
		}
	}

	err = tx.Commit()
	if err != nil {
		logger.Error("failed to commit transaction", "error", err.Error())
		return models.Response{
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
			return models.Response{
				Data:  nil,
				Error: rollBackErr.Error(),
			}
		}
		return models.Response{
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
			return models.Response{
				Data:  nil,
				Error: rollBackErr.Error(),
			}
		}
		return models.Response{
			Data:  nil,
			Error: err.Error(),
		}
	}
	data := []interface{}{lastInsertedId, rowsAffected}

	return models.Response{
		Data:  data,
		Error: "",
	}
}

func insertJobQueue(logger hclog.Logger, command *protobuffs.Command, db db.DataStore, useQueues bool, queue chan []interface{}) models.Response {
	db.ConnectionLock()
	defer db.ConnectionUnlock()

	var jobIds []interface{}
	err := json.Unmarshal(command.Data, &jobIds)
	if err != nil {
		logger.Error("failed unmarshal json bytes to jobs", "error", err.Error())
		return models.Response{
			Data:  nil,
			Error: err.Error(),
		}
	}
	lowerBound := jobIds[0].(float64)
	upperBound := jobIds[1].(float64)
	lastVersion := jobIds[2].(float64)

	schedulerTime := utils.GetSchedulerTime()
	now := schedulerTime.GetTime(time.Now())

	insertBuilder := sq.Insert(constants.JobQueuesTableName).
		Columns(
			constants.JobQueueNodeIdColumn,
			constants.JobQueueLowerBoundJobId,
			constants.JobQueueUpperBound,
			constants.JobQueueVersion,
			constants.JobQueueDateCreatedColumn,
		).
		Values(
			command.TargetNode,
			lowerBound,
			upperBound,
			lastVersion,
			now,
		).
		RunWith(db.GetOpenConnection())

	_, err = insertBuilder.Exec()
	if err != nil {
		logger.Error("failed to insert new job queues", "error", err.Error())
		return models.Response{
			Data:  nil,
			Error: err.Error(),
		}
	}
	if useQueues {
		queue <- []interface{}{command.TargetNode, int64(lowerBound), int64(upperBound)}
	}
	return models.Response{
		Data:  nil,
		Error: "",
	}
}

func localDataCommit(logger hclog.Logger, command *protobuffs.Command, db db.DataStore, shardRepo shared_repo.SharedRepo) models.Response {
	var payload []models.CommitLocalData
	err := json.Unmarshal(command.Data, &payload)
	if err != nil {
		logger.Error("failed to unmarshal local data to commit", "error", err.Error())
		return models.Response{
			Data:  nil,
			Error: err.Error(),
		}
	}
	localData := payload[0]
	logger.Debug(fmt.Sprintf("received %d local execution logs to commit", len(localData.Data.ExecutionLogs)))

	if len(localData.Data.ExecutionLogs) > 0 {
		insertErr := shardRepo.InsertExecutionLogs(db, true, localData.Data.ExecutionLogs)
		if insertErr != nil {
			return models.Response{
				Data:  nil,
				Error: insertErr.Error(),
			}
		}
		deleteErr := shardRepo.DeleteExecutionLogs(db, false, localData.Data.ExecutionLogs)
		if deleteErr != nil {
			return models.Response{
				Data:  nil,
				Error: deleteErr.Error(),
			}
		}
	}

	logger.Debug(fmt.Sprintf("received %d local async tasks to commit", len(localData.Data.AsyncTasks)))

	if len(localData.Data.AsyncTasks) > 0 {
		insertErr := shardRepo.InsertAsyncTasksLogs(db, true, localData.Data.AsyncTasks)
		if insertErr != nil {
			return models.Response{
				Data:  nil,
				Error: insertErr.Error(),
			}
		}
		deleteErr := shardRepo.DeleteAsyncTasksLogs(db, false, localData.Data.AsyncTasks)
		if deleteErr != nil {
			return models.Response{
				Data:  nil,
				Error: deleteErr.Error(),
			}
		}
	}

	return models.Response{
		Data:  nil,
		Error: "",
	}
}
