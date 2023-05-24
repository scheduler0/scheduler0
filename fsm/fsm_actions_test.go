package fsm

import (
	"fmt"
	sq "github.com/Masterminds/squirrel"
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
	defer os.Remove(tempFile.Name())

	sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.RunMigration()
	sqliteDb.OpenConnectionToExistingDB()
	scheduler0Store := NewFSMStore(logger, scheduler0RaftActions, sqliteDb)
	cluster := raft.MakeClusterCustom(t, &raft.MakeClusterOpts{
		Peers:          3,
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

	res, err := scheduler0RaftActions.WriteCommandToRaftLog(scheduler0Store.GetRaft(), constants.CommandTypeDbExecute, query, 0, params)
	if err != nil {
		t.Fatalf("failed to write to raft log %v %v", err, err == nil)
	}
	t.Log("response from raft write log", res)

	conn := sqliteDb.GetOpenConnection()
	rows, err := conn.Query(fmt.Sprintf("select id, api_key, api_secret, archived, date_created from %s", constants.CredentialTableName))
	if err != nil {
		t.Fatalf("failed to query credentials table %v", err)
	}
	defer rows.Close()
	var credential models.CredentialModel
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
