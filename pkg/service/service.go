package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	boltdb "github.com/hashicorp/raft-boltdb/v2"
	"log"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"scheduler0/pkg/config"
	"scheduler0/pkg/constants"
	"scheduler0/pkg/db"
	"scheduler0/pkg/fsm"
	"scheduler0/pkg/models"
	"scheduler0/pkg/network"
	async_task_repo "scheduler0/pkg/repository/async_task"
	credential_repo "scheduler0/pkg/repository/credential"
	job_repo "scheduler0/pkg/repository/job"
	job_execution_repo "scheduler0/pkg/repository/job_execution"
	job_queue_repo "scheduler0/pkg/repository/job_queue"
	project_repo "scheduler0/pkg/repository/project"
	"scheduler0/pkg/scheduler0time"
	"scheduler0/pkg/secrets"
	"scheduler0/pkg/service/async_task"
	"scheduler0/pkg/service/credential"
	"scheduler0/pkg/service/executor"
	"scheduler0/pkg/service/executor/executors"
	"scheduler0/pkg/service/job"
	"scheduler0/pkg/service/node"
	"scheduler0/pkg/service/processor"
	"scheduler0/pkg/service/project"
	"scheduler0/pkg/service/queue"
	"scheduler0/pkg/shared_repo"
	"scheduler0/pkg/utils"
	"time"
)

type Service struct {
	Dispatcher         *utils.Dispatcher
	JobService         job.JobService
	ProjectService     project.ProjectService
	CredentialService  credential.CredentialService
	JobExecutorService executor.JobExecutorService
	NodeService        node.NodeService
	JobQueueService    queue.JobQueueService
	AsyncTaskService   async_task.AsyncTaskService
}

func connectRaftLogsAndTransport(scheduler0Config config.Scheduler0Config) (
	*boltdb.BoltStore,
	*boltdb.BoltStore,
	*raft.FileSnapshotStore,
	raft.Transport,
) {
	logger := log.New(os.Stderr, "[get-raft-logs-and-transport] ", log.LstdFlags)

	configs := scheduler0Config.GetConfigurations()
	dirPath := fmt.Sprintf("%v/%v", constants.RaftDir, configs.NodeId)

	ldb, err := boltdb.NewBoltStore(filepath.Join(dirPath, constants.RaftLog))
	if err != nil {
		logger.Fatal("failed to create log store\n", err)
	}
	sdb, err := boltdb.NewBoltStore(filepath.Join(dirPath, constants.RaftStableLog))
	if err != nil {
		logger.Fatal("failed to create stable store\n", err)
	}
	fss, err := raft.NewFileSnapshotStore(dirPath, 3, os.Stderr)
	if err != nil {
		logger.Fatal("failed to create snapshot store\n", err)
	}
	ln, err := net.Listen("tcp", configs.RaftAddress)
	if err != nil {
		logger.Fatalf("failed to listen to tcp net. raft address %v. %v\n", configs.RaftAddress, err)
	}

	adv := network.NameAddress{
		Address: configs.NodeAdvAddress,
	}

	mux := network.NewMux(ln, adv)
	go func() {
		err := mux.Serve()
		if err != nil {
			logger.Fatal("failed mux serve\n", err)
		}
	}()

	muxLn := mux.Listen(1)

	tm := raft.NewNetworkTransport(network.NewTransport(muxLn), int(configs.RaftTransportMaxPool), time.Second*time.Duration(configs.RaftTransportTimeout), nil)
	return ldb, sdb, fss, tm
}

func NewService(ctx context.Context, logger hclog.Logger) *Service {
	logger.Info("Setting Up Services")

	scheduler0Configs := config.NewScheduler0Config()
	scheduler0Secrets := secrets.NewScheduler0Secrets()
	configs := scheduler0Configs.GetConfigurations()

	serviceCtx, cancelServiceContext := context.WithCancel(ctx)

	schedulerTime := scheduler0time.GetSchedulerTime()
	err := schedulerTime.SetTimezone("UTC")
	if err != nil {
		log.Fatal("failed to set timezone for s")
	}
	dispatcher := utils.NewDispatcher(
		serviceCtx,
		int64(configs.MaxWorkers),
		int64(configs.MaxQueue),
	)

	postProcessChannel := make(chan models.PostProcess, 1)

	sqliteDb := db.CreateConnectionFromNewDbIfNonExists(logger)
	sharedRep := shared_repo.NewSharedRepo(logger, scheduler0Configs)
	fsmActions := fsm.NewScheduler0RaftActions(sharedRep, postProcessChannel)

	dirPath := fmt.Sprintf("%v", constants.RaftDir)
	dirPath, exists := utils.MakeDirIfNotExist(dirPath)
	dirPath = fmt.Sprintf("%v/%v", constants.RaftDir, configs.NodeId)
	utils.MakeDirIfNotExist(dirPath)

	ldb, stb, fss, tm := connectRaftLogsAndTransport(scheduler0Configs)
	fsmStr := fsm.NewFSMStore(logger, fsmActions, scheduler0Configs, sqliteDb, ldb, stb, fss, tm, sharedRep)
	//repository
	credentialRepo := credential_repo.NewCredentialRepo(logger, fsmActions, fsmStr)
	jobRepo := job_repo.NewJobRepo(logger, fsmActions, fsmStr)
	projectRepo := project_repo.NewProjectRepo(logger, fsmActions, fsmStr, jobRepo)
	executionsRepo := job_execution_repo.NewExecutionsRepo(logger, fsmActions, fsmStr)
	jobQueueRepo := job_queue_repo.NewJobQueuesRepo(logger, fsmActions, fsmStr)
	asyncTaskRepo := async_task_repo.NewAsyncTasksRepo(serviceCtx, logger, fsmActions, fsmStr)

	asyncTaskService := async_task.NewAsyncTaskManager(serviceCtx, logger, fsmStr, asyncTaskRepo, scheduler0Configs)
	httpJobExecutor := executors.NewHTTTPExecutor(logger, serviceCtx, scheduler0Configs, dispatcher)
	jobExecutor := executor.NewJobExecutor(
		serviceCtx,
		logger,
		scheduler0Configs,
		fsmActions,
		jobRepo,
		executionsRepo,
		jobQueueRepo,
		httpJobExecutor,
		dispatcher,
	)
	jobQueueService := queue.NewJobQueue(serviceCtx, logger, scheduler0Configs, fsmActions, fsmStr, jobQueueRepo)
	nodeHTTPClient := node.NewHTTPClient(logger, scheduler0Configs, scheduler0Secrets)
	jobProcessor := processor.NewJobProcessor(
		ctx,
		logger,
		scheduler0Configs,
		jobRepo,
		projectRepo,
		jobQueueService,
		jobExecutor,
		executionsRepo,
		jobQueueRepo,
	)

	nodeService := node.NewNode(
		serviceCtx,
		logger,
		scheduler0Configs,
		scheduler0Secrets,
		fsmStr,
		fsmActions,
		jobExecutor,
		jobQueueService,
		jobProcessor,
		jobRepo,
		sharedRep,
		executionsRepo,
		asyncTaskService,
		dispatcher,
		nodeHTTPClient,
		postProcessChannel,
		exists,
	)

	service := Service{
		JobService:         job.NewJobService(serviceCtx, logger, jobRepo, jobQueueService, projectRepo, dispatcher, asyncTaskService),
		ProjectService:     project.NewProjectService(logger, projectRepo),
		CredentialService:  credential.NewCredentialService(serviceCtx, logger, scheduler0Secrets, credentialRepo, dispatcher),
		JobExecutorService: jobExecutor,
		NodeService:        nodeService,
		JobQueueService:    jobQueueService,
		AsyncTaskService:   asyncTaskService,
	}

	service.Dispatcher = dispatcher
	service.Dispatcher.Run()
	service.JobExecutorService.ListenForJobsToInvoke()
	service.AsyncTaskService.ListenForNotifications()
	fsmStr.InitRaft()

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
