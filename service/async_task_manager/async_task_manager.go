package async_task_manager

import (
	"context"
	"log"
	"scheduler0/models"
	"scheduler0/repository"
	"scheduler0/utils"
	"sync"
)

type AsyncTaskManager struct {
	task                 sync.Map // map[uint64]models.AsyncTask
	taskIdRequestIdMap   sync.Map // map[string]uint64
	subscribers          sync.Map // map[uint64][]func(task models.AsyncTask)
	asyncTaskManagerRepo repository.AsyncTasksRepo
	context              context.Context
	logger               *log.Logger
	notificationsCh      chan models.AsyncTask
}

func NewAsyncTaskManager(context context.Context, logger *log.Logger, asyncTaskManagerRepo repository.AsyncTasksRepo) *AsyncTaskManager {
	return &AsyncTaskManager{
		context:              context,
		logger:               logger,
		asyncTaskManagerRepo: asyncTaskManagerRepo,
		notificationsCh:      make(chan models.AsyncTask, 1),
	}
}

func (m *AsyncTaskManager) AddTasks(input, requestId string, service string) ([]uint64, *utils.GenericError) {
	tasks := []models.AsyncTask{
		models.AsyncTask{
			Input:     input,
			RequestId: requestId,
			Service:   service,
		},
	}
	ids, err := m.asyncTaskManagerRepo.BatchInsert(tasks)
	if err != nil {
		return nil, err
	}
	tasks[0].Id = ids[0]
	m.task.Store(ids[0], tasks[0])
	m.taskIdRequestIdMap.Store(tasks[0].RequestId, ids[0])
	return ids, nil
}

func (m *AsyncTaskManager) UpdateTasksById(taskId uint64, state models.AsyncTaskState, output string) *utils.GenericError {
	t, ok := m.task.Load(taskId)
	if !ok {
		m.logger.Println("could not find task with id", taskId)
		return nil
	}
	myT := t.(models.AsyncTask)
	myT.State = state
	m.task.Store(taskId, myT)
	err := m.asyncTaskManagerRepo.UpdateTaskState(myT, state, output)
	if err != nil {
		m.logger.Println("could not update task with id", taskId)
		return err
	}
	go func() { m.notificationsCh <- myT }()
	return nil
}

func (m *AsyncTaskManager) UpdateTasksByRequestId(requestId string, state models.AsyncTaskState, output string) *utils.GenericError {
	tId, ok := m.taskIdRequestIdMap.Load(requestId)
	if !ok {
		m.logger.Println("could not find task id for request id", requestId)
		return nil
	}
	t, ok := m.task.Load(tId)
	if !ok {
		m.logger.Println("could not find task with request id task id", requestId)
		return nil
	}
	myT := t.(models.AsyncTask)
	myT.State = state
	m.task.Store(myT.Id, myT)
	err := m.asyncTaskManagerRepo.UpdateTaskState(myT, state, output)
	if err != nil {
		m.logger.Println("could not update task request id", requestId)
		return err
	}
	go func() { m.notificationsCh <- myT }()
	return nil
}

func (m *AsyncTaskManager) AddSubscriber(taskId uint64, subscriber func(task models.AsyncTask)) {
	t, ok := m.task.Load(taskId)
	if !ok {
		m.logger.Println("could not find task with id", taskId)
		return
	}
	myt := t.(models.AsyncTask)
	sb, ok := m.subscribers.Load(myt.Id)
	var subscribers = []func(task models.AsyncTask){}
	if ok {
		storedsubs := sb.([]func(task models.AsyncTask))
		subscribers = storedsubs
	}
	subscribers = append(subscribers, subscriber)
	m.subscribers.Store(taskId, subscribers)
}

func (m *AsyncTaskManager) GetTask(taskId uint64) (*models.AsyncTask, *utils.GenericError) {
	task, err := m.asyncTaskManagerRepo.GetTask(taskId)
	if err != nil {
		return nil, err
	}
	if task.State != models.AsyncTaskInProgress && task.State != models.AsyncTaskNotStated {
		return task, nil
	}

	var taskCh = make(chan models.AsyncTask, 1)

	m.AddSubscriber(taskId, func(task models.AsyncTask) {
		taskCh <- task
	})

	updatedTask := <-taskCh

	return &updatedTask, nil
}

func (m *AsyncTaskManager) GetTaskWithRequestId(requestId string) (*models.AsyncTask, *utils.GenericError) {
	taskId, ok := m.taskIdRequestIdMap.Load(requestId)
	if !ok {
		return nil, nil
	}
	return m.GetTask(taskId.(uint64))
}

func (m *AsyncTaskManager) ListenForNotifications() {
	go func() {
		for {
			select {
			case taskNotification := <-m.notificationsCh:
				t, ok := m.task.Load(taskNotification.Id)
				if !ok {
					m.logger.Println("could not find task with id", taskNotification.Id)
					return
				}
				myt := t.(models.AsyncTask)
				sb, ok := m.subscribers.Load(myt.Id)

				var subscribers = []func(task models.AsyncTask){}
				if ok {
					subscribers = sb.([]func(task models.AsyncTask))
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
