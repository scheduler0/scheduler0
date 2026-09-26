package fsm

import (
	"fmt"
	"os"
	"testing"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/assert"

	"scheduler0/pkg/config"
	"scheduler0/pkg/constants"
	"scheduler0/pkg/db"
	"scheduler0/pkg/models"
	"scheduler0/pkg/shared_repo"
)

func Test_WriteCommandToRaftLog_Executes_SQL(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "fsm-actions-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)

	scheduler0RaftActions := NewScheduler0RaftActions(sharedRepo, nil)

	tempFile, err := os.CreateTemp("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	tempFile.Close()

	sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.RunMigration(logger)
	sqliteDb.OpenConnectionToExistingDB()
	scheduler0Store := NewFSMStore(logger, scheduler0RaftActions, scheduler0config, sqliteDb, nil, nil, nil, nil, sharedRepo)
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

	query, params, err := sq.Insert(constants.CredentialTableName).
		Columns(
			constants.CredentialsApiKeyColumn,
			constants.CredentialsApiSecretColumn,
			constants.CredentialsArchivedColumn,
			constants.CredentialsDateCreatedColumn,
			constants.CredentialsAccountIdColumn,
			constants.CredentialsCreatedByColumn,
		).
		Values(
			"some-api-key",
			"some-api-secret",
			false,
			time.Now().UTC(),
			uint64(1), // System account ID from seed
			"test",
		).ToSql()
	if err != nil {
		t.Fatalf("failed to create sql to insert into raft log %v", err)
	}

	res, writeErr := scheduler0RaftActions.WriteCommandToRaftLog(scheduler0Store.GetRaft(), constants.CommandTypeDbExecute, query, params, nil, 0)
	if writeErr != nil {
		t.Fatalf("failed to write to raft log %v", writeErr)
	}
	assert.NotNil(t, res, "FSMResponse should not be nil")
	assert.Empty(t, res.Error, "FSMResponse should not have an error")
	assert.Equal(t, int64(1), res.Data.RowsAffected, "should have 1 row affected")
	assert.Equal(t, int64(1), res.Data.LastInsertedId, "should have last inserted id of 1")

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
		t.Fatalf("failed to rows err %v", rows.Err())
	}
	assert.Equal(t, credential.ID, uint64(1))
	assert.Equal(t, credential.ApiKey, "some-api-key")
	assert.Equal(t, credential.ApiSecret, "some-api-secret")
	assert.Equal(t, credential.Archived, false)
}

func Test_WriteCommandToRaftLog_PostProcessChannel(t *testing.T) {
	tests := []struct {
		Name                         string
		CommandPostProcessActionType constants.CommandAction
	}{
		{
			Name:                         "CommandActionCleanUncommittedAsyncTasksLogs",
			CommandPostProcessActionType: constants.CommandActionCleanUncommittedAsyncTasksLogs,
		},
		{
			Name:                         "CommandActionQueueJob",
			CommandPostProcessActionType: constants.CommandActionQueueJob,
		},
		{
			Name:                         "CommandActionCleanUncommittedExecutionLogs",
			CommandPostProcessActionType: constants.CommandActionCleanUncommittedExecutionLogs,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			scheduler0config := config.NewScheduler0Config()
			logger := hclog.New(&hclog.LoggerOptions{
				Name:  "fsm-actions-test",
				Level: hclog.LevelFromString("DEBUG"),
			})
			sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
			postProcessChannel := make(chan models.PostProcess, 1)
			scheduler0RaftActions := NewScheduler0RaftActions(sharedRepo, postProcessChannel)

			tempFile, err := os.CreateTemp("", "test-db")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tempFile.Name())
			tempFile.Close()

			sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
			sqliteDb.RunMigration(logger)
			sqliteDb.OpenConnectionToExistingDB()
			scheduler0Store := NewFSMStore(logger, scheduler0RaftActions, scheduler0config, sqliteDb, nil, nil, nil, nil, sharedRepo)
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

			query, params, err := sq.Insert(constants.CredentialTableName).
				Columns(
					constants.CredentialsApiKeyColumn,
					constants.CredentialsApiSecretColumn,
					constants.CredentialsArchivedColumn,
					constants.CredentialsDateCreatedColumn,
					constants.CredentialsAccountIdColumn,
					constants.CredentialsCreatedByColumn,
				).
				Values(
					"some-api-key",
					"some-api-secret",
					false,
					time.Now().UTC(),
					uint64(1), // System account ID from seed
					"test",
				).ToSql()
			if err != nil {
				t.Fatalf("failed to create sql to insert into raft log %v", err)
			}

			res, writeErr := scheduler0RaftActions.WriteCommandToRaftLog(scheduler0Store.GetRaft(), constants.CommandTypeDbExecute, query, params, []uint64{1}, test.CommandPostProcessActionType)
			if writeErr != nil {
				t.Fatalf("failed to write to raft log %v", writeErr)
			}
			assert.NotNil(t, res, "FSMResponse should not be nil")
			assert.Empty(t, res.Error, "FSMResponse should not have an error")
			assert.Equal(t, int64(1), res.Data.RowsAffected, "should have 1 row affected")
			assert.Equal(t, int64(1), res.Data.LastInsertedId, "should have last inserted id of 1")

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
				t.Fatalf("failed to rows err %v", rows.Err())
			}
			assert.Equal(t, credential.ID, uint64(1))
			assert.Equal(t, credential.ApiKey, "some-api-key")
			assert.Equal(t, credential.ApiSecret, "some-api-secret")
			assert.Equal(t, credential.Archived, false)

			postProcess := <-postProcessChannel

			assert.Equal(t, postProcess.Action, test.CommandPostProcessActionType)
			assert.Equal(t, postProcess.TargetNodes, []uint64{1})
			assert.Equal(t, models.SQLResponse{
				RowsAffected:   1,
				LastInsertedId: 1,
			}, postProcess.Data)
		})
	}
}
