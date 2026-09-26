// Package sendtime implements a deterministic send-time recommendation engine.
//
// Given a sender, recipients, a message, and scheduling context, it generates
// candidate send instants, converts each to every participant's local time,
// filters candidates that violate hard constraints, scores the survivors with a
// versioned policy, and returns the top ranked suggestions. The same request,
// reference time, and policy version always produce the same output.
//
// No large language model, no external service, and no network calls are used.
package sendtime

import (
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	// Embed the IANA timezone database so time.LoadLocation works in minimal
	// container images that do not ship /usr/share/zoneinfo.
	_ "time/tzdata"

	"scheduler0-private/pkg/config"
	"scheduler0-private/pkg/constants"
	"scheduler0-private/pkg/models"

	"github.com/hashicorp/go-hclog"
	"github.com/segmentio/ksuid"
)

// Rejection reason codes (spec section 13).
const (
	reasonBeforeEarliest        = "BEFORE_EARLIEST_SEND_TIME"
	reasonAfterLatest           = "AFTER_LATEST_SEND_TIME"
	reasonBelowMinDelay         = "BELOW_MINIMUM_DELAY"
	reasonRecipientOutsideWork  = "RECIPIENT_OUTSIDE_WORKING_HOURS"
	reasonRecipientQuiet        = "RECIPIENT_QUIET_HOURS"
	reasonSenderQuiet           = "SENDER_QUIET_HOURS"
	reasonWeekend               = "WEEKEND"
	reasonHoliday               = "HOLIDAY"
	reasonInsufficientCoverage  = "INSUFFICIENT_RECIPIENT_COVERAGE"
	reasonCalendarBusy          = "CALENDAR_BUSY"
)

// Scoring weights and penalties (spec section 15). These are initial product
// assumptions bound to SendTimePolicyVersion.
const (
	wWorking   = 0.30
	wPreferred = 0.20
	wOverlap   = 0.15
	wCoverage  = 0.15
	wPriority  = 0.10
	wDelay     = 0.05
	wDay       = 0.05

	pLunch      = 0.10
	pEndOfDay   = 0.10
	pStartOfDay = 0.03
	pQuietProx  = 0.10
	pFriday     = 0.10
	pLongDelay  = 0.15
	pAvoidWin   = 0.10
)

var supportedIntervals = map[int]bool{5: true, 10: true, 15: true, 30: true, 60: true}

// ValidationError is a structured, client-facing input error. Status maps to the
// HTTP status the controller should return.
type ValidationError struct {
	Code    string
	Message string
	Field   string
	Status  int
}

// SendTimeService recommends future send times.
type SendTimeService interface {
	// Validate checks the request shape and returns a structured error, or nil.
	Validate(req models.SendTimeRequest) *ValidationError
	// Suggest runs the deterministic pipeline. It returns an internal error only
	// for unexpected failures; "no valid candidates" is a normal 200 result.
	Suggest(req models.SendTimeRequest) (*models.SendTimeResponse, error)
}

type sendTimeService struct {
	logger hclog.Logger
	cfg    *config.Scheduler0Configurations
}

// NewSendTimeService constructs the deterministic send-time engine.
func NewSendTimeService(logger hclog.Logger, cfg *config.Scheduler0Configurations) SendTimeService {
	return &sendTimeService{
		logger: logger.Named("send-time"),
		cfg:    cfg,
	}
}

// ---------------------------------------------------------------------------
// Validation
// ---------------------------------------------------------------------------

func (s *sendTimeService) Validate(req models.SendTimeRequest) *ValidationError {
	if len(req.Recipients) == 0 {
		return &ValidationError{Code: "MISSING_RECIPIENTS", Message: "at least one recipient is required", Field: "recipients", Status: http.StatusBadRequest}
	}
	if len(req.Recipients) > constants.SendTimeMaxRecipients {
		return &ValidationError{Code: "TOO_MANY_RECIPIENTS", Message: fmt.Sprintf("a maximum of %d recipients is allowed", constants.SendTimeMaxRecipients), Field: "recipients", Status: http.StatusBadRequest}
	}

	if req.Sender != nil && strings.TrimSpace(req.Sender.Timezone) != "" {
		if _, err := time.LoadLocation(req.Sender.Timezone); err != nil {
			return &ValidationError{Code: "INVALID_TIMEZONE", Message: "sender has an invalid IANA timezone", Field: "sender.timezone", Status: http.StatusBadRequest}
		}
	}
	for i, r := range req.Recipients {
		if strings.TrimSpace(r.Timezone) == "" {
			return &ValidationError{Code: "MISSING_TIMEZONE", Message: fmt.Sprintf("recipient %s is missing a timezone", recipientLabel(r, i)), Field: fmt.Sprintf("recipients[%d].timezone", i), Status: http.StatusBadRequest}
		}
		if _, err := time.LoadLocation(r.Timezone); err != nil {
			return &ValidationError{Code: "INVALID_TIMEZONE", Message: fmt.Sprintf("recipient %s has an invalid IANA timezone", recipientLabel(r, i)), Field: fmt.Sprintf("recipients[%d].timezone", i), Status: http.StatusBadRequest}
		}
	}

	if req.Options != nil {
		if req.Options.CandidateIntervalMinutes != nil && !supportedIntervals[*req.Options.CandidateIntervalMinutes] {
			return &ValidationError{Code: "UNSUPPORTED_CANDIDATE_INTERVAL", Message: "candidate_interval_minutes must be one of 5, 10, 15, 30, or 60", Field: "options.candidate_interval_minutes", Status: http.StatusBadRequest}
		}
		if req.Options.SuggestionCount != nil && (*req.Options.SuggestionCount < 1 || *req.Options.SuggestionCount > constants.SendTimeMaxSuggestions) {
			return &ValidationError{Code: "INVALID_SUGGESTION_COUNT", Message: fmt.Sprintf("suggestion_count must be between 1 and %d", constants.SendTimeMaxSuggestions), Field: "options.suggestion_count", Status: http.StatusBadRequest}
		}
		if req.Options.SearchHorizonDays != nil && *req.Options.SearchHorizonDays < 1 {
			return &ValidationError{Code: "INVALID_SEARCH_HORIZON", Message: "search_horizon_days must be at least 1", Field: "options.search_horizon_days", Status: http.StatusBadRequest}
		}
		if req.Options.ReferenceTime != "" {
			if _, err := parseInstant(req.Options.ReferenceTime); err != nil {
				return &ValidationError{Code: "INVALID_REFERENCE_TIME", Message: "reference_time must be an RFC3339 timestamp", Field: "options.reference_time", Status: http.StatusBadRequest}
			}
		}
	}

	var earliest, latest *time.Time
	if req.Constraints != nil {
		if req.Constraints.EarliestSendAt != "" {
			t, err := parseInstant(req.Constraints.EarliestSendAt)
			if err != nil {
				return &ValidationError{Code: "INVALID_SEARCH_WINDOW", Message: "earliest_send_at must be an RFC3339 timestamp", Field: "constraints.earliest_send_at", Status: http.StatusBadRequest}
			}
			earliest = &t
		}
		if req.Constraints.LatestSendAt != "" {
			t, err := parseInstant(req.Constraints.LatestSendAt)
			if err != nil {
				return &ValidationError{Code: "INVALID_SEARCH_WINDOW", Message: "latest_send_at must be an RFC3339 timestamp", Field: "constraints.latest_send_at", Status: http.StatusBadRequest}
			}
			latest = &t
		}
		if earliest != nil && latest != nil && !latest.After(*earliest) {
			return &ValidationError{Code: "INVALID_SEARCH_WINDOW", Message: "latest_send_at must be later than earliest_send_at", Field: "constraints.latest_send_at", Status: http.StatusBadRequest}
		}
	}

	if req.GroupPolicy != nil && req.GroupPolicy.MinimumRecipientCoverage != nil {
		c := *req.GroupPolicy.MinimumRecipientCoverage
		if c < 0.0 || c > 1.0 {
			return &ValidationError{Code: "INVALID_RECIPIENT_COVERAGE", Message: "minimum_recipient_coverage must be between 0.0 and 1.0", Field: "group_policy.minimum_recipient_coverage", Status: http.StatusBadRequest}
		}
	}

	if req.Message != nil && len(req.Message.Text) > constants.SendTimeMaxMessageText {
		return &ValidationError{Code: "MESSAGE_TOO_LONG", Message: fmt.Sprintf("message text exceeds %d characters", constants.SendTimeMaxMessageText), Field: "message.text", Status: http.StatusBadRequest}
	}

	for i, a := range req.Availability {
		if len(a.BusyIntervals) > constants.SendTimeMaxBusyIntervals {
			return &ValidationError{Code: "TOO_MANY_BUSY_INTERVALS", Message: fmt.Sprintf("a maximum of %d busy intervals is allowed", constants.SendTimeMaxBusyIntervals), Field: fmt.Sprintf("availability[%d].busy_intervals", i), Status: http.StatusBadRequest}
		}
	}

	return nil
}

// ---------------------------------------------------------------------------
// Resolved participants (parsed once per request)
// ---------------------------------------------------------------------------

type workInterval struct {
	days     map[time.Weekday]bool
	startMin int
	endMin   int
}

type quietInterval struct {
	startMin int
	endMin   int
}

type busyRange struct {
	start time.Time
	end   time.Time
}

type resolvedParticipant struct {
	id          string
	displayName string
	role        string
	tz          string
	loc         *time.Location
	work        workInterval
	quiet       *quietInterval
	busy        []busyRange
	defaultedWH bool
}

// localEval holds the per-candidate local evaluation for one participant.
type localEval struct {
	local         time.Time
	day           time.Weekday
	minOfDay      int
	withinWorking bool
	withinQuiet   bool
	isWeekend     bool
	isHoliday     bool
	isBusy        bool
}

// ---------------------------------------------------------------------------
// Suggest
// ---------------------------------------------------------------------------

func (s *sendTimeService) Suggest(req models.SendTimeRequest) (*models.SendTimeResponse, error) {
	var warnings []models.SendTimeWarning

	// Reference time (echoed exactly when supplied).
	referenceInstant := time.Now().UTC()
	referenceOut := ""
	if req.Options != nil && req.Options.ReferenceTime != "" {
		t, err := parseInstant(req.Options.ReferenceTime)
		if err != nil {
			return nil, fmt.Errorf("send-time: parse reference_time: %w", err)
		}
		referenceInstant = t.UTC()
		referenceOut = req.Options.ReferenceTime
	}

	// Options + defaults.
	interval := constants.SendTimeDefaultInterval
	suggestionCount := constants.SendTimeDefaultSuggestions
	horizonDays := constants.SendTimeDefaultHorizonDays
	includeBreakdown := false
	includeRejected := false
	evaluateSendNow := true
	diversify := true
	spacingMinutes := 120
	if req.Options != nil {
		if req.Options.CandidateIntervalMinutes != nil {
			interval = *req.Options.CandidateIntervalMinutes
		}
		if req.Options.SuggestionCount != nil {
			suggestionCount = *req.Options.SuggestionCount
		}
		if req.Options.SearchHorizonDays != nil {
			horizonDays = *req.Options.SearchHorizonDays
		}
		if req.Options.IncludeScoreBreakdown != nil {
			includeBreakdown = *req.Options.IncludeScoreBreakdown
		}
		if req.Options.IncludeRejectedSummary != nil {
			includeRejected = *req.Options.IncludeRejectedSummary
		}
		if req.Options.EvaluateSendNow != nil {
			evaluateSendNow = *req.Options.EvaluateSendNow
		}
		if req.Options.DiversifySuggestions != nil {
			diversify = *req.Options.DiversifySuggestions
		}
		if req.Options.MinimumSuggestionSpacingMinutes != nil {
			spacingMinutes = *req.Options.MinimumSuggestionSpacingMinutes
		}
	}
	if horizonDays > constants.SendTimeMaxHorizonDays {
		horizonDays = constants.SendTimeMaxHorizonDays
	}
	if suggestionCount < 1 {
		suggestionCount = 1
	}
	if suggestionCount > constants.SendTimeMaxSuggestions {
		suggestionCount = constants.SendTimeMaxSuggestions
	}

	// Constraint flags + defaults.
	minDelay := int64(0)
	workingHoursOnly := false
	avoidWeekends := true
	avoidHolidays := false
	respectQuiet := true
	requireCalFree := false
	var earliest, latest *time.Time
	if req.Constraints != nil {
		if req.Constraints.MinimumDelaySeconds != nil {
			minDelay = *req.Constraints.MinimumDelaySeconds
		}
		if req.Constraints.WorkingHoursOnly != nil {
			workingHoursOnly = *req.Constraints.WorkingHoursOnly
		}
		if req.Constraints.AvoidWeekends != nil {
			avoidWeekends = *req.Constraints.AvoidWeekends
		}
		if req.Constraints.AvoidHolidays != nil {
			avoidHolidays = *req.Constraints.AvoidHolidays
		}
		if req.Constraints.RespectQuietHours != nil {
			respectQuiet = *req.Constraints.RespectQuietHours
		}
		if req.Constraints.RequireCalendarFree != nil {
			requireCalFree = *req.Constraints.RequireCalendarFree
		}
		if req.Constraints.EarliestSendAt != "" {
			t, _ := parseInstant(req.Constraints.EarliestSendAt)
			tu := t.UTC()
			earliest = &tu
		}
		if req.Constraints.LatestSendAt != "" {
			t, _ := parseInstant(req.Constraints.LatestSendAt)
			tu := t.UTC()
			latest = &tu
		}
	}

	// Holiday dates (caller-supplied only for the first release).
	holidaySet := map[string]bool{}
	if req.HolidayPolicy != nil {
		for _, d := range req.HolidayPolicy.Dates {
			holidaySet[strings.TrimSpace(d)] = true
		}
	}
	if avoidHolidays && len(holidaySet) == 0 {
		warnings = append(warnings, models.SendTimeWarning{
			Code:    "HOLIDAY_POLICY_UNAVAILABLE",
			Message: "avoid_holidays was requested, but no holiday dates were supplied; holiday filtering was skipped",
		})
	}

	// Resolve sender + recipients.
	var sender *resolvedParticipant
	if req.Sender != nil {
		rp, w := resolveParticipant(*req.Sender, req.Availability, true)
		sender = rp
		warnings = append(warnings, w...)
	}
	recipients := make([]resolvedParticipant, 0, len(req.Recipients))
	for i, r := range req.Recipients {
		rp, w := resolveParticipant(r, req.Availability, false)
		if rp.id == "" {
			rp.id = fmt.Sprintf("recipient_%d", i)
		}
		recipients = append(recipients, *rp)
		warnings = append(warnings, w...)
	}

	// Message priority.
	priority := "normal"
	if req.Message != nil && strings.TrimSpace(req.Message.Priority) != "" {
		priority = strings.ToLower(strings.TrimSpace(req.Message.Priority))
	}

	// Preferences.
	preferOverlap := true
	var preferredWindows, avoidWindows []models.SendTimeWindow
	if req.Preferences != nil {
		if req.Preferences.PreferSenderRecipientOverlap != nil {
			preferOverlap = *req.Preferences.PreferSenderRecipientOverlap
		}
		preferredWindows = req.Preferences.PreferredRecipientWindows
		avoidWindows = req.Preferences.AvoidRecipientWindows
	}

	// Group policy.
	strategy := "all_recipients"
	if req.GroupPolicy != nil && strings.TrimSpace(req.GroupPolicy.Strategy) != "" {
		strategy = strings.ToLower(strings.TrimSpace(req.GroupPolicy.Strategy))
	}
	requiredCoverage := 1.0
	if strategy == "majority" {
		requiredCoverage = 0.5
	}
	if req.GroupPolicy != nil && req.GroupPolicy.MinimumRecipientCoverage != nil {
		requiredCoverage = *req.GroupPolicy.MinimumRecipientCoverage
	}

	// Search window.
	windowStart := referenceInstant.Add(time.Duration(minDelay) * time.Second)
	if earliest != nil && earliest.After(windowStart) {
		windowStart = *earliest
	}
	windowEnd := referenceInstant.Add(time.Duration(horizonDays) * 24 * time.Hour)
	if latest != nil {
		windowEnd = *latest
	}

	scoringCtx := &scoringContext{
		referenceInstant: referenceInstant,
		horizonDays:      horizonDays,
		priority:         priority,
		preferOverlap:    preferOverlap,
		preferredWindows: preferredWindows,
		avoidWindows:     avoidWindows,
		sender:           sender,
		recipients:       recipients,
		holidaySet:       holidaySet,
		avoidWeekends:    avoidWeekends,
		avoidHolidays:    avoidHolidays && len(holidaySet) > 0,
		workingHoursOnly: workingHoursOnly,
		respectQuiet:     respectQuiet,
		requireCalFree:   requireCalFree,
		strategy:         strategy,
		requiredCoverage: requiredCoverage,
	}

	// Generate + evaluate candidates.
	intervalDur := time.Duration(interval) * time.Minute
	rejectedSummary := map[string]int{}
	generated := 0
	var scored []*scoredCandidate

	candidate := alignUp(windowStart, intervalDur)
	guard := 0
	for !candidate.After(windowEnd) {
		guard++
		if guard > 200000 { // safety valve; far beyond 30d @ 5min
			break
		}
		generated++

		accepted, reasons, coverage, evals := scoringCtx.evaluate(candidate)
		if !accepted {
			for _, code := range reasons {
				rejectedSummary[code]++
			}
			candidate = candidate.Add(intervalDur)
			continue
		}

		sc := scoringCtx.score(candidate, coverage, evals)
		scored = append(scored, sc)
		candidate = candidate.Add(intervalDur)
	}

	// Sort with deterministic tie-breakers (spec section 17).
	sort.SliceStable(scored, func(i, j int) bool {
		return lessCandidate(scored[i], scored[j])
	})

	// Urgent: prepend an immediate-send recommendation that ignores working-hour
	// preferences (spec section 16.4).
	var ordered []*scoredCandidate
	if priority == "urgent" {
		immediate := scoringCtx.buildImmediate(windowStart)
		ordered = append(ordered, immediate)
		for _, sc := range scored {
			if sc.instant.Equal(immediate.instant) {
				continue
			}
			ordered = append(ordered, sc)
		}
	} else {
		ordered = scored
	}

	// Diversify (keeping the top candidate first) then take top N.
	selected := selectSuggestions(ordered, diversify, spacingMinutes, suggestionCount)

	suggestions := make([]models.SendTimeSuggestion, 0, len(selected))
	for idx, sc := range selected {
		sug := sc.toModel(idx+1, includeBreakdown)
		suggestions = append(suggestions, sug)
	}

	resp := &models.SendTimeResponse{
		RequestID:     "req_" + ksuid.New().String(),
		ReferenceTime: referenceOut,
		Policy:        models.SendTimePolicyRef{ID: constants.SendTimePolicyID, Version: constants.SendTimePolicyVersion},
		Engine:        models.SendTimeEngineRef{Version: constants.SendTimeEngineVersion, TimezoneDatabaseVersion: tzdbVersion()},
		Suggestions:   suggestions,
		Search: models.SendTimeSearchInfo{
			WindowStart:              windowStart.Format(time.RFC3339),
			WindowEnd:                windowEnd.Format(time.RFC3339),
			CandidateIntervalMinutes: interval,
			CandidatesGenerated:      generated,
			CandidatesRejected:       generated - len(scored),
			CandidatesScored:         len(scored),
		},
		Warnings: warnings,
		Metadata: req.Metadata,
	}
	if resp.ReferenceTime == "" {
		refTZ := time.UTC
		if sender != nil {
			refTZ = sender.loc
		}
		resp.ReferenceTime = referenceInstant.In(refTZ).Format(time.RFC3339)
	}
	if resp.Warnings == nil {
		resp.Warnings = []models.SendTimeWarning{}
	}
	if includeRejected {
		resp.RejectedSummary = rejectedSummary
	}

	// No-suggestion result is still a 200 (spec section 10).
	if len(suggestions) == 0 {
		resp.NoSuggestion = &models.SendTimeNoSuggestion{
			Code:    "NO_VALID_CANDIDATES",
			Message: "No candidate satisfied all hard constraints.",
			Recommendations: []string{
				"Extend latest_send_at or search_horizon_days.",
				"Allow delivery outside working hours (working_hours_only=false).",
				"Use the best_effort group strategy.",
			},
		}
	}

	// Optional immediate-send evaluation (spec section 19).
	if evaluateSendNow {
		resp.SendNow = scoringCtx.evaluateSendNow(referenceInstant, windowStart)
	}

	return resp, nil
}

// ---------------------------------------------------------------------------
// Scoring context + candidate evaluation
// ---------------------------------------------------------------------------

type scoringContext struct {
	referenceInstant time.Time
	horizonDays      int
	priority         string
	preferOverlap    bool
	preferredWindows []models.SendTimeWindow
	avoidWindows     []models.SendTimeWindow
	sender           *resolvedParticipant
	recipients       []resolvedParticipant
	holidaySet       map[string]bool
	avoidWeekends    bool
	avoidHolidays    bool
	workingHoursOnly bool
	respectQuiet     bool
	requireCalFree   bool
	strategy         string
	requiredCoverage float64
}

type scoredCandidate struct {
	instant        time.Time
	score          float64
	breakdown      map[string]float64
	flags          []string
	reason         string
	label          string
	coverage       models.SendTimeCoverage
	senderLocal    *models.SendTimeLocal
	recipientLocal []models.SendTimeLocal
	primaryCover   float64
	overlapFrac    float64
	forcedFirst    bool
}

func (ctx *scoringContext) evalParticipant(rp *resolvedParticipant, t time.Time) localEval {
	local := t.In(rp.loc)
	minOfDay := local.Hour()*60 + local.Minute()
	e := localEval{
		local:    local,
		day:      local.Weekday(),
		minOfDay: minOfDay,
	}
	e.withinWorking = rp.work.days[local.Weekday()] && minOfDay >= rp.work.startMin && minOfDay < rp.work.endMin
	if rp.quiet != nil {
		e.withinQuiet = inQuiet(minOfDay, rp.quiet.startMin, rp.quiet.endMin)
	}
	e.isWeekend = local.Weekday() == time.Saturday || local.Weekday() == time.Sunday
	if len(ctx.holidaySet) > 0 {
		e.isHoliday = ctx.holidaySet[local.Format("2006-01-02")]
	}
	for _, b := range rp.busy {
		if !t.Before(b.start) && t.Before(b.end) {
			e.isBusy = true
			break
		}
	}
	return e
}

// recipientValid reports whether a recipient passes its hard constraints for the
// candidate, plus the list of blocking reason codes.
func (ctx *scoringContext) recipientValid(e localEval) (bool, []string) {
	var reasons []string
	valid := true
	if ctx.avoidWeekends && e.isWeekend {
		valid = false
		reasons = append(reasons, reasonWeekend)
	}
	if ctx.avoidHolidays && e.isHoliday {
		valid = false
		reasons = append(reasons, reasonHoliday)
	}
	if ctx.workingHoursOnly && !e.withinWorking {
		valid = false
		reasons = append(reasons, reasonRecipientOutsideWork)
	}
	if ctx.respectQuiet && e.withinQuiet {
		valid = false
		reasons = append(reasons, reasonRecipientQuiet)
	}
	if ctx.requireCalFree && e.isBusy {
		valid = false
		reasons = append(reasons, reasonCalendarBusy)
	}
	return valid, reasons
}

// evaluate applies global + per-recipient hard constraints. It returns whether
// the candidate is accepted, the reason codes when rejected, the recipient
// coverage, and the per-recipient local evaluations.
func (ctx *scoringContext) evaluate(t time.Time) (bool, []string, models.SendTimeCoverage, []localEval) {
	// Global: minimum delay + window bounds.
	if t.Before(ctx.referenceInstant) {
		return false, []string{reasonBelowMinDelay}, models.SendTimeCoverage{}, nil
	}

	// Global: sender quiet hours.
	if ctx.respectQuiet && ctx.sender != nil && ctx.sender.quiet != nil {
		se := ctx.evalParticipant(ctx.sender, t)
		if se.withinQuiet {
			return false, []string{reasonSenderQuiet}, models.SendTimeCoverage{}, nil
		}
	}

	evals := make([]localEval, len(ctx.recipients))
	reasonSet := map[string]bool{}
	validCount := 0
	primaryTotal := 0
	primaryValid := 0
	for i := range ctx.recipients {
		e := ctx.evalParticipant(&ctx.recipients[i], t)
		evals[i] = e
		ok, reasons := ctx.recipientValid(e)
		isPrimary := strings.EqualFold(ctx.recipients[i].role, "primary")
		if isPrimary {
			primaryTotal++
		}
		if ok {
			validCount++
			if isPrimary {
				primaryValid++
			}
		} else {
			for _, r := range reasons {
				reasonSet[r] = true
			}
		}
	}

	total := len(ctx.recipients)
	ratio := 0.0
	if total > 0 {
		ratio = float64(validCount) / float64(total)
	}
	coverage := models.SendTimeCoverage{ValidRecipients: validCount, TotalRecipients: total, Ratio: round2(ratio)}

	accepted := false
	switch ctx.strategy {
	case "best_effort":
		accepted = true
	case "primary_recipients":
		if primaryTotal == 0 {
			accepted = validCount == total
		} else {
			accepted = primaryValid == primaryTotal
		}
	case "majority":
		accepted = ratio >= ctx.requiredCoverage
	default: // all_recipients
		if ctx.requiredCoverage < 1.0 {
			accepted = ratio >= ctx.requiredCoverage
		} else {
			accepted = validCount == total
		}
	}

	if accepted {
		return true, nil, coverage, evals
	}

	reasons := reasonSlice(reasonSet)
	if ctx.strategy == "majority" || ctx.requiredCoverage < 1.0 {
		reasons = append([]string{reasonInsufficientCoverage}, reasons...)
	}
	if len(reasons) == 0 {
		reasons = []string{reasonInsufficientCoverage}
	}
	return false, reasons, coverage, evals
}

func (ctx *scoringContext) score(t time.Time, coverage models.SendTimeCoverage, evals []localEval) *scoredCandidate {
	total := len(ctx.recipients)

	// Positive dimensions.
	workingSum := 0.0
	preferredSum := 0.0
	anyPreferred := len(ctx.preferredWindows) > 0
	recipientWithinAvg := 0.0
	for _, e := range evals {
		if e.withinWorking {
			workingSum++
			recipientWithinAvg++
		}
		if anyPreferred && inAnyWindow(e.minOfDay, ctx.preferredWindows) {
			preferredSum++
		}
	}
	workingFrac := safeDiv(workingSum, float64(total))
	recipientWithinAvg = safeDiv(recipientWithinAvg, float64(total))
	preferredFrac := 0.0
	if anyPreferred {
		preferredFrac = safeDiv(preferredSum, float64(total))
	}

	overlapFrac := 0.0
	if ctx.preferOverlap {
		senderWithin := true
		if ctx.sender != nil {
			se := ctx.evalParticipant(ctx.sender, t)
			senderWithin = se.withinWorking
		}
		if senderWithin {
			overlapFrac = recipientWithinAvg
		}
	}

	coverageFrac := coverage.Ratio
	priorityFrac := priorityFraction(ctx.priority)

	delaySeconds := t.Sub(ctx.referenceInstant).Seconds()
	horizonSeconds := float64(ctx.horizonDays) * 86400.0
	delayFrac := 1.0 - clamp(delaySeconds/horizonSeconds, 0, 1)

	dayFrac := dayFraction(evals)

	// Penalties.
	penalties := 0.0
	if anyRecipient(evals, func(e localEval) bool { return e.minOfDay >= 720 && e.minOfDay < 780 }) {
		penalties += pLunch
	}
	if ctx.anyNearEndOfDay(evals) {
		penalties += pEndOfDay
	}
	if ctx.anyNearStartOfDay(evals) {
		penalties += pStartOfDay
	}
	if ctx.anyNearQuietBoundary(evals) {
		penalties += pQuietProx
	}
	if anyRecipient(evals, func(e localEval) bool { return e.day == time.Friday && e.minOfDay >= 900 }) {
		penalties += pFriday
	}
	if (ctx.priority == "high" || ctx.priority == "urgent") && delaySeconds > 48*3600 {
		penalties += pLongDelay
	}
	if len(ctx.avoidWindows) > 0 && anyRecipient(evals, func(e localEval) bool { return inAnyWindow(e.minOfDay, ctx.avoidWindows) }) {
		penalties += pAvoidWin
	}

	rawPositive := workingFrac*wWorking + preferredFrac*wPreferred + overlapFrac*wOverlap +
		coverageFrac*wCoverage + priorityFrac*wPriority + delayFrac*wDelay + dayFrac*wDay
	score := clamp(rawPositive-penalties, 0, 1)

	breakdown := map[string]float64{
		"recipient_working_hours":    round2(workingFrac * wWorking),
		"preferred_recipient_window": round2(preferredFrac * wPreferred),
		"sender_recipient_overlap":   round2(overlapFrac * wOverlap),
		"recipient_coverage":         round2(coverageFrac * wCoverage),
		"message_priority":           round2(priorityFrac * wPriority),
		"delay_preference":           round2(delayFrac * wDelay),
		"day_preference":             round2(dayFrac * wDay),
		"penalties":                  round2(penalties),
	}

	// Flags.
	var flags []string
	workdayStart := ctx.anyNearStartOfDay60(evals)
	if workdayStart {
		flags = append(flags, "RECIPIENT_WORKDAY_START")
	}
	if overlapFrac > 0 {
		flags = append(flags, "SENDER_RECIPIENT_OVERLAP")
	}
	if preferredFrac > 0 {
		flags = append(flags, "PREFERRED_RECIPIENT_WINDOW")
	}

	primaryCover := 0.0
	primaryTotal, primaryOK := 0, 0
	for i, e := range evals {
		if strings.EqualFold(ctx.recipients[i].role, "primary") {
			primaryTotal++
			if ok, _ := ctx.recipientValid(e); ok {
				primaryOK++
			}
		}
	}
	if primaryTotal > 0 {
		primaryCover = float64(primaryOK) / float64(primaryTotal)
	} else {
		primaryCover = coverage.Ratio
	}

	sc := &scoredCandidate{
		instant:        t,
		score:          round2(score),
		breakdown:      breakdown,
		flags:          flags,
		coverage:       coverage,
		recipientLocal: ctx.recipientLocals(evals),
		primaryCover:   primaryCover,
		overlapFrac:    overlapFrac,
	}
	if ctx.sender != nil {
		sc.senderLocal = ctx.senderLocal(t)
	}
	sc.label = ctx.buildLabel(t)
	sc.reason = ctx.buildReason(flags, preferredFrac > 0, overlapFrac > 0)
	return sc
}

// ---------------------------------------------------------------------------
// Immediate send + send-now evaluation
// ---------------------------------------------------------------------------

func (ctx *scoringContext) buildImmediate(windowStart time.Time) *scoredCandidate {
	evals := make([]localEval, len(ctx.recipients))
	validCount := 0
	for i := range ctx.recipients {
		evals[i] = ctx.evalParticipant(&ctx.recipients[i], windowStart)
		if ok, _ := ctx.recipientValid(evals[i]); ok {
			validCount++
		}
	}
	total := len(ctx.recipients)
	ratio := 0.0
	if total > 0 {
		ratio = float64(validCount) / float64(total)
	}
	sc := &scoredCandidate{
		instant:        windowStart,
		score:          1.0,
		flags:          []string{"IMMEDIATE_SEND_RECOMMENDED", "URGENT_MESSAGE"},
		reason:         "Urgent message: immediate delivery is recommended.",
		label:          "Send now",
		coverage:       models.SendTimeCoverage{ValidRecipients: validCount, TotalRecipients: total, Ratio: round2(ratio)},
		recipientLocal: ctx.recipientLocals(evals),
		primaryCover:   ratio,
		forcedFirst:    true,
		breakdown: map[string]float64{
			"message_priority": wPriority,
			"penalties":        0.0,
		},
	}
	if ctx.sender != nil {
		sc.senderLocal = ctx.senderLocal(windowStart)
	}
	return sc
}

func (ctx *scoringContext) evaluateSendNow(referenceInstant, windowStart time.Time) *models.SendTimeSendNow {
	t := referenceInstant
	if windowStart.After(t) {
		t = windowStart
	}
	accepted, _, coverage, evals := ctx.evaluate(t)
	recommended := ctx.priority == "urgent"
	var score float64
	var reason string
	if accepted {
		sc := ctx.score(t, coverage, evals)
		score = sc.score
		if ctx.priority == "urgent" || score >= 0.7 {
			recommended = true
			reason = "The current time is suitable for all gating recipients."
		} else {
			reason = "The current time is acceptable but not ideal."
		}
	} else {
		reason = "The current time violates a hard constraint for at least one recipient."
		if ctx.priority == "urgent" {
			reason = "Urgent message: immediate delivery is recommended despite suboptimal timing."
		}
	}
	return &models.SendTimeSendNow{
		Recommended:         recommended,
		Score:               round2(score),
		Reason:              reason,
		RecipientLocalTimes: ctx.recipientLocals(evals),
	}
}

// ---------------------------------------------------------------------------
// Local-time projections
// ---------------------------------------------------------------------------

func (ctx *scoringContext) recipientLocals(evals []localEval) []models.SendTimeLocal {
	out := make([]models.SendTimeLocal, 0, len(ctx.recipients))
	for i := range ctx.recipients {
		rp := ctx.recipients[i]
		var e localEval
		if i < len(evals) {
			e = evals[i]
		} else {
			e = ctx.evalParticipant(&rp, ctx.referenceInstant)
		}
		out = append(out, models.SendTimeLocal{
			RecipientID:        rp.id,
			Datetime:           e.local.Format(time.RFC3339),
			Timezone:           rp.tz,
			Day:                weekdayCode(e.day),
			Time:               e.local.Format("15:04"),
			WithinWorkingHours: boolPtr(e.withinWorking),
			WithinQuietHours:   boolPtr(e.withinQuiet),
			IsWeekend:          boolPtr(e.isWeekend),
			IsHoliday:          boolPtr(e.isHoliday),
		})
	}
	return out
}

func (ctx *scoringContext) senderLocal(t time.Time) *models.SendTimeLocal {
	if ctx.sender == nil {
		return nil
	}
	local := t.In(ctx.sender.loc)
	return &models.SendTimeLocal{
		Datetime: local.Format(time.RFC3339),
		Timezone: ctx.sender.tz,
		Day:      weekdayCode(local.Weekday()),
		Time:     local.Format("15:04"),
	}
}

// ---------------------------------------------------------------------------
// Labels + reasons
// ---------------------------------------------------------------------------

func (ctx *scoringContext) buildLabel(t time.Time) string {
	if len(ctx.recipients) == 0 {
		return t.UTC().Format("Mon 15:04")
	}
	rp := ctx.recipients[0]
	local := t.In(rp.loc)
	refLocal := ctx.referenceInstant.In(rp.loc)
	dayDiff := calendarDayDiff(refLocal, local)
	part := partOfDay(local.Hour()*60 + local.Minute())
	switch dayDiff {
	case 0:
		return "Today " + part
	case 1:
		return "Tomorrow " + part
	default:
		return local.Weekday().String() + " " + part
	}
}

func (ctx *scoringContext) buildReason(flags []string, preferred, overlap bool) string {
	name := "the recipient"
	if len(ctx.recipients) > 0 && ctx.recipients[0].displayName != "" {
		name = ctx.recipients[0].displayName
	}
	parts := []string{fmt.Sprintf("This falls within %s's working hours.", name)}
	if preferred {
		parts = append(parts, "It matches a preferred recipient window.")
	}
	if overlap {
		parts = append(parts, "It also overlaps with the sender's working hours.")
	}
	if containsFlag(flags, "RECIPIENT_WORKDAY_START") {
		parts = append(parts, "It is near the start of the recipient's working day.")
	}
	return strings.Join(parts, " ")
}

// ---------------------------------------------------------------------------
// Selection / diversification
// ---------------------------------------------------------------------------

func selectSuggestions(ordered []*scoredCandidate, diversify bool, spacingMinutes, count int) []*scoredCandidate {
	if len(ordered) == 0 {
		return nil
	}
	if !diversify || spacingMinutes <= 0 {
		if len(ordered) > count {
			return ordered[:count]
		}
		return ordered
	}
	spacing := time.Duration(spacingMinutes) * time.Minute
	var picked []*scoredCandidate
	for _, sc := range ordered {
		if len(picked) == 0 || sc.forcedFirst {
			picked = append(picked, sc)
			if len(picked) >= count {
				return picked
			}
			continue
		}
		ok := true
		for _, p := range picked {
			if absDuration(sc.instant.Sub(p.instant)) < spacing {
				ok = false
				break
			}
		}
		if ok {
			picked = append(picked, sc)
			if len(picked) >= count {
				return picked
			}
		}
	}
	// Backfill with the highest-scoring leftovers if diversification left gaps.
	if len(picked) < count {
		inPicked := map[time.Time]bool{}
		for _, p := range picked {
			inPicked[p.instant] = true
		}
		for _, sc := range ordered {
			if inPicked[sc.instant] {
				continue
			}
			picked = append(picked, sc)
			if len(picked) >= count {
				break
			}
		}
	}
	return picked
}

func lessCandidate(a, b *scoredCandidate) bool {
	if a.score != b.score {
		return a.score > b.score
	}
	if !a.instant.Equal(b.instant) {
		return a.instant.Before(b.instant) // earliest first
	}
	if a.primaryCover != b.primaryCover {
		return a.primaryCover > b.primaryCover
	}
	if a.coverage.Ratio != b.coverage.Ratio {
		return a.coverage.Ratio > b.coverage.Ratio
	}
	if a.overlapFrac != b.overlapFrac {
		return a.overlapFrac > b.overlapFrac
	}
	return a.instant.UTC().Format(time.RFC3339) < b.instant.UTC().Format(time.RFC3339)
}

func (sc *scoredCandidate) toModel(rank int, includeBreakdown bool) models.SendTimeSuggestion {
	sug := models.SendTimeSuggestion{
		ID:                  fmt.Sprintf("sts_%03d", rank),
		SendAt:              sc.instant.Format(time.RFC3339),
		Label:               sc.label,
		Score:               sc.score,
		Rank:                rank,
		Reason:              sc.reason,
		SenderLocal:         sc.senderLocal,
		RecipientLocalTimes: sc.recipientLocal,
		Coverage:            sc.coverage,
		Flags:               sc.flags,
	}
	if includeBreakdown {
		sug.ScoreBreakdown = sc.breakdown
	}
	if sug.RecipientLocalTimes == nil {
		sug.RecipientLocalTimes = []models.SendTimeLocal{}
	}
	return sug
}

// ---------------------------------------------------------------------------
// Participant resolution helpers
// ---------------------------------------------------------------------------

func resolveParticipant(p models.SendTimeParticipant, availability []models.SendTimeAvailability, isSender bool) (*resolvedParticipant, []models.SendTimeWarning) {
	var warnings []models.SendTimeWarning
	loc, err := time.LoadLocation(p.Timezone)
	if err != nil {
		loc = time.UTC
	}
	rp := &resolvedParticipant{
		id:          p.ID,
		displayName: p.DisplayName,
		role:        p.Role,
		tz:          p.Timezone,
		loc:         loc,
	}

	if p.WorkingHours != nil && (len(p.WorkingHours.Days) > 0 || p.WorkingHours.Start != "" || p.WorkingHours.End != "") {
		rp.work = resolveWorkingHours(p.WorkingHours)
	} else {
		rp.work = defaultWorkingHours()
		rp.defaultedWH = true
		warnings = append(warnings, models.SendTimeWarning{
			Code:          "RECIPIENT_WORKING_HOURS_DEFAULTED",
			ParticipantID: p.ID,
			Message:       "Default working hours of 09:00-17:00 (Mon-Fri) were used.",
		})
	}

	if p.QuietHours != nil && (p.QuietHours.Start != "" || p.QuietHours.End != "") {
		start, errS := parseHHMM(p.QuietHours.Start)
		end, errE := parseHHMM(p.QuietHours.End)
		if errS == nil && errE == nil {
			rp.quiet = &quietInterval{startMin: start, endMin: end}
		}
	}

	if p.ID != "" {
		for _, a := range availability {
			if a.ParticipantID != p.ID {
				continue
			}
			for _, b := range a.BusyIntervals {
				bs, e1 := parseInstant(b.Start)
				be, e2 := parseInstant(b.End)
				if e1 == nil && e2 == nil && be.After(bs) {
					rp.busy = append(rp.busy, busyRange{start: bs.UTC(), end: be.UTC()})
				}
			}
		}
	}

	return rp, warnings
}

func resolveWorkingHours(wh *models.SendTimeWorkingHours) workInterval {
	days := map[time.Weekday]bool{}
	if len(wh.Days) == 0 {
		days = defaultWorkdays()
	} else {
		for _, d := range wh.Days {
			if wd, ok := parseWeekday(d); ok {
				days[wd] = true
			}
		}
	}
	start, err := parseHHMM(wh.Start)
	if err != nil {
		start = 9 * 60
	}
	end, err := parseHHMM(wh.End)
	if err != nil {
		end = 17 * 60
	}
	return workInterval{days: days, startMin: start, endMin: end}
}

func defaultWorkingHours() workInterval {
	return workInterval{days: defaultWorkdays(), startMin: 9 * 60, endMin: 17 * 60}
}

func defaultWorkdays() map[time.Weekday]bool {
	return map[time.Weekday]bool{
		time.Monday:    true,
		time.Tuesday:   true,
		time.Wednesday: true,
		time.Thursday:  true,
		time.Friday:    true,
	}
}

// ---------------------------------------------------------------------------
// Penalty helpers
// ---------------------------------------------------------------------------

func (ctx *scoringContext) anyNearEndOfDay(evals []localEval) bool {
	for i, e := range evals {
		if e.withinWorking && e.minOfDay >= ctx.recipients[i].work.endMin-30 {
			return true
		}
	}
	return false
}

func (ctx *scoringContext) anyNearStartOfDay(evals []localEval) bool {
	for i, e := range evals {
		if e.withinWorking && e.minOfDay < ctx.recipients[i].work.startMin+15 {
			return true
		}
	}
	return false
}

func (ctx *scoringContext) anyNearStartOfDay60(evals []localEval) bool {
	for i, e := range evals {
		if e.withinWorking && e.minOfDay < ctx.recipients[i].work.startMin+60 {
			return true
		}
	}
	return false
}

func (ctx *scoringContext) anyNearQuietBoundary(evals []localEval) bool {
	for i, e := range evals {
		q := ctx.recipients[i].quiet
		if q == nil {
			continue
		}
		if minuteDistance(e.minOfDay, q.startMin) <= 30 || minuteDistance(e.minOfDay, q.endMin) <= 30 {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Small pure helpers
// ---------------------------------------------------------------------------

func parseInstant(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, strings.TrimSpace(s))
}

func parseHHMM(s string) (int, error) {
	s = strings.TrimSpace(s)
	var h, m int
	if _, err := fmt.Sscanf(s, "%d:%d", &h, &m); err != nil {
		return 0, err
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, fmt.Errorf("invalid time %q", s)
	}
	return h*60 + m, nil
}

func parseWeekday(code string) (time.Weekday, bool) {
	switch strings.ToUpper(strings.TrimSpace(code)) {
	case "MON":
		return time.Monday, true
	case "TUE":
		return time.Tuesday, true
	case "WED":
		return time.Wednesday, true
	case "THU":
		return time.Thursday, true
	case "FRI":
		return time.Friday, true
	case "SAT":
		return time.Saturday, true
	case "SUN":
		return time.Sunday, true
	}
	return time.Sunday, false
}

func weekdayCode(d time.Weekday) string {
	switch d {
	case time.Monday:
		return "MON"
	case time.Tuesday:
		return "TUE"
	case time.Wednesday:
		return "WED"
	case time.Thursday:
		return "THU"
	case time.Friday:
		return "FRI"
	case time.Saturday:
		return "SAT"
	default:
		return "SUN"
	}
}

// inQuiet reports whether minute-of-day m falls inside a quiet interval that may
// cross midnight.
func inQuiet(m, start, end int) bool {
	if start == end {
		return false
	}
	if start < end {
		return m >= start && m < end
	}
	return m >= start || m < end
}

func inAnyWindow(m int, windows []models.SendTimeWindow) bool {
	for _, w := range windows {
		start, errS := parseHHMM(w.Start)
		end, errE := parseHHMM(w.End)
		if errS != nil || errE != nil {
			continue
		}
		if start <= end {
			if m >= start && m < end {
				return true
			}
		} else if m >= start || m < end {
			return true
		}
	}
	return false
}

func priorityFraction(priority string) float64 {
	switch priority {
	case "urgent":
		return 1.0
	case "high":
		return 0.8
	case "low":
		return 0.4
	default:
		return 0.6
	}
}

func dayFraction(evals []localEval) float64 {
	if len(evals) == 0 {
		return 0.5
	}
	e := evals[0]
	switch e.day {
	case time.Saturday, time.Sunday:
		return 0.2
	case time.Friday:
		return 0.5
	default:
		return 1.0
	}
}

func anyRecipient(evals []localEval, pred func(localEval) bool) bool {
	for _, e := range evals {
		if pred(e) {
			return true
		}
	}
	return false
}

func partOfDay(minOfDay int) string {
	switch {
	case minOfDay < 12*60:
		return "morning"
	case minOfDay < 17*60:
		return "afternoon"
	default:
		return "evening"
	}
}

func calendarDayDiff(from, to time.Time) int {
	fy, fm, fd := from.Date()
	ty, tm, td := to.Date()
	fromDay := time.Date(fy, fm, fd, 0, 0, 0, 0, time.UTC)
	toDay := time.Date(ty, tm, td, 0, 0, 0, 0, time.UTC)
	return int(toDay.Sub(fromDay).Hours() / 24)
}

func alignUp(t time.Time, d time.Duration) time.Time {
	truncated := t.Truncate(d)
	if truncated.Before(t) {
		return truncated.Add(d)
	}
	return truncated
}

func minuteDistance(a, b int) int {
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	if diff > 720 {
		diff = 1440 - diff
	}
	return diff
}

func reasonSlice(set map[string]bool) []string {
	order := []string{reasonWeekend, reasonHoliday, reasonRecipientOutsideWork, reasonRecipientQuiet, reasonCalendarBusy}
	var out []string
	for _, code := range order {
		if set[code] {
			out = append(out, code)
		}
	}
	return out
}

func containsFlag(flags []string, target string) bool {
	for _, f := range flags {
		if f == target {
			return true
		}
	}
	return false
}

func recipientLabel(r models.SendTimeParticipant, idx int) string {
	if r.ID != "" {
		return r.ID
	}
	if r.DisplayName != "" {
		return r.DisplayName
	}
	return fmt.Sprintf("recipients[%d]", idx)
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func safeDiv(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func boolPtr(b bool) *bool { return &b }

func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

// tzdbVersion best-effort reports the embedded tz database version for the
// engine block. It is informational only.
func tzdbVersion() string {
	return "system"
}
