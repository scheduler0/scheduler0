package models

// ExecutionTotalsResponse represents the response for execution totals
type ExecutionTotalsResponse struct {
	AccountID uint64 `json:"accountId"`
	Scheduled uint64 `json:"scheduled"` // Total scheduled executions
	Success   uint64 `json:"success"`   // Total successful executions
	Failed    uint64 `json:"failed"`    // Total failed executions
}

