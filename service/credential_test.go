package service

import (
	"context"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
	"scheduler0/config"
	"scheduler0/db"
	"scheduler0/fsm"
	"scheduler0/models"
	"scheduler0/repository"
	"scheduler0/secrets"
	"scheduler0/shared_repo"
	"scheduler0/utils"
	"testing"
)

func Test_CreateNewCredential(t *testing.T) {
	ctx := context.Background()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "async-task-manager-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	scheduler0config := config.NewScheduler0Config()
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo)
	tempFile, err := ioutil.TempFile("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.RunMigration()
	sqliteDb.OpenConnectionToExistingDB()
	scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, sqliteDb)

	// Create a mock Raft cluster
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
	scheduler0Secrets := secrets.NewScheduler0Secrets()
	credentialRepo := repository.NewCredentialRepo(logger, scheduler0RaftActions, scheduler0Store)

	callback := func(effector func(sch chan any, ech chan any), successCh chan any, errorCh chan any) {
		effector(successCh, errorCh)
	}

	os.Setenv("SCHEDULER0_SECRET_KEY", "AB551DED82B93DC8035D624A625920E2121367C7538C02277D2D4DB3C0BFFE94")

	dispatcher := utils.NewDispatcher(
		int64(1),
		int64(1),
		callback,
	)

	dispatcher.Run()

	credentialService := NewCredentialService(ctx, logger, scheduler0Secrets, credentialRepo, dispatcher)

	id, createErr := credentialService.CreateNewCredential(models.Credential{})
	if createErr != nil {
		t.Fatal("failed to create new credential", createErr)
	}
	assert.Equal(t, id, uint64(1))
}
