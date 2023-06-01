package async_task_manager

import (
	"context"
	"fmt"
	"github.com/hashicorp/go-hclog"
	"net/http"
	"scheduler0/fsm"
	"scheduler0/models"
	"scheduler0/repository"
	"scheduler0/utils"
	"sync"
)

type AsyncTaskManager struct {
	task                 sync.Map // map[uint64]models.AsyncTask
	taskIdRequestIdMap   sync.Map // map[string]uint64
	subscribers          sync.Map // map[uint64]map[uint64]func(task models.AsyncTask)
	subscriberIds        sync.Map // map[uint64][]uint64
	asyncTaskManagerRepo repository.AsyncTasksRepo
	context              context.Context
	logger               hclog.Logger
	notificationsCh      chan models.AsyncTask
	fsm                  fsm.Scheduler0RaftStore
	SingleNodeMode       bool
}

func NewAsyncTaskManager(context context.Context, logger hclog.Logger, fsm fsm.Scheduler0RaftStore, asyncTaskManagerRepo repository.AsyncTasksRepo) *AsyncTaskManager {
	return &AsyncTaskManager{
		context:              context,
		logger:               logger.Named("async-task-service"),
		asyncTaskManagerRepo: asyncTaskManagerRepo,
		fsm:                  fsm,
		notificationsCh:      make(chan models.AsyncTask, 1),
	}
}

func (m *AsyncTaskManager) AddTasks(input, requestId, service string) ([]uint64, *utils.GenericError) {
	tasks := []models.AsyncTask{
		models.AsyncTask{
			Input:     input,
			RequestId: requestId,
			Service:   service,
		},
	}

	var sids []uint64
	if m.SingleNodeMode {
		ids, err := m.asyncTaskManagerRepo.RaftBatchInsert(tasks)
		if err != nil {
			return nil, err
		}
		sids = ids
	} else {
		f := m.fsm.GetRaft().VerifyLeader()
		if f.Error() != nil {
			ids, err := m.asyncTaskManagerRepo.BatchInsert(tasks, false)
			if err != nil {
				return nil, err
			}
			sids = ids
		} else {
			ids, err := m.asyncTaskManagerRepo.RaftBatchInsert(tasks)
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

func (m *AsyncTaskManager) UpdateTasksById(taskId uint64, state models.AsyncTaskState, output string) *utils.GenericError {
	t, ok := m.task.Load(taskId)
	if !ok {
		m.logger.Error("could not find task with id", "taskI-d", taskId)
		return utils.HTTPGenericError(http.StatusNotFound, fmt.Sprintf("could not find task with id %v", taskId))
	}
	myT := t.(models.AsyncTask)
	myT.State = state
	m.task.Store(taskId, myT)
	if m.SingleNodeMode {
		err := m.asyncTaskManagerRepo.RaftUpdateTaskState(myT, state, output)
		if err != nil {
			m.logger.Error("could not update task with id", taskId)
			return utils.HTTPGenericError(http.StatusNotFound, fmt.Sprintf("could not update task with id %v", taskId))
		}
	} else {
		f := m.fsm.GetRaft().VerifyLeader()
		if f.Error() != nil {
			err := m.asyncTaskManagerRepo.UpdateTaskState(myT, state, output)
			if err != nil {
				m.logger.Error("could not update task with id", taskId)
				return utils.HTTPGenericError(http.StatusNotFound, fmt.Sprintf("could not update task with id %v", taskId))
			}
		} else {
			err := m.asyncTaskManagerRepo.RaftUpdateTaskState(myT, state, output)
			if err != nil {
				m.logger.Error("could not update task with id", taskId)
				return utils.HTTPGenericError(http.StatusNotFound, fmt.Sprintf("could not update task with id %v", taskId))
			}
		}
	}
	go func() { m.notificationsCh <- myT }()
	return nil
}

func (m *AsyncTaskManager) UpdateTasksByRequestId(requestId string, state models.AsyncTaskState, output string) *utils.GenericError {
	tId, ok := m.taskIdRequestIdMap.Load(requestId)
	if !ok {
		m.logger.Error("could not find task id for request id", "request-id", requestId)
		return utils.HTTPGenericError(http.StatusNotFound, fmt.Sprintf("could not find task id for request id %v", requestId))
	}
	t, ok := m.task.Load(tId)
	if !ok {
		m.logger.Error("could not find task with request id task id", requestId)
		return utils.HTTPGenericError(http.StatusNotFound, fmt.Sprintf("could not find task with request id task id %v", requestId))
	}
	myT := t.(models.AsyncTask)
	myT.State = state
	m.task.Store(myT.Id, myT)
	if m.SingleNodeMode {
		err := m.asyncTaskManagerRepo.RaftUpdateTaskState(myT, state, output)
		if err != nil {
			m.logger.Error("could not update task with id", requestId)
			return utils.HTTPGenericError(http.StatusNotFound, fmt.Sprintf("could not update task with id %v", requestId))
		}
	} else {
		f := m.fsm.GetRaft().VerifyLeader()
		if f.Error() != nil {
			err := m.asyncTaskManagerRepo.UpdateTaskState(myT, state, output)
			if err != nil {
				m.logger.Error("could not update task with id", requestId)
				return utils.HTTPGenericError(http.StatusNotFound, fmt.Sprintf("could not update task with id %v", requestId))
			}
		} else {
			err := m.asyncTaskManagerRepo.RaftUpdateTaskState(myT, state, output)
			if err != nil {
				m.logger.Error("could not update task with id", requestId)
				return utils.HTTPGenericError(http.StatusNotFound, fmt.Sprintf("could not update task with id %v", requestId))
			}
		}
	}
	go func() { m.notificationsCh <- myT }()
	return nil
}

func (m *AsyncTaskManager) AddSubscriber(taskId uint64, subscriber func(task models.AsyncTask)) (uint64, *utils.GenericError) {
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

func (m *AsyncTaskManager) GetTaskBlocking(taskId uint64) (chan models.AsyncTask, uint64, *utils.GenericError) {
	task, err := m.asyncTaskManagerRepo.GetTask(taskId)
	if err != nil {
		m.logger.Error("failed to get async task", "error", err.Message)
		return nil, 0, err
	}
	if task.State != models.AsyncTaskInProgress && task.State != models.AsyncTaskNotStated {
		return nil, 0, nil
	}

	var taskCh = make(chan models.AsyncTask, 1)

	subs, addErr := m.AddSubscriber(taskId, func(task models.AsyncTask) {
		taskCh <- task
	})

	if addErr != nil {
		m.logger.Error("failed to add subscriber to async task with id", "task-id", taskId, "error", addErr.Message)
		return nil, 0, addErr
	}

	return taskCh, subs, nil
}

func (m *AsyncTaskManager) GetTaskWithRequestIdNonBlocking(requestId string) (*models.AsyncTask, *utils.GenericError) {
	taskId, ok := m.taskIdRequestIdMap.Load(requestId)
	if !ok {
		m.logger.Error("failed to find async task with request id", "request-id", requestId)
		return nil, utils.HTTPGenericError(http.StatusNotFound, "task doesn't exist")
	}

	task, err := m.asyncTaskManagerRepo.GetTask(taskId.(uint64))
	if err != nil {
		m.logger.Error("failed to get async task", "taskId", taskId, "error", err.Message)
		return nil, err
	}
	return task, nil
}

func (m *AsyncTaskManager) GetTaskWithRequestIdBlocking(requestId string) (chan models.AsyncTask, uint64, *utils.GenericError) {
	taskId, ok := m.taskIdRequestIdMap.Load(requestId)
	if !ok {
		return nil, 0, nil
	}
	return m.GetTaskBlocking(taskId.(uint64))
}

func (m *AsyncTaskManager) GetTaskIdWithRequestId(requestId string) (uint64, *utils.GenericError) {
	taskId, ok := m.taskIdRequestIdMap.Load(requestId)
	if ok {
		return taskId.(uint64), nil
	}
	return 0, nil
}

func (m *AsyncTaskManager) DeleteSubscriber(taskId, subscriberId uint64) *utils.GenericError {
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

func (m *AsyncTaskManager) GetUnCommittedTasks() ([]models.AsyncTask, *utils.GenericError) {
	tasks, err := m.asyncTaskManagerRepo.GetAllTasks(false)
	if err != nil {
		return nil, err
	}
	return tasks, err
}

func (m *AsyncTaskManager) ListenForNotifications() {
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
