package models

import "time"

// Classify-request status values recorded in the account_classify_requests log.
const (
	ClassifyRequestStatusSuccess = "success"
	ClassifyRequestStatusFailed  = "failed"
)

// Classify-request kinds distinguish the intent classifier from the suggestions analyzer,
// which share the same monthly classify-request quota.
const (
	ClassifyRequestKindClassify = "classify"
	ClassifyRequestKindAnalyze  = "analyze"
)

// AccountClassifyRequest is a persisted record of a single AI classify/analyze operation for
// an account. Usage is derived by counting the successful rows within the account's current
// period, mirroring the prompt-request log.
type AccountClassifyRequest struct {
	ID          uint64    `json:"id"`
	AccountID   uint64    `json:"account_id"`
	Kind        string    `json:"kind"`
	Prompt      string    `json:"prompt"`
	Decision    string    `json:"decision"`
	Status      string    `json:"status"`
	Error       string    `json:"error,omitempty"`
	DateCreated time.Time `json:"date_created"`
}
