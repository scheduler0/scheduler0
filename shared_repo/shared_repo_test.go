package shared_repo

import (
	"github.com/brianvoe/gofakeit/v6"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"scheduler0/config"
	"scheduler0/db"
	"scheduler0/models"
	"scheduler0/test_helpers"
	"testing"
)

func Test_GetUncommittedExecutionLogs_Returns_Uncommitted_Execution_Logs(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "shard-repo-test",
		Level: hclog.LevelFromString("DEBUG"),
	})

	sharedRepo := NewSharedRepo(logger, scheduler0config)
	sqliteDb := db.GetDBMEMConnection(logger)
	sqliteDb.RunMigration()
	conn := sqliteDb.GetOpenConnection()

	projectIds := test_helpers.InsertFakeProjects(1, conn, t)
	jobIds := test_helpers.InsertFakeJobs(1, projectIds, conn, t)
	jobE := test_helpers.InsertFakeJobExecutionLogs(1, jobIds, conn, t)
	uce := jobE[0]
	foundUncommittedLogs, err := sharedRepo.GetExecutionLogs(sqliteDb, false)
	if err != nil {
		t.Fatalf("Failed to get uncommitted execution logs: %v", err)
	}

	assert.Equal(t, foundUncommittedLogs[0].NodeId, uce.NodeId)
	assert.Equal(t, foundUncommittedLogs[0].UniqueId, uce.UniqueId)
	assert.Equal(t, foundUncommittedLogs[0].State, uce.State)
	assert.Equal(t, foundUncommittedLogs[0].JobId, uce.JobId)
	assert.Equal(t, foundUncommittedLogs[0].JobQueueVersion, uce.JobQueueVersion)
	assert.Equal(t, foundUncommittedLogs[0].ExecutionVersion, uce.ExecutionVersion)
	assert.Equal(t, foundUncommittedLogs[0].LastExecutionDatetime, uce.LastExecutionDatetime)
	assert.Equal(t, foundUncommittedLogs[0].NextExecutionDatetime, uce.NextExecutionDatetime)
	assert.Equal(t, foundUncommittedLogs[0].DataCreated, uce.DataCreated)
}

func Test_InsertToCommittedExecutionLogs(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "shard-repo-test",
		Level: hclog.LevelFromString("DEBUG"),
	})

	sharedRepo := NewSharedRepo(logger, scheduler0config)

	sqliteDb := db.GetDBMEMConnection(logger)
	sqliteDb.RunMigration()
	conn := sqliteDb.GetOpenConnection()

	projectIds := test_helpers.InsertFakeProjects(1, conn, t)
	jobIds := test_helpers.InsertFakeJobs(1, projectIds, conn, t)

	nodeId := 3
	numberOfJEL := 20

	var jobExecutionLogs []models.JobExecutionLog
	for i := 0; i < numberOfJEL; i++ {
		var uce models.JobExecutionLog
		gofakeit.Struct(&uce)
		uce.JobId = uint64(jobIds[i%len(jobIds)])
		uce.NodeId = uint64(nodeId)
		jobExecutionLogs = append(jobExecutionLogs, uce)
	}

	err := sharedRepo.InsertExecutionLogs(sqliteDb, true, jobExecutionLogs)
	if err != nil {
		t.Fatalf("Failed to insert committed job execution logs: %v", err)
	}

	executionLogs, err := sharedRepo.GetExecutionLogs(sqliteDb, true)
	if err != nil {
		t.Fatalf("Failed to get uncommitted execution logs: %v", err)
	}

	assert.Equal(t, len(executionLogs), numberOfJEL)
}

func Test_DeleteUncommittedExecutionLogs(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "shard-repo-test",
		Level: hclog.LevelFromString("DEBUG"),
	})

	sharedRepo := NewSharedRepo(logger, scheduler0config)

	sqliteDb := db.GetDBMEMConnection(logger)
	sqliteDb.RunMigration()
	conn := sqliteDb.GetOpenConnection()

	projectIds := test_helpers.InsertFakeProjects(1, conn, t)
	jobIds := test_helpers.InsertFakeJobs(1, projectIds, conn, t)

	nodeId := 3
	numberOfJEL := 20

	var jobExecutionLogs []models.JobExecutionLog
	for i := 0; i < numberOfJEL; i++ {
		var uce models.JobExecutionLog
		gofakeit.Struct(&uce)
		uce.JobId = uint64(jobIds[i%len(jobIds)])
		uce.NodeId = uint64(nodeId)
		jobExecutionLogs = append(jobExecutionLogs, uce)
	}

	err := sharedRepo.InsertExecutionLogs(sqliteDb, false, jobExecutionLogs)
	if err != nil {
		t.Fatalf("Failed to insert committed job execution logs: %v", err)
	}

	executionLogs, err := sharedRepo.GetExecutionLogs(sqliteDb, false)
	if err != nil {
		t.Fatalf("Failed to get uncommitted execution logs: %v", err)
	}

	err = sharedRepo.DeleteExecutionLogs(sqliteDb, false, executionLogs)
	if err != nil {
		t.Fatalf("Failed to delete uncommitted execution logs: %v", err)
	}

	executionLogs, err = sharedRepo.GetExecutionLogs(sqliteDb, false)
	if err != nil {
		t.Fatalf("Failed to get uncommitted execution logs: %v", err)
	}

	assert.Equal(t, len(executionLogs), 0)
}

func Test_InsertAsyncTasksLogs(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "shard-repo-test",
		Level: hclog.LevelFromString("DEBUG"),
	})

	sharedRepo := NewSharedRepo(logger, scheduler0config)
	sqliteDb := db.GetDBMEMConnection(logger)
	sqliteDb.RunMigration()
	conn := sqliteDb.GetOpenConnection()
	numberOfJEL := 20
	asyncTasks := test_helpers.CreateFakeAsyncTasks(numberOfJEL, conn, t)
	err := sharedRepo.InsertAsyncTasksLogs(sqliteDb, true, asyncTasks)
	if err != nil {
		t.Fatalf("Failed to insert async tasks: %v", err)
	}
	ast := test_helpers.GetAllAsyncTasks(true, conn, t)
	assert.Equal(t, len(ast), len(asyncTasks))
}

func Test_DeleteAsyncTasksLogs(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "shard-repo-test",
		Level: hclog.LevelFromString("DEBUG"),
	})

	sharedRepo := NewSharedRepo(logger, scheduler0config)

	sqliteDb := db.GetDBMEMConnection(logger)
	sqliteDb.RunMigration()
	conn := sqliteDb.GetOpenConnection()
	numberOfJEL := 20
	asyncTasks := test_helpers.CreateFakeAsyncTasks(numberOfJEL, conn, t)
	err := sharedRepo.InsertAsyncTasksLogs(sqliteDb, false, asyncTasks)
	if err != nil {
		t.Fatalf("Failed to insert async tasks: %v", err)
	}
	ast := test_helpers.GetAllAsyncTasks(false, conn, t)
	assert.Equal(t, len(ast), len(asyncTasks))
	err = sharedRepo.DeleteAsyncTasksLogs(sqliteDb, false, ast)
	if err != nil {
		t.Fatalf("Failed to delete async tasks: %v", err)
	}
	ast = test_helpers.GetAllAsyncTasks(false, conn, t)
	assert.Equal(t, 0, len(ast))
}
