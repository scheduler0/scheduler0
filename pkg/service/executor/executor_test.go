package executor

import (
	"context"
	"fmt"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	"github.com/robfig/cron"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"io/ioutil"
	"os"
	"scheduler0/pkg/config"
	"scheduler0/pkg/constants"
	"scheduler0/pkg/db"
	"scheduler0/pkg/fsm"
	"scheduler0/pkg/models"
	async_task_repo "scheduler0/pkg/repository/async_task"
	job_repo "scheduler0/pkg/repository/job"
	job_execution_repo "scheduler0/pkg/repository/job_execution"
	job_queue_repo "scheduler0/pkg/repository/job_queue"
	project_repo "scheduler0/pkg/repository/project"
	"scheduler0/pkg/scheduler0time"
	"scheduler0/pkg/service/async_task"
	"scheduler0/pkg/service/executor/executors"
	"scheduler0/pkg/service/job"
	"scheduler0/pkg/service/queue"
	"scheduler0/pkg/shared_repo"
	"scheduler0/pkg/utils"
	"testing"
	"time"
)

func Test_JobExecutor_QueueExecutions_JobsLessThanJobMaxBatch(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "job-service-test",
		Level: hclog.LevelFromString("trace"),
	})

	// Create a temporary SQLite database file
	tempFile, err := ioutil.TempFile("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	// Create a new SQLite database connection
	sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.RunMigration()
	sqliteDb.OpenConnectionToExistingDB()

	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)

	// Create a new FSM store
	scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, scheduler0config, sqliteDb, nil, nil, nil, nil, sharedRepo)

	// Create a mock raft cluster
	cluster := raft.MakeClusterCustom(t, &raft.MakeClusterOpts{
		Peers:          1,
		Bootstrap:      true,
		Conf:           raft.DefaultConfig(),
		ConfigStoreFSM: false,
		MakeFSMFunc: func() raft.FSM {
			return scheduler0Store.GetFSM()
		},
	})
	defer cluster.Close()
	cluster.FullyConnect()
	scheduler0Store.UpdateRaft(cluster.Leader())

	ctx, canceler := context.WithCancel(context.Background())
	defer canceler()

	jobRepo := job_repo.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)
	projectRepo := project_repo.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)
	asyncTaskManagerRepo := async_task_repo.NewAsyncTasksRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)
	asyncTaskManager := async_task.NewAsyncTaskManager(ctx, logger, scheduler0Store, asyncTaskManagerRepo, scheduler0config)
	jobQueueRepo := job_queue_repo.NewJobQueuesRepo(logger, scheduler0RaftActions, scheduler0Store)
	jobExecutionsRepo := job_execution_repo.NewExecutionsRepo(
		logger,
		scheduler0RaftActions,
		scheduler0Store,
	)

	dispatcher := utils.NewDispatcher(
		ctx,
		int64(1),
		int64(1),
	)

	dispatcher.Run()

	queueRepo := queue.NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, jobQueueRepo)
	queueRepo.SetSingleNodeMode(true)
	// Create a new JobService instance
	jobService := job.NewJobService(ctx, logger, jobRepo, queueRepo, projectRepo, dispatcher, asyncTaskManager)

	httpJobExecutor := executors.NewMockHTTPExecutor(t)

	service := NewJobExecutor(
		ctx,
		logger,
		scheduler0config,
		scheduler0RaftActions,
		jobRepo,
		jobExecutionsRepo,
		jobQueueRepo,
		httpJobExecutor,
		dispatcher,
	)

	service.SetSingleNodeMode(true)
	asyncTaskManager.SetSingleNodeMode(true)
	asyncTaskManager.ListenForNotifications()

	// Define the input jobs
	jobs := []models.Job{
		{
			ID:        1,
			Spec:      "* * * * *",
			Timezone:  "UTC",
			ProjectID: 1,
		},
		{
			ID:        2,
			Spec:      "0 0 * * *",
			Timezone:  "America/New_York",
			ProjectID: 2,
		},
	}

	// Create the projects using the project repo
	for _, job := range jobs {
		project := models.Project{
			ID:          job.ProjectID,
			Name:        fmt.Sprintf("Project %d", job.ProjectID),
			Description: fmt.Sprintf("Project %d description", job.ProjectID),
		}
		_, createErr := projectRepo.CreateOne(&project)
		if createErr != nil {
			t.Fatalf("Failed to create project: %v", createErr)
		}
	}

	// Call the BatchInsertJobs method of the job service
	_, batchErr := jobService.BatchInsertJobs("request123", jobs)
	if batchErr != nil {
		t.Fatalf("Failed to insert jobs: %v", batchErr)
	}

	time.Sleep(time.Second * time.Duration(1))

	serr := os.Setenv("SCHEDULER0_NODE_ID", "1")
	defer os.Unsetenv("SCHEDULER0_NODE_ID")
	if serr != nil {
		t.Fatal("failed to set env", serr)
	}

	service.QueueExecutions(
		int64(1),
		int64(2),
	)

	_, ok := service.GetScheduledJobs().Load(jobs[0].ID)
	assert.Equal(t, true, ok)
	_, ok = service.GetScheduledJobs().Load(jobs[1].ID)
	assert.Equal(t, true, ok)
}

func Test_JobExecutor_QueueExecutions_JobsMoreThanJobMaxBatch(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "job-service-test",
		Level: hclog.LevelFromString("trace"),
	})

	// Create a temporary SQLite database file
	tempFile, err := ioutil.TempFile("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	// Create a new SQLite database connection
	sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.RunMigration()
	sqliteDb.OpenConnectionToExistingDB()

	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)

	// Create a new FSM store
	scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, scheduler0config, sqliteDb, nil, nil, nil, nil, sharedRepo)

	// Create a mock raft cluster
	cluster := raft.MakeClusterCustom(t, &raft.MakeClusterOpts{
		Peers:          1,
		Bootstrap:      true,
		Conf:           raft.DefaultConfig(),
		ConfigStoreFSM: false,
		MakeFSMFunc: func() raft.FSM {
			return scheduler0Store.GetFSM()
		},
	})
	defer cluster.Close()
	cluster.FullyConnect()
	scheduler0Store.UpdateRaft(cluster.Leader())

	ctx, canceler := context.WithCancel(context.Background())
	defer canceler()

	jobRepo := job_repo.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)
	projectRepo := project_repo.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)
	asyncTaskManagerRepo := async_task_repo.NewAsyncTasksRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)
	asyncTaskManager := async_task.NewAsyncTaskManager(ctx, logger, scheduler0Store, asyncTaskManagerRepo, scheduler0config)
	jobQueueRepo := job_queue_repo.NewJobQueuesRepo(logger, scheduler0RaftActions, scheduler0Store)
	jobExecutionsRepo := job_execution_repo.NewExecutionsRepo(
		logger,
		scheduler0RaftActions,
		scheduler0Store,
	)

	dispatcher := utils.NewDispatcher(
		ctx,
		int64(1),
		int64(1),
	)

	dispatcher.Run()

	queueRepo := queue.NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, jobQueueRepo)
	queueRepo.SetSingleNodeMode(true)
	// Create a new JobService instance
	jobService := job.NewJobService(ctx, logger, jobRepo, queueRepo, projectRepo, dispatcher, asyncTaskManager)

	httpJobExecutor := executors.NewMockHTTPExecutor(t)

	service := NewJobExecutor(
		ctx,
		logger,
		scheduler0config,
		scheduler0RaftActions,
		jobRepo,
		jobExecutionsRepo,
		jobQueueRepo,
		httpJobExecutor,
		dispatcher,
	)

	asyncTaskManager.SetSingleNodeMode(true)
	asyncTaskManager.ListenForNotifications()

	// Define the input jobs
	jobs := []models.Job{}

	i := 1
	for i < constants.JobMaxBatchSize+100 {
		jobs = append(jobs, models.Job{
			ID:        uint64(i),
			Spec:      "0 0 * * *",
			Timezone:  "America/New_York",
			ProjectID: 1,
		})
		i++
	}

	// Create the projects using the project repo
	project := models.Project{
		ID:          1,
		Name:        fmt.Sprintf("Project %d", 1),
		Description: fmt.Sprintf("Project %d description", 1),
	}
	_, createErr := projectRepo.CreateOne(&project)
	if createErr != nil {
		t.Fatalf("Failed to create project: %v", createErr)
	}

	// Call the BatchInsertJobs method of the job service
	_, batchErr := jobService.BatchInsertJobs("request123", jobs)
	if batchErr != nil {
		t.Fatalf("Failed to insert jobs: %v", batchErr)
	}

	time.Sleep(time.Second * time.Duration(1))

	serr := os.Setenv("SCHEDULER0_NODE_ID", "1")
	defer os.Unsetenv("SCHEDULER0_NODE_ID")
	if serr != nil {
		t.Fatal("failed to set env", serr)
	}
	service.QueueExecutions(
		int64(1),
		int64(constants.JobMaxBatchSize+100),
	)

	i = 0
	for i < constants.JobMaxBatchSize+99 {
		_, ok := service.GetScheduledJobs().Load(jobs[i].ID)
		assert.Equal(t, true, ok)
		i++
	}
}

func Test_JobExecutor_QueueExecutions_DoesNotQueueJobsForOtherServers(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "job-service-test",
		Level: hclog.LevelFromString("trace"),
	})

	// Create a temporary SQLite database file
	tempFile, err := ioutil.TempFile("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	// Create a new SQLite database connection
	sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.RunMigration()
	sqliteDb.OpenConnectionToExistingDB()

	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)

	// Create a new FSM store
	scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, scheduler0config, sqliteDb, nil, nil, nil, nil, sharedRepo)

	// Create a mock raft cluster
	cluster := raft.MakeClusterCustom(t, &raft.MakeClusterOpts{
		Peers:          1,
		Bootstrap:      true,
		Conf:           raft.DefaultConfig(),
		ConfigStoreFSM: false,
		MakeFSMFunc: func() raft.FSM {
			return scheduler0Store.GetFSM()
		},
	})
	defer cluster.Close()
	cluster.FullyConnect()
	scheduler0Store.UpdateRaft(cluster.Leader())

	ctx, canceler := context.WithCancel(context.Background())
	defer canceler()

	jobRepo := job_repo.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)
	projectRepo := project_repo.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)
	asyncTaskManagerRepo := async_task_repo.NewAsyncTasksRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)
	asyncTaskManager := async_task.NewAsyncTaskManager(ctx, logger, scheduler0Store, asyncTaskManagerRepo, scheduler0config)
	jobQueueRepo := job_queue_repo.NewJobQueuesRepo(logger, scheduler0RaftActions, scheduler0Store)
	jobExecutionsRepo := job_execution_repo.NewExecutionsRepo(
		logger,
		scheduler0RaftActions,
		scheduler0Store,
	)

	dispatcher := utils.NewDispatcher(
		ctx,
		int64(1),
		int64(1),
	)

	dispatcher.Run()

	queueRepo := queue.NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, jobQueueRepo)
	// Create a new JobService instance
	jobService := job.NewJobService(ctx, logger, jobRepo, queueRepo, projectRepo, dispatcher, asyncTaskManager)

	httpJobExecutor := executors.NewMockHTTPExecutor(t)

	service := NewJobExecutor(
		ctx,
		logger,
		scheduler0config,
		scheduler0RaftActions,
		jobRepo,
		jobExecutionsRepo,
		jobQueueRepo,
		httpJobExecutor,
		dispatcher,
	)

	asyncTaskManager.SetSingleNodeMode(true)
	asyncTaskManager.ListenForNotifications()

	// Define the input jobs
	jobs := []models.Job{}

	i := 1
	for i < constants.JobMaxBatchSize+100 {
		jobs = append(jobs, models.Job{
			ID:        uint64(i),
			Spec:      "0 0 * * *",
			Timezone:  "America/New_York",
			ProjectID: 1,
		})
		i++
	}

	// Create the projects using the project repo
	project := models.Project{
		ID:          1,
		Name:        fmt.Sprintf("Project %d", 1),
		Description: fmt.Sprintf("Project %d description", 1),
	}
	_, createErr := projectRepo.CreateOne(&project)
	if createErr != nil {
		t.Fatalf("Failed to create project: %v", createErr)
	}

	// Call the BatchInsertJobs method of the job service
	_, batchErr := jobService.BatchInsertJobs("request123", jobs)
	if batchErr != nil {
		t.Fatalf("Failed to insert jobs: %v", batchErr)
	}

	time.Sleep(time.Second * time.Duration(4))

	serr := os.Setenv("SCHEDULER0_NODE_ID", "2")
	defer os.Unsetenv("SCHEDULER0_NODE_ID")
	if serr != nil {
		t.Fatal("failed to set env", serr)
	}

	service.QueueExecutions(
		int64(1),
		int64(constants.JobMaxBatchSize+100),
	)

	i = 0
	for i < constants.JobMaxBatchSize+99 {
		_, ok := service.GetScheduledJobs().Load(jobs[i].ID)
		assert.Equal(t, ok, false)
		i++
	}
}

func Test_JobExecutor_ScheduleJobs_WithScheduledStateAsLastKnowState_NextTimeExecution_One(t *testing.T) {
	testCases := []struct {
		JobState models.JobExecutionLogState
	}{{
		JobState: models.ExecutionLogScheduleState,
	}, {
		JobState: models.ExecutionLogFailedState,
	}, {
		JobState: models.ExecutionLogSuccessState,
	}}

	for _, testCase := range testCases {
		logger := hclog.New(&hclog.LoggerOptions{
			Name:  "job-service-test",
			Level: hclog.LevelFromString("trace"),
		})

		// Create a temporary SQLite database file
		tempFile, err := ioutil.TempFile("", "test-db")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		defer os.Remove(tempFile.Name())

		// Create a new SQLite database connection
		sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
		sqliteDb.RunMigration()
		sqliteDb.OpenConnectionToExistingDB()

		scheduler0config := config.NewScheduler0Config()
		sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
		scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)

		// Create a new FSM store
		scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, scheduler0config, sqliteDb, nil, nil, nil, nil, sharedRepo)

		// Create a mock raft cluster
		cluster := raft.MakeClusterCustom(t, &raft.MakeClusterOpts{
			Peers:          1,
			Bootstrap:      true,
			Conf:           raft.DefaultConfig(),
			ConfigStoreFSM: false,
			MakeFSMFunc: func() raft.FSM {
				return scheduler0Store.GetFSM()
			},
		})
		defer cluster.Close()
		cluster.FullyConnect()
		scheduler0Store.UpdateRaft(cluster.Leader())

		ctx, canceler := context.WithCancel(context.Background())
		defer canceler()

		jobRepo := job_repo.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)
		projectRepo := project_repo.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)
		asyncTaskManagerRepo := async_task_repo.NewAsyncTasksRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)
		asyncTaskManager := async_task.NewAsyncTaskManager(ctx, logger, scheduler0Store, asyncTaskManagerRepo, scheduler0config)
		jobQueueRepo := job_queue_repo.NewJobQueuesRepo(logger, scheduler0RaftActions, scheduler0Store)
		jobExecutionsRepo := job_execution_repo.NewExecutionsRepo(
			logger,
			scheduler0RaftActions,
			scheduler0Store,
		)

		dispatcher := utils.NewDispatcher(
			ctx,
			int64(1),
			int64(1),
		)

		dispatcher.Run()

		queueRepo := queue.NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, jobQueueRepo)
		queueRepo.SetSingleNodeMode(true)
		// Create a new JobService instance
		jobService := job.NewJobService(ctx, logger, jobRepo, queueRepo, projectRepo, dispatcher, asyncTaskManager)

		httpJobExecutor := executors.NewMockHTTPExecutor(t)

		service := NewJobExecutor(
			ctx,
			logger,
			scheduler0config,
			scheduler0RaftActions,
			jobRepo,
			jobExecutionsRepo,
			jobQueueRepo,
			httpJobExecutor,
			dispatcher,
		)

		asyncTaskManager.SetSingleNodeMode(true)
		asyncTaskManager.ListenForNotifications()

		// Define the input jobs
		jobs := []models.Job{}

		schedule, parseErr := cron.Parse("@every 1m")
		if parseErr != nil {
			t.Fatal("cron spec error", parseErr)
		}

		schedulerTime := scheduler0time.GetSchedulerTime()
		now := schedulerTime.GetTime(time.Now())
		lastTime := now
		nextTime := schedule.Next(lastTime)

		i := 1
		for i < 100 {
			jobs = append(jobs, models.Job{
				ID:        uint64(i),
				Spec:      "@every 1m",
				Timezone:  "America/New_York",
				ProjectID: 1,
			})
			i++
		}

		// Create the projects using the project repo
		project := models.Project{
			ID:          1,
			Name:        fmt.Sprintf("Project %d", 1),
			Description: fmt.Sprintf("Project %d description", 1),
		}
		_, createErr := projectRepo.CreateOne(&project)
		if createErr != nil {
			t.Fatalf("Failed to create project: %v", createErr)
		}

		// Call the BatchInsertJobs method of the job service
		_, batchErr := jobService.BatchInsertJobs("request123", jobs)
		if batchErr != nil {
			t.Fatalf("Failed to insert jobs: %v", batchErr)
		}

		time.Sleep(time.Second * time.Duration(1))

		serr := os.Setenv("SCHEDULER0_NODE_ID", "1")
		defer os.Unsetenv("SCHEDULER0_NODE_ID")
		if serr != nil {
			t.Fatal("failed to set env", serr)
		}
		uncommittedExecutionsLogs := []models.JobExecutionLog{}
		i = 0
		for i < 24 {
			uncommittedExecutionsLogs = append(uncommittedExecutionsLogs, models.JobExecutionLog{
				JobId:                 jobs[i].ID,
				UniqueId:              fmt.Sprintf("%d-%d", jobs[i].ID, i),
				State:                 testCase.JobState,
				LastExecutionDatetime: lastTime,
				NextExecutionDatetime: nextTime,
			})
			i++
		}
		err = sharedRepo.InsertExecutionLogs(sqliteDb, false, uncommittedExecutionsLogs)
		if err != nil {
			t.Fatal("failed to insert execution logs", err)
		}

		committedExecutionsLogs := []models.JobExecutionLog{}
		i = 24
		for i < 50 {
			committedExecutionsLogs = append(uncommittedExecutionsLogs, models.JobExecutionLog{
				JobId:                 jobs[i].ID,
				UniqueId:              fmt.Sprintf("%d-%d", jobs[i].ID, i),
				State:                 testCase.JobState,
				LastExecutionDatetime: lastTime,
				NextExecutionDatetime: nextTime,
			})
			i++
		}
		err = sharedRepo.InsertExecutionLogs(sqliteDb, true, committedExecutionsLogs)
		if err != nil {
			t.Fatal("failed to insert execution logs", err)
		}
		service.QueueExecutions(
			int64(1),
			int64(100),
		)

		i = 0
		for i < 99 {
			sched, ok := service.GetScheduledJobs().Load(jobs[i].ID)
			scheduler := sched.(models.JobSchedule)
			assert.Equal(t, scheduler.ExecutionTime.Sub(nextTime).Round(1*time.Second) < time.Duration(1)*time.Second, true)
			assert.Equal(t, scheduler.ExecutionTime.Sub(nextTime).Round(1*time.Second) > time.Duration(-1)*time.Second, true)
			assert.Equal(t, ok, true)
			i++
		}
	}
}

func Test_JobExecutor_ScheduleJobs_WithScheduledStateAsLastKnowState_NextTimeExecution_Two(t *testing.T) {
	testCases := []struct {
		JobState models.JobExecutionLogState
	}{{
		JobState: models.ExecutionLogScheduleState,
	}, {
		JobState: models.ExecutionLogFailedState,
	}, {
		JobState: models.ExecutionLogSuccessState,
	}}

	for _, testCase := range testCases {
		logger := hclog.New(&hclog.LoggerOptions{
			Name:  "job-service-test",
			Level: hclog.LevelFromString("trace"),
		})

		// Create a temporary SQLite database file
		tempFile, err := ioutil.TempFile("", "test-db")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		defer os.Remove(tempFile.Name())

		// Create a new SQLite database connection
		sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
		sqliteDb.RunMigration()
		sqliteDb.OpenConnectionToExistingDB()

		scheduler0config := config.NewScheduler0Config()
		sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
		scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)

		// Create a new FSM store
		scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, scheduler0config, sqliteDb, nil, nil, nil, nil, sharedRepo)

		// Create a mock raft cluster
		cluster := raft.MakeClusterCustom(t, &raft.MakeClusterOpts{
			Peers:          1,
			Bootstrap:      true,
			Conf:           raft.DefaultConfig(),
			ConfigStoreFSM: false,
			MakeFSMFunc: func() raft.FSM {
				return scheduler0Store.GetFSM()
			},
		})
		defer cluster.Close()
		cluster.FullyConnect()
		scheduler0Store.UpdateRaft(cluster.Leader())

		ctx, canceler := context.WithCancel(context.Background())
		defer canceler()

		jobRepo := job_repo.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)
		projectRepo := project_repo.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)
		asyncTaskManagerRepo := async_task_repo.NewAsyncTasksRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)
		asyncTaskManager := async_task.NewAsyncTaskManager(ctx, logger, scheduler0Store, asyncTaskManagerRepo, scheduler0config)
		jobQueueRepo := job_queue_repo.NewJobQueuesRepo(logger, scheduler0RaftActions, scheduler0Store)
		jobExecutionsRepo := job_execution_repo.NewExecutionsRepo(
			logger,
			scheduler0RaftActions,
			scheduler0Store,
		)

		dispatcher := utils.NewDispatcher(
			ctx,
			int64(1),
			int64(1),
		)

		dispatcher.Run()

		queueRepo := queue.NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, jobQueueRepo)
		queueRepo.SetSingleNodeMode(true)
		// Create a new JobService instance
		jobService := job.NewJobService(ctx, logger, jobRepo, queueRepo, projectRepo, dispatcher, asyncTaskManager)

		httpJobExecutor := executors.NewMockHTTPExecutor(t)

		service := NewJobExecutor(
			ctx,
			logger,
			scheduler0config,
			scheduler0RaftActions,
			jobRepo,
			jobExecutionsRepo,
			jobQueueRepo,
			httpJobExecutor,
			dispatcher,
		)

		asyncTaskManager.SetSingleNodeMode(true)
		asyncTaskManager.ListenForNotifications()

		// Define the input jobs
		jobs := []models.Job{}

		schedule, parseErr := cron.Parse("@every 1h")
		if parseErr != nil {
			t.Fatal("cron spec error", parseErr)
		}

		schedulerTime := scheduler0time.GetSchedulerTime()
		now := schedulerTime.GetTime(time.Now())
		nextTime := schedule.Next(now)
		prevNextTime := nextTime.Add(-nextTime.Sub(now))
		lastTime := nextTime.Add(-nextTime.Sub(now)).Add(-nextTime.Sub(now))

		fmt.Printf("now:: %v, lastTime:: %v, prevNextTime:: %v, nextTime:: %v ", now, lastTime, prevNextTime, nextTime)

		i := 1
		for i < 100 {
			jobs = append(jobs, models.Job{
				ID:        uint64(i),
				Spec:      "@every 1h",
				Timezone:  "America/New_York",
				ProjectID: 1,
			})
			i++
		}

		// Create the projects using the project repo
		project := models.Project{
			ID:          1,
			Name:        fmt.Sprintf("Project %d", 1),
			Description: fmt.Sprintf("Project %d description", 1),
		}
		_, createErr := projectRepo.CreateOne(&project)
		if createErr != nil {
			t.Fatalf("Failed to create project: %v", createErr)
		}

		// Call the BatchInsertJobs method of the job service
		_, batchErr := jobService.BatchInsertJobs("request123", jobs)
		if batchErr != nil {
			t.Fatalf("Failed to insert jobs: %v", batchErr)
		}

		time.Sleep(time.Second * time.Duration(4))

		serr := os.Setenv("SCHEDULER0_NODE_ID", "1")
		defer os.Unsetenv("SCHEDULER0_NODE_ID")
		if serr != nil {
			t.Fatal("failed to set env", serr)
		}
		uncommittedExecutionsLogs := []models.JobExecutionLog{}
		i = 0
		for i < 24 {
			uncommittedExecutionsLogs = append(uncommittedExecutionsLogs, models.JobExecutionLog{
				JobId:                 jobs[i].ID,
				UniqueId:              fmt.Sprintf("%d-%d", jobs[i].ID, i),
				State:                 testCase.JobState,
				LastExecutionDatetime: lastTime,
				NextExecutionDatetime: prevNextTime,
			})
			i++
		}
		err = sharedRepo.InsertExecutionLogs(sqliteDb, false, uncommittedExecutionsLogs)
		if err != nil {
			t.Fatal("failed to insert execution logs", err)
		}

		committedExecutionsLogs := []models.JobExecutionLog{}
		i = 24
		for i < 50 {
			committedExecutionsLogs = append(uncommittedExecutionsLogs, models.JobExecutionLog{
				JobId:                 jobs[i].ID,
				UniqueId:              fmt.Sprintf("%d-%d", jobs[i].ID, i),
				State:                 testCase.JobState,
				LastExecutionDatetime: lastTime,
				NextExecutionDatetime: prevNextTime,
			})
			i++
		}
		err = sharedRepo.InsertExecutionLogs(sqliteDb, true, committedExecutionsLogs)
		if err != nil {
			t.Fatal("failed to insert execution logs", err)
		}
		service.QueueExecutions(
			int64(1),
			int64(100),
		)

		i = 0
		for i < 99 {
			sched, ok := service.GetScheduledJobs().Load(jobs[i].ID)
			scheduler := sched.(models.JobSchedule)
			assert.Equal(t, scheduler.ExecutionTime.Sub(nextTime.Add(time.Duration(4)*time.Second)).Round(time.Minute*1) < time.Duration(1)*time.Second, true)
			assert.Equal(t, ok, true)
			i++
		}
	}
}

func Test_JobExecutor_ScheduleJobs_WithScheduledStateAsLastKnowState_NextTimeExecution_Three(t *testing.T) {
	testCases := []struct {
		JobState models.JobExecutionLogState
	}{{
		JobState: models.ExecutionLogScheduleState,
	}, {
		JobState: models.ExecutionLogFailedState,
	}, {
		JobState: models.ExecutionLogSuccessState,
	}}

	for _, testCase := range testCases {
		logger := hclog.New(&hclog.LoggerOptions{
			Name:  "job-service-test",
			Level: hclog.LevelFromString("trace"),
		})

		// Create a temporary SQLite database file
		tempFile, err := ioutil.TempFile("", "test-db")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		defer os.Remove(tempFile.Name())

		// Create a new SQLite database connection
		sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
		sqliteDb.RunMigration()
		sqliteDb.OpenConnectionToExistingDB()

		scheduler0config := config.NewScheduler0Config()
		sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
		scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)

		// Create a new FSM store
		scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, scheduler0config, sqliteDb, nil, nil, nil, nil, sharedRepo)

		// Create a mock raft cluster
		cluster := raft.MakeClusterCustom(t, &raft.MakeClusterOpts{
			Peers:          1,
			Bootstrap:      true,
			Conf:           raft.DefaultConfig(),
			ConfigStoreFSM: false,
			MakeFSMFunc: func() raft.FSM {
				return scheduler0Store.GetFSM()
			},
		})
		defer cluster.Close()
		cluster.FullyConnect()
		scheduler0Store.UpdateRaft(cluster.Leader())

		ctx, canceler := context.WithCancel(context.Background())
		defer canceler()

		jobRepo := job_repo.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)
		projectRepo := project_repo.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)
		asyncTaskManagerRepo := async_task_repo.NewAsyncTasksRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)
		asyncTaskManager := async_task.NewAsyncTaskManager(ctx, logger, scheduler0Store, asyncTaskManagerRepo, scheduler0config)
		jobQueueRepo := job_queue_repo.NewJobQueuesRepo(logger, scheduler0RaftActions, scheduler0Store)
		jobExecutionsRepo := job_execution_repo.NewExecutionsRepo(
			logger,
			scheduler0RaftActions,
			scheduler0Store,
		)

		dispatcher := utils.NewDispatcher(
			ctx,
			int64(1),
			int64(1),
		)

		dispatcher.Run()

		queueRepo := queue.NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, jobQueueRepo)
		queueRepo.SetSingleNodeMode(true)
		// Create a new JobService instance
		jobService := job.NewJobService(ctx, logger, jobRepo, queueRepo, projectRepo, dispatcher, asyncTaskManager)
		httpJobExecutor := executors.NewMockHTTPExecutor(t)

		service := NewJobExecutor(
			ctx,
			logger,
			scheduler0config,
			scheduler0RaftActions,
			jobRepo,
			jobExecutionsRepo,
			jobQueueRepo,
			httpJobExecutor,
			dispatcher,
		)

		asyncTaskManager.SetSingleNodeMode(true)
		asyncTaskManager.ListenForNotifications()

		// Define the input jobs
		jobs := []models.Job{}

		schedule, parseErr := cron.Parse("@every 1h")
		if parseErr != nil {
			t.Fatal("cron spec error", parseErr)
		}

		schedulerTime := scheduler0time.GetSchedulerTime()
		now := schedulerTime.GetTime(time.Now())
		nextTime := schedule.Next(now)
		prevNextTime := nextTime.Add(-nextTime.Sub(now)).Add(-nextTime.Sub(now)).Add(-nextTime.Sub(now))
		lastTime := nextTime.Add(-nextTime.Sub(now)).Add(-nextTime.Sub(now)).Add(-nextTime.Sub(now)).Add(-nextTime.Sub(now))

		fmt.Printf("now:: %v, lastTime:: %v, prevNextTime:: %v, nextTime:: %v ", now, lastTime, prevNextTime, nextTime)

		i := 1
		for i < 100 {
			jobs = append(jobs, models.Job{
				ID:        uint64(i),
				Spec:      "@every 1h",
				Timezone:  "America/New_York",
				ProjectID: 1,
			})
			i++
		}

		// Create the projects using the project repo
		project := models.Project{
			ID:          1,
			Name:        fmt.Sprintf("Project %d", 1),
			Description: fmt.Sprintf("Project %d description", 1),
		}
		_, createErr := projectRepo.CreateOne(&project)
		if createErr != nil {
			t.Fatalf("Failed to create project: %v", createErr)
		}

		// Call the BatchInsertJobs method of the job service
		_, batchErr := jobService.BatchInsertJobs("request123", jobs)
		if batchErr != nil {
			t.Fatalf("Failed to insert jobs: %v", batchErr)
		}

		time.Sleep(time.Second * time.Duration(4))

		serr := os.Setenv("SCHEDULER0_NODE_ID", "1")
		defer os.Unsetenv("SCHEDULER0_NODE_ID")
		if serr != nil {
			t.Fatal("failed to set env", serr)
		}
		uncommittedExecutionsLogs := []models.JobExecutionLog{}
		i = 0
		for i < 24 {
			uncommittedExecutionsLogs = append(uncommittedExecutionsLogs, models.JobExecutionLog{
				JobId:                 jobs[i].ID,
				UniqueId:              fmt.Sprintf("%d-%d", jobs[i].ID, i),
				State:                 testCase.JobState,
				LastExecutionDatetime: lastTime,
				NextExecutionDatetime: prevNextTime,
			})
			i++
		}
		err = sharedRepo.InsertExecutionLogs(sqliteDb, false, uncommittedExecutionsLogs)
		if err != nil {
			t.Fatal("failed to insert execution logs", err)
		}

		committedExecutionsLogs := []models.JobExecutionLog{}
		i = 24
		for i < 50 {
			committedExecutionsLogs = append(uncommittedExecutionsLogs, models.JobExecutionLog{
				JobId:                 jobs[i].ID,
				UniqueId:              fmt.Sprintf("%d-%d", jobs[i].ID, i),
				State:                 testCase.JobState,
				LastExecutionDatetime: lastTime,
				NextExecutionDatetime: prevNextTime,
			})
			i++
		}
		err = sharedRepo.InsertExecutionLogs(sqliteDb, true, committedExecutionsLogs)
		if err != nil {
			t.Fatal("failed to insert execution logs", err)
		}
		service.QueueExecutions(
			int64(1),
			int64(100),
		)

		i = 0
		for i < 99 {
			sched, ok := service.GetScheduledJobs().Load(jobs[i].ID)
			scheduler := sched.(models.JobSchedule)
			assert.Equal(t, scheduler.ExecutionTime.Sub(nextTime.Add(time.Duration(4)*time.Second)).Round(time.Minute*1) < time.Duration(1)*time.Second, true)
			assert.Equal(t, ok, true)
			i++
		}
	}
}

func Test_ListenForJobsToInvoke(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "job-service-test",
		Level: hclog.LevelFromString("trace"),
	})

	// Create a temporary SQLite database file
	tempFile, err := ioutil.TempFile("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	// Create a new SQLite database connection
	sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.RunMigration()
	sqliteDb.OpenConnectionToExistingDB()

	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)

	// Create a new FSM store
	scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, scheduler0config, sqliteDb, nil, nil, nil, nil, sharedRepo)

	// Create a mock raft cluster
	cluster := raft.MakeClusterCustom(t, &raft.MakeClusterOpts{
		Peers:          1,
		Bootstrap:      true,
		Conf:           raft.DefaultConfig(),
		ConfigStoreFSM: false,
		MakeFSMFunc: func() raft.FSM {
			return scheduler0Store.GetFSM()
		},
	})
	defer cluster.Close()
	cluster.FullyConnect()
	scheduler0Store.UpdateRaft(cluster.Leader())

	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	jobRepo := job_repo.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)
	projectRepo := project_repo.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)
	asyncTaskManagerRepo := async_task_repo.NewAsyncTasksRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)
	asyncTaskManager := async_task.NewAsyncTaskManager(ctx, logger, scheduler0Store, asyncTaskManagerRepo, scheduler0config)
	jobQueueRepo := job_queue_repo.NewJobQueuesRepo(logger, scheduler0RaftActions, scheduler0Store)
	jobExecutionsRepo := job_execution_repo.NewExecutionsRepo(
		logger,
		scheduler0RaftActions,
		scheduler0Store,
	)

	dispatcher := utils.NewDispatcher(
		ctx,
		int64(1),
		int64(1),
	)

	dispatcher.Run()

	queueRepo := queue.NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, jobQueueRepo)
	// Create a new JobService instance
	jobService := job.NewJobService(ctx, logger, jobRepo, queueRepo, projectRepo, dispatcher, asyncTaskManager)
	httpJobExecutor := executors.NewMockHTTPExecutor(t)
	httpJobExecutor.On("ExecuteHTTPJob", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	service := NewJobExecutor(
		ctx,
		logger,
		scheduler0config,
		scheduler0RaftActions,
		jobRepo,
		jobExecutionsRepo,
		jobQueueRepo,
		httpJobExecutor,
		dispatcher,
	)

	asyncTaskManager.SetSingleNodeMode(true)
	asyncTaskManager.ListenForNotifications()

	// Define the input jobs
	jobs := []models.Job{}

	schedule, parseErr := cron.Parse("@every 1m")
	if parseErr != nil {
		t.Fatal("cron spec error", parseErr)
	}

	schedulerTime := scheduler0time.GetSchedulerTime()
	now := schedulerTime.GetTime(time.Now())
	nextTime := schedule.Next(now)

	job := models.Job{
		ID:            uint64(1),
		Spec:          "@every 1h",
		Timezone:      "America/New_York",
		ProjectID:     1,
		ExecutionType: "http",
		CallbackUrl:   "http://someaddress",
	}

	jobs = []models.Job{job}

	// Create the projects using the project repo
	project := models.Project{
		ID:          1,
		Name:        fmt.Sprintf("Project %d", 1),
		Description: fmt.Sprintf("Project %d description", 1),
	}
	_, createErr := projectRepo.CreateOne(&project)
	if createErr != nil {
		t.Fatalf("Failed to create project: %v", createErr)
	}

	// Call the BatchInsertJobs method of the job service
	_, batchErr := jobService.BatchInsertJobs("request123", jobs)
	if batchErr != nil {
		t.Fatalf("Failed to insert jobs: %v", batchErr)
	}

	time.Sleep(time.Second * time.Duration(1))

	serr := os.Setenv("SCHEDULER0_NODE_ID", "1")
	defer os.Unsetenv("SCHEDULER0_NODE_ID")
	if serr != nil {
		t.Fatal("failed to set env", serr)
	}

	service.ListenForJobsToInvoke()

	service.GetScheduledJobs().Store(1, models.JobSchedule{
		Job:           job,
		ExecutionTime: schedulerTime.GetTime(nextTime),
	})

	time.Sleep(1*time.Minute + 2*time.Second)
}

func Test_handleFailedJobs(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "job-service-test",
		Level: hclog.LevelFromString("trace"),
	})

	// Create a temporary SQLite database file
	tempFile, err := ioutil.TempFile("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	// Create a new SQLite database connection
	sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.RunMigration()
	sqliteDb.OpenConnectionToExistingDB()

	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)

	// Create a new FSM store
	scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, scheduler0config, sqliteDb, nil, nil, nil, nil, sharedRepo)

	// Create a mock raft cluster
	cluster := raft.MakeClusterCustom(t, &raft.MakeClusterOpts{
		Peers:          1,
		Bootstrap:      true,
		Conf:           raft.DefaultConfig(),
		ConfigStoreFSM: false,
		MakeFSMFunc: func() raft.FSM {
			return scheduler0Store.GetFSM()
		},
	})
	defer cluster.Close()
	cluster.FullyConnect()
	scheduler0Store.UpdateRaft(cluster.Leader())

	ctx, canceler := context.WithCancel(context.Background())
	defer canceler()

	jobRepo := job_repo.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)
	projectRepo := project_repo.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)
	asyncTaskManagerRepo := async_task_repo.NewAsyncTasksRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)
	asyncTaskManager := async_task.NewAsyncTaskManager(ctx, logger, scheduler0Store, asyncTaskManagerRepo, scheduler0config)
	jobQueueRepo := job_queue_repo.NewJobQueuesRepo(logger, scheduler0RaftActions, scheduler0Store)
	jobExecutionsRepo := job_execution_repo.NewExecutionsRepo(
		logger,
		scheduler0RaftActions,
		scheduler0Store,
	)

	dispatcher := utils.NewDispatcher(
		ctx,
		int64(1),
		int64(1),
	)

	dispatcher.Run()

	queueRepo := queue.NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, jobQueueRepo)
	// Create a new JobService instance
	jobService := job.NewJobService(ctx, logger, jobRepo, queueRepo, projectRepo, dispatcher, asyncTaskManager)

	httpJobExecutor := executors.NewHTTTPExecutor(logger, ctx, scheduler0config, dispatcher)
	service := NewJobExecutor(
		ctx,
		logger,
		scheduler0config,
		scheduler0RaftActions,
		jobRepo,
		jobExecutionsRepo,
		jobQueueRepo,
		httpJobExecutor,
		dispatcher,
	)

	service.ListenForJobsToInvoke()

	asyncTaskManager.SetSingleNodeMode(true)
	asyncTaskManager.ListenForNotifications()

	// Define the input jobs
	jobs := []models.Job{}
	job := models.Job{
		ID:            uint64(1),
		Spec:          "@every 1m",
		Timezone:      "America/New_York",
		ProjectID:     1,
		ExecutionType: "http",
		CallbackUrl:   "http://%s",
	}

	time.Sleep(1 * time.Second)

	jobs = []models.Job{job}

	// Create the projects using the project repo
	project := models.Project{
		ID:          1,
		Name:        fmt.Sprintf("Project %d", 1),
		Description: fmt.Sprintf("Project %d description", 1),
	}
	_, createErr := projectRepo.CreateOne(&project)
	if createErr != nil {
		t.Fatalf("Failed to create project: %v", createErr)
	}

	// Call the BatchInsertJobs method of the job service
	_, batchErr := jobService.BatchInsertJobs("request123", jobs)
	if batchErr != nil {
		t.Fatalf("Failed to insert jobs: %v", batchErr)
	}

	time.Sleep(time.Second * time.Duration(1))

	serr := os.Setenv("SCHEDULER0_NODE_ID", "1")
	defer os.Unsetenv("SCHEDULER0_NODE_ID")
	if serr != nil {
		t.Fatal("failed to set env", serr)
	}

	serr = os.Setenv("SCHEDULER0_JOB_EXECUTION_RETRY_MAX", "2")
	defer os.Unsetenv("SCHEDULER0_JOB_EXECUTION_RETRY_MAX")
	if serr != nil {
		t.Fatal("failed to set env", serr)
	}

	schedule, parseErr := cron.Parse("@every 1m")
	if parseErr != nil {
		t.Fatal("cron spec error", parseErr)
	}

	schedulerTime := scheduler0time.GetSchedulerTime()
	now := schedulerTime.GetTime(time.Now())
	nextTime := schedule.Next(now)

	service.GetScheduledJobs().Store(1, models.JobSchedule{
		Job:           job,
		ExecutionTime: nextTime,
	})

	service.UpdateRaft(scheduler0Store.GetRaft())

	time.Sleep(time.Minute*1 + time.Second*5)

	exec, ok := service.GetExecutionsCache().Load(uint64(1))
	if !ok {
		t.Fatal("should store job execution in cache")
	}
	cachedJobExecutionLog := (exec).(models.MemJobExecution)
	assert.Equal(t, 2, int(cachedJobExecutionLog.FailCount))
	uncommittedExecutionLogsCount := jobExecutionsRepo.CountExecutionLogs(false)
	assert.Equal(t, 3, int(uncommittedExecutionLogsCount))
}

func Test_StopAll(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "job-service-test",
		Level: hclog.LevelFromString("trace"),
	})

	// Create a temporary SQLite database file
	tempFile, err := ioutil.TempFile("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	// Create a new SQLite database connection
	sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.RunMigration()
	sqliteDb.OpenConnectionToExistingDB()

	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)

	// Create a new FSM store
	scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, scheduler0config, sqliteDb, nil, nil, nil, nil, sharedRepo)

	// Create a mock raft cluster
	cluster := raft.MakeClusterCustom(t, &raft.MakeClusterOpts{
		Peers:          1,
		Bootstrap:      true,
		Conf:           raft.DefaultConfig(),
		ConfigStoreFSM: false,
		MakeFSMFunc: func() raft.FSM {
			return scheduler0Store.GetFSM()
		},
	})
	defer cluster.Close()
	cluster.FullyConnect()
	scheduler0Store.UpdateRaft(cluster.Leader())

	ctx, canceler := context.WithCancel(context.Background())
	defer canceler()

	jobRepo := job_repo.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)
	jobQueueRepo := job_queue_repo.NewJobQueuesRepo(logger, scheduler0RaftActions, scheduler0Store)
	jobExecutionsRepo := job_execution_repo.NewExecutionsRepo(
		logger,
		scheduler0RaftActions,
		scheduler0Store,
	)

	dispatcher := utils.NewDispatcher(
		ctx,
		int64(1),
		int64(1),
	)

	dispatcher.Run()

	httpJobExecutor := executors.NewMockHTTPExecutor(t)

	service := NewJobExecutor(
		ctx,
		logger,
		scheduler0config,
		scheduler0RaftActions,
		jobRepo,
		jobExecutionsRepo,
		jobQueueRepo,
		httpJobExecutor,
		dispatcher,
	)

	for i := 0; i < 10; i++ {
		service.GetScheduledJobs().Store(i, i)
	}
	service.StopAll()
	count := 0
	service.GetScheduledJobs().Range(func(key, value any) bool {
		count += 1
		return true
	})
	assert.Equal(t, count, 0)
}
