package service

import (
	"context"
	"fmt"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	"github.com/robfig/cron"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"scheduler0/config"
	"scheduler0/constants"
	"scheduler0/db"
	"scheduler0/fsm"
	"scheduler0/models"
	"scheduler0/repository"
	"scheduler0/scheduler0time"
	"scheduler0/shared_repo"
	"scheduler0/utils"
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
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo)

	// Create a new FSM store
	scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, sqliteDb)

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

	jobRepo := repository.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)
	projectRepo := repository.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)
	asyncTaskManagerRepo := repository.NewAsyncTasksRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)
	asyncTaskManager := NewAsyncTaskManager(ctx, logger, scheduler0Store, asyncTaskManagerRepo)
	jobQueueRepo := repository.NewJobQueuesRepo(logger, scheduler0RaftActions, scheduler0Store)
	jobExecutionsRepo := repository.NewExecutionsRepo(
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

	queueRepo := NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, jobQueueRepo)
	// Create a new JobService instance
	jobService := NewJobService(ctx, logger, jobRepo, queueRepo, projectRepo, dispatcher, asyncTaskManager)

	service := NewJobExecutor(
		ctx,
		logger,
		scheduler0config,
		scheduler0RaftActions,
		jobRepo,
		jobExecutionsRepo,
		jobQueueRepo,
		dispatcher,
	)

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

	service.QueueExecutions([]interface{}{
		uint64(1),
		int64(1),
		int64(2),
	})

	_, ok := service.GetScheduledJobs().Load(jobs[0].ID)
	assert.Equal(t, ok, true)
	_, ok = service.GetScheduledJobs().Load(jobs[1].ID)
	assert.Equal(t, ok, true)
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
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo)

	// Create a new FSM store
	scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, sqliteDb)

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

	jobRepo := repository.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)
	projectRepo := repository.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)
	asyncTaskManagerRepo := repository.NewAsyncTasksRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)
	asyncTaskManager := NewAsyncTaskManager(ctx, logger, scheduler0Store, asyncTaskManagerRepo)
	jobQueueRepo := repository.NewJobQueuesRepo(logger, scheduler0RaftActions, scheduler0Store)
	jobExecutionsRepo := repository.NewExecutionsRepo(
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

	queueRepo := NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, jobQueueRepo)
	// Create a new JobService instance
	jobService := NewJobService(ctx, logger, jobRepo, queueRepo, projectRepo, dispatcher, asyncTaskManager)

	service := NewJobExecutor(
		ctx,
		logger,
		scheduler0config,
		scheduler0RaftActions,
		jobRepo,
		jobExecutionsRepo,
		jobQueueRepo,
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
	service.QueueExecutions([]interface{}{
		uint64(1),
		int64(1),
		int64(constants.JobMaxBatchSize + 100),
	})

	i = 0
	for i < constants.JobMaxBatchSize+99 {
		_, ok := service.GetScheduledJobs().Load(jobs[i].ID)
		assert.Equal(t, ok, true)
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
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo)

	// Create a new FSM store
	scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, sqliteDb)

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

	jobRepo := repository.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)
	projectRepo := repository.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)
	asyncTaskManagerRepo := repository.NewAsyncTasksRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)
	asyncTaskManager := NewAsyncTaskManager(ctx, logger, scheduler0Store, asyncTaskManagerRepo)
	jobQueueRepo := repository.NewJobQueuesRepo(logger, scheduler0RaftActions, scheduler0Store)
	jobExecutionsRepo := repository.NewExecutionsRepo(
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

	queueRepo := NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, jobQueueRepo)
	// Create a new JobService instance
	jobService := NewJobService(ctx, logger, jobRepo, queueRepo, projectRepo, dispatcher, asyncTaskManager)

	service := NewJobExecutor(
		ctx,
		logger,
		scheduler0config,
		scheduler0RaftActions,
		jobRepo,
		jobExecutionsRepo,
		jobQueueRepo,
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

	service.QueueExecutions([]interface{}{
		uint64(1),
		int64(1),
		int64(constants.JobMaxBatchSize + 100),
	})

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
		scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo)

		// Create a new FSM store
		scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, sqliteDb)

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

		jobRepo := repository.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)
		projectRepo := repository.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)
		asyncTaskManagerRepo := repository.NewAsyncTasksRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)
		asyncTaskManager := NewAsyncTaskManager(ctx, logger, scheduler0Store, asyncTaskManagerRepo)
		jobQueueRepo := repository.NewJobQueuesRepo(logger, scheduler0RaftActions, scheduler0Store)
		jobExecutionsRepo := repository.NewExecutionsRepo(
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

		queueRepo := NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, jobQueueRepo)
		// Create a new JobService instance
		jobService := NewJobService(ctx, logger, jobRepo, queueRepo, projectRepo, dispatcher, asyncTaskManager)

		service := NewJobExecutor(
			ctx,
			logger,
			scheduler0config,
			scheduler0RaftActions,
			jobRepo,
			jobExecutionsRepo,
			jobQueueRepo,
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
		service.QueueExecutions([]interface{}{
			uint64(1),
			int64(1),
			int64(100),
		})

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
		scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo)

		// Create a new FSM store
		scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, sqliteDb)

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

		jobRepo := repository.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)
		projectRepo := repository.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)
		asyncTaskManagerRepo := repository.NewAsyncTasksRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)
		asyncTaskManager := NewAsyncTaskManager(ctx, logger, scheduler0Store, asyncTaskManagerRepo)
		jobQueueRepo := repository.NewJobQueuesRepo(logger, scheduler0RaftActions, scheduler0Store)
		jobExecutionsRepo := repository.NewExecutionsRepo(
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

		queueRepo := NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, jobQueueRepo)
		// Create a new JobService instance
		jobService := NewJobService(ctx, logger, jobRepo, queueRepo, projectRepo, dispatcher, asyncTaskManager)

		service := NewJobExecutor(
			ctx,
			logger,
			scheduler0config,
			scheduler0RaftActions,
			jobRepo,
			jobExecutionsRepo,
			jobQueueRepo,
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
		service.QueueExecutions([]interface{}{
			uint64(1),
			int64(1),
			int64(100),
		})

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
		scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo)

		// Create a new FSM store
		scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, sqliteDb)

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

		jobRepo := repository.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)
		projectRepo := repository.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)
		asyncTaskManagerRepo := repository.NewAsyncTasksRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)
		asyncTaskManager := NewAsyncTaskManager(ctx, logger, scheduler0Store, asyncTaskManagerRepo)
		jobQueueRepo := repository.NewJobQueuesRepo(logger, scheduler0RaftActions, scheduler0Store)
		jobExecutionsRepo := repository.NewExecutionsRepo(
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

		queueRepo := NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, jobQueueRepo)
		// Create a new JobService instance
		jobService := NewJobService(ctx, logger, jobRepo, queueRepo, projectRepo, dispatcher, asyncTaskManager)

		service := NewJobExecutor(
			ctx,
			logger,
			scheduler0config,
			scheduler0RaftActions,
			jobRepo,
			jobExecutionsRepo,
			jobQueueRepo,
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
		service.QueueExecutions([]interface{}{
			uint64(1),
			int64(1),
			int64(100),
		})

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
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo)

	// Create a new FSM store
	scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, sqliteDb)

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

	jobRepo := repository.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)
	projectRepo := repository.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)
	asyncTaskManagerRepo := repository.NewAsyncTasksRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)
	asyncTaskManager := NewAsyncTaskManager(ctx, logger, scheduler0Store, asyncTaskManagerRepo)
	jobQueueRepo := repository.NewJobQueuesRepo(logger, scheduler0RaftActions, scheduler0Store)
	jobExecutionsRepo := repository.NewExecutionsRepo(
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

	queueRepo := NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, jobQueueRepo)
	// Create a new JobService instance
	jobService := NewJobService(ctx, logger, jobRepo, queueRepo, projectRepo, dispatcher, asyncTaskManager)

	service := NewJobExecutor(
		ctx,
		logger,
		scheduler0config,
		scheduler0RaftActions,
		jobRepo,
		jobExecutionsRepo,
		jobQueueRepo,
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

	jobExecuted := false
	callbackUrl := ""

	go func() {
		ln, err := net.Listen("tcp", "127.0.0.1:")
		if err != nil {
			t.Error("failed to create tcp listener", err)
			return
		}
		testMux := http.NewServeMux()
		testMux.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
			t.Log("executing test job")
			jobExecuted = true
		})
		callbackUrl = ln.Addr().String()
		err = http.Serve(ln, testMux)
		if err != nil {
			t.Error("failed to listen and serve http request", err)
			return
		}
		time.Sleep(1*time.Minute + 2*time.Second)
		ln.Close()
	}()

	time.Sleep(1 * time.Second)

	if callbackUrl == "" {
		t.Fatal("callback url is empty string", callbackUrl)
	}

	job := models.Job{
		ID:            uint64(1),
		Spec:          "@every 1h",
		Timezone:      "America/New_York",
		ProjectID:     1,
		ExecutionType: "http",
		CallbackUrl:   fmt.Sprintf("http://%s", callbackUrl),
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
	assert.Equal(t, true, jobExecuted)
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
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo)

	// Create a new FSM store
	scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, sqliteDb)

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

	jobRepo := repository.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)
	projectRepo := repository.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)
	asyncTaskManagerRepo := repository.NewAsyncTasksRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)
	asyncTaskManager := NewAsyncTaskManager(ctx, logger, scheduler0Store, asyncTaskManagerRepo)
	jobQueueRepo := repository.NewJobQueuesRepo(logger, scheduler0RaftActions, scheduler0Store)
	jobExecutionsRepo := repository.NewExecutionsRepo(
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

	queueRepo := NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, jobQueueRepo)
	// Create a new JobService instance
	jobService := NewJobService(ctx, logger, jobRepo, queueRepo, projectRepo, dispatcher, asyncTaskManager)

	service := NewJobExecutor(
		ctx,
		logger,
		scheduler0config,
		scheduler0RaftActions,
		jobRepo,
		jobExecutionsRepo,
		jobQueueRepo,
		dispatcher,
	)

	asyncTaskManager.SetSingleNodeMode(true)
	asyncTaskManager.ListenForNotifications()

	// Define the input jobs
	jobs := []models.Job{}
	job := models.Job{
		ID:            uint64(1),
		Spec:          "@every 1h",
		Timezone:      "America/New_York",
		ProjectID:     1,
		ExecutionType: "http",
		CallbackUrl:   "http://%s",
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

	serr = os.Setenv("SCHEDULER0_JOB_EXECUTION_RETRY_MAX", "2")
	defer os.Unsetenv("SCHEDULER0_JOB_EXECUTION_RETRY_MAX")
	if serr != nil {
		t.Fatal("failed to set env", serr)
	}

	schedule, parseErr := cron.Parse("@every 1h")
	if parseErr != nil {
		t.Fatal("cron spec error", parseErr)
	}

	schedulerTime := scheduler0time.GetSchedulerTime()
	now := schedulerTime.GetTime(time.Now())
	nextTime := schedule.Next(now)
	lastTime := nextTime.Add(-nextTime.Sub(now))

	service.GetExecutionsCache().Store(uint64(1), models.MemJobExecution{
		ExecutionVersion:      1,
		FailCount:             0,
		LastState:             models.ExecutionLogFailedState,
		LastExecutionDatetime: lastTime,
		NextExecutionDatetime: nextTime,
	})

	service.UpdateRaft(scheduler0Store.GetRaft())
	service.handleFailedJobs([]models.Job{job})

	time.Sleep(time.Second * 2)

	exec, ok := service.GetExecutionsCache().Load(uint64(1))
	if !ok {
		t.Fatal("should store job execution in cache")
	}
	cachedJobExecutionLog := (exec).(models.MemJobExecution)
	assert.Equal(t, 1, int(cachedJobExecutionLog.FailCount))
	uncommittedExecutionLogsCount := jobExecutionsRepo.CountExecutionLogs(false)
	assert.Equal(t, 1, int(uncommittedExecutionLogsCount))
}

func Test_logJobExecutionStateInRaft(t *testing.T) {
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
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo)

	// Create a new FSM store
	scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, sqliteDb)

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

	jobRepo := repository.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)
	projectRepo := repository.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)
	asyncTaskManagerRepo := repository.NewAsyncTasksRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)
	asyncTaskManager := NewAsyncTaskManager(ctx, logger, scheduler0Store, asyncTaskManagerRepo)
	jobQueueRepo := repository.NewJobQueuesRepo(logger, scheduler0RaftActions, scheduler0Store)
	jobExecutionsRepo := repository.NewExecutionsRepo(
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

	queueRepo := NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, jobQueueRepo)
	// Create a new JobService instance
	jobService := NewJobService(ctx, logger, jobRepo, queueRepo, projectRepo, dispatcher, asyncTaskManager)

	service := NewJobExecutor(
		ctx,
		logger,
		scheduler0config,
		scheduler0RaftActions,
		jobRepo,
		jobExecutionsRepo,
		jobQueueRepo,
		dispatcher,
	)

	service.UpdateRaft(scheduler0Store.GetRaft())

	asyncTaskManager.SetSingleNodeMode(true)
	asyncTaskManager.ListenForNotifications()

	// Define the input jobs
	jobs := []models.Job{}
	job := models.Job{
		ID:            uint64(1),
		Spec:          "@every 1h",
		Timezone:      "America/New_York",
		ProjectID:     1,
		ExecutionType: "http",
		CallbackUrl:   "http://%s",
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

	service.logJobExecutionStateInRaft(jobs, models.ExecutionLogScheduleState, map[uint64]uint64{1: 1})
	time.Sleep(time.Second * time.Duration(1))

	uncommittedExecutionLogsCount := jobExecutionsRepo.CountExecutionLogs(true)
	assert.Equal(t, 1, int(uncommittedExecutionLogsCount))
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
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo)

	// Create a new FSM store
	scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, sqliteDb)

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

	jobRepo := repository.NewJobRepo(logger, scheduler0RaftActions, scheduler0Store)
	projectRepo := repository.NewProjectRepo(logger, scheduler0RaftActions, scheduler0Store, jobRepo)
	asyncTaskManagerRepo := repository.NewAsyncTasksRepo(ctx, logger, scheduler0RaftActions, scheduler0Store)
	asyncTaskManager := NewAsyncTaskManager(ctx, logger, scheduler0Store, asyncTaskManagerRepo)
	jobQueueRepo := repository.NewJobQueuesRepo(logger, scheduler0RaftActions, scheduler0Store)
	jobExecutionsRepo := repository.NewExecutionsRepo(
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

	queueRepo := NewJobQueue(ctx, logger, scheduler0config, scheduler0RaftActions, scheduler0Store, jobQueueRepo)
	// Create a new JobService instance
	jobService := NewJobService(ctx, logger, jobRepo, queueRepo, projectRepo, dispatcher, asyncTaskManager)

	service := NewJobExecutor(
		ctx,
		logger,
		scheduler0config,
		scheduler0RaftActions,
		jobRepo,
		jobExecutionsRepo,
		jobQueueRepo,
		dispatcher,
	)

	service.UpdateRaft(scheduler0Store.GetRaft())

	asyncTaskManager.SetSingleNodeMode(true)
	asyncTaskManager.ListenForNotifications()

	// Define the input jobs
	jobs := []models.Job{}
	job := models.Job{
		ID:            uint64(1),
		Spec:          "@every 1h",
		Timezone:      "America/New_York",
		ProjectID:     1,
		ExecutionType: "http",
		CallbackUrl:   "http://%s",
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

	oldContext := service.context
	service.StopAll()

	if oldContext == service.context {
		t.Fatal("context should be replaced")
	}
}
