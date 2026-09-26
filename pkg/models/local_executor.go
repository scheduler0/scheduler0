package models

// LocalExecutionReport is a single execution event reported by the scheduler0-cli
// local executor. It mirrors the client-side LocalExecutionReport in scheduler0-go-client.
type LocalExecutionReport struct {
	JobID             uint64 `json:"jobId"`
	UniqueID          string `json:"uniqueId"`
	State             uint64 `json:"state"` // 0=scheduled, 1=success, 2=failed
	LastExecutionTime string `json:"lastExecutionTime"`
	NextExecutionTime string `json:"nextExecutionTime"`
	ExecutionVersion  uint64 `json:"executionVersion"`
	JobQueueVersion   uint64 `json:"jobQueueVersion"`
}
