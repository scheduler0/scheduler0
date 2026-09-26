package models

import "encoding/json"

// SuggestionParticipant is a member of a conversation. Timezone is optional and
// falls back to the request-level default_timezone when absent.
type SuggestionParticipant struct {
	ID          string `json:"id,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	Timezone    string `json:"timezone,omitempty"`
}

// SuggestionMessage is a single conversation message. Speaker is kept as raw JSON
// so both the full object form ({"id","display_name","timezone"}) and the minimal
// bare-string form (just a display name) are forwarded unchanged to the analyzer.
type SuggestionMessage struct {
	ID        string          `json:"id,omitempty"`
	Speaker   json.RawMessage `json:"speaker,omitempty"`
	Timestamp string          `json:"timestamp"`
	Message   string          `json:"message"`
}

// SuggestionOptions carries per-request analysis options. Locale is English-only
// for the first release; any non-en* value is rejected with UNSUPPORTED_LOCALE.
type SuggestionOptions struct {
	ReferenceTime              string   `json:"reference_time,omitempty"`
	Locale                     string   `json:"locale,omitempty"`
	DefaultTimezone            string   `json:"default_timezone,omitempty"`
	MinimumConfidence          *float64 `json:"minimum_confidence,omitempty"`
	IncludeLowConfidence       *bool    `json:"include_low_confidence,omitempty"`
	IncludeResolvedObligations *bool    `json:"include_resolved_obligations,omitempty"`
	DefaultDueTime             string   `json:"default_due_time,omitempty"`
	DefaultDeadlineTime        string   `json:"default_deadline_time,omitempty"`
}

// SuggestionAnalyzeRequest is the request body for POST /api/v1/suggestions/analyze.
type SuggestionAnalyzeRequest struct {
	ConversationID string                  `json:"conversation_id,omitempty"`
	Messages       []SuggestionMessage     `json:"messages"`
	Participants   []SuggestionParticipant `json:"participants,omitempty"`
	Options        *SuggestionOptions      `json:"options,omitempty"`
}

// SuggestionAnalyzeResult mirrors the analyzer's JSON response. Individual
// suggestions/obligations are kept as generic maps because the edge analyzer owns
// their rich, evolving shape; extra fields are preserved on passthrough.
type SuggestionAnalyzeResult struct {
	RequestID      string           `json:"request_id,omitempty"`
	ConversationID string           `json:"conversation_id,omitempty"`
	AnalyzedAt     string           `json:"analyzed_at,omitempty"`
	Suggestions    []map[string]any `json:"suggestions"`
	Obligations    []map[string]any `json:"obligations"`
	Warnings       []map[string]any `json:"warnings"`
	Engine         map[string]any   `json:"engine,omitempty"`
}
