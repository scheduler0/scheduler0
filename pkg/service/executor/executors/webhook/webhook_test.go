package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"scheduler0/pkg/config"
	"scheduler0/pkg/models"
	"scheduler0/pkg/utils"
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

// createTestDispatcher creates a dispatcher that executes immediately for testing
func createTestDispatcher(ctx context.Context) *utils.Dispatcher {
	dispatcher := utils.NewDispatcher(ctx, 1, 1)
	dispatcher.Run()
	return dispatcher
}

func payloadFor(job models.Job) models.JobInvocationPayload {
	return models.JobInvocationPayload{
		Job:                 job,
		LastExecutionStatus: models.ExecutionStateScheduled,
	}
}

func TestNewWebhookExecutor(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	ctx := context.Background()
	scheduler0config := config.NewScheduler0Config()
	dispatcher := utils.NewDispatcher(ctx, 1, 1)

	executor := NewWebhookExecutor(logger, ctx, scheduler0config, dispatcher)
	assert.NotNil(t, executor)
	assert.Implements(t, (*WebhookExecutor)(nil), executor)
}

func TestNewWebhookExecutorWithClient(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	ctx := context.Background()
	scheduler0config := config.NewScheduler0Config()
	dispatcher := utils.NewDispatcher(ctx, 1, 1)
	mockClient := new(MockHTTPClient)

	executor := NewWebhookExecutorWithClient(logger, ctx, scheduler0config, dispatcher, mockClient)
	assert.NotNil(t, executor)
	assert.Implements(t, (*WebhookExecutor)(nil), executor)
}

func TestWebhookExecutionHandler_ExecuteWebhookJob_Success(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	ctx := context.Background()
	scheduler0config := config.NewScheduler0Config()
	dispatcher := createTestDispatcher(ctx)
	mockClient := new(MockHTTPClient)

	executor := NewWebhookExecutorWithClient(logger, ctx, scheduler0config, dispatcher, mockClient).(*WebhookExecutionHandler)

	job := models.Job{
		ID:        1,
		ProjectID: 1,
		Spec:      "* * * * *",
		Data:      "test data",
		Timezone:  "UTC",
		RetryMax:  0,
	}

	executorModel := models.JobExecutor{
		ID:            1,
		WebhookUrl:    "https://example.com/webhook",
		WebhookMethod: "POST",
		WebhookSecret: "secret-key",
	}

	successCalled := false
	errorCalled := false

	successCallback := func(job models.Job) {
		successCalled = true
		assert.Equal(t, uint64(1), job.ID)
	}

	errorCallback := func(job models.Job) {
		errorCalled = true
	}

	// Mock successful HTTP response
	mockClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		return req.Method == "POST" &&
			req.URL.String() == "https://example.com/webhook" &&
			req.Header.Get("Content-Type") == "application/json" &&
			req.Header.Get("X-Webhook-Secret") == "secret-key"
	})).Return(&http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString("OK")),
	}, nil)

	executor.ExecuteWebhookJob(executorModel, payloadFor(job), successCallback, errorCallback)

	time.Sleep(100 * time.Millisecond)

	assert.True(t, successCalled, "success callback should be called")
	assert.False(t, errorCalled, "error callback should not be called")
	mockClient.AssertExpectations(t)
}

func TestWebhookExecutionHandler_ExecuteWebhookJob_DefaultMethod(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	ctx := context.Background()
	scheduler0config := config.NewScheduler0Config()
	dispatcher := createTestDispatcher(ctx)
	mockClient := new(MockHTTPClient)

	executor := NewWebhookExecutorWithClient(logger, ctx, scheduler0config, dispatcher, mockClient).(*WebhookExecutionHandler)

	job := models.Job{
		ID:       1,
		RetryMax: 0,
	}

	executorModel := models.JobExecutor{
		ID:         1,
		WebhookUrl: "https://example.com/webhook",
		// WebhookMethod is empty, should default to POST
	}

	successCalled := false

	successCallback := func(job models.Job) {
		successCalled = true
	}

	errorCallback := func(job models.Job) {
		t.Fatal("error callback should not be called")
	}

	mockClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		return req.Method == "POST" // Should default to POST
	})).Return(&http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString("OK")),
	}, nil)

	executor.ExecuteWebhookJob(executorModel, payloadFor(job), successCallback, errorCallback)

	time.Sleep(100 * time.Millisecond)

	assert.True(t, successCalled, "success callback should be called")
	mockClient.AssertExpectations(t)
}

func TestWebhookExecutionHandler_ExecuteWebhookJob_DifferentMethods(t *testing.T) {
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			logger := hclog.New(&hclog.LoggerOptions{
				Name:  "test",
				Level: hclog.LevelFromString("DEBUG"),
			})
			ctx := context.Background()
			scheduler0config := config.NewScheduler0Config()
			dispatcher := createTestDispatcher(ctx)
			mockClient := new(MockHTTPClient)

			executor := NewWebhookExecutorWithClient(logger, ctx, scheduler0config, dispatcher, mockClient).(*WebhookExecutionHandler)

			job := models.Job{
				ID:       1,
				RetryMax: 0,
			}

			executorModel := models.JobExecutor{
				ID:            1,
				WebhookUrl:    "https://example.com/webhook",
				WebhookMethod: method,
			}

			successCalled := false

			successCallback := func(job models.Job) {
				successCalled = true
			}

			errorCallback := func(job models.Job) {
				t.Fatal("error callback should not be called")
			}

			mockClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
				return req.Method == method
			})).Return(&http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString("OK")),
			}, nil)

			executor.ExecuteWebhookJob(executorModel, payloadFor(job), successCallback, errorCallback)

			time.Sleep(100 * time.Millisecond)

			assert.True(t, successCalled, "success callback should be called for method "+method)
			mockClient.AssertExpectations(t)
		})
	}
}

func TestWebhookExecutionHandler_ExecuteWebhookJob_NoWebhookSecret(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	ctx := context.Background()
	scheduler0config := config.NewScheduler0Config()
	dispatcher := createTestDispatcher(ctx)
	mockClient := new(MockHTTPClient)

	executor := NewWebhookExecutorWithClient(logger, ctx, scheduler0config, dispatcher, mockClient).(*WebhookExecutionHandler)

	job := models.Job{
		ID:       1,
		RetryMax: 0,
	}

	executorModel := models.JobExecutor{
		ID:            1,
		WebhookUrl:    "https://example.com/webhook",
		WebhookMethod: "POST",
		// WebhookSecret is empty
	}

	successCalled := false

	successCallback := func(job models.Job) {
		successCalled = true
	}

	errorCallback := func(job models.Job) {
		t.Fatal("error callback should not be called")
	}

	mockClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		// X-Webhook-Secret header should not be set
		return req.Header.Get("X-Webhook-Secret") == ""
	})).Return(&http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString("OK")),
	}, nil)

	executor.ExecuteWebhookJob(executorModel, payloadFor(job), successCallback, errorCallback)

	time.Sleep(100 * time.Millisecond)

	assert.True(t, successCalled, "success callback should be called")
	mockClient.AssertExpectations(t)
}

func TestWebhookExecutionHandler_ExecuteWebhookJob_JSONMarshalError(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	ctx := context.Background()
	scheduler0config := config.NewScheduler0Config()
	dispatcher := createTestDispatcher(ctx)
	mockClient := new(MockHTTPClient)

	executor := NewWebhookExecutorWithClient(logger, ctx, scheduler0config, dispatcher, mockClient).(*WebhookExecutionHandler)

	// Create a valid job (Job struct should always be marshalable)
	// Note: We cannot easily create an unmarshalable job with the Job struct,
	// so this test verifies that the marshal path is executed correctly
	job := models.Job{
		ID:        1,
		ProjectID: 1,
		Spec:      "* * * * *",
		Timezone:  "UTC",
		RetryMax:  0,
	}

	executorModel := models.JobExecutor{
		ID:            1,
		WebhookUrl:    "https://example.com/webhook",
		WebhookMethod: "POST",
	}

	var successCalled bool
	var errorCalled bool

	successCallback := func(job models.Job) {
		successCalled = true
	}

	errorCallback := func(job models.Job) {
		errorCalled = true
	}

	// Mock network error to verify the marshal path was executed
	// (marshaling succeeds, but HTTP request fails)
	mockClient.On("Do", mock.Anything).Return(nil, errors.New("network error"))

	executor.ExecuteWebhookJob(executorModel, payloadFor(job), successCallback, errorCallback)

	time.Sleep(200 * time.Millisecond) // Wait for retry logic

	// Verify that marshaling succeeded (no error at marshal stage)
	// but HTTP request failed (error callback called)
	assert.False(t, successCalled, "success callback should not be called on network error")
	assert.True(t, errorCalled, "error callback should be called on network error")
	mockClient.AssertExpectations(t)
}

func TestWebhookExecutionHandler_ExecuteWebhookJob_RequestCreationError(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	ctx := context.Background()
	scheduler0config := config.NewScheduler0Config()
	dispatcher := createTestDispatcher(ctx)

	executor := NewWebhookExecutor(logger, ctx, scheduler0config, dispatcher).(*WebhookExecutionHandler)

	job := models.Job{
		ID:       1,
		RetryMax: 0,
	}

	// Use an invalid URL that will cause http.NewRequestWithContext to fail
	// An invalid URL format will cause the request creation to fail
	executorModel := models.JobExecutor{
		ID:            1,
		WebhookUrl:    "://invalid-url-format", // Invalid URL format
		WebhookMethod: "POST",
	}

	successCalled := false
	errorCalled := false

	successCallback := func(job models.Job) {
		successCalled = true
	}

	errorCallback := func(job models.Job) {
		errorCalled = true
	}

	executor.ExecuteWebhookJob(executorModel, payloadFor(job), successCallback, errorCallback)

	time.Sleep(100 * time.Millisecond)

	assert.False(t, successCalled, "success callback should not be called")
	assert.True(t, errorCalled, "error callback should be called when request creation fails")
}

func TestWebhookExecutionHandler_ExecuteWebhookJob_HTTPError(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	ctx := context.Background()
	scheduler0config := config.NewScheduler0Config()
	dispatcher := createTestDispatcher(ctx)
	mockClient := new(MockHTTPClient)

	executor := NewWebhookExecutorWithClient(logger, ctx, scheduler0config, dispatcher, mockClient).(*WebhookExecutionHandler)

	job := models.Job{
		ID:       1,
		RetryMax: 0, // No retries for faster test
	}

	executorModel := models.JobExecutor{
		ID:            1,
		WebhookUrl:    "https://example.com/webhook",
		WebhookMethod: "POST",
	}

	successCalled := false
	errorCalled := false

	successCallback := func(job models.Job) {
		successCalled = true
	}

	errorCallback := func(job models.Job) {
		errorCalled = true
	}

	// Mock HTTP error
	mockClient.On("Do", mock.Anything).Return(nil, errors.New("network error"))

	executor.ExecuteWebhookJob(executorModel, payloadFor(job), successCallback, errorCallback)

	time.Sleep(200 * time.Millisecond) // Wait for retry logic

	assert.False(t, successCalled, "success callback should not be called")
	assert.True(t, errorCalled, "error callback should be called")
	mockClient.AssertExpectations(t)
}

func TestWebhookExecutionHandler_ExecuteWebhookJob_StatusError(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	ctx := context.Background()
	scheduler0config := config.NewScheduler0Config()
	dispatcher := createTestDispatcher(ctx)
	mockClient := new(MockHTTPClient)

	executor := NewWebhookExecutorWithClient(logger, ctx, scheduler0config, dispatcher, mockClient).(*WebhookExecutionHandler)

	job := models.Job{
		ID:       1,
		RetryMax: 0, // No retries
	}

	executorModel := models.JobExecutor{
		ID:            1,
		WebhookUrl:    "https://example.com/webhook",
		WebhookMethod: "POST",
	}

	successCalled := false
	errorCalled := false

	successCallback := func(job models.Job) {
		successCalled = true
	}

	errorCallback := func(job models.Job) {
		errorCalled = true
	}

	// Mock error status code
	mockClient.On("Do", mock.Anything).Return(&http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       io.NopCloser(bytes.NewBufferString("Error")),
	}, nil)

	executor.ExecuteWebhookJob(executorModel, payloadFor(job), successCallback, errorCallback)

	time.Sleep(200 * time.Millisecond)

	assert.False(t, successCalled, "success callback should not be called")
	assert.True(t, errorCalled, "error callback should be called")
	mockClient.AssertExpectations(t)
}

// Regression: the executor used to build one *http.Request and hand the same
// pointer to every retry. The body reader was drained by the first client.Do,
// so attempts 2..N went out with an empty body (and the original
// Content-Length), which remote ends reject with 400. Every attempt must now
// carry the full payload and its own request instance.
func TestWebhookExecutionHandler_ExecuteWebhookJob_RetriesResendFullBody(t *testing.T) {
	t.Setenv("SCHEDULER0_JOB_EXECUTION_RETRY_DELAY", "0")

	logger := hclog.New(&hclog.LoggerOptions{Name: "test", Level: hclog.LevelFromString("DEBUG")})
	ctx := context.Background()
	scheduler0config := config.NewScheduler0Config()
	dispatcher := createTestDispatcher(ctx)
	mockClient := new(MockHTTPClient)

	executor := NewWebhookExecutorWithClient(logger, ctx, scheduler0config, dispatcher, mockClient).(*WebhookExecutionHandler)

	job := models.Job{ID: 42, ProjectID: 7, RetryMax: 2, Data: `{"jobType":"welcome_email"}`}
	executorModel := models.JobExecutor{
		ID:            1,
		WebhookUrl:    "https://example.com/webhook",
		WebhookMethod: "POST",
		WebhookSecret: "s3cret",
	}

	expected, err := json.Marshal(payloadFor(job))
	assert.NoError(t, err)

	var bodies [][]byte
	var reqs []*http.Request
	mockClient.On("Do", mock.Anything).Run(func(args mock.Arguments) {
		req := args.Get(0).(*http.Request)
		reqs = append(reqs, req)
		b, _ := io.ReadAll(req.Body) // drain, exactly as a real transport would
		bodies = append(bodies, b)
		assert.Equal(t, "s3cret", req.Header.Get("X-Webhook-Secret"))
		assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
	}).Return(&http.Response{
		StatusCode: http.StatusBadGateway,
		Body:       io.NopCloser(bytes.NewBufferString("upstream unavailable")),
	}, nil)

	done := make(chan struct{})
	successCalled := false
	executor.ExecuteWebhookJob(executorModel, payloadFor(job),
		func(models.Job) { successCalled = true; close(done) },
		func(models.Job) { close(done) },
	)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("callback not invoked")
	}

	assert.False(t, successCalled)
	// 1 initial attempt + RetryMax retries.
	assert.Len(t, bodies, 3, "expected initial attempt plus 2 retries")
	for i, b := range bodies {
		assert.JSONEq(t, string(expected), string(b), "attempt %d must carry the full payload", i+1)
	}
	assert.NotSame(t, reqs[0], reqs[1], "each attempt must use a fresh *http.Request")
	assert.NotSame(t, reqs[1], reqs[2], "each attempt must use a fresh *http.Request")
	mockClient.AssertExpectations(t)
}

// The error surfaced to the caller/logs should include a bounded snippet of the
// failing response body so operators can tell a WAF 403 from an app 400.
func TestWebhookExecutionHandler_ExecuteWebhookJob_ErrorIncludesResponseSnippet(t *testing.T) {
	t.Setenv("SCHEDULER0_JOB_EXECUTION_RETRY_DELAY", "0")

	logger := hclog.New(&hclog.LoggerOptions{Name: "test", Level: hclog.LevelFromString("DEBUG")})
	ctx := context.Background()
	scheduler0config := config.NewScheduler0Config()
	mockClient := new(MockHTTPClient)

	executor := NewWebhookExecutorWithClient(logger, ctx, scheduler0config, createTestDispatcher(ctx), mockClient).(*WebhookExecutionHandler)
	executorModel := models.JobExecutor{ID: 1, WebhookUrl: "https://example.com/webhook", WebhookMethod: "POST"}

	long := bytes.Repeat([]byte("x"), maxErrorBodyBytes*4)
	mockClient.On("Do", mock.Anything).Return(&http.Response{
		StatusCode: http.StatusForbidden,
		Body:       io.NopCloser(bytes.NewReader(long)),
	}, nil)

	err := executor.sendWithRetries(executorModel, []byte(`{}`), 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "status code 403")
	assert.LessOrEqual(t, len(err.Error()), maxErrorBodyBytes+len("webhook returned status code 403: "),
		"response snippet must be bounded")
	mockClient.AssertExpectations(t)
}

func TestWebhookExecutionHandler_ExecuteWebhookJob_RequestBodyVerification(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	ctx := context.Background()
	scheduler0config := config.NewScheduler0Config()
	dispatcher := createTestDispatcher(ctx)
	mockClient := new(MockHTTPClient)

	executor := NewWebhookExecutorWithClient(logger, ctx, scheduler0config, dispatcher, mockClient).(*WebhookExecutionHandler)

	job := models.Job{
		ID:          42,
		ProjectID:   100,
		Spec:        "0 0 * * *",
		Timezone:    "America/New_York",
		Data:        `{"custom": "data"}`,
		RetryMax:    0,
		DateCreated: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	executorModel := models.JobExecutor{
		ID:            1,
		WebhookUrl:    "https://example.com/webhook",
		WebhookMethod: "POST",
	}

	var receivedPayload []byte

	successCallback := func(job models.Job) {
		// Verify job data
		assert.Equal(t, uint64(42), job.ID)
	}

	errorCallback := func(job models.Job) {
		t.Fatal("error callback should not be called")
	}

	mockClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		// Read the body
		if req.Body != nil {
			receivedPayload, _ = io.ReadAll(req.Body)
			req.Body = io.NopCloser(bytes.NewBuffer(receivedPayload))
		}
		return true
	})).Return(&http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString("OK")),
	}, nil)

	executor.ExecuteWebhookJob(executorModel, payloadFor(job), successCallback, errorCallback)

	time.Sleep(100 * time.Millisecond)

	// Verify payload matches JobInvocationPayload shape
	var received models.JobInvocationPayload
	err := json.Unmarshal(receivedPayload, &received)
	assert.NoError(t, err)
	assert.Equal(t, job.ID, received.Job.ID)
	assert.Equal(t, job.ProjectID, received.Job.ProjectID)
	assert.Equal(t, job.Spec, received.Job.Spec)
	assert.Equal(t, job.Data, received.Job.Data)
	mockClient.AssertExpectations(t)
}

func TestWebhookExecutionHandler_ExecuteWebhookJobBatch_IncludesAllJobsInPayload(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "test",
		Level: hclog.LevelFromString("DEBUG"),
	})
	ctx := context.Background()
	scheduler0config := config.NewScheduler0Config()
	dispatcher := createTestDispatcher(ctx)
	mockClient := new(MockHTTPClient)

	executor := NewWebhookExecutorWithClient(logger, ctx, scheduler0config, dispatcher, mockClient).(*WebhookExecutionHandler)

	executorID := uint64(9)
	jobs := []models.Job{
		{ID: 4, ProjectID: 2, Spec: "@every 10s", Data: `{"job":"alpha"}`, ExecutorId: &executorID, RetryMax: 0, Status: models.JobStatusActive},
		{ID: 5, ProjectID: 2, Spec: "@every 10s", Data: `{"job":"beta"}`, ExecutorId: &executorID, RetryMax: 0, Status: models.JobStatusActive},
		{ID: 6, ProjectID: 2, Spec: "@every 10s", Data: `{"job":"gamma"}`, ExecutorId: &executorID, RetryMax: 0, Status: models.JobStatusActive},
	}

	batch := models.AggregatedJobInvocationPayload{Aggregated: true}
	for _, job := range jobs {
		batch.Jobs = append(batch.Jobs, payloadFor(job))
	}

	executorModel := models.JobExecutor{
		ID:                 executorID,
		Type:               string(models.ExecutorTypeWebhookUrl),
		WebhookUrl:         "https://example.com/hook",
		WebhookMethod:      "POST",
		WebhookSecret:      "agg-secret",
		PayloadAggregation: true,
	}

	var receivedPayload []byte
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

	mockClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		if req.Method != "POST" || req.URL.String() != "https://example.com/hook" {
			return false
		}
		if req.Header.Get("Content-Type") != "application/json" {
			return false
		}
		if req.Header.Get("X-Webhook-Secret") != "agg-secret" {
			return false
		}
		if req.Body != nil {
			receivedPayload, _ = io.ReadAll(req.Body)
			req.Body = io.NopCloser(bytes.NewBuffer(receivedPayload))
		}
		return true
	})).Return(&http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString("OK")),
	}, nil)

	executor.ExecuteWebhookJobBatch(executorModel, batch, successCallback, errorCallback)
	time.Sleep(100 * time.Millisecond)

	assert.False(t, errorCalled)
	assert.Equal(t, []uint64{4, 5, 6}, successIDs)

	var received models.AggregatedJobInvocationPayload
	err := json.Unmarshal(receivedPayload, &received)
	assert.NoError(t, err)
	assert.True(t, received.Aggregated)
	assert.Len(t, received.Jobs, 3)

	gotIDs := make([]uint64, 0, len(received.Jobs))
	gotData := make([]string, 0, len(received.Jobs))
	for _, entry := range received.Jobs {
		gotIDs = append(gotIDs, entry.Job.ID)
		gotData = append(gotData, entry.Job.Data)
		assert.NotNil(t, entry.Job.ExecutorId)
		assert.Equal(t, executorID, *entry.Job.ExecutorId)
	}
	assert.Equal(t, []uint64{4, 5, 6}, gotIDs)
	assert.Equal(t, []string{`{"job":"alpha"}`, `{"job":"beta"}`, `{"job":"gamma"}`}, gotData)
	mockClient.AssertExpectations(t)
}
