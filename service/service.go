package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/hashicorp/go-hclog"
	"log"
	"runtime"
	"scheduler0/config"
	"scheduler0/db"
	"scheduler0/fsm"
	"scheduler0/repository"
	"scheduler0/service/async_task_manager"
	"scheduler0/service/executor"
	"scheduler0/service/node"
	"scheduler0/service/queue"
	"scheduler0/utils"
	"scheduler0/utils/workers"
	"time"
)

type Service struct {
	Dispatcher         *workers.Dispatcher
	JobService         Job
	ProjectService     Project
	CredentialService  Credential
	JobExecutorService *executor.JobExecutor
	NodeService        *node.Node
	JobQueueService    *queue.JobQueue
	AsyncTaskManager   *async_task_manager.AsyncTaskManager
}

func NewService(ctx context.Context, logger hclog.Logger) *Service {
	configs := config.GetConfigurations()

	serviceCtx, cancelServiceContext := context.WithCancel(ctx)

	schedulerTime := utils.GetSchedulerTime()
	err := schedulerTime.SetTimezone("UTC")
	if err != nil {
		log.Fatal("failed to set timezone for s")
	}
	sqliteDb := db.GetDBConnection(logger)
	fsmStr := fsm.NewFSMStore(sqliteDb, logger)

	dispatcher := workers.NewDispatcher(
		int64(configs.MaxWorkers),
		int64(configs.MaxQueue),
		func(effector func(successChannel, errorChannel chan any), successChannel, errorChannel chan any) {
			effector(successChannel, errorChannel)
		},
	)

	//repository
	credentialRepo := repository.NewCredentialRepo(logger, fsmStr)
	jobRepo := repository.NewJobRepo(logger, fsmStr)
	projectRepo := repository.NewProjectRepo(logger, fsmStr, jobRepo)
	executionsRepo := repository.NewExecutionsRepo(logger, fsmStr)
	jobQueueRepo := repository.NewJobQueuesRepo(logger, fsmStr)
	asyncTaskRepo := repository.NewAsyncTasksRepo(serviceCtx, logger, fsmStr)

	asyncTaskManager := async_task_manager.NewAsyncTaskManager(serviceCtx, logger, fsmStr, asyncTaskRepo)
	jobExecutor := executor.NewJobExecutor(serviceCtx, logger, jobRepo, executionsRepo, jobQueueRepo, dispatcher)
	jobQueue := queue.NewJobQueue(serviceCtx, logger, fsmStr, jobExecutor, jobQueueRepo)
	nodeService := node.NewNode(
		serviceCtx,
		logger,
		jobExecutor,
		jobQueue,
		jobRepo,
		projectRepo,
		executionsRepo,
		jobQueueRepo,
		asyncTaskManager,
		dispatcher,
	)

	nodeService.FsmStore = fsmStr

	service := Service{
		JobService:         NewJobService(serviceCtx, logger, jobRepo, jobQueue, projectRepo, dispatcher, asyncTaskManager),
		ProjectService:     NewProjectService(logger, projectRepo),
		CredentialService:  NewCredentialService(serviceCtx, logger, credentialRepo, dispatcher),
		JobExecutorService: jobExecutor,
		NodeService:        nodeService,
		JobQueueService:    jobQueue,
		AsyncTaskManager:   asyncTaskManager,
	}

	service.Dispatcher = dispatcher
	service.Dispatcher.Run()
	service.JobExecutorService.ListenForJobsToInvoke()
	service.AsyncTaskManager.ListenForNotifications()

	memCheckerCh := make(chan bool, 1)

	memStats := runtime.MemStats{}
	memChecker := utils.NewMemoryLimitChecker(configs.MaxMemory, &memStats, memCheckerCh, time.Duration(1)*time.Second)

	go memChecker.StartMemoryUsageChecker()

	go func() {
		for {
			select {
			case <-memCheckerCh:
				cancelServiceContext()
				memChecker.StopMemoryUsageChecker()
				panic(
					errors.New(
						fmt.Sprintf(
							"stopping internal service due to memory limit exceed mem-limit(Mb) %v mem-usage(Mb) %v",
							configs.MaxMemory,
							memStats.Sys/(1024*1024),
						),
					),
				)
			}
		}
	}()

	return &service
}
