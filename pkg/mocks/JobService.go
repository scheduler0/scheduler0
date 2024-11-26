// Code generated by mockery v2.26.1. DO NOT EDIT.

package mocks

import (
	models "scheduler0/pkg/models"

	mock "github.com/stretchr/testify/mock"

	utils "scheduler0/pkg/utils"
)

// JobService is an autogenerated mock type for the JobService type
type JobService struct {
	mock.Mock
}

// BatchInsertJobs provides a mock function with given fields: requestId, jobs
func (_m *JobService) BatchInsertJobs(requestId string, jobs []models.Job) ([]uint64, *utils.GenericError) {
	ret := _m.Called(requestId, jobs)

	var r0 []uint64
	var r1 *utils.GenericError
	if rf, ok := ret.Get(0).(func(string, []models.Job) ([]uint64, *utils.GenericError)); ok {
		return rf(requestId, jobs)
	}
	if rf, ok := ret.Get(0).(func(string, []models.Job) []uint64); ok {
		r0 = rf(requestId, jobs)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]uint64)
		}
	}

	if rf, ok := ret.Get(1).(func(string, []models.Job) *utils.GenericError); ok {
		r1 = rf(requestId, jobs)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*utils.GenericError)
		}
	}

	return r0, r1
}

// DeleteJob provides a mock function with given fields: job
func (_m *JobService) DeleteJob(job models.Job) *utils.GenericError {
	ret := _m.Called(job)

	var r0 *utils.GenericError
	if rf, ok := ret.Get(0).(func(models.Job) *utils.GenericError); ok {
		r0 = rf(job)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*utils.GenericError)
		}
	}

	return r0
}

// GetJob provides a mock function with given fields: job
func (_m *JobService) GetJob(job models.Job) (*models.Job, *utils.GenericError) {
	ret := _m.Called(job)

	var r0 *models.Job
	var r1 *utils.GenericError
	if rf, ok := ret.Get(0).(func(models.Job) (*models.Job, *utils.GenericError)); ok {
		return rf(job)
	}
	if rf, ok := ret.Get(0).(func(models.Job) *models.Job); ok {
		r0 = rf(job)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*models.Job)
		}
	}

	if rf, ok := ret.Get(1).(func(models.Job) *utils.GenericError); ok {
		r1 = rf(job)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*utils.GenericError)
		}
	}

	return r0, r1
}

// GetJobsByProjectID provides a mock function with given fields: projectID, offset, limit, orderBy
func (_m *JobService) GetJobsByProjectID(projectID uint64, offset uint64, limit uint64, orderBy string) (*models.PaginatedJob, *utils.GenericError) {
	ret := _m.Called(projectID, offset, limit, orderBy)

	var r0 *models.PaginatedJob
	var r1 *utils.GenericError
	if rf, ok := ret.Get(0).(func(uint64, uint64, uint64, string) (*models.PaginatedJob, *utils.GenericError)); ok {
		return rf(projectID, offset, limit, orderBy)
	}
	if rf, ok := ret.Get(0).(func(uint64, uint64, uint64, string) *models.PaginatedJob); ok {
		r0 = rf(projectID, offset, limit, orderBy)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*models.PaginatedJob)
		}
	}

	if rf, ok := ret.Get(1).(func(uint64, uint64, uint64, string) *utils.GenericError); ok {
		r1 = rf(projectID, offset, limit, orderBy)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*utils.GenericError)
		}
	}

	return r0, r1
}

// QueueJobs provides a mock function with given fields: jobs
func (_m *JobService) QueueJobs(jobs []models.Job) {
	_m.Called(jobs)
}

// UpdateJob provides a mock function with given fields: job
func (_m *JobService) UpdateJob(job models.Job) (*models.Job, *utils.GenericError) {
	ret := _m.Called(job)

	var r0 *models.Job
	var r1 *utils.GenericError
	if rf, ok := ret.Get(0).(func(models.Job) (*models.Job, *utils.GenericError)); ok {
		return rf(job)
	}
	if rf, ok := ret.Get(0).(func(models.Job) *models.Job); ok {
		r0 = rf(job)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*models.Job)
		}
	}

	if rf, ok := ret.Get(1).(func(models.Job) *utils.GenericError); ok {
		r1 = rf(job)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*utils.GenericError)
		}
	}

	return r0, r1
}

type mockConstructorTestingTNewJobService interface {
	mock.TestingT
	Cleanup(func())
}

// NewJobService creates a new instance of JobService. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewJobService(t mockConstructorTestingTNewJobService) *JobService {
	mock := &JobService{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}