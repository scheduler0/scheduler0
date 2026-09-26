package job_execution_service

import (
	"errors"
	"scheduler0-private/pkg/models"
	"scheduler0-private/pkg/mocks"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_NewJobExecutionLogService(t *testing.T) {
	mockRepo := mocks.NewMockJobExecutionsRepo(t)

	service := NewJobExecutionLogService(mockRepo)

	assert.NotNil(t, service)
	assert.Implements(t, (*JobExecutionLogService)(nil), service)
}

func Test_JobExecutionLogService_GetExecutionLogsFiltered_Success(t *testing.T) {
	mockRepo := mocks.NewMockJobExecutionsRepo(t)
	service := NewJobExecutionLogService(mockRepo)

	accountId := uint64(1)
	startDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2024, 1, 31, 23, 59, 59, 0, time.UTC)

	expectedLogs := []models.JobExecutionLog{
		{
			Id:                    1,
			UniqueId:              "unique-1",
			State:                 models.ExecutionLogSuccessState,
			NodeId:                1,
			LastExecutionDatetime: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			NextExecutionDatetime: time.Date(2024, 1, 16, 10, 0, 0, 0, time.UTC),
			JobId:                 100,
			JobQueueVersion:       1,
			ExecutionVersion:      1,
			DateCreated:           time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			AccountId:             accountId,
		},
		{
			Id:                    2,
			UniqueId:              "unique-2",
			State:                 models.ExecutionLogFailedState,
			NodeId:                1,
			LastExecutionDatetime: time.Date(2024, 1, 20, 10, 0, 0, 0, time.UTC),
			NextExecutionDatetime: time.Date(2024, 1, 21, 10, 0, 0, 0, time.UTC),
			JobId:                 101,
			JobQueueVersion:       1,
			ExecutionVersion:      2,
			DateCreated:           time.Date(2024, 1, 20, 10, 0, 0, 0, time.UTC),
			AccountId:             accountId,
		},
	}

	mockRepo.EXPECT().
		GetExecutionLogsFiltered(accountId, startDate, endDate, (*uint64)(nil), (*uint64)(nil)).
		Return(expectedLogs, nil).
		Once()

	logs, err := service.GetExecutionLogsFiltered(accountId, startDate, endDate, nil, nil)

	assert.Nil(t, err)
	assert.NotNil(t, logs)
	assert.Equal(t, len(expectedLogs), len(logs))
	assert.Equal(t, expectedLogs[0].Id, logs[0].Id)
	assert.Equal(t, expectedLogs[0].UniqueId, logs[0].UniqueId)
	assert.Equal(t, expectedLogs[1].Id, logs[1].Id)
	assert.Equal(t, expectedLogs[1].UniqueId, logs[1].UniqueId)
}

func Test_JobExecutionLogService_GetExecutionLogsFiltered_WithProjectId(t *testing.T) {
	mockRepo := mocks.NewMockJobExecutionsRepo(t)
	service := NewJobExecutionLogService(mockRepo)

	accountId := uint64(1)
	projectId := uint64(10)
	startDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2024, 1, 31, 23, 59, 59, 0, time.UTC)

	expectedLogs := []models.JobExecutionLog{
		{
			Id:                    1,
			UniqueId:              "unique-1",
			State:                 models.ExecutionLogSuccessState,
			NodeId:                1,
			LastExecutionDatetime: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			NextExecutionDatetime: time.Date(2024, 1, 16, 10, 0, 0, 0, time.UTC),
			JobId:                 100,
			JobQueueVersion:       1,
			ExecutionVersion:      1,
			DateCreated:           time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			AccountId:             accountId,
		},
	}

	mockRepo.EXPECT().
		GetExecutionLogsFiltered(accountId, startDate, endDate, &projectId, (*uint64)(nil)).
		Return(expectedLogs, nil).
		Once()

	logs, err := service.GetExecutionLogsFiltered(accountId, startDate, endDate, &projectId, nil)

	assert.Nil(t, err)
	assert.NotNil(t, logs)
	assert.Equal(t, 1, len(logs))
	assert.Equal(t, expectedLogs[0].Id, logs[0].Id)
}

func Test_JobExecutionLogService_GetExecutionLogsFiltered_WithJobId(t *testing.T) {
	mockRepo := mocks.NewMockJobExecutionsRepo(t)
	service := NewJobExecutionLogService(mockRepo)

	accountId := uint64(1)
	jobId := uint64(100)
	startDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2024, 1, 31, 23, 59, 59, 0, time.UTC)

	expectedLogs := []models.JobExecutionLog{
		{
			Id:                    1,
			UniqueId:              "unique-1",
			State:                 models.ExecutionLogSuccessState,
			NodeId:                1,
			LastExecutionDatetime: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			NextExecutionDatetime: time.Date(2024, 1, 16, 10, 0, 0, 0, time.UTC),
			JobId:                 jobId,
			JobQueueVersion:       1,
			ExecutionVersion:      1,
			DateCreated:           time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			AccountId:             accountId,
		},
	}

	mockRepo.EXPECT().
		GetExecutionLogsFiltered(accountId, startDate, endDate, (*uint64)(nil), &jobId).
		Return(expectedLogs, nil).
		Once()

	logs, err := service.GetExecutionLogsFiltered(accountId, startDate, endDate, nil, &jobId)

	assert.Nil(t, err)
	assert.NotNil(t, logs)
	assert.Equal(t, 1, len(logs))
	assert.Equal(t, jobId, logs[0].JobId)
}

func Test_JobExecutionLogService_GetExecutionLogsFiltered_WithProjectIdAndJobId(t *testing.T) {
	mockRepo := mocks.NewMockJobExecutionsRepo(t)
	service := NewJobExecutionLogService(mockRepo)

	accountId := uint64(1)
	projectId := uint64(10)
	jobId := uint64(100)
	startDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2024, 1, 31, 23, 59, 59, 0, time.UTC)

	expectedLogs := []models.JobExecutionLog{
		{
			Id:                    1,
			UniqueId:              "unique-1",
			State:                 models.ExecutionLogSuccessState,
			NodeId:                1,
			LastExecutionDatetime: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			NextExecutionDatetime: time.Date(2024, 1, 16, 10, 0, 0, 0, time.UTC),
			JobId:                 jobId,
			JobQueueVersion:       1,
			ExecutionVersion:      1,
			DateCreated:           time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			AccountId:             accountId,
		},
	}

	mockRepo.EXPECT().
		GetExecutionLogsFiltered(accountId, startDate, endDate, &projectId, &jobId).
		Return(expectedLogs, nil).
		Once()

	logs, err := service.GetExecutionLogsFiltered(accountId, startDate, endDate, &projectId, &jobId)

	assert.Nil(t, err)
	assert.NotNil(t, logs)
	assert.Equal(t, 1, len(logs))
	assert.Equal(t, jobId, logs[0].JobId)
}

func Test_JobExecutionLogService_GetExecutionLogsFiltered_EmptyResults(t *testing.T) {
	mockRepo := mocks.NewMockJobExecutionsRepo(t)
	service := NewJobExecutionLogService(mockRepo)

	accountId := uint64(1)
	startDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2024, 1, 31, 23, 59, 59, 0, time.UTC)

	expectedLogs := []models.JobExecutionLog{}

	mockRepo.EXPECT().
		GetExecutionLogsFiltered(accountId, startDate, endDate, (*uint64)(nil), (*uint64)(nil)).
		Return(expectedLogs, nil).
		Once()

	logs, err := service.GetExecutionLogsFiltered(accountId, startDate, endDate, nil, nil)

	assert.Nil(t, err)
	assert.NotNil(t, logs)
	assert.Equal(t, 0, len(logs))
}

func Test_JobExecutionLogService_GetExecutionLogsFiltered_RepositoryError(t *testing.T) {
	mockRepo := mocks.NewMockJobExecutionsRepo(t)
	service := NewJobExecutionLogService(mockRepo)

	accountId := uint64(1)
	startDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2024, 1, 31, 23, 59, 59, 0, time.UTC)

	expectedError := errors.New("database connection error")

	mockRepo.EXPECT().
		GetExecutionLogsFiltered(accountId, startDate, endDate, (*uint64)(nil), (*uint64)(nil)).
		Return(nil, expectedError).
		Once()

	logs, err := service.GetExecutionLogsFiltered(accountId, startDate, endDate, nil, nil)

	assert.NotNil(t, err)
	assert.Nil(t, logs)
	assert.Equal(t, expectedError, err)
}

func Test_JobExecutionLogService_GetExecutionLogsFiltered_AllStates(t *testing.T) {
	mockRepo := mocks.NewMockJobExecutionsRepo(t)
	service := NewJobExecutionLogService(mockRepo)

	accountId := uint64(1)
	startDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2024, 1, 31, 23, 59, 59, 0, time.UTC)

	expectedLogs := []models.JobExecutionLog{
		{
			Id:                    1,
			UniqueId:              "unique-1",
			State:                 models.ExecutionLogScheduleState,
			NodeId:                1,
			LastExecutionDatetime: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			NextExecutionDatetime: time.Date(2024, 1, 16, 10, 0, 0, 0, time.UTC),
			JobId:                 100,
			JobQueueVersion:       1,
			ExecutionVersion:      1,
			DateCreated:           time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			AccountId:             accountId,
		},
		{
			Id:                    2,
			UniqueId:              "unique-2",
			State:                 models.ExecutionLogSuccessState,
			NodeId:                1,
			LastExecutionDatetime: time.Date(2024, 1, 20, 10, 0, 0, 0, time.UTC),
			NextExecutionDatetime: time.Date(2024, 1, 21, 10, 0, 0, 0, time.UTC),
			JobId:                 101,
			JobQueueVersion:       1,
			ExecutionVersion:      2,
			DateCreated:           time.Date(2024, 1, 20, 10, 0, 0, 0, time.UTC),
			AccountId:             accountId,
		},
		{
			Id:                    3,
			UniqueId:              "unique-3",
			State:                 models.ExecutionLogFailedState,
			NodeId:                1,
			LastExecutionDatetime: time.Date(2024, 1, 25, 10, 0, 0, 0, time.UTC),
			NextExecutionDatetime: time.Date(2024, 1, 26, 10, 0, 0, 0, time.UTC),
			JobId:                 102,
			JobQueueVersion:       1,
			ExecutionVersion:      3,
			DateCreated:           time.Date(2024, 1, 25, 10, 0, 0, 0, time.UTC),
			AccountId:             accountId,
		},
	}

	mockRepo.EXPECT().
		GetExecutionLogsFiltered(accountId, startDate, endDate, (*uint64)(nil), (*uint64)(nil)).
		Return(expectedLogs, nil).
		Once()

	logs, err := service.GetExecutionLogsFiltered(accountId, startDate, endDate, nil, nil)

	assert.Nil(t, err)
	assert.NotNil(t, logs)
	assert.Equal(t, 3, len(logs))
	assert.Equal(t, models.ExecutionLogScheduleState, logs[0].State)
	assert.Equal(t, models.ExecutionLogSuccessState, logs[1].State)
	assert.Equal(t, models.ExecutionLogFailedState, logs[2].State)
}

func Test_JobExecutionLogService_GetExecutionLogsFiltered_DifferentAccounts(t *testing.T) {
	mockRepo := mocks.NewMockJobExecutionsRepo(t)
	service := NewJobExecutionLogService(mockRepo)

	accountId1 := uint64(1)
	accountId2 := uint64(2)
	startDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2024, 1, 31, 23, 59, 59, 0, time.UTC)

	expectedLogs1 := []models.JobExecutionLog{
		{
			Id:        1,
			UniqueId:  "unique-1",
			State:     models.ExecutionLogSuccessState,
			AccountId: accountId1,
		},
	}

	expectedLogs2 := []models.JobExecutionLog{
		{
			Id:        2,
			UniqueId:  "unique-2",
			State:     models.ExecutionLogSuccessState,
			AccountId: accountId2,
		},
	}

	mockRepo.EXPECT().
		GetExecutionLogsFiltered(accountId1, startDate, endDate, (*uint64)(nil), (*uint64)(nil)).
		Return(expectedLogs1, nil).
		Once()

	mockRepo.EXPECT().
		GetExecutionLogsFiltered(accountId2, startDate, endDate, (*uint64)(nil), (*uint64)(nil)).
		Return(expectedLogs2, nil).
		Once()

	logs1, err1 := service.GetExecutionLogsFiltered(accountId1, startDate, endDate, nil, nil)
	assert.Nil(t, err1)
	assert.Equal(t, 1, len(logs1))
	assert.Equal(t, accountId1, logs1[0].AccountId)

	logs2, err2 := service.GetExecutionLogsFiltered(accountId2, startDate, endDate, nil, nil)
	assert.Nil(t, err2)
	assert.Equal(t, 1, len(logs2))
	assert.Equal(t, accountId2, logs2[0].AccountId)
}

func Test_JobExecutionLogService_GetExecutionLogsFiltered_DateRange(t *testing.T) {
	mockRepo := mocks.NewMockJobExecutionsRepo(t)
	service := NewJobExecutionLogService(mockRepo)

	accountId := uint64(1)
	startDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2024, 1, 31, 23, 59, 59, 0, time.UTC)

	expectedLogs := []models.JobExecutionLog{
		{
			Id:          1,
			UniqueId:    "unique-1",
			State:       models.ExecutionLogSuccessState,
			DateCreated: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			AccountId:   accountId,
		},
	}

	mockRepo.EXPECT().
		GetExecutionLogsFiltered(accountId, startDate, endDate, (*uint64)(nil), (*uint64)(nil)).
		Return(expectedLogs, nil).
		Once()

	logs, err := service.GetExecutionLogsFiltered(accountId, startDate, endDate, nil, nil)

	assert.Nil(t, err)
	assert.NotNil(t, logs)
	assert.Equal(t, 1, len(logs))
	// Verify the date is within the range
	assert.True(t, logs[0].DateCreated.After(startDate) || logs[0].DateCreated.Equal(startDate))
	assert.True(t, logs[0].DateCreated.Before(endDate) || logs[0].DateCreated.Equal(endDate))
}

