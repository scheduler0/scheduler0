// Code generated by mockery v2.26.1. DO NOT EDIT.

package fsm

import (
	constants "scheduler0/constants"
	db "scheduler0/db"

	hclog "github.com/hashicorp/go-hclog"

	mock "github.com/stretchr/testify/mock"

	models "scheduler0/models"

	raft "github.com/hashicorp/raft"

	utils "scheduler0/utils"
)

// MockScheduler0RaftActions is an autogenerated mock type for the Scheduler0RaftActions type
type MockScheduler0RaftActions struct {
	mock.Mock
}

// ApplyRaftLog provides a mock function with given fields: logger, l, _a2, ignorePostProcessChannel
func (_m *MockScheduler0RaftActions) ApplyRaftLog(logger hclog.Logger, l *raft.Log, _a2 db.DataStore, ignorePostProcessChannel bool) interface{} {
	ret := _m.Called(logger, l, _a2, ignorePostProcessChannel)

	var r0 interface{}
	if rf, ok := ret.Get(0).(func(hclog.Logger, *raft.Log, db.DataStore, bool) interface{}); ok {
		r0 = rf(logger, l, _a2, ignorePostProcessChannel)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(interface{})
		}
	}

	return r0
}

// WriteCommandToRaftLog provides a mock function with given fields: rft, commandType, sqlString, params, nodeIds, action
func (_m *MockScheduler0RaftActions) WriteCommandToRaftLog(rft *raft.Raft, commandType constants.Command, sqlString string, params []interface{}, nodeIds []uint64, action constants.CommandAction) (*models.FSMResponse, *utils.GenericError) {
	ret := _m.Called(rft, commandType, sqlString, params, nodeIds, action)

	var r0 *models.FSMResponse
	var r1 *utils.GenericError
	if rf, ok := ret.Get(0).(func(*raft.Raft, constants.Command, string, []interface{}, []uint64, constants.CommandAction) (*models.FSMResponse, *utils.GenericError)); ok {
		return rf(rft, commandType, sqlString, params, nodeIds, action)
	}
	if rf, ok := ret.Get(0).(func(*raft.Raft, constants.Command, string, []interface{}, []uint64, constants.CommandAction) *models.FSMResponse); ok {
		r0 = rf(rft, commandType, sqlString, params, nodeIds, action)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*models.FSMResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(*raft.Raft, constants.Command, string, []interface{}, []uint64, constants.CommandAction) *utils.GenericError); ok {
		r1 = rf(rft, commandType, sqlString, params, nodeIds, action)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*utils.GenericError)
		}
	}

	return r0, r1
}

type mockConstructorTestingTNewMockScheduler0RaftActions interface {
	mock.TestingT
	Cleanup(func())
}

// NewMockScheduler0RaftActions creates a new instance of MockScheduler0RaftActions. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewMockScheduler0RaftActions(t mockConstructorTestingTNewMockScheduler0RaftActions) *MockScheduler0RaftActions {
	mock := &MockScheduler0RaftActions{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}