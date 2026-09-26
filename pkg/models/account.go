package models

import "time"

type FeatureRequest struct {
	FeatureId uint64 `json:"featureId"`
}

type Feature struct {
	ID        uint64     `json:"id"`
	Name      string     `json:"name"`
	CreatedAt time.Time  `json:"dateCreated"`
	UpdatedAt *time.Time `json:"dateModified"`
}

type Account struct {
	ID        uint64           `json:"id"`
	Name      string           `json:"name"`
	Features  []AccountFeature `json:"features"`
	CreatedAt time.Time        `json:"dateCreated"`
	UpdatedAt *time.Time       `json:"dateModified"`
}

type AccountFeature struct {
	AccountId uint64 `json:"accountId"`
	FeatureId uint64 `json:"featureId"`
	Feature   string `json:"feature"`
}

type AccountJobExecutionsCount struct {
	ID             uint64    `json:"id"`
	AccountId      uint64    `json:"accountId"`
	ExecutionCount uint64    `json:"executionCount"`
	Tokens         uint64    `json:"tokens"`
	DateCreated    time.Time `json:"dateCreated"`
	DateModified   time.Time `json:"dateModified"`
	NextResetDate  time.Time `json:"nextResetDate"`
}

// AIQuotaPeriod is an account's current monthly AI-quota window. AI usage is log-derived, so
// this boundary is the only quota state persisted per account: usage is counted from the
// request logs since PeriodStart, and the window advances lazily once NextResetDate passes.
type AIQuotaPeriod struct {
	AccountId     uint64    `json:"accountId"`
	PeriodStart   time.Time `json:"periodStart"`
	NextResetDate time.Time `json:"nextResetDate"`
	DateCreated   time.Time `json:"dateCreated"`
	DateModified  time.Time `json:"dateModified"`
}

// AIUsageDimension reports one quota dimension (prompt or classify): the feature-derived
// monthly Limit, the number of successful requests Used in the current period, and the
// Remaining allowance (Limit-Used, floored at zero).
type AIUsageDimension struct {
	Limit     uint64 `json:"limit"`
	Used      uint64 `json:"used"`
	Remaining uint64 `json:"remaining"`
}

// AIUsage is the authoritative, log-derived view of an account's AI request usage for the
// current period. It is what the dashboard renders and what request handlers enforce against.
type AIUsage struct {
	AccountId     uint64           `json:"accountId"`
	PeriodStart   time.Time        `json:"periodStart"`
	NextResetDate time.Time        `json:"nextResetDate"`
	Prompt        AIUsageDimension `json:"prompt"`
	Classify      AIUsageDimension `json:"classify"`
	// EstimatedCostUSD is the sum of estimated_cost_usd for all prompt-request rows in the
	// current period (all statuses). Failed requests that consumed tokens are included;
	// skipped-intent and unknown-model rows contribute $0.
	EstimatedCostUSD float64 `json:"estimatedCostUsd"`
}
