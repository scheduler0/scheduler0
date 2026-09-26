package async_task

import (
	"context"
	"fmt"
	"net/http"
	"scheduler0/pkg/config"
	"scheduler0/pkg/fsm"
	"scheduler0/pkg/models"
	"scheduler0/pkg/repository/async_task"
	"scheduler0/pkg/utils"
	"sync"

	"github.com/hashicorp/go-hclog"
)

type AsyncTaskService interface {
	AddTasks(input, requestId, service string, accountId uint64) ([]uint64, *utils.GenericError)
	UpdateTasksById(taskId uint64, state models.AsyncTaskState, output string) *utils.GenericError
	UpdateTasksByRequestId(requestId string, state models.AsyncTaskState, output string) *utils.GenericError
	AddSubscriber(taskId uint64, subscriber func(task models.AsyncTask)) (uint64, *utils.GenericError)
	GetTaskBlocking(taskId uint64) (chan models.AsyncTask, uint64, *utils.GenericError)
	GetTaskWithRequestIdNonBlocking(requestId string, accountId uint64) (*models.AsyncTask, *utils.GenericError)
	GetTaskWithRequestIdBlocking(requestId string, accountId uint64) (chan models.AsyncTask, uint64, *utils.GenericError)
	GetTaskIdWithRequestId(requestId string, accountId uint64) (uint64, *utils.GenericError)
	DeleteSubscriber(taskId, subscriberId uint64) *utils.GenericError
	GetUnCommittedTasks() ([]models.AsyncTask, *utils.GenericError)
	SetSingleNodeMode(singleNodeMode bool)
	GetSingleNodeMode() bool
	SetNodeIsLeader(nodeIsLeader bool)
	GetNodeIsLeader() bool
	ListenForNotifications()
	DeleteNewUncommittedAsyncLogs(lastInsertedId, rowsAffected int64)
}

type asyncTaskService struct {
	task                 sync.Map // map[uint64]models.AsyncTask
	taskIdRequestIdMap   sync.Map // map[string]uint64
	subscribers          sync.Map // map[uint64]map[uint64]func(task models.AsyncTask)
	subscriberIds        sync.Map // map[uint64][]uint64
	asyncTaskManagerRepo async_task.AsyncTasksRepo
	context              context.Context
	logger               hclog.Logger
	notificationsCh      chan models.AsyncTask
	fsm                  fsm.Scheduler0RaftStore
	singleNodeMode       bool
	nodeIsLeader         bool
	mtx                  sync.Mutex
	scheduler0Config     config.Scheduler0Config
}

func NewAsyncTaskManager(
	context context.Context,
	logger hclog.Logger,
	fsm fsm.Scheduler0RaftStore,
	asyncTaskManagerRepo async_task.AsyncTasksRepo,
	scheduler0Config config.Scheduler0Config,
) AsyncTaskService {
	return &asyncTaskService{
		context:              context,
		logger:               logger.Named("async-task-service"),
		asyncTaskManagerRepo: asyncTaskManagerRepo,
		fsm:                  fsm,
		notificationsCh:      make(chan models.AsyncTask, 1),
		scheduler0Config:     scheduler0Config,
	}
}

func (m *asyncTaskService) AddTasks(input, requestId, service string, accountId uint64) ([]uint64, *utils.GenericError) {
	tasks := []models.AsyncTask{
		models.AsyncTask{
			Input:     input,
			RequestId: requestId,
			Service:   service,
			AccountId: accountId,
		},
	}

	config := m.scheduler0Config.GetConfigurations()

	var sids []uint64
	if m.singleNodeMode {
		ids, err := m.asyncTaskManagerRepo.RaftBatchInsert(tasks, config.NodeId)
		if err != nil {
			return nil, err
		}
		sids = ids
	} else {
		if !m.nodeIsLeader {
			ids, err := m.asyncTaskManagerRepo.BatchInsert(tasks, false)
			if err != nil {
				return nil, err
			}
			sids = ids
		} else {
			ids, err := m.asyncTaskManagerRepo.RaftBatchInsert(tasks, config.NodeId)
			if err != nil {
				return nil, err
			}
			sids = ids
		}
	}

	tasks[0].Id = sids[0]
	m.task.Store(sids[0], tasks[0])
	m.taskIdRequestIdMap.Store(tasks[0].RequestId, sids[0])
	return sids, nil
}

func (m *asyncTaskService) UpdateTasksById(taskId uint64, state models.AsyncTaskState, output string) *utils.GenericError {
	t, ok := m.task.Load(taskId)
	if !ok {
		m.logger.Error("could not find task with id", "taskI-d", taskId)
		return utils.HTTPGenericError(http.StatusNotFound, fmt.Sprintf("could not find task with id %v", taskId))
	}
	myT := t.(models.AsyncTask)
	myT.State = state
	m.task.Store(taskId, myT)
	if m.singleNodeMode {
		err := m.asyncTaskManagerRepo.RaftUpdateTaskState(myT, state, output)
		if err != nil {
			m.logger.Error("could not update task with id", "task-id", taskId)
			return utils.HTTPGenericError(http.StatusNotFound, fmt.Sprintf("could not update task with id %v", taskId))
		}
	} else {
		if !m.nodeIsLeader {
			err := m.asyncTaskManagerRepo.UpdateTaskState(myT, state, output)
			if err != nil {
				m.logger.Error("could not update task with id", "task-id", taskId)
				return utils.HTTPGenericError(http.StatusNotFound, fmt.Sprintf("could not update task with id %v", taskId))
			}
		} else {
			err := m.asyncTaskManagerRepo.RaftUpdateTaskState(myT, state, output)
			if err != nil {
				m.logger.Error("could not update task with id", "task-id", taskId)
				return utils.HTTPGenericError(http.StatusNotFound, fmt.Sprintf("could not update task with id %v", taskId))
			}
		}
	}
	go func() { m.notificationsCh <- myT }()
	return nil
}

func (m *asyncTaskService) UpdateTasksByRequestId(requestId string, state models.AsyncTaskState, output string) *utils.GenericError {
	tId, ok := m.taskIdRequestIdMap.Load(requestId)
	if !ok {
		m.logger.Error("could not find task id for request id", "request-id", requestId)
		return utils.HTTPGenericError(http.StatusNotFound, fmt.Sprintf("could not find task id for request id %v", requestId))
	}
	t, ok := m.task.Load(tId)
	if !ok {
		m.logger.Error("could not find task with request id task id", "request-id", requestId)
		return utils.HTTPGenericError(http.StatusNotFound, fmt.Sprintf("could not find task with request id task id %v", requestId))
	}
	myT := t.(models.AsyncTask)
	myT.State = state
	myT.Output = output
	m.task.Store(myT.Id, myT)
	if m.singleNodeMode {
		err := m.asyncTaskManagerRepo.RaftUpdateTaskState(myT, state, output)
		if err != nil {
			m.logger.Error("could not update task with id", "request-id", requestId)
			return utils.HTTPGenericError(http.StatusNotFound, fmt.Sprintf("could not update task with id %v", requestId))
		}
	} else {
		if !m.nodeIsLeader {
			m.logger.Error("error updating async task with request id, node is not leader.", "request-id", requestId)
			err := m.asyncTaskManagerRepo.UpdateTaskState(myT, state, output)
			if err != nil {
				m.logger.Error("could not update task with id", "request-id", requestId)
				return utils.HTTPGenericError(http.StatusNotFound, fmt.Sprintf("could not update task with id %v", requestId))
			}
		} else {
			err := m.asyncTaskManagerRepo.RaftUpdateTaskState(myT, state, output)
			if err != nil {
				m.logger.Error("could not update task with id", "request-id", requestId)
				return utils.HTTPGenericError(http.StatusNotFound, fmt.Sprintf("could not update task with id %v", requestId))
			}
		}
	}
	go func() { m.notificationsCh <- myT }()
	return nil
}

func (m *asyncTaskService) AddSubscriber(taskId uint64, subscriber func(task models.AsyncTask)) (uint64, *utils.GenericError) {
	t, ok := m.task.Load(taskId)
	if !ok {
		m.logger.Error("could not find task with id", "task-id", taskId)
		return 0, utils.HTTPGenericError(http.StatusNotFound, fmt.Sprintf("could not find task with id %d", taskId))
	}
	myt := t.(models.AsyncTask)
	subIds, ok := m.subscriberIds.Load(taskId)
	var maxId uint64 = 0
	if ok {
		maxId = subIds.(uint64)
	}
	subId := maxId + 1

	sb, ok := m.subscribers.Load(myt.Id)
	var subscribers = map[uint64]func(task models.AsyncTask){}
	if ok {
		storedsubs := sb.(map[uint64]func(task models.AsyncTask))
		subscribers = storedsubs
	}
	subscribers[subId] = subscriber
	m.subscribers.Store(taskId, subscribers)
	m.subscriberIds.Store(taskId, subId)

	return subId, nil
}

func (m *asyncTaskService) GetTaskBlocking(taskId uint64) (chan models.AsyncTask, uint64, *utils.GenericError) {
	task, err := m.asyncTaskManagerRepo.GetTask(taskId)
	if err != nil {
		m.logger.Error("failed to get async task", "error", err.Message)
		return nil, 0, err
	}
	var taskCh = make(chan models.AsyncTask, 1)

	if task.State == models.AsyncTaskSuccess {
		taskCh <- *task
		return taskCh, 0, nil
	}

	if task.State != models.AsyncTaskInProgress && task.State != models.AsyncTaskNotStated {
		return nil, 0, nil
	}

	subs, addErr := m.AddSubscriber(taskId, func(task models.AsyncTask) {
		taskCh <- task
	})

	if addErr != nil {
		m.logger.Error("failed to add subscriber to async task with id", "task-id", taskId, "error", addErr.Message)
		return nil, 0, addErr
	}

	return taskCh, subs, nil
}

func (m *asyncTaskService) GetTaskWithRequestIdNonBlocking(requestId string, accountId uint64) (*models.AsyncTask, *utils.GenericError) {
	taskId, ok := m.taskIdRequestIdMap.Load(requestId)
	if !ok {
		m.logger.Info("task not found in map, getting from repo", "requestId", requestId, "accountId", accountId)
		task, err := m.asyncTaskManagerRepo.GetTaskByRequestIdAndAccountId(requestId, accountId)
		if err != nil {
			m.logger.Error("failed to get async task", "requestId", requestId, "accountId", accountId, "error", err.Message)
			return nil, err
		}
		if task == nil {
			return nil, utils.HTTPGenericError(http.StatusNotFound, "task doesn't exist")
		}
		m.logger.Info("task found in repo", "task", task)
		m.task.Store(task.Id, *task)
		m.taskIdRequestIdMap.Store(requestId, task.Id)
		taskId = task.Id
	}

	task, err := m.asyncTaskManagerRepo.GetTask(taskId.(uint64))
	if err != nil {
		m.logger.Error("failed to get async task", "taskId", taskId, "error", err.Message)
		return nil, err
	}
	if task == nil || task.AccountId != accountId {
		return nil, utils.HTTPGenericError(http.StatusNotFound, "task doesn't exist")
	}
	return task, nil
}

func (m *asyncTaskService) GetTaskWithRequestIdBlocking(requestId string, accountId uint64) (chan models.AsyncTask, uint64, *utils.GenericError) {
	taskId, ok := m.taskIdRequestIdMap.Load(requestId)
	if !ok {
		task, err := m.asyncTaskManagerRepo.GetTaskByRequestIdAndAccountId(requestId, accountId)
		if err != nil {
			m.logger.Error("failed to get async task", "requestId", requestId, "accountId", accountId, "error", err.Message)
			return nil, 0, err
		}
		if task == nil {
			return nil, 0, utils.HTTPGenericError(http.StatusNotFound, "task doesn't exist")
		}
		m.task.Store(task.Id, *task)
		m.taskIdRequestIdMap.Store(requestId, task.Id)
		taskId = task.Id

		return m.GetTaskBlocking(task.Id)
	}

	task, err := m.asyncTaskManagerRepo.GetTask(taskId.(uint64))
	if err != nil {
		m.logger.Error("failed to get async task", "taskId", taskId, "error", err.Message)
		return nil, 0, err
	}
	if task == nil || task.AccountId != accountId {
		return nil, 0, utils.HTTPGenericError(http.StatusNotFound, "task doesn't exist")
	}
	return m.GetTaskBlocking(taskId.(uint64))
}

func (m *asyncTaskService) GetTaskIdWithRequestId(requestId string, accountId uint64) (uint64, *utils.GenericError) {
	taskId, ok := m.taskIdRequestIdMap.Load(requestId)
	if ok {
		task, err := m.asyncTaskManagerRepo.GetTask(taskId.(uint64))
		if err != nil {
			return 0, err
		}
		if task == nil || task.AccountId != accountId {
			return 0, utils.HTTPGenericError(http.StatusNotFound, "task doesn't exist")
		}
		return task.Id, nil
	}
	task, err := m.asyncTaskManagerRepo.GetTaskByRequestIdAndAccountId(requestId, accountId)
	if err != nil {
		return 0, err
	}
	if task == nil {
		return 0, utils.HTTPGenericError(http.StatusNotFound, "task doesn't exist")
	}
	return task.Id, nil
}

func (m *asyncTaskService) DeleteSubscriber(taskId, subscriberId uint64) *utils.GenericError {
	t, ok := m.task.Load(taskId)
	if !ok {
		return utils.HTTPGenericError(http.StatusNotFound, fmt.Sprintf("could not find task with id %d", taskId))
	}
	myt := t.(models.AsyncTask)
	sb, ok := m.subscribers.Load(myt.Id)
	if !ok {
		return utils.HTTPGenericError(http.StatusNotFound, fmt.Sprintf("could not find subscribers for task with id %d", taskId))
	}
	subscribers := sb.(map[uint64]func(task models.AsyncTask))
	delete(subscribers, subscriberId)
	m.subscribers.Store(taskId, subscribers)
	return nil
}

func (m *asyncTaskService) GetUnCommittedTasks() ([]models.AsyncTask, *utils.GenericError) {
	tasks, err := m.asyncTaskManagerRepo.GetAllTasks(false)
	if err != nil {
		return nil, err
	}
	return tasks, err
}

func (m *asyncTaskService) SetSingleNodeMode(singleNodeMode bool) {
	m.singleNodeMode = singleNodeMode
}

func (m *asyncTaskService) SetNodeIsLeader(nodeIsLeader bool) {
	m.nodeIsLeader = nodeIsLeader
}

func (m *asyncTaskService) GetSingleNodeMode() bool {
	return m.singleNodeMode
}

func (m *asyncTaskService) ListenForNotifications() {
	go func() {
		for {
			select {
			case taskNotification := <-m.notificationsCh:
				t, ok := m.task.Load(taskNotification.Id)
				if !ok {
					m.logger.Error("could not find task with id", taskNotification.Id)
					return
				}
				myt := t.(models.AsyncTask)
				sb, ok := m.subscribers.Load(myt.Id)

				var subscribers = map[uint64]func(task models.AsyncTask){}
				if ok {
					subscribers = sb.(map[uint64]func(task models.AsyncTask))
				}

				for _, subscriber := range subscribers {
					subscriber(taskNotification)
				}
				if taskNotification.State == models.AsyncTaskFail || taskNotification.State == models.AsyncTaskSuccess {
					m.task.Delete(myt.Id)
				}
				m.subscribers.Delete(myt.Id)
			case <-m.context.Done():
				return
			}
		}
	}()
}

func (m *asyncTaskService) DeleteNewUncommittedAsyncLogs(lastInsertedId, rowsAffected int64) {
}

func (m *asyncTaskService) GetNodeIsLeader() bool {
	return m.nodeIsLeader
}
