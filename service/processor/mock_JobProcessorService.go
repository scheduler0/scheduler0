// Code generated by mockery v2.26.1. DO NOT EDIT.

package processor

import mock "github.com/stretchr/testify/mock"

// MockJobProcessorService is an autogenerated mock type for the JobProcessorService type
type MockJobProcessorService struct {
	mock.Mock
}

// RecoverJobs provides a mock function with given fields:
func (_m *MockJobProcessorService) RecoverJobs() {
	_m.Called()
}

// StartJobs provides a mock function with given fields:
func (_m *MockJobProcessorService) StartJobs() {
	_m.Called()
}

type mockConstructorTestingTNewMockJobProcessorService interface {
	mock.TestingT
	Cleanup(func())
}

// NewMockJobProcessorService creates a new instance of MockJobProcessorService. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewMockJobProcessorService(t mockConstructorTestingTNewMockJobProcessorService) *MockJobProcessorService {
	mock := &MockJobProcessorService{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}