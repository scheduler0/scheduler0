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
	"scheduler0/secrets"
	"scheduler0/shared_repo"
	"scheduler0/utils"
	"time"
)

type Service struct {
	Dispatcher         *utils.Dispatcher
	JobService         JobService
	ProjectService     ProjectService
	CredentialService  CredentialService
	JobExecutorService *JobExecutor
	NodeService        *Node
	JobQueueService    *JobQueue
	AsyncTaskManager   *AsyncTaskManager
}

func NewService(ctx context.Context, logger hclog.Logger) *Service {
	scheduler0Configs := config.NewScheduler0Config()
	scheduler0Secrets := secrets.NewScheduler0Secrets()
	configs := scheduler0Configs.GetConfigurations()

	serviceCtx, cancelServiceContext := context.WithCancel(ctx)

	schedulerTime := utils.GetSchedulerTime()
	err := schedulerTime.SetTimezone("UTC")
	if err != nil {
		log.Fatal("failed to set timezone for s")
	}
	dispatcher := utils.NewDispatcher(
		int64(configs.MaxWorkers),
		int64(configs.MaxQueue),
		func(effector func(successChannel, errorChannel chan any), successChannel, errorChannel chan any) {
			effector(successChannel, errorChannel)
		},
	)
	sqliteDb := db.CreateConnectionFromNewDbIfNonExists(logger)
	sharedRep := shared_repo.NewSharedRepo(logger, scheduler0Configs)
	fsmActions := fsm.NewScheduler0RaftActions(sharedRep)
	fsmStr := fsm.NewFSMStore(logger, fsmActions, sqliteDb)

	//repository
	credentialRepo := repository.NewCredentialRepo(logger, fsmActions, fsmStr)
	jobRepo := repository.NewJobRepo(logger, fsmActions, fsmStr)
	projectRepo := repository.NewProjectRepo(logger, fsmActions, fsmStr, jobRepo)
	executionsRepo := repository.NewExecutionsRepo(logger, fsmActions, fsmStr)
	jobQueueRepo := repository.NewJobQueuesRepo(logger, fsmActions, fsmStr)
	asyncTaskRepo := repository.NewAsyncTasksRepo(serviceCtx, logger, fsmActions, fsmStr)

	asyncTaskManager := NewAsyncTaskManager(serviceCtx, logger, fsmStr, asyncTaskRepo)
	jobExecutor := NewJobExecutor(
		serviceCtx,
		logger,
		scheduler0Configs,
		fsmActions,
		jobRepo,
		executionsRepo,
		jobQueueRepo,
		dispatcher,
	)
	jobQueue := NewJobQueue(serviceCtx, logger, scheduler0Configs, fsmActions, fsmStr, jobQueueRepo)
	nodeService := NewNode(
		serviceCtx,
		logger,
		scheduler0Configs,
		scheduler0Secrets,
		fsmActions,
		jobExecutor,
		jobQueue,
		jobRepo,
		projectRepo,
		executionsRepo,
		jobQueueRepo,
		sharedRep,
		asyncTaskManager,
		dispatcher,
	)

	nodeService.FsmStore = fsmStr

	service := Service{
		JobService:         NewJobService(serviceCtx, logger, jobRepo, jobQueue, projectRepo, dispatcher, asyncTaskManager),
		ProjectService:     NewProjectService(logger, projectRepo),
		CredentialService:  NewCredentialService(serviceCtx, logger, scheduler0Secrets, credentialRepo, dispatcher),
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
