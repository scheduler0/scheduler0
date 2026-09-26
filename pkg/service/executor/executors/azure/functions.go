package azure

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"scheduler0/pkg/models"
	"time"

	"github.com/hashicorp/go-hclog"
)

// HTTPClientInterface defines the interface for HTTP client operations
// This allows for easier testing by injecting mock implementations
type HTTPClientInterface interface {
	Do(req *http.Request) (*http.Response, error)
}

type FunctionsExecutionHandler struct {
	logger     hclog.Logger
	ctx        context.Context
	httpClient HTTPClientInterface
}

type FunctionsExecutor interface {
	ExecuteFunctionJob(
		functionURL string,
		functionKey string,
		pendingJob models.JobInvocationPayload,
		successCallback func(job models.Job),
		errorCallback func(job models.Job),
	)
	ExecuteFunctionJobBatch(
		functionURL string,
		functionKey string,
		pendingJobs models.AggregatedJobInvocationPayload,
		successCallback func(jobs []models.Job),
		errorCallback func(jobs []models.Job),
	)
}

func NewFunctionsExecutor(logger hclog.Logger, ctx context.Context) FunctionsExecutor {
	return &FunctionsExecutionHandler{
		logger: logger,
		ctx:    ctx,
	}
}

// NewFunctionsExecutorWithClient creates an Azure Functions executor with a custom HTTP client (useful for testing)
func NewFunctionsExecutorWithClient(logger hclog.Logger, ctx context.Context, httpClient HTTPClientInterface) FunctionsExecutor {
	return &FunctionsExecutionHandler{
		logger:     logger,
		ctx:        ctx,
		httpClient: httpClient,
	}
}

func (functionExecutor *FunctionsExecutionHandler) ExecuteFunctionJob(
	functionURL string,
	functionKey string,
	pendingJob models.JobInvocationPayload,
	successCallback func(job models.Job),
	errorCallback func(job models.Job),
) {
	// Convert jobs to JSON
	payload, err := json.Marshal(pendingJob)
	if err != nil {
		functionExecutor.logger.Error("failed to marshal jobs payload for Azure function", "error", err, "functionURL", functionURL, "functionKey", functionKey, "pendingJob.ID", pendingJob.ID, "pendingJob.ProjectID", pendingJob.ProjectID, "pendingJob.Spec", pendingJob.Spec, "pendingJob.Timezone", pendingJob.Timezone, "pendingJob.Data", pendingJob.Data, "pendingJob.Status", pendingJob.Status, "pendingJob.DateCreated", pendingJob.DateCreated)
		errorCallback(pendingJob.Job)
		return
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(functionExecutor.ctx, http.MethodPost, functionURL, bytes.NewReader(payload))
	if err != nil {
		functionExecutor.logger.Error("failed to create request for Azure function", "error", err, "functionURL", functionURL, "functionKey", functionKey, "pendingJob.ID", pendingJob.ID, "pendingJob.ProjectID", pendingJob.ProjectID, "pendingJob.Spec", pendingJob.Spec, "pendingJob.Timezone", pendingJob.Timezone, "pendingJob.Data", pendingJob.Data, "pendingJob.Status", pendingJob.Status, "pendingJob.DateCreated", pendingJob.DateCreated)
		errorCallback(pendingJob.Job)
		return
	}

	// Add headers
	req.Header.Set("Content-Type", "application/json")
	if functionKey != "" {
		req.Header.Set("x-functions-key", functionKey)
	}

	// Use injected HTTP client or create a new one
	var client HTTPClientInterface
	if functionExecutor.httpClient != nil {
		client = functionExecutor.httpClient
	} else {
		client = &http.Client{
			Timeout: 30 * time.Second,
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		functionExecutor.logger.Error("failed to execute function for Azure function", "error", err, "functionURL", functionURL, "functionKey", functionKey, "pendingJob.ID", pendingJob.ID, "pendingJob.ProjectID", pendingJob.ProjectID, "pendingJob.Spec", pendingJob.Spec, "pendingJob.Timezone", pendingJob.Timezone, "pendingJob.Data", pendingJob.Data, "pendingJob.Status", pendingJob.Status, "pendingJob.DateCreated", pendingJob.DateCreated)
		errorCallback(pendingJob.Job)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		functionExecutor.logger.Info(fmt.Sprintf("Successfully executed Azure function, status: %d", resp.StatusCode))
		successCallback(pendingJob.Job)
	} else {
		functionExecutor.logger.Error("function execution failed for Azure function", "status", resp.StatusCode, "functionURL", functionURL, "functionKey", functionKey, "pendingJob.ID", pendingJob.ID, "pendingJob.ProjectID", pendingJob.ProjectID, "pendingJob.Spec", pendingJob.Spec, "pendingJob.Timezone", pendingJob.Timezone, "pendingJob.Data", pendingJob.Data, "pendingJob.Status", pendingJob.Status, "pendingJob.DateCreated", pendingJob.DateCreated)
		errorCallback(pendingJob.Job)
	}
}

// ExecuteFunctionJobBatch invokes the function once with an aggregated payload
// for a group of jobs sharing this executor and the same scheduled fire time.
// The whole batch succeeds or fails together, so the callbacks receive every job.
func (functionExecutor *FunctionsExecutionHandler) ExecuteFunctionJobBatch(
	functionURL string,
	functionKey string,
	pendingJobs models.AggregatedJobInvocationPayload,
	successCallback func(jobs []models.Job),
	errorCallback func(jobs []models.Job),
) {
	jobs := pendingJobs.JobList()

	payload, err := json.Marshal(pendingJobs)
	if err != nil {
		functionExecutor.logger.Error("failed to marshal aggregated jobs payload for Azure function", "error", err, "functionURL", functionURL, "jobCount", len(jobs))
		errorCallback(jobs)
		return
	}

	req, err := http.NewRequestWithContext(functionExecutor.ctx, http.MethodPost, functionURL, bytes.NewReader(payload))
	if err != nil {
		functionExecutor.logger.Error("failed to create request for aggregated Azure function", "error", err, "functionURL", functionURL, "jobCount", len(jobs))
		errorCallback(jobs)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	if functionKey != "" {
		req.Header.Set("x-functions-key", functionKey)
	}

	var client HTTPClientInterface
	if functionExecutor.httpClient != nil {
		client = functionExecutor.httpClient
	} else {
		client = &http.Client{
			Timeout: 30 * time.Second,
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		functionExecutor.logger.Error("failed to execute function for aggregated Azure function", "error", err, "functionURL", functionURL, "jobCount", len(jobs))
		errorCallback(jobs)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		functionExecutor.logger.Info(fmt.Sprintf("Successfully executed aggregated Azure function, status: %d", resp.StatusCode))
		successCallback(jobs)
	} else {
		functionExecutor.logger.Error("function execution failed for aggregated Azure function", "status", resp.StatusCode, "functionURL", functionURL, "jobCount", len(jobs))
		errorCallback(jobs)
	}
}
