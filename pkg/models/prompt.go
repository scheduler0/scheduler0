package models

import (
	"time"
)

// IntentClassification is the response from the edge classifier.
type IntentClassification struct {
	Text     string `json:"text"`
	Decision string `json:"decision"`
	Reason   string `json:"reason"`
}

// PromptResult is the top-level response for POST /prompt.
// Classification is nil when the intent guardrail is disabled or errored (fail-open).
type PromptResult struct {
	Providers      []PromptProviderResult `json:"providers"`
	Classification *IntentClassification  `json:"classification,omitempty"`
}

type PromptJobResponseKind string

const (
	PromptJobResponseKindFollowUp PromptJobResponseKind = "FOLLOW_UP"
	PromptJobResponseKindReminder PromptJobResponseKind = "REMINDER"
	PromptJobResponseKindDigest   PromptJobResponseKind = "DIGEST"
)

type PromptJobResponseRecurrence string

const (
	PromptJobResponseRecurrenceMinute  PromptJobResponseRecurrence = "every minute"
	PromptJobResponseRecurrenceHourly  PromptJobResponseRecurrence = "every hour"
	PromptJobResponseRecurrenceDaily   PromptJobResponseRecurrence = "every day"
	PromptJobResponseRecurrenceWeekly  PromptJobResponseRecurrence = "every week"
	PromptJobResponseRecurrenceMonthly PromptJobResponseRecurrence = "every month"
	PromptJobResponseRecurrenceYearly  PromptJobResponseRecurrence = "every year"
	PromptJobResponseRecurrenceNone    PromptJobResponseRecurrence = "none"
)

type PromptProviderResult struct {
	Provider     string              `json:"provider"`
	Model        string              `json:"model"`
	Jobs         []PromptJobResponse `json:"jobs"`
	InputTokens  uint64              `json:"inputTokens"`
	OutputTokens uint64              `json:"outputTokens"`
	TotalTokens  uint64              `json:"totalTokens"`
	DurationMs   uint64              `json:"durationMs"`
}

// ClassifyPromptRequest is the request body for POST /api/v1/ai/prompt/classify.
// Locale only supports English (en*); other locales are rejected.
type ClassifyPromptRequest struct {
	Prompt string `json:"prompt,omitempty"`
	Locale string `json:"locale,omitempty"`
}

type PropmptJobRequest struct {
	Prompt     string   `json:"prompt,omitempty"`
	Purposes   []string `json:"purposes,omitempty"`
	Events     []string `json:"events,omitempty"`
	Recipients []string `json:"recipients,omitempty"`
	Channels   []string `json:"channels,omitempty"`
	Timezone   string   `json:"timezone,omitempty"`
	Locale     string   `json:"locale,omitempty"`
}

type PromptJobResponse struct {
	Kind           PromptJobResponseKind       `json:"kind,omitempty"`
	Purpose        string                      `json:"purpose,omitempty"`
	Subject        string                      `json:"subject,omitempty"`
	NextRunAt      *time.Time                  `json:"nextRunAt,omitempty"`
	Recurrence     PromptJobResponseRecurrence `json:"recurrence,omitempty"`
	Event          string                      `json:"event,omitempty"`
	Delivery       string                      `json:"delivery,omitempty"`
	CronExpression string                      `json:"cronExpression,omitempty"`
	Channel        string                      `json:"channel,omitempty"`
	Recipients     []string                    `json:"recipients,omitempty"`
	StartDate      *time.Time                  `json:"startDate,omitempty"`
	EndDate        *time.Time                  `json:"endDate,omitempty"`
	Timezone       string                      `json:"timezone,omitempty"`
	Metadata       *map[string]any             `json:"metadata,omitempty"`
}
