package service

import (
	"context"
	"log"
	"scheduler0/config"
	"scheduler0/db"
	"scheduler0/fsm"
	"scheduler0/repository"
	"scheduler0/service/executor"
	"scheduler0/service/node"
	"scheduler0/service/queue"
	"scheduler0/utils"
	"scheduler0/utils/workers"
)

type Service struct {
	Dispatcher         *workers.Dispatcher
	JobService         Job
	ProjectService     Project
	CredentialService  Credential
	JobExecutorService *executor.JobExecutor
	NodeService        *node.Node
	JobQueueService    *queue.JobQueue
}

func NewService(ctx context.Context, logger *log.Logger) *Service {
	configs := config.GetConfigurations(logger)

	schedulerTime := utils.GetSchedulerTime()
	err := schedulerTime.SetTimezone("UTC")
	if err != nil {
		logger.Fatalln("failed to set timezone for s")
	}
	sqliteDb := db.GetDBConnection(logger)
	fsmStr := fsm.NewFSMStore(sqliteDb, logger)

	dispatcher := workers.NewDispatcher(
		int64(configs.IncomingRequestMaxWorkers),
		int64(configs.IncomingRequestMaxQueue),
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

	jobExecutor := executor.NewJobExecutor(ctx, logger, jobRepo, executionsRepo, jobQueueRepo, dispatcher)
	jobQueue := queue.NewJobQueue(ctx, logger, fsmStr, jobExecutor, jobQueueRepo)
	nodeService := node.NewNode(ctx, logger, jobExecutor, jobQueue, jobRepo, projectRepo, executionsRepo, jobQueueRepo)

	nodeService.FsmStore = fsmStr

	service := Service{
		JobService:         NewJobService(ctx, logger, jobRepo, jobQueue, projectRepo, dispatcher),
		ProjectService:     NewProjectService(logger, projectRepo),
		CredentialService:  NewCredentialService(ctx, logger, credentialRepo),
		JobExecutorService: jobExecutor,
		NodeService:        nodeService,
		JobQueueService:    jobQueue,
	}

	service.Dispatcher = dispatcher

	service.Dispatcher.Run()

	return &service
}
