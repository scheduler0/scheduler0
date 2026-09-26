package models

// Send-time suggestion request/response DTOs for POST /api/v1/ai/suggestions/time.
// All JSON keys are snake_case to match the public API contract and the client
// libraries. The engine is deterministic: the same request + reference time +
// policy version produces the same suggestions.

// SendTimeWorkingHours is a single continuous working interval that applies to
// the listed weekday codes (MON, TUE, WED, THU, FRI, SAT, SUN). Times are local
// "HH:MM" 24-hour values in the participant's timezone.
type SendTimeWorkingHours struct {
	Days  []string `json:"days,omitempty"`
	Start string   `json:"start,omitempty"`
	End   string   `json:"end,omitempty"`
}

// SendTimeQuietHours is a local "HH:MM" interval during which delivery is
// discouraged. It may cross midnight (e.g. 21:00 -> 08:00).
type SendTimeQuietHours struct {
	Start string `json:"start,omitempty"`
	End   string `json:"end,omitempty"`
}

// SendTimeParticipant is a sender or recipient. Timezone is a required IANA
// identifier (e.g. America/Toronto). Role only applies to recipients.
type SendTimeParticipant struct {
	ID           string                `json:"id,omitempty"`
	DisplayName  string                `json:"display_name,omitempty"`
	Timezone     string                `json:"timezone"`
	Role         string                `json:"role,omitempty"`
	WorkingHours *SendTimeWorkingHours `json:"working_hours,omitempty"`
	QuietHours   *SendTimeQuietHours   `json:"quiet_hours,omitempty"`
}

// SendTimeMessage carries the timing-relevant metadata. Text is optional; the
// engine never requires message content.
type SendTimeMessage struct {
	Channel            string `json:"channel,omitempty"`
	Priority           string `json:"priority,omitempty"`
	Intent             string `json:"intent,omitempty"`
	Text               string `json:"text,omitempty"`
	EstimatedAttention string `json:"estimated_attention,omitempty"`
}

// SendTimeConstraints are hard rules. A candidate that violates any active
// constraint is rejected and never scored.
type SendTimeConstraints struct {
	EarliestSendAt      string `json:"earliest_send_at,omitempty"`
	LatestSendAt        string `json:"latest_send_at,omitempty"`
	MinimumDelaySeconds *int64 `json:"minimum_delay_seconds,omitempty"`
	WorkingHoursOnly    *bool  `json:"working_hours_only,omitempty"`
	AvoidWeekends       *bool  `json:"avoid_weekends,omitempty"`
	AvoidHolidays       *bool  `json:"avoid_holidays,omitempty"`
	RespectQuietHours   *bool  `json:"respect_quiet_hours,omitempty"`
	RequireCalendarFree *bool  `json:"require_calendar_free,omitempty"`
}

// SendTimeWindow is a local "HH:MM" preference window.
type SendTimeWindow struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// SendTimePreferences influence scoring but do not invalidate a candidate.
type SendTimePreferences struct {
	PreferredRecipientWindows    []SendTimeWindow `json:"preferred_recipient_windows,omitempty"`
	AvoidRecipientWindows        []SendTimeWindow `json:"avoid_recipient_windows,omitempty"`
	PreferSenderRecipientOverlap *bool            `json:"prefer_sender_recipient_overlap,omitempty"`
}

// SendTimeGroupPolicy controls how multi-recipient coverage is evaluated.
type SendTimeGroupPolicy struct {
	Strategy                 string   `json:"strategy,omitempty"`
	MinimumRecipientCoverage *float64 `json:"minimum_recipient_coverage,omitempty"`
}

// SendTimeOptions tune candidate generation and the response shape.
type SendTimeOptions struct {
	ReferenceTime                   string `json:"reference_time,omitempty"`
	SuggestionCount                 *int   `json:"suggestion_count,omitempty"`
	CandidateIntervalMinutes        *int   `json:"candidate_interval_minutes,omitempty"`
	Locale                          string `json:"locale,omitempty"`
	IncludeScoreBreakdown           *bool  `json:"include_score_breakdown,omitempty"`
	IncludeRejectedSummary          *bool  `json:"include_rejected_summary,omitempty"`
	SearchHorizonDays               *int   `json:"search_horizon_days,omitempty"`
	EvaluateSendNow                 *bool  `json:"evaluate_send_now,omitempty"`
	DiversifySuggestions            *bool  `json:"diversify_suggestions,omitempty"`
	MinimumSuggestionSpacingMinutes *int   `json:"minimum_suggestion_spacing_minutes,omitempty"`
}

// SendTimeHolidayPolicy supplies holiday information. Only caller-supplied
// Dates are honored in the first release; country/region/calendar_id lookups
// are reserved for a future version and produce a warning if avoid_holidays is
// requested without dates.
type SendTimeHolidayPolicy struct {
	Country    string   `json:"country,omitempty"`
	Region     string   `json:"region,omitempty"`
	CalendarID string   `json:"calendar_id,omitempty"`
	Dates      []string `json:"dates,omitempty"`
}

// SendTimeBusyInterval is an absolute (RFC3339) busy period.
type SendTimeBusyInterval struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// SendTimeAvailability is a participant's calendar busy intervals.
type SendTimeAvailability struct {
	ParticipantID string                 `json:"participant_id"`
	BusyIntervals []SendTimeBusyInterval `json:"busy_intervals,omitempty"`
}

// SendTimeRequest is the request body for POST /api/v1/ai/suggestions/time.
type SendTimeRequest struct {
	Sender        *SendTimeParticipant   `json:"sender,omitempty"`
	Recipients    []SendTimeParticipant  `json:"recipients"`
	Message       *SendTimeMessage       `json:"message,omitempty"`
	Constraints   *SendTimeConstraints   `json:"constraints,omitempty"`
	Preferences   *SendTimePreferences   `json:"preferences,omitempty"`
	GroupPolicy   *SendTimeGroupPolicy   `json:"group_policy,omitempty"`
	Options       *SendTimeOptions       `json:"options,omitempty"`
	HolidayPolicy *SendTimeHolidayPolicy `json:"holiday_policy,omitempty"`
	Availability  []SendTimeAvailability `json:"availability,omitempty"`
	Metadata      map[string]any         `json:"metadata,omitempty"`
}

// SendTimePolicyRef identifies the versioned scoring policy used.
type SendTimePolicyRef struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}

// SendTimeEngineRef identifies the engine + tz database for reproducibility.
type SendTimeEngineRef struct {
	Version                 string `json:"version"`
	TimezoneDatabaseVersion string `json:"timezone_database_version,omitempty"`
}

// SendTimeLocal is a participant-local representation of an absolute instant.
// The boolean flags and recipient_id are omitted for the sender_local block.
type SendTimeLocal struct {
	RecipientID        string `json:"recipient_id,omitempty"`
	Datetime           string `json:"datetime"`
	Timezone           string `json:"timezone"`
	Day                string `json:"day"`
	Time               string `json:"time"`
	WithinWorkingHours *bool  `json:"within_working_hours,omitempty"`
	WithinQuietHours   *bool  `json:"within_quiet_hours,omitempty"`
	IsWeekend          *bool  `json:"is_weekend,omitempty"`
	IsHoliday          *bool  `json:"is_holiday,omitempty"`
}

// SendTimeCoverage summarizes recipient suitability for a candidate.
type SendTimeCoverage struct {
	ValidRecipients int     `json:"valid_recipients"`
	TotalRecipients int     `json:"total_recipients"`
	Ratio           float64 `json:"ratio"`
}

// SendTimeSuggestion is a single ranked recommendation.
type SendTimeSuggestion struct {
	ID                  string             `json:"id"`
	SendAt              string             `json:"send_at"`
	Label               string             `json:"label"`
	Score               float64            `json:"score"`
	Rank                int                `json:"rank"`
	Reason              string             `json:"reason"`
	SenderLocal         *SendTimeLocal     `json:"sender_local,omitempty"`
	RecipientLocalTimes []SendTimeLocal    `json:"recipient_local_times"`
	Coverage            SendTimeCoverage   `json:"coverage"`
	ScoreBreakdown      map[string]float64 `json:"score_breakdown,omitempty"`
	Flags               []string           `json:"flags,omitempty"`
}

// SendTimeSearchInfo describes the candidate search that produced the result.
type SendTimeSearchInfo struct {
	WindowStart              string `json:"window_start"`
	WindowEnd                string `json:"window_end"`
	CandidateIntervalMinutes int    `json:"candidate_interval_minutes"`
	CandidatesGenerated      int    `json:"candidates_generated"`
	CandidatesRejected       int    `json:"candidates_rejected"`
	CandidatesScored         int    `json:"candidates_scored"`
}

// SendTimeNoSuggestion explains why zero suggestions were produced.
type SendTimeNoSuggestion struct {
	Code            string   `json:"code"`
	Message         string   `json:"message"`
	Recommendations []string `json:"recommendations,omitempty"`
}

// SendTimeSendNow is the optional immediate-send evaluation.
type SendTimeSendNow struct {
	Recommended         bool            `json:"recommended"`
	Score               float64         `json:"score"`
	Reason              string          `json:"reason"`
	RecipientLocalTimes []SendTimeLocal `json:"recipient_local_times,omitempty"`
}

// SendTimeWarning is a non-fatal issue (e.g. defaulted working hours).
type SendTimeWarning struct {
	Code          string `json:"code"`
	ParticipantID string `json:"participant_id,omitempty"`
	Message       string `json:"message"`
}

// SendTimeResponse is the body returned inside the standard {success,data} envelope.
type SendTimeResponse struct {
	RequestID       string                `json:"request_id"`
	ReferenceTime   string                `json:"reference_time"`
	Policy          SendTimePolicyRef     `json:"policy"`
	Engine          SendTimeEngineRef     `json:"engine"`
	Suggestions     []SendTimeSuggestion  `json:"suggestions"`
	Search          SendTimeSearchInfo    `json:"search"`
	RejectedSummary map[string]int        `json:"rejected_summary,omitempty"`
	NoSuggestion    *SendTimeNoSuggestion `json:"no_suggestion,omitempty"`
	SendNow         *SendTimeSendNow      `json:"send_now,omitempty"`
	Warnings        []SendTimeWarning     `json:"warnings"`
	Metadata        map[string]any        `json:"metadata,omitempty"`
}
