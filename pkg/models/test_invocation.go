package models

import "time"

// TestInvocationRequest is the body of POST /api/v1/executors/{id}/test-invoke.
//
// It lets a developer fire a synthetic ("test") job through an existing executor
// without creating, persisting, or scheduling a real job. The executor is invoked
// immediately and synchronously: no execution log is written and nothing is
// rescheduled. This exists so cron/scheduled jobs can be exercised on demand
// instead of waiting for their spec/start date to elapse.
type TestInvocationRequest struct {
	// Job carries the standard job attributes to include in the invocation
	// payload (spec, data, timezone, timezoneOffset, retryMax, projectId,
	// startDate, endDate, status, ...). Server-managed fields (id, accountId,
	// executorId, dateCreated, lastExecutionDate) are ignored and overridden by
	// the server.
	Job Job `json:"job"`

	// Age, when set, marks how old the synthetic job entry should appear. It is a
	// Go duration string (e.g. "24h", "1h30m", "15m"). The job's DateCreated and
	// LastExecutionDate are set to now-Age so downstream logic and the receiving
	// executor observe a job that was created / last ran that long ago. Must be a
	// positive duration.
	Age string `json:"age,omitempty"`

	// ExecutionTime, when set, is the moment the developer wants the job treated
	// as executing at (RFC3339). It becomes the payload's LastExecutionDateTime.
	// Defaults to the current time when omitted.
	ExecutionTime *time.Time `json:"executionTime,omitempty"`
}

// TestInvocationResult is returned by a test invocation. It reports whether the
// executor accepted the synthetic job, how long the call took, and echoes the
// exact payload that was delivered so developers can verify what their endpoint
// received.
type TestInvocationResult struct {
	// Test is always true; it lets receivers/clients distinguish a test result.
	Test bool `json:"test"`
	// ExecutorId / ExecutorType identify the executor that was invoked.
	ExecutorId   uint64 `json:"executorId"`
	ExecutorType string `json:"executorType"`
	// Success reports whether the executor accepted the invocation (e.g. the
	// webhook returned < 400, or the cloud function invocation succeeded).
	Success bool `json:"success"`
	// Error carries a human-readable failure reason when Success is false.
	Error string `json:"error,omitempty"`
	// StartedAt / FinishedAt / DurationMs describe the timing of the invocation.
	StartedAt  time.Time `json:"startedAt"`
	FinishedAt time.Time `json:"finishedAt"`
	DurationMs int64     `json:"durationMs"`
	// Payload is the exact JobInvocationPayload delivered to the executor.
	Payload JobInvocationPayload `json:"payload"`
}
