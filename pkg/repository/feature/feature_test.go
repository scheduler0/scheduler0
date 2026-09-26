package feature

import (
	"context"
	"os"
	"scheduler0/pkg/config"
	"scheduler0/pkg/db"
	"scheduler0/pkg/fsm"
	"scheduler0/pkg/shared_repo"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/assert"
)

func setupTestFSMStore(t *testing.T) (fsm.Scheduler0RaftStore, func()) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "feature-repo-test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	sharedRepo := shared_repo.NewSharedRepo(logger, scheduler0config)
	scheduler0RaftActions := fsm.NewScheduler0RaftActions(sharedRepo, nil)
	tempFile, err := os.CreateTemp("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tempFile.Close()
	sqliteDb := db.NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.RunMigration(logger)
	sqliteDb.OpenConnectionToExistingDB()
	scheduler0Store := fsm.NewFSMStore(logger, scheduler0RaftActions, scheduler0config, sqliteDb, nil, nil, nil, nil, sharedRepo)

	// Create a mock raft cluster
	raftConf := raft.DefaultConfig()
	raftConf.HeartbeatTimeout = 50 * time.Millisecond
	raftConf.ElectionTimeout = 50 * time.Millisecond
	raftConf.CommitTimeout = 50 * time.Millisecond
	raftConf.LeaderLeaseTimeout = 25 * time.Millisecond

	cluster := raft.MakeClusterCustom(t, &raft.MakeClusterOpts{
		Peers:          1,
		Bootstrap:      true,
		Conf:           raftConf,
		ConfigStoreFSM: false,
		MakeFSMFunc: func() raft.FSM {
			return scheduler0Store.GetFSM()
		},
	})
	cluster.FullyConnect()
	scheduler0Store.UpdateRaft(cluster.Leader())

	cleanup := func() {
		cluster.Close()
		os.Remove(tempFile.Name())
	}

	return scheduler0Store, cleanup
}

func TestNewFeatureRepository(t *testing.T) {
	t.Parallel()
	scheduler0Store, cleanup := setupTestFSMStore(t)
	defer cleanup()

	repo := NewFeatureRepository(context.TODO(), scheduler0Store)
	assert.NotNil(t, repo)
}

func TestGetFeatures_Success(t *testing.T) {
	t.Parallel()
	scheduler0Store, cleanup := setupTestFSMStore(t)
	defer cleanup()

	repo := NewFeatureRepository(context.TODO(), scheduler0Store)

	features, err := repo.GetFeatures()
	assert.Nil(t, err)
	assert.NotNil(t, features)
	// Features are seeded via migrations, so we should have at least some features
	assert.Greater(t, len(*features), 0)

	// Verify feature structure
	for _, feature := range *features {
		assert.Greater(t, feature.ID, uint64(0))
		assert.NotEmpty(t, feature.Name)
		assert.False(t, feature.CreatedAt.IsZero())
	}
}

func TestGetFeatureByID_Success(t *testing.T) {
	t.Parallel()
	scheduler0Store, cleanup := setupTestFSMStore(t)
	defer cleanup()

	repo := NewFeatureRepository(context.TODO(), scheduler0Store)

	// First, get all features to find a valid ID
	allFeatures, err := repo.GetFeatures()
	assert.Nil(t, err)
	assert.NotNil(t, allFeatures)
	assert.Greater(t, len(*allFeatures), 0)

	// Get the first feature by ID
	firstFeature := (*allFeatures)[0]
	feature, getErr := repo.GetFeatureByID(firstFeature.ID)
	assert.Nil(t, getErr)
	assert.NotNil(t, feature)
	assert.Equal(t, firstFeature.ID, feature.ID)
	assert.Equal(t, firstFeature.Name, feature.Name)
	assert.Equal(t, firstFeature.CreatedAt, feature.CreatedAt)
}

func TestGetFeatureByID_NotFound(t *testing.T) {
	t.Parallel()
	scheduler0Store, cleanup := setupTestFSMStore(t)
	defer cleanup()

	repo := NewFeatureRepository(context.TODO(), scheduler0Store)

	// Use a very large ID that shouldn't exist
	feature, err := repo.GetFeatureByID(99999)
	assert.NotNil(t, err)
	assert.Nil(t, feature)
	assert.Equal(t, 404, err.Type)
	assert.Contains(t, err.Message, "feature not found")
}

func TestGetFeatureByID_ZeroID(t *testing.T) {
	t.Parallel()
	scheduler0Store, cleanup := setupTestFSMStore(t)
	defer cleanup()

	repo := NewFeatureRepository(context.TODO(), scheduler0Store)

	feature, err := repo.GetFeatureByID(0)
	assert.NotNil(t, err)
	assert.Nil(t, feature)
	assert.Equal(t, 404, err.Type)
	assert.Contains(t, err.Message, "feature not found")
}

