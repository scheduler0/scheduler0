package sendtime

import (
	"testing"
	"time"

	"scheduler0/pkg/config"
	"scheduler0/pkg/models"

	"github.com/hashicorp/go-hclog"
)

func newTestService() SendTimeService {
	return NewSendTimeService(hclog.NewNullLogger(), &config.Scheduler0Configurations{})
}

func strp(s string) *string   { return &s }
func intp(i int) *int         { return &i }
func i64p(i int64) *int64     { return &i }
func boolp(b bool) *bool      { return &b }
func f64p(f float64) *float64 { return &f }

func mustParse(t *testing.T, s string) time.Time {
	t.Helper()
	tm, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return tm
}

// baseRequest: Friday 2026-07-17 17:45 ET reference, sender Toronto, one LA recipient.
func baseRequest() models.SendTimeRequest {
	return models.SendTimeRequest{
		Sender: &models.SendTimeParticipant{
			ID:       "user_123",
			Timezone: "America/Toronto",
		},
		Recipients: []models.SendTimeParticipant{
			{ID: "user_456", DisplayName: "John", Timezone: "America/Los_Angeles", Role: "primary"},
		},
		Message: &models.SendTimeMessage{Priority: "normal"},
		Constraints: &models.SendTimeConstraints{
			WorkingHoursOnly: boolp(true),
			AvoidWeekends:    boolp(true),
		},
		Options: &models.SendTimeOptions{
			ReferenceTime:         "2026-07-17T17:45:00-04:00",
			SuggestionCount:       intp(3),
			IncludeScoreBreakdown: boolp(true),
		},
	}
}

func TestValidate(t *testing.T) {
	svc := newTestService()

	if err := svc.Validate(baseRequest()); err != nil {
		t.Fatalf("expected valid request, got %+v", err)
	}

	noRecipients := baseRequest()
	noRecipients.Recipients = nil
	if err := svc.Validate(noRecipients); err == nil || err.Status != 400 {
		t.Errorf("expected 400 for missing recipients, got %+v", err)
	}

	badTZ := baseRequest()
	badTZ.Recipients[0].Timezone = "PST"
	if err := svc.Validate(badTZ); err == nil || err.Code != "INVALID_TIMEZONE" {
		t.Errorf("expected INVALID_TIMEZONE, got %+v", err)
	}

	badInterval := baseRequest()
	badInterval.Options.CandidateIntervalMinutes = intp(7)
	if err := svc.Validate(badInterval); err == nil || err.Code != "UNSUPPORTED_CANDIDATE_INTERVAL" {
		t.Errorf("expected UNSUPPORTED_CANDIDATE_INTERVAL, got %+v", err)
	}

	badWindow := baseRequest()
	badWindow.Constraints.EarliestSendAt = "2026-07-20T10:00:00-04:00"
	badWindow.Constraints.LatestSendAt = "2026-07-19T10:00:00-04:00"
	if err := svc.Validate(badWindow); err == nil || err.Code != "INVALID_SEARCH_WINDOW" {
		t.Errorf("expected INVALID_SEARCH_WINDOW, got %+v", err)
	}

	badCoverage := baseRequest()
	badCoverage.GroupPolicy = &models.SendTimeGroupPolicy{Strategy: "majority", MinimumRecipientCoverage: f64p(1.5)}
	if err := svc.Validate(badCoverage); err == nil || err.Code != "INVALID_RECIPIENT_COVERAGE" {
		t.Errorf("expected INVALID_RECIPIENT_COVERAGE, got %+v", err)
	}
}

func TestBasicSuggestionsWithinWorkingHoursAndNoWeekend(t *testing.T) {
	svc := newTestService()
	resp, err := svc.Suggest(baseRequest())
	if err != nil {
		t.Fatalf("Suggest error: %v", err)
	}
	if len(resp.Suggestions) == 0 {
		t.Fatal("expected at least one suggestion")
	}
	windowStart := mustParse(t, "2026-07-17T17:45:00-04:00")
	for i, s := range resp.Suggestions {
		if s.Rank != i+1 {
			t.Errorf("suggestion %d: rank = %d, want %d", i, s.Rank, i+1)
		}
		if s.Score < 0 || s.Score > 1 {
			t.Errorf("score out of range: %v", s.Score)
		}
		sendAt := mustParse(t, s.SendAt)
		if sendAt.Before(windowStart) {
			t.Errorf("suggestion before window start: %s", s.SendAt)
		}
		for _, rl := range s.RecipientLocalTimes {
			if rl.WithinWorkingHours == nil || !*rl.WithinWorkingHours {
				t.Errorf("recipient not within working hours: %+v", rl)
			}
			if rl.IsWeekend != nil && *rl.IsWeekend {
				t.Errorf("suggestion falls on a weekend: %+v", rl)
			}
		}
	}
	if resp.Policy.Version != "1.0.0" {
		t.Errorf("policy version = %q", resp.Policy.Version)
	}
}

func TestDeterminism(t *testing.T) {
	svc := newTestService()
	r1, err1 := svc.Suggest(baseRequest())
	r2, err2 := svc.Suggest(baseRequest())
	if err1 != nil || err2 != nil {
		t.Fatalf("errors: %v %v", err1, err2)
	}
	if len(r1.Suggestions) != len(r2.Suggestions) {
		t.Fatalf("non-deterministic count: %d vs %d", len(r1.Suggestions), len(r2.Suggestions))
	}
	for i := range r1.Suggestions {
		if r1.Suggestions[i].SendAt != r2.Suggestions[i].SendAt {
			t.Errorf("suggestion %d send_at differs: %s vs %s", i, r1.Suggestions[i].SendAt, r2.Suggestions[i].SendAt)
		}
		if r1.Suggestions[i].Score != r2.Suggestions[i].Score {
			t.Errorf("suggestion %d score differs: %v vs %v", i, r1.Suggestions[i].Score, r2.Suggestions[i].Score)
		}
	}
}

func TestMinimumDelayRespected(t *testing.T) {
	svc := newTestService()
	req := baseRequest()
	req.Constraints.MinimumDelaySeconds = i64p(3 * 24 * 3600) // 3 days
	resp, err := svc.Suggest(req)
	if err != nil {
		t.Fatalf("Suggest error: %v", err)
	}
	ref := mustParse(t, "2026-07-17T17:45:00-04:00")
	floor := ref.Add(3 * 24 * time.Hour)
	for _, s := range resp.Suggestions {
		if mustParse(t, s.SendAt).Before(floor) {
			t.Errorf("suggestion %s violates minimum delay (floor %s)", s.SendAt, floor.Format(time.RFC3339))
		}
	}
}

func TestCrossMidnightQuietHoursRejected(t *testing.T) {
	svc := newTestService()
	req := baseRequest()
	// Allow non-working candidates but enforce quiet hours crossing midnight.
	req.Constraints.WorkingHoursOnly = boolp(false)
	req.Constraints.RespectQuietHours = boolp(true)
	req.Recipients[0].QuietHours = &models.SendTimeQuietHours{Start: "21:00", End: "08:00"}
	resp, err := svc.Suggest(req)
	if err != nil {
		t.Fatalf("Suggest error: %v", err)
	}
	if len(resp.Suggestions) == 0 {
		t.Fatal("expected suggestions outside quiet hours")
	}
	for _, s := range resp.Suggestions {
		for _, rl := range s.RecipientLocalTimes {
			if rl.WithinQuietHours != nil && *rl.WithinQuietHours {
				t.Errorf("suggestion inside quiet hours: %s (%s)", s.SendAt, rl.Time)
			}
		}
	}
}

func TestDaylightSavingTransition(t *testing.T) {
	svc := newTestService()
	req := baseRequest()
	// Friday evening just before US spring-forward (2026-03-08). All working-hour
	// candidates land on/after Monday 2026-03-09, which is PDT (-07:00).
	req.Options.ReferenceTime = "2026-03-06T18:00:00-08:00"
	req.Sender.Timezone = "America/Los_Angeles"
	resp, err := svc.Suggest(req)
	if err != nil {
		t.Fatalf("Suggest error: %v", err)
	}
	if len(resp.Suggestions) == 0 {
		t.Fatal("expected suggestions after the DST transition")
	}
	foundPDT := false
	for _, s := range resp.Suggestions {
		sendAt := mustParse(t, s.SendAt)
		// Recipient local must remain within 09:00-17:00 despite the offset change.
		for _, rl := range s.RecipientLocalTimes {
			if rl.WithinWorkingHours == nil || !*rl.WithinWorkingHours {
				t.Errorf("post-DST suggestion outside working hours: %+v", rl)
			}
		}
		if _, offset := sendAt.In(mustLoadLA(t)).Zone(); offset == -7*3600 {
			foundPDT = true
		}
	}
	if !foundPDT {
		t.Error("expected at least one suggestion in PDT (-07:00) after the transition")
	}
}

func mustLoadLA(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatalf("load LA: %v", err)
	}
	return loc
}

func TestAllRecipientsRequiresBothValid(t *testing.T) {
	svc := newTestService()
	req := baseRequest()
	req.Recipients = []models.SendTimeParticipant{
		{ID: "a", Timezone: "America/Los_Angeles", Role: "primary"},
		{ID: "b", Timezone: "Europe/London", Role: "primary"},
	}
	req.GroupPolicy = &models.SendTimeGroupPolicy{Strategy: "all_recipients"}
	resp, err := svc.Suggest(req)
	if err != nil {
		t.Fatalf("Suggest error: %v", err)
	}
	for _, s := range resp.Suggestions {
		if s.Coverage.ValidRecipients != s.Coverage.TotalRecipients {
			t.Errorf("all_recipients suggestion has partial coverage: %+v", s.Coverage)
		}
		for _, rl := range s.RecipientLocalTimes {
			if rl.WithinWorkingHours == nil || !*rl.WithinWorkingHours {
				t.Errorf("recipient outside working hours under all_recipients: %+v", rl)
			}
		}
	}
}

func TestBestEffortReturnsSuggestions(t *testing.T) {
	svc := newTestService()
	req := baseRequest()
	req.Recipients = []models.SendTimeParticipant{
		{ID: "a", Timezone: "America/Los_Angeles", Role: "primary"},
		{ID: "b", Timezone: "Asia/Tokyo", Role: "secondary"},
	}
	req.GroupPolicy = &models.SendTimeGroupPolicy{Strategy: "best_effort"}
	resp, err := svc.Suggest(req)
	if err != nil {
		t.Fatalf("Suggest error: %v", err)
	}
	if len(resp.Suggestions) == 0 {
		t.Fatal("best_effort should still produce suggestions")
	}
}

func TestNoValidCandidates(t *testing.T) {
	svc := newTestService()
	req := baseRequest()
	// Window covers only Friday evening (after LA work) and the weekend.
	req.Constraints.EarliestSendAt = "2026-07-17T21:00:00-04:00"
	req.Constraints.LatestSendAt = "2026-07-19T23:00:00-04:00"
	resp, err := svc.Suggest(req)
	if err != nil {
		t.Fatalf("Suggest error: %v", err)
	}
	if len(resp.Suggestions) != 0 {
		t.Fatalf("expected no suggestions, got %d", len(resp.Suggestions))
	}
	if resp.NoSuggestion == nil || resp.NoSuggestion.Code != "NO_VALID_CANDIDATES" {
		t.Errorf("expected NO_VALID_CANDIDATES, got %+v", resp.NoSuggestion)
	}
}

func TestUrgentPrependsImmediateSend(t *testing.T) {
	svc := newTestService()
	req := baseRequest()
	req.Message.Priority = "urgent"
	resp, err := svc.Suggest(req)
	if err != nil {
		t.Fatalf("Suggest error: %v", err)
	}
	if len(resp.Suggestions) == 0 {
		t.Fatal("expected suggestions for urgent message")
	}
	first := resp.Suggestions[0]
	if !containsFlag(first.Flags, "IMMEDIATE_SEND_RECOMMENDED") {
		t.Errorf("urgent: first suggestion missing IMMEDIATE_SEND_RECOMMENDED, flags=%v", first.Flags)
	}
}

func TestWeekendReferenceDefersToWeekday(t *testing.T) {
	svc := newTestService()
	req := baseRequest()
	// Saturday reference; avoid_weekends should push suggestions to Monday+.
	req.Options.ReferenceTime = "2026-07-18T10:00:00-04:00"
	resp, err := svc.Suggest(req)
	if err != nil {
		t.Fatalf("Suggest error: %v", err)
	}
	if len(resp.Suggestions) == 0 {
		t.Fatal("expected weekday suggestions")
	}
	for _, s := range resp.Suggestions {
		for _, rl := range s.RecipientLocalTimes {
			if rl.Day == "SAT" || rl.Day == "SUN" {
				t.Errorf("suggestion on weekend: %+v", rl)
			}
		}
	}
}
