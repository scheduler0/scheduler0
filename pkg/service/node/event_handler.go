package node

import (
	"encoding/json"
	"scheduler0/pkg/constants"
	"scheduler0/pkg/models"
	"scheduler0/pkg/utils"
)

type EventHandler interface {
	ListenOnInputQueues()
	HandleUncommittedAsyncTasks(asyncTasks []models.AsyncTask)
}

type eventHandler struct {
	node *nodeService
}

func newEventHandler(node *nodeService) *eventHandler {
	return &eventHandler{
		node: node,
	}
}

func (e *eventHandler) ListenOnInputQueues() {
	e.node.logger.Info("begin listening on input channels")

	for {
		select {
		case isLeader := <-e.node.scheduler0RaftStore.GetLeaderChangeChannel():
			e.node.logger.Debug("received leader change", "isLeader", isLeader)
			e.node.raftCluster.HandleRaftLeadershipChangesDebounced(isLeader)
		case o := <-e.node.peerObserverChannels:
			e.node.logger.Debug("received peer observer channel")
			go e.node.raftCluster.HandleRaftObserverChannelChanges(o)
		case postProcess := <-e.node.postProcessingChannel:
			e.node.logger.Debug("received post processing")
			{
				for _, postProcessTargetNode := range postProcess.TargetNodes {
					if postProcessTargetNode == e.node.scheduler0Config.GetConfigurations().NodeId {
						switch postProcess.Action {
						case constants.CommandActionQueueJob:
							e.node.logger.Debug("received queue job")
							go e.node.jobExecutor.QueueExecutions(postProcess.Data.LastInsertedId, postProcess.Data.RowsAffected)
						case constants.CommandActionCleanUncommittedAsyncTasksLogs:
							e.node.logger.Debug("received clean uncommitted async tasks logs")
							go e.node.asyncTaskManager.DeleteNewUncommittedAsyncLogs(postProcess.Data.LastInsertedId, postProcess.Data.RowsAffected)
						case constants.CommandActionCleanUncommittedExecutionLogs:
							e.node.logger.Debug("clean uncommitted execution logs")
							go e.node.jobExecutor.DeleteNewUncommittedExecutionLogs(postProcess.Data.LastInsertedId, postProcess.Data.RowsAffected)
						}
					}
				}
			}
		case <-e.node.ctx.Done():
			return
		}
	}
}

func (e *eventHandler) HandleUncommittedAsyncTasks(asyncTasks []models.AsyncTask) {
	for _, asyncTask := range asyncTasks {
		if asyncTask.State == models.AsyncTaskNotStated || asyncTask.State == models.AsyncTaskInProgress && asyncTask.Service == constants.CreateJobAsyncTaskService {
			var jobsPayload []models.Job
			err := json.Unmarshal([]byte(asyncTask.Input), &jobsPayload)
			if err != nil {
				e.node.logger.Error("failed to convert jobs payload from async task with id", "id", asyncTask.Id, "error", err.Error())
			}
			jobIds, batchInsertErr := e.node.jobRepo.BatchInsertJobs(jobsPayload)
			if batchInsertErr != nil {
				e.node.logger.Error("failed to create jobs from async task with id", "id", asyncTask.Id, "error", batchInsertErr.Error())
			}
			resObj := utils.Response{Data: jobIds, Success: true}
			updateTaskErr := e.node.asyncTaskManager.UpdateTasksByRequestId(asyncTask.RequestId, models.AsyncTaskSuccess, string(resObj.ToJSON()))
			if updateTaskErr != nil {
				e.node.logger.Error("failed to update state of uncommitted async task", "error", updateTaskErr)
			}
			e.node.logger.Info("successfully created jobs from async task with id", "id", asyncTask.Id, "job-ids", jobIds)
		}
		if asyncTask.State == models.AsyncTaskInProgress && asyncTask.Service == constants.JobExecutorAsyncTaskService {
			err := e.node.asyncTaskManager.UpdateTasksByRequestId(asyncTask.RequestId, models.AsyncTaskSuccess, "")
			if err != nil {
				e.node.logger.Error("failed to update state of uncommitted job executor async tasks to success", "error", err.Message)
			}
		}
	}
}
