package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"scheduler0/pkg/models"
	"strconv"
	"time"
)

const JobFailureJobType = "job_execution_failed"

const AICreditsLowJobType = "ai_credits_low"

type AICreditsLowData struct {
	JobType           string `json:"jobType"`
	AccountID         string `json:"accountId"`
	BalanceMicros     int64  `json:"balanceMicros"`
	TopupAmountMicros int64  `json:"topupAmountMicros"`
	IdempotencyKey    string `json:"idempotencyKey"`
}

type JobFailureData struct {
	JobType          string `json:"jobType"`
	AccountID        string `json:"accountId"`
	JobID            string `json:"jobId"`
	ProjectID        string `json:"projectId"`
	FailCount        uint64 `json:"failCount"`
	ExecutionVersion uint64 `json:"executionVersion"`
}

type Notifier struct {
	URL        string
	Secret     string
	HTTPClient *http.Client
}

func NewNotifier(url, secret string) *Notifier {
	return &Notifier{
		URL:    url,
		Secret: secret,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (n *Notifier) NotifyJobFailure(job models.Job, failCount uint64, executionVersion uint64) error {
	if n == nil || n.URL == "" {
		return nil
	}

	data, err := json.Marshal(JobFailureData{
		JobType:          JobFailureJobType,
		AccountID:        strconv.FormatUint(job.AccountId, 10),
		JobID:            strconv.FormatUint(job.ID, 10),
		ProjectID:        strconv.FormatUint(job.ProjectID, 10),
		FailCount:        failCount,
		ExecutionVersion: executionVersion,
	})
	if err != nil {
		return fmt.Errorf("marshal job failure data: %w", err)
	}

	payload := models.JobInvocationPayload{
		Job: models.Job{
			ID:        job.ID,
			AccountId: job.AccountId,
			ProjectID: job.ProjectID,
			Data:      string(data),
			Status:    job.Status,
		},
		LastExecutionStatus: "failed",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal webhook payload: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if n.Secret != "" {
		req.Header.Set("X-Webhook-Secret", n.Secret)
	}

	client := n.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("post platform webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("platform webhook returned status %d", resp.StatusCode)
	}
	return nil
}

func (n *Notifier) NotifyAICreditsLow(accountID uint64, balanceMicros int64, topupAmountMicros int64) error {
	if n == nil || n.URL == "" {
		return nil
	}

	data, err := json.Marshal(AICreditsLowData{
		JobType:           AICreditsLowJobType,
		AccountID:         strconv.FormatUint(accountID, 10),
		BalanceMicros:     balanceMicros,
		TopupAmountMicros: topupAmountMicros,
		IdempotencyKey:    fmt.Sprintf("auto_topup:%d:%d", accountID, balanceMicros),
	})
	if err != nil {
		return fmt.Errorf("marshal ai credits low data: %w", err)
	}

	payload := models.JobInvocationPayload{
		Job: models.Job{
			AccountId: accountID,
			Data:      string(data),
		},
		LastExecutionStatus: "failed",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal webhook payload: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if n.Secret != "" {
		req.Header.Set("X-Webhook-Secret", n.Secret)
	}

	client := n.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("post platform webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("platform webhook returned status %d", resp.StatusCode)
	}
	return nil
}
