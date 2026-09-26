package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"scheduler0/pkg/config"
	"scheduler0/pkg/models"
	"scheduler0/pkg/utils"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
)

// HTTPClientInterface defines the interface for HTTP client operations
// This allows for easier testing by injecting mock implementations
type HTTPClientInterface interface {
	Do(req *http.Request) (*http.Response, error)
}

type WebhookExecutionHandler struct {
	logger     hclog.Logger
	ctx        context.Context
	config     config.Scheduler0Config
	dispatcher *utils.Dispatcher
	httpClient HTTPClientInterface
}

type WebhookExecutor interface {
	ExecuteWebhookJob(
		executor models.JobExecutor,
		pendingJob models.JobInvocationPayload,
		successCallback func(job models.Job),
		errorCallback func(job models.Job),
	)
	ExecuteWebhookJobBatch(
		executor models.JobExecutor,
		pendingJobs models.AggregatedJobInvocationPayload,
		successCallback func(jobs []models.Job),
		errorCallback func(jobs []models.Job),
	)
}

func NewWebhookExecutor(logger hclog.Logger, ctx context.Context, config config.Scheduler0Config, dispatcher *utils.Dispatcher) WebhookExecutor {
	return &WebhookExecutionHandler{
		logger:     logger,
		ctx:        ctx,
		config:     config,
		dispatcher: dispatcher,
	}
}

// NewWebhookExecutorWithClient creates a webhook executor with a custom HTTP client (useful for testing)
func NewWebhookExecutorWithClient(logger hclog.Logger, ctx context.Context, config config.Scheduler0Config, dispatcher *utils.Dispatcher, httpClient HTTPClientInterface) WebhookExecutor {
	return &WebhookExecutionHandler{
		logger:     logger,
		ctx:        ctx,
		config:     config,
		dispatcher: dispatcher,
		httpClient: httpClient,
	}
}

func (webhookExecutor *WebhookExecutionHandler) ExecuteWebhookJob(
	executor models.JobExecutor,
	pendingJob models.JobInvocationPayload,
	successCallback func(job models.Job),
	errorCallback func(job models.Job),
) {
	webhookExecutor.dispatcher.NoBlockQueue(func(successChannel chan any, errorChannel chan any) {
		defer func() {
			close(errorChannel)
			close(successChannel)
		}()

		// Convert jobs to JSON payload
		payload, err := json.Marshal(pendingJob)
		if err != nil {
			webhookExecutor.logger.Error("failed to marshal jobs payload for Webhook job", "error", err, "executor.ID", executor.ID, "executor.WebhookUrl", executor.WebhookUrl, "executor.WebhookMethod", executor.WebhookMethod, "pendingJob.ID", pendingJob.ID, "pendingJob.ProjectID", pendingJob.ProjectID, "pendingJob.Spec", pendingJob.Spec, "pendingJob.Timezone", pendingJob.Timezone, "pendingJob.Data", pendingJob.Data, "pendingJob.Status", pendingJob.Status, "pendingJob.DateCreated", pendingJob.DateCreated)
			errorCallback(pendingJob.Job)
			return
		}

		err = webhookExecutor.sendWithRetries(executor, payload, uint64(pendingJob.RetryMax))
		if err != nil {
			if errors.Is(err, errBuildRequest) {
				webhookExecutor.logger.Error("failed to create request for Webhook job", "error", err, "executor.ID", executor.ID, "executor.WebhookUrl", executor.WebhookUrl, "executor.WebhookMethod", executor.WebhookMethod, "pendingJob.ID", pendingJob.ID, "pendingJob.ProjectID", pendingJob.ProjectID, "pendingJob.Spec", pendingJob.Spec, "pendingJob.Timezone", pendingJob.Timezone, "pendingJob.Data", pendingJob.Data, "pendingJob.Status", pendingJob.Status, "pendingJob.DateCreated", pendingJob.DateCreated)
			} else {
				webhookExecutor.logger.Error("webhook execution failed for Webhook job", "error", err, "executor.ID", executor.ID, "executor.WebhookUrl", executor.WebhookUrl, "executor.WebhookMethod", executor.WebhookMethod, "pendingJob.ID", pendingJob.ID, "pendingJob.ProjectID", pendingJob.ProjectID, "pendingJob.Spec", pendingJob.Spec, "pendingJob.Timezone", pendingJob.Timezone, "pendingJob.Data", pendingJob.Data, "pendingJob.Status", pendingJob.Status, "pendingJob.DateCreated", pendingJob.DateCreated)
			}
			errorCallback(pendingJob.Job)
			return
		}

		successCallback(pendingJob.Job)
	})
}

// errBuildRequest marks failures in constructing the *http.Request itself (bad
// URL/method), as opposed to failures sending it.
var errBuildRequest = errors.New("failed to build webhook request")

// maxErrorBodyBytes bounds how much of a failing response body is captured
// into the returned error so operators can see *why* an endpoint rejected the
// webhook (e.g. a WAF or application validation message) without unbounded
// logging.
const maxErrorBodyBytes = 512

// httpClient returns the injected client (tests) or a fresh one honoring
// JobExecutionTimeout (seconds).
func (webhookExecutor *WebhookExecutionHandler) client() HTTPClientInterface {
	if webhookExecutor.httpClient != nil {
		return webhookExecutor.httpClient
	}
	scheduler0Config := webhookExecutor.config.GetConfigurations()
	timeout := time.Duration(scheduler0Config.JobExecutionTimeout) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &http.Client{Timeout: timeout}
}

// newRequest builds a fresh *http.Request carrying a complete copy of payload.
// It must be called once per attempt: an *http.Request body is an io.Reader
// that is drained by the first client.Do, so reusing the same request across
// retries sends an empty body with the original Content-Length (the remote end
// sees a truncated request and typically answers 400).
func (webhookExecutor *WebhookExecutionHandler) newRequest(executor models.JobExecutor, payload []byte) (*http.Request, error) {
	method := strings.ToUpper(executor.WebhookMethod)
	if method == "" {
		method = http.MethodPost
	}

	req, err := http.NewRequestWithContext(webhookExecutor.ctx, method, executor.WebhookUrl, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errBuildRequest, err)
	}

	req.Header.Set("Content-Type", "application/json")
	if executor.WebhookSecret != "" {
		req.Header.Set("X-Webhook-Secret", executor.WebhookSecret)
	}
	return req, nil
}

// sendWithRetries delivers payload to the executor's webhook, retrying up to
// retryMax more times with JobExecutionRetryDelay seconds between attempts.
// Each attempt uses a newly built request so the body is intact on every try.
// Non-2xx/3xx responses are treated as failures and include a bounded snippet
// of the response body in the error for diagnosability.
func (webhookExecutor *WebhookExecutionHandler) sendWithRetries(executor models.JobExecutor, payload []byte, retryMax uint64) error {
	// Fail fast on a malformed URL/method rather than retrying something that
	// can never succeed.
	if _, err := webhookExecutor.newRequest(executor, payload); err != nil {
		return err
	}

	client := webhookExecutor.client()
	scheduler0Config := webhookExecutor.config.GetConfigurations()

	return utils.RetryOnError(func() error {
		req, err := webhookExecutor.newRequest(executor, payload)
		if err != nil {
			return err
		}

		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 400 {
			snippet, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
			body := strings.TrimSpace(string(snippet))
			if body == "" {
				return fmt.Errorf("webhook returned status code %d", resp.StatusCode)
			}
			return fmt.Errorf("webhook returned status code %d: %s", resp.StatusCode, body)
		}
		// Drain so the underlying connection can be reused.
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}, retryMax, scheduler0Config.JobExecutionRetryDelay)
}

// ExecuteWebhookJobBatch delivers several jobs that share this executor and the
// same scheduled fire time in a single request, using the aggregated payload
// shape ({aggregated:true, jobs:[...]}). The whole batch succeeds or fails as a
// unit based on the single HTTP response, so the callbacks receive every job.
func (webhookExecutor *WebhookExecutionHandler) ExecuteWebhookJobBatch(
	executor models.JobExecutor,
	pendingJobs models.AggregatedJobInvocationPayload,
	successCallback func(jobs []models.Job),
	errorCallback func(jobs []models.Job),
) {
	webhookExecutor.dispatcher.NoBlockQueue(func(successChannel chan any, errorChannel chan any) {
		defer func() {
			close(errorChannel)
			close(successChannel)
		}()

		jobs := pendingJobs.JobList()

		payload, err := json.Marshal(pendingJobs)
		if err != nil {
			webhookExecutor.logger.Error("failed to marshal aggregated jobs payload for Webhook job", "error", err, "executor.ID", executor.ID, "executor.WebhookUrl", executor.WebhookUrl, "executor.WebhookMethod", executor.WebhookMethod, "jobCount", len(jobs))
			errorCallback(jobs)
			return
		}

		err = webhookExecutor.sendWithRetries(executor, payload, uint64(pendingJobs.MaxRetryMax()))
		if err != nil {
			if errors.Is(err, errBuildRequest) {
				webhookExecutor.logger.Error("failed to create request for aggregated Webhook job", "error", err, "executor.ID", executor.ID, "executor.WebhookUrl", executor.WebhookUrl, "executor.WebhookMethod", executor.WebhookMethod, "jobCount", len(jobs))
			} else {
				webhookExecutor.logger.Error("aggregated webhook execution failed for Webhook job", "error", err, "executor.ID", executor.ID, "executor.WebhookUrl", executor.WebhookUrl, "executor.WebhookMethod", executor.WebhookMethod, "jobCount", len(jobs))
			}
			errorCallback(jobs)
			return
		}

		successCallback(jobs)
	})
}
