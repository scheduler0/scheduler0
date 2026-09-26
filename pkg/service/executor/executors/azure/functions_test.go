package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"scheduler0-private/pkg/models"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockHTTPClient is a mock implementation of HTTPClientInterface
type MockHTTPClient struct {
	mock.Mock
}

func (m *MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	args := m.Called(req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*http.Response), args.Error(1)
}

func payloadFor(job models.Job) models.JobInvocationPayload {
	return models.JobInvocationPayload{
		Job:                 job,
		LastExecutionStatus: models.ExecutionStateScheduled,
	}
}

func TestNewFunctionsExecutor(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "test",
		Level: hclog.LevelFromString("debug"),
	})
	ctx := context.Background()

	executor := NewFunctionsExecutor(logger, ctx)
	assert.NotNil(t, executor)
	assert.Implements(t, (*FunctionsExecutor)(nil), executor)
}

func TestExecuteFunctionJob_Success(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "test",
		Level: hclog.LevelFromString("debug"),
	})
	ctx := context.Background()

	// Create a test job
	testJob := models.Job{
		ID:          1,
		ProjectID:   1,
		Spec:        "* * * * *",
		Timezone:    "UTC",
		Data:        `{"key": "value"}`,
		Status:      "active",
		DateCreated: time.Now(),
	}

	// Track callback invocations
	var successCalled bool
	var errorCalled bool
	var callbackJob models.Job

	successCallback := func(job models.Job) {
		successCalled = true
		callbackJob = job
	}

	errorCallback := func(job models.Job) {
		errorCalled = true
		callbackJob = job
	}

	// Create a mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method
		assert.Equal(t, http.MethodPost, r.Method)

		// Verify headers
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "test-function-key", r.Header.Get("x-functions-key"))

		// Verify request body
		var received models.JobInvocationPayload
		err := json.NewDecoder(r.Body).Decode(&received)
		assert.NoError(t, err)
		assert.Equal(t, testJob.ID, received.Job.ID)
		assert.Equal(t, testJob.ProjectID, received.Job.ProjectID)

		// Return success status
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "success"}`))
	}))
	defer server.Close()

	executor := NewFunctionsExecutor(logger, ctx)
	executor.ExecuteFunctionJob(
		server.URL,
		"test-function-key",
		payloadFor(testJob),
		successCallback,
		errorCallback,
	)

	// Wait a bit for async operations
	time.Sleep(100 * time.Millisecond)

	// Verify callbacks
	assert.True(t, successCalled, "success callback should be called")
	assert.False(t, errorCalled, "error callback should not be called")
	assert.Equal(t, testJob.ID, callbackJob.ID)
}

func TestExecuteFunctionJob_Success_NoFunctionKey(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "test",
		Level: hclog.LevelFromString("debug"),
	})
	ctx := context.Background()

	testJob := models.Job{
		ID:          1,
		ProjectID:   1,
		Spec:        "* * * * *",
		Timezone:    "UTC",
		Status:      "active",
		DateCreated: time.Now(),
	}

	var successCalled bool
	var errorCalled bool

	successCallback := func(job models.Job) {
		successCalled = true
	}

	errorCallback := func(job models.Job) {
		errorCalled = true
	}

	// Create a mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify that x-functions-key header is not set when functionKey is empty
		assert.Empty(t, r.Header.Get("x-functions-key"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	executor := NewFunctionsExecutor(logger, ctx)
	executor.ExecuteFunctionJob(
		server.URL,
		"", // Empty function key
		payloadFor(testJob),
		successCallback,
		errorCallback,
	)

	time.Sleep(100 * time.Millisecond)

	assert.True(t, successCalled, "success callback should be called")
	assert.False(t, errorCalled, "error callback should not be called")
}

func TestExecuteFunctionJob_Success_2xxStatusCodes(t *testing.T) {
	statusCodes := []int{200, 201, 202, 204}

	for _, statusCode := range statusCodes {
		t.Run(fmt.Sprintf("Status_%d", statusCode), func(t *testing.T) {
			logger := hclog.New(&hclog.LoggerOptions{
				Name:  "test",
				Level: hclog.LevelFromString("debug"),
			})
			ctx := context.Background()

			testJob := models.Job{
				ID:          1,
				ProjectID:   1,
				Spec:        "* * * * *",
				Timezone:    "UTC",
				Status:      "active",
				DateCreated: time.Now(),
			}

			var successCalled bool
			var errorCalled bool

			successCallback := func(job models.Job) {
				successCalled = true
			}

			errorCallback := func(job models.Job) {
				errorCalled = true
			}

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(statusCode)
			}))
			defer server.Close()

			executor := NewFunctionsExecutor(logger, ctx)
			executor.ExecuteFunctionJob(
				server.URL,
				"test-key",
				payloadFor(testJob),
				successCallback,
				errorCallback,
			)

			time.Sleep(100 * time.Millisecond)

			assert.True(t, successCalled, "success callback should be called for status %d", statusCode)
			assert.False(t, errorCalled, "error callback should not be called for status %d", statusCode)
		})
	}
}

func TestExecuteFunctionJob_Failure_Non2xxStatusCodes(t *testing.T) {
	statusCodes := []int{400, 401, 403, 404, 500, 502, 503}

	for _, statusCode := range statusCodes {
		t.Run(fmt.Sprintf("Status_%d", statusCode), func(t *testing.T) {
			logger := hclog.New(&hclog.LoggerOptions{
				Name:  "test",
				Level: hclog.LevelFromString("debug"),
			})
			ctx := context.Background()

			testJob := models.Job{
				ID:          1,
				ProjectID:   1,
				Spec:        "* * * * *",
				Timezone:    "UTC",
				Status:      "active",
				DateCreated: time.Now(),
			}

			var successCalled bool
			var errorCalled bool

			successCallback := func(job models.Job) {
				successCalled = true
			}

			errorCallback := func(job models.Job) {
				errorCalled = true
			}

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(statusCode)
				w.Write([]byte(`{"error": "something went wrong"}`))
			}))
			defer server.Close()

			executor := NewFunctionsExecutor(logger, ctx)
			executor.ExecuteFunctionJob(
				server.URL,
				"test-key",
				payloadFor(testJob),
				successCallback,
				errorCallback,
			)

			time.Sleep(100 * time.Millisecond)

			assert.False(t, successCalled, "success callback should not be called for status %d", statusCode)
			assert.True(t, errorCalled, "error callback should be called for status %d", statusCode)
		})
	}
}

func TestExecuteFunctionJob_NetworkError(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "test",
		Level: hclog.LevelFromString("debug"),
	})
	ctx := context.Background()

	testJob := models.Job{
		ID:          1,
		ProjectID:   1,
		Spec:        "* * * * *",
		Timezone:    "UTC",
		Status:      "active",
		DateCreated: time.Now(),
	}

	var successCalled bool
	var errorCalled bool

	successCallback := func(job models.Job) {
		successCalled = true
	}

	errorCallback := func(job models.Job) {
		errorCalled = true
	}

	// Use an invalid URL to simulate network error
	executor := NewFunctionsExecutor(logger, ctx)
	executor.ExecuteFunctionJob(
		"http://invalid-url-that-does-not-exist:9999",
		"test-key",
		payloadFor(testJob),
		successCallback,
		errorCallback,
	)

	// Wait longer for network timeout
	time.Sleep(200 * time.Millisecond)

	assert.False(t, successCalled, "success callback should not be called on network error")
	assert.True(t, errorCalled, "error callback should be called on network error")
}

func TestExecuteFunctionJob_RequestTimeout(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "test",
		Level: hclog.LevelFromString("debug"),
	})
	ctx := context.Background()
	mockClient := new(MockHTTPClient)

	executor := NewFunctionsExecutorWithClient(logger, ctx, mockClient).(*FunctionsExecutionHandler)

	testJob := models.Job{
		ID:          1,
		ProjectID:   1,
		Spec:        "* * * * *",
		Timezone:    "UTC",
		Status:      "active",
		DateCreated: time.Now(),
	}

	var successCalled bool
	var errorCalled bool

	successCallback := func(job models.Job) {
		successCalled = true
	}

	errorCallback := func(job models.Job) {
		errorCalled = true
	}

	// Mock timeout error (context deadline exceeded)
	mockClient.On("Do", mock.Anything).Return(nil, context.DeadlineExceeded)

	executor.ExecuteFunctionJob(
		"https://example.com/function",
		"test-key",
		payloadFor(testJob),
		successCallback,
		errorCallback,
	)

	time.Sleep(100 * time.Millisecond)

	assert.False(t, successCalled, "success callback should not be called on timeout")
	assert.True(t, errorCalled, "error callback should be called on timeout")
	mockClient.AssertExpectations(t)
}

func TestExecuteFunctionJob_RequestBodyVerification(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "test",
		Level: hclog.LevelFromString("debug"),
	})
	ctx := context.Background()

	testJob := models.Job{
		ID:          42,
		ProjectID:   100,
		Spec:        "0 0 * * *",
		Timezone:    "America/New_York",
		Data:        `{"custom": "data"}`,
		Status:      "active",
		DateCreated: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	var received models.JobInvocationPayload
	var successCalled bool

	successCallback := func(job models.Job) {
		successCalled = true
	}

	errorCallback := func(job models.Job) {
		t.Fatal("error callback should not be called")
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := json.NewDecoder(r.Body).Decode(&received)
		assert.NoError(t, err)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	executor := NewFunctionsExecutor(logger, ctx)
	executor.ExecuteFunctionJob(
		server.URL,
		"test-key",
		payloadFor(testJob),
		successCallback,
		errorCallback,
	)

	time.Sleep(100 * time.Millisecond)

	assert.True(t, successCalled)
	assert.Equal(t, testJob.ID, received.Job.ID)
	assert.Equal(t, testJob.ProjectID, received.Job.ProjectID)
	assert.Equal(t, testJob.Spec, received.Job.Spec)
	assert.Equal(t, testJob.Timezone, received.Job.Timezone)
	assert.Equal(t, testJob.Data, received.Job.Data)
}

func TestExecuteFunctionJobBatch_IncludesAllJobsInPayload(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "test",
		Level: hclog.LevelFromString("debug"),
	})
	ctx := context.Background()

	executorID := uint64(9)
	jobs := []models.Job{
		{ID: 4, ProjectID: 2, Spec: "@every 10s", Data: `{"job":"alpha"}`, ExecutorId: &executorID, Status: models.JobStatusActive},
		{ID: 5, ProjectID: 2, Spec: "@every 10s", Data: `{"job":"beta"}`, ExecutorId: &executorID, Status: models.JobStatusActive},
		{ID: 6, ProjectID: 2, Spec: "@every 10s", Data: `{"job":"gamma"}`, ExecutorId: &executorID, Status: models.JobStatusActive},
	}
	batch := models.AggregatedJobInvocationPayload{Aggregated: true}
	for _, job := range jobs {
		batch.Jobs = append(batch.Jobs, payloadFor(job))
	}

	var received models.AggregatedJobInvocationPayload
	successIDs := []uint64{}
	errorCalled := false

	successCallback := func(got []models.Job) {
		for _, j := range got {
			successIDs = append(successIDs, j.ID)
		}
	}
	errorCallback := func(got []models.Job) {
		errorCalled = true
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "agg-key", r.Header.Get("x-functions-key"))
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&received))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	executor := NewFunctionsExecutor(logger, ctx)
	executor.ExecuteFunctionJobBatch(server.URL, "agg-key", batch, successCallback, errorCallback)
	time.Sleep(100 * time.Millisecond)

	assert.False(t, errorCalled)
	assert.Equal(t, []uint64{4, 5, 6}, successIDs)
	assert.True(t, received.Aggregated)
	assert.Len(t, received.Jobs, 3)
	assert.Equal(t, uint64(4), received.Jobs[0].Job.ID)
	assert.Equal(t, `{"job":"alpha"}`, received.Jobs[0].Job.Data)
	assert.Equal(t, uint64(5), received.Jobs[1].Job.ID)
	assert.Equal(t, `{"job":"beta"}`, received.Jobs[1].Job.Data)
	assert.Equal(t, uint64(6), received.Jobs[2].Job.ID)
	assert.Equal(t, `{"job":"gamma"}`, received.Jobs[2].Job.Data)
}
