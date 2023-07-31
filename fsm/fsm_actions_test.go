package fsm

import (
	"fmt"
	sq "github.com/Masterminds/squirrel"
	"github.com/brianvoe/gofakeit/v6"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
	"scheduler0/config"
	"scheduler0/constants"
	"scheduler0/db"
	"scheduler0/models"
	"scheduler0/shared_repo"
	"scheduler0/test_helpers"
	"testing"
	"time"
)

func Test_WriteCommandToRaftLog_Executes_SQL(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "fsm-actions-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := NewScheduler0RaftActions(sharedRepo)

	tempFile, err := ioutil.TempFile("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Unsetenv(tempFile.Name())

	sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.RunMigration()
	sqliteDb.OpenConnectionToExistingDB()
	scheduler0Store := NewFSMStore(logger, scheduler0RaftActions, sqliteDb)
	cluster := raft.MakeClusterCustom(t, &raft.MakeClusterOpts{
		Peers:          1,
		Bootstrap:      true,
		Conf:           raft.DefaultConfig(),
		ConfigStoreFSM: false,
		MakeFSMFunc: func() raft.FSM {
			return scheduler0Store.GetFSM()
		},
	})
	cluster.FullyConnect()
	scheduler0Store.UpdateRaft(cluster.Leader())

	query, params, err := sq.Insert(constants.CredentialTableName).
		Columns(
			constants.CredentialsApiKeyColumn,
			constants.CredentialsApiSecretColumn,
			constants.CredentialsArchivedColumn,
			constants.CredentialsDateCreatedColumn,
		).
		Values(
			"some-api-key",
			"some-api-secret",
			false,
			time.Now().UTC(),
		).ToSql()
	if err != nil {
		t.Fatalf("failed to create sql to insert into raft log %v", err)
	}

	res, writeErr := scheduler0RaftActions.WriteCommandToRaftLog(scheduler0Store.GetRaft(), constants.CommandTypeDbExecute, query, 0, params)
	if writeErr != nil {
		t.Fatalf("failed to write to raft log %v", writeErr)
	}
	t.Log("response from raft write log", res)

	conn := sqliteDb.GetOpenConnection()
	rows, err := conn.Query(fmt.Sprintf("select id, api_key, api_secret, archived, date_created from %s", constants.CredentialTableName))
	if err != nil {
		t.Fatalf("failed to query credentials table %v", err)
	}
	defer rows.Close()
	var credential models.Credential
	for rows.Next() {
		scanErr := rows.Scan(
			&credential.ID,
			&credential.ApiKey,
			&credential.ApiSecret,
			&credential.Archived,
			&credential.DateCreated,
		)
		if scanErr != nil {
			t.Fatalf("failed to scan rows %v", scanErr)
		}
	}
	if rows.Err() != nil {
		if rows.Err() != nil {
			t.Fatalf("failed to rows err %v", err)
		}
	}
	assert.Equal(t, credential.ID, uint64(1))
	assert.Equal(t, credential.ApiKey, "some-api-key")
	assert.Equal(t, credential.ApiSecret, "some-api-secret")
	assert.Equal(t, credential.Archived, false)
}

func Test_WriteCommandToRaftLog_Job_Queue(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "fsm-actions-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := NewScheduler0RaftActions(sharedRepo)

	tempFile, err := ioutil.TempFile("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Unsetenv(tempFile.Name())

	sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.RunMigration()
	sqliteDb.OpenConnectionToExistingDB()
	scheduler0Store := NewFSMStore(logger, scheduler0RaftActions, sqliteDb)
	cluster := raft.MakeClusterCustom(t, &raft.MakeClusterOpts{
		Peers:          1,
		Bootstrap:      true,
		Conf:           raft.DefaultConfig(),
		ConfigStoreFSM: false,
		MakeFSMFunc: func() raft.FSM {
			return scheduler0Store.GetFSM()
		},
	})
	cluster.FullyConnect()
	scheduler0Store.UpdateRaft(cluster.Leader())

	lowerBoundId := 0
	upperBoundId := 10
	nodeId := 1

	batchRange := []interface{}{
		lowerBoundId,
		upperBoundId,
		nodeId,
	}

	res, writeErr := scheduler0RaftActions.WriteCommandToRaftLog(scheduler0Store.GetRaft(), constants.CommandTypeJobQueue, "", 1, batchRange)
	if writeErr != nil {
		t.Fatalf("failed to write to raft log %v", writeErr)
	}
	t.Log("response from raft write log", res)

	conn := sqliteDb.GetOpenConnection()
	rows, err := conn.Query(fmt.Sprintf("select %s, %s, %s, %s, %s, %s from %s",
		constants.JobQueueIdColumn,
		constants.JobQueueLowerBoundJobId,
		constants.JobQueueUpperBound,
		constants.JobQueueNodeIdColumn,
		constants.JobQueueVersion,
		constants.JobQueueDateCreatedColumn,
		constants.JobQueuesTableName))
	if err != nil {
		t.Fatalf("failed to query job_queues table %v", err)
	}
	defer rows.Close()
	var jobQueue models.JobQueueLog
	for rows.Next() {
		scanErr := rows.Scan(
			&jobQueue.Id,
			&jobQueue.LowerBoundJobId,
			&jobQueue.UpperBoundJobId,
			&jobQueue.NodeId,
			&jobQueue.Version,
			&jobQueue.DateCreated,
		)
		if scanErr != nil {
			t.Fatalf("failed to scan rows %v", scanErr)
		}
	}
	if rows.Err() != nil {
		if rows.Err() != nil {
			t.Fatalf("failed to rows err %v", err)
		}
	}

	assert.Equal(t, jobQueue.Id, uint64(1))
	assert.Equal(t, jobQueue.LowerBoundJobId, uint64(lowerBoundId))
	assert.Equal(t, jobQueue.UpperBoundJobId, uint64(upperBoundId))
	assert.Equal(t, jobQueue.NodeId, uint64(nodeId))
	assert.Equal(t, jobQueue.Version, uint64(1))

	jobQueueParams := <-scheduler0Store.GetQueueJobsChannel()

	serverId := jobQueueParams[0].(uint64)
	lowerBound := jobQueueParams[1].(int64)
	upperBound := jobQueueParams[2].(int64)

	assert.Equal(t, lowerBound, int64(lowerBoundId))
	assert.Equal(t, upperBound, int64(upperBoundId))
	assert.Equal(t, serverId, uint64(nodeId))
}

func Test_WriteCommandToRaftLog_Local_Data_Commit(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "fsm-actions-tests",
		Level: hclog.LevelFromString("DEBUG"),
	})

	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)

	tempFile, err := ioutil.TempFile("", "test-db.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Unsetenv(tempFile.Name())

	sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.OpenConnectionToExistingDB()
	sqliteDb.RunMigration()
	scheduler0RaftActions := NewScheduler0RaftActions(sharedRepo)

	scheduler0Store := NewFSMStore(logger, scheduler0RaftActions, sqliteDb)
	cluster := raft.MakeClusterCustom(t, &raft.MakeClusterOpts{
		Peers:          1,
		Bootstrap:      true,
		Conf:           raft.DefaultConfig(),
		ConfigStoreFSM: false,
		MakeFSMFunc: func() raft.FSM {
			return scheduler0Store.GetFSM()
		},
	})
	cluster.FullyConnect()
	scheduler0Store.UpdateRaft(cluster.Leader())

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

	err = sharedRepo.InsertExecutionLogs(sqliteDb, false, jobExecutionLogs)
	if err != nil {
		t.Fatalf("Failed to insert committed job execution logs: %v", err)
	}

	asyncTasks := test_helpers.CreateFakeAsyncTasks(numberOfJEL, conn, t)
	err = sharedRepo.InsertAsyncTasksLogs(sqliteDb, false, asyncTasks)
	if err != nil {
		t.Fatalf("Failed to insert async tasks: %v", err)
	}

	params := models.CommitLocalData{
		Data: models.LocalData{
			ExecutionLogs: jobExecutionLogs,
			AsyncTasks:    asyncTasks,
		},
	}
	_, writeError := scheduler0RaftActions.WriteCommandToRaftLog(
		scheduler0Store.GetRaft(),
		constants.CommandTypeLocalData,
		"",
		uint64(nodeId),
		[]interface{}{params},
	)
	if writeError != nil {
		t.Fatalf("failed to write to raft log %v", writeError)
	}
	committedExecutionLogs, err := sharedRepo.GetExecutionLogs(sqliteDb, true)
	if err != nil {
		t.Fatalf("Failed to get committed job execution logs: %v", err)
	}
	assert.Equal(t, len(jobExecutionLogs), len(committedExecutionLogs))
	unCommittedExecutionLogs, err := sharedRepo.GetExecutionLogs(sqliteDb, false)
	if err != nil {
		t.Fatalf("Failed to get uncommitted job execution logs: %v", err)
	}
	assert.Equal(t, 0, len(unCommittedExecutionLogs))

	unCommittedAsyncTasks := test_helpers.GetAllAsyncTasks(false, conn, t)
	assert.Equal(t, 0, len(unCommittedAsyncTasks))
	committedAsyncTasks := test_helpers.GetAllAsyncTasks(true, conn, t)
	assert.Equal(t, len(committedAsyncTasks), len(asyncTasks))
}

func Test_WriteCommandToRaftLog_Stop_All_Jobs(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "fsm-actions-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := NewScheduler0RaftActions(sharedRepo)

	tempFile, err := ioutil.TempFile("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Unsetenv(tempFile.Name())

	sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.RunMigration()
	sqliteDb.OpenConnectionToExistingDB()
	scheduler0Store := NewFSMStore(logger, scheduler0RaftActions, sqliteDb)
	cluster := raft.MakeClusterCustom(t, &raft.MakeClusterOpts{
		Peers:          1,
		Bootstrap:      true,
		Conf:           raft.DefaultConfig(),
		ConfigStoreFSM: false,
		MakeFSMFunc: func() raft.FSM {
			return scheduler0Store.GetFSM()
		},
	})
	cluster.FullyConnect()
	scheduler0Store.UpdateRaft(cluster.Leader())

	res, writeErr := scheduler0RaftActions.WriteCommandToRaftLog(scheduler0Store.GetRaft(), constants.CommandTypeStopJobs, "", 0, nil)
	if writeErr != nil {
		t.Fatalf("failed to write to raft log %v", writeErr)
	}
	t.Log("response from raft write log", res)

	stoppedAllJobs := <-scheduler0Store.GetStopAllJobsChannel()

	assert.Equal(t, stoppedAllJobs, true)
}

func Test_WriteCommandToRaftLog_Recovery_All_Jobs(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "fsm-actions-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := NewScheduler0RaftActions(sharedRepo)

	tempFile, err := ioutil.TempFile("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Unsetenv(tempFile.Name())

	sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.RunMigration()
	sqliteDb.OpenConnectionToExistingDB()
	scheduler0Store := NewFSMStore(logger, scheduler0RaftActions, sqliteDb)
	cluster := raft.MakeClusterCustom(t, &raft.MakeClusterOpts{
		Peers:          1,
		Bootstrap:      true,
		Conf:           raft.DefaultConfig(),
		ConfigStoreFSM: false,
		MakeFSMFunc: func() raft.FSM {
			return scheduler0Store.GetFSM()
		},
	})
	cluster.FullyConnect()
	scheduler0Store.UpdateRaft(cluster.Leader())

	res, writeErr := scheduler0RaftActions.WriteCommandToRaftLog(scheduler0Store.GetRaft(), constants.CommandTypeRecoverJobs, "", 0, nil)
	if writeErr != nil {
		t.Fatalf("failed to write to raft log %v", writeErr)
	}
	t.Log("response from raft write log", res)

	recoverAllJobs := <-scheduler0Store.GetRecoverJobsChannel()

	assert.Equal(t, recoverAllJobs, true)
}
