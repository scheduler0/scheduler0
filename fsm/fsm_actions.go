package fsm

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	"google.golang.org/protobuf/proto"
	"net/http"
	"scheduler0/constants"
	"scheduler0/db"
	"scheduler0/models"
	"scheduler0/protobuffs"
	"scheduler0/shared_repo"
	"scheduler0/utils"
	"time"
)

//go:generate mockery --name Scheduler0RaftActions --output ./ --inpackage
type Scheduler0RaftActions interface {
	WriteCommandToRaftLog(
		rft *raft.Raft,
		commandType constants.Command,
		sqlString string,
		params []interface{},

		nodeIds []uint64,
		action constants.CommandAction) (*models.FSMResponse, *utils.GenericError)
	ApplyRaftLog(
		logger hclog.Logger,
		l *raft.Log,
		db db.DataStore,
		ignorePostProcessChannel bool,
	) interface{}
}

type scheduler0RaftActions struct {
	sharedRepo         shared_repo.SharedRepo
	postProcessChannel chan models.PostProcess
}

func NewScheduler0RaftActions(sharedRepo shared_repo.SharedRepo, postProcessChannel chan models.PostProcess) Scheduler0RaftActions {
	return &scheduler0RaftActions{
		sharedRepo:         sharedRepo,
		postProcessChannel: postProcessChannel,
	}
}

func (_ *scheduler0RaftActions) WriteCommandToRaftLog(
	rft *raft.Raft,
	commandType constants.Command,
	sqlString string,
	params []interface{},
	nodeIds []uint64,
	action constants.CommandAction) (*models.FSMResponse, *utils.GenericError) {
	data, err := json.Marshal(params)
	if err != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	command := &protobuffs.Command{
		Type:         protobuffs.Command_Type(commandType),
		Sql:          sqlString,
		Data:         data,
		TargetNodes:  nodeIds,
		TargetAction: uint64(action),
	}

	if err != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	commandData, err := proto.Marshal(command)
	if err != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	af := rft.Apply(commandData, time.Duration(15)*time.Second).(raft.ApplyFuture)
	if af.Error() != nil {
		if errors.Is(af.Error(), raft.ErrNotLeader) {
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, "server not raft leader")
		}
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, af.Error().Error())
	}

	if af.Response() != nil {
		r, ok := af.Response().(models.FSMResponse)
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
	ignorePostProcessChannel bool,
) interface{} {

	if l.Type == raft.LogConfiguration {
		return nil
	}

	command := &protobuffs.Command{}

	marshalErr := proto.Unmarshal(l.Data, command)
	if marshalErr != nil {
		logger.Error("failed to unmarshal command", marshalErr.Error())
		return models.FSMResponse{
			Error: marshalErr.Error(),
		}
	}

	result := dbExecute(logger, command, db)

	if raftActions.postProcessChannel != nil && !ignorePostProcessChannel && result.Error == "" {
		switch command.TargetAction {
		case uint64(constants.CommandActionQueueJob):
			raftActions.postProcessChannel <- models.PostProcess{
				Action:      constants.CommandActionQueueJob,
				TargetNodes: command.TargetNodes,
				Data:        result.Data,
			}
		case uint64(constants.CommandActionCleanUncommittedExecutionLogs):
			raftActions.postProcessChannel <- models.PostProcess{
				Action:      constants.CommandActionCleanUncommittedExecutionLogs,
				TargetNodes: command.TargetNodes,
				Data:        result.Data,
			}
		case uint64(constants.CommandActionCleanUncommittedAsyncTasksLogs):
			raftActions.postProcessChannel <- models.PostProcess{
				Action:      constants.CommandActionCleanUncommittedAsyncTasksLogs,
				TargetNodes: command.TargetNodes,
				Data:        result.Data,
			}
		}
	}

	return result
}

func dbExecute(logger hclog.Logger, command *protobuffs.Command, db db.DataStore) models.FSMResponse {
	db.ConnectionLock()
	defer db.ConnectionUnlock()

	var params []interface{}
	err := json.Unmarshal(command.Data, &params)
	if err != nil {
		return models.FSMResponse{
			Error: err.Error(),
		}
	}
	ctx := context.Background()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		logger.Error("failed to execute sql command", "error", err.Error())
		return models.FSMResponse{
			Error: err.Error(),
		}
	}

	exec, err := tx.Exec(command.Sql, params...)
	if err != nil {
		logger.Error("failed to execute sql command", "error", err.Error())
		rollBackErr := tx.Rollback()
		if rollBackErr != nil {
			return models.FSMResponse{
				Error: err.Error(),
			}
		}
		return models.FSMResponse{
			Error: err.Error(),
		}
	}

	err = tx.Commit()
	if err != nil {
		logger.Error("failed to commit transaction", "error", err.Error())
		return models.FSMResponse{
			Error: err.Error(),
		}
	}

	lastInsertedId, err := exec.LastInsertId()
	if err != nil {
		logger.Error("failed to get last inserted id", "error", err.Error())
		rollBackErr := tx.Rollback()
		if rollBackErr != nil {
			logger.Error("failed to roll back transaction", "error", rollBackErr.Error())
			return models.FSMResponse{
				Error: rollBackErr.Error(),
			}
		}
		return models.FSMResponse{
			Error: err.Error(),
		}
	}
	rowsAffected, err := exec.RowsAffected()

	if err != nil {
		logger.Error("failed to get number of rows affected", "error", err.Error())

		rollBackErr := tx.Rollback()
		if rollBackErr != nil {
			logger.Error("failed to roll back transaction", "error", rollBackErr.Error())
			return models.FSMResponse{
				Error: rollBackErr.Error(),
			}
		}
		return models.FSMResponse{
			Error: err.Error(),
		}
	}

	return models.FSMResponse{
		Data: models.SQLResponse{
			LastInsertedId: lastInsertedId,
			RowsAffected:   rowsAffected,
		},
		Error: "",
	}
}

//func localDataCommit(logger hclog.Logger, command *protobuffs.Command, db db.DataStore, shardRepo shared_repo.SharedRepo) models.FSMResponse {
//	var payload []models.CommitLocalData
//	err := json.Unmarshal(command.Data, &payload)
//	if err != nil {
//		logger.Error("failed to unmarshal local data to commit", "error", err.Error())
//		return models.FSMResponse{
//			Data:  nil,
//			Error: err.Error(),
//		}
//	}
//	localData := payload[0]
//	logger.Debug(fmt.Sprintf("received %d local execution logs to commit", len(localData.Data.ExecutionLogs)))
//
//	if len(localData.Data.ExecutionLogs) > 0 {
//		insertErr := shardRepo.InsertExecutionLogs(db, true, localData.Data.ExecutionLogs)
//		if insertErr != nil {
//			return models.FSMResponse{
//				Data:  nil,
//				Error: insertErr.Error(),
//			}
//		}
//		deleteErr := shardRepo.DeleteExecutionLogs(db, false, localData.Data.ExecutionLogs)
//		if deleteErr != nil {
//			return models.FSMResponse{
//				Data:  nil,
//				Error: deleteErr.Error(),
//			}
//		}
//	}
//
//	logger.Debug(fmt.Sprintf("received %d local async tasks to commit", len(localData.Data.AsyncTasks)))
//
//	if len(localData.Data.AsyncTasks) > 0 {
//		insertErr := shardRepo.InsertAsyncTasksLogs(db, true, localData.Data.AsyncTasks)
//		if insertErr != nil {
//			return models.FSMResponse{
//				Data:  nil,
//				Error: insertErr.Error(),
//			}
//		}
//		deleteErr := shardRepo.DeleteAsyncTasksLogs(db, false, localData.Data.AsyncTasks)
//		if deleteErr != nil {
//			return models.FSMResponse{
//				Data:  nil,
//				Error: deleteErr.Error(),
//			}
//		}
//	}
//
//	return models.FSMResponse{
//		Data:  nil,
//		Error: "",
//	}
//}
