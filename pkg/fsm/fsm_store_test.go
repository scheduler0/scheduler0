package fsm

import (
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"scheduler0/pkg/config"
	"scheduler0/pkg/mocks"
	"testing"
)

func TestNewFSMStore(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{})
	db := mocks.NewDataStore(t)
	raftActions := mocks.NewScheduler0RaftActions(t)
	scheduler0config := config.NewScheduler0Config()

	fsmStore := NewFSMStore(logger, raftActions, scheduler0config, db, nil, nil, nil, nil, nil)

	// Test that the store struct is not nil
	assert.NotNil(t, fsmStore)

	// assert that the fields are initialized correctly
	assert.Equal(t, logger.Named("fsm-store").Name(), fsmStore.(*store).logger.Name())
	assert.Equal(t, db, fsmStore.(*store).dataStore)
	assert.NotNil(t, fsmStore.(*store).queueJobsChannel)
	assert.Equal(t, raftActions, fsmStore.(*store).scheduler0RaftActions)
}

func TestGetFSM(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{})
	db := mocks.NewDataStore(t)
	raftActions := mocks.NewScheduler0RaftActions(t)
	scheduler0config := config.NewScheduler0Config()

	fsmStore := NewFSMStore(logger, raftActions, scheduler0config, db, nil, nil, nil, nil, nil)

	assert.NotNil(t, fsmStore.GetFSM())
}

func TestGetRaft(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{})
	db := mocks.NewDataStore(t)
	raftActions := mocks.NewScheduler0RaftActions(t)
	scheduler0config := config.NewScheduler0Config()

	fsmStore := NewFSMStore(logger, raftActions, scheduler0config, db, nil, nil, nil, nil, nil)

	assert.Nil(t, fsmStore.GetRaft())
	r := &raft.Raft{}
	fsmStore.UpdateRaft(r)
	assert.Equal(t, r, fsmStore.GetRaft())
}

func TestGetDataStore(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{})
	db := mocks.NewDataStore(t)
	raftActions := mocks.NewScheduler0RaftActions(t)
	scheduler0config := config.NewScheduler0Config()

	fsmStore := NewFSMStore(logger, raftActions, scheduler0config, db, nil, nil, nil, nil, nil)

	assert.Equal(t, db, fsmStore.GetDataStore())
}

func TestApply(t *testing.T) {
	// Create a mock logger and data store
	logger := hclog.New(&hclog.LoggerOptions{})
	dataStore := &mocks.DataStore{}

	// Create a mock implementation of the Scheduler0RaftActions interface
	actions := &mocks.Scheduler0RaftActions{}
	actions.On("ApplyRaftLog",
		mock.Anything,
		mock.Anything,
		dataStore,
		false,
	).Return(mock.Anything)

	scheduler0config := config.NewScheduler0Config()

	// Create the FSM store
	fsmStore := NewFSMStore(logger, actions, scheduler0config, dataStore, nil, nil, nil, nil, nil)

	// Apply a mock log to the FSM
	log := &raft.Log{
		Type:  raft.LogCommand,
		Index: 1,
		Term:  1,
		Data:  []byte("test"),
	}
	fsmStore.GetFSM().Apply(log)
}

func TestApplyBatch(t *testing.T) {
	// Create a mock logger and data store
	logger := hclog.New(&hclog.LoggerOptions{})
	dataStore := &mocks.DataStore{}

	// Create a mock implementation of the Scheduler0RaftActions interface
	actions := &mocks.Scheduler0RaftActions{}

	// Mock the ApplyRaftLog function to return a result
	actions.On("ApplyRaftLog",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(mock.Anything)

	scheduler0config := config.NewScheduler0Config()

	// Create the FSM store
	fsmStore := NewFSMStore(logger, actions, scheduler0config, dataStore, nil, nil, nil, nil, nil)

	// Create a mock batch of logs
	logs := []*raft.Log{
		{
			Type:  raft.LogCommand,
			Index: 1,
			Term:  1,
			Data:  []byte("test1"),
		},
		{
			Type:  raft.LogCommand,
			Index: 2,
			Term:  2,
			Data:  []byte("test2"),
		},
	}

	// Apply the batch of logs to the FSM store
	results := fsmStore.GetBatchingFSM().ApplyBatch(logs)

	// Check that ApplyRaftLog was called once for each log in the batch
	actions.AssertNumberOfCalls(t, "ApplyRaftLog", len(logs))

	// Check that the results returned by ApplyBatch match the expected results
	assert.Equal(t, len(logs), len(results))
}

func TestSnapshot(t *testing.T) {
	// Create a mock data store
	dataStore := &mocks.DataStore{}

	// Create an instance of store
	logger := hclog.New(&hclog.LoggerOptions{})
	actions := &mocks.Scheduler0RaftActions{}
	scheduler0config := config.NewScheduler0Config()

	fsmStore := NewFSMStore(logger, actions, scheduler0config, dataStore, nil, nil, nil, nil, nil)

	// Call Snapshot
	snapshot, err := fsmStore.GetFSM().Snapshot()

	// Check that the snapshot is not nil
	assert.NotNil(t, snapshot)

	// Check that there is no error returned
	assert.Nil(t, err)

	// Check that the snapshot implements the FSMSnapshot interface
	_, ok := snapshot.(raft.FSMSnapshot)
	assert.True(t, ok)
}
