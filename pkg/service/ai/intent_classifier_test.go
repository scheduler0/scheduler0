package ai

import (
	"context"
	"errors"
	"testing"

	"scheduler0/pkg/models"

	"github.com/hashicorp/go-hclog"
)

// stubClassifier returns a fixed decision/error for testing the guardrail wiring.
type stubClassifier struct {
	classification models.IntentClassification
	err            error
	calls          int
}

func (s *stubClassifier) Classify(_ context.Context, _ string) (models.IntentClassification, error) {
	s.calls++
	return s.classification, s.err
}

// recordingRecorder captures persisted prompt-request rows.
type recordingRecorder struct {
	rows []models.AccountPromptRequest
}

func (r *recordingRecorder) Record(row models.AccountPromptRequest) error {
	r.rows = append(r.rows, row)
	return nil
}

func newTestPromptService(classifier IntentClassifier, rec PromptRequestRecorder) *PromptService {
	return &PromptService{
		globalExecutors: nil, // never reached when the guardrail rejects
		classifier:      classifier,
		recorder:        rec,
		logger:          hclog.NewNullLogger(),
	}
}

func TestGuardrail_RejectSkipsExecution(t *testing.T) {
	rec := &recordingRecorder{}
	c := &stubClassifier{classification: models.IntentClassification{Decision: IntentDecisionReject, Reason: "not_a_schedule_request"}}
	s := newTestPromptService(c, rec)

	_, _, _, _, err := s.CreateJobFromPrompt(context.Background(), 7, "What is Kubernetes?", nil, nil, nil, nil, "", "")

	var skipped *IntentSkippedError
	if !errors.As(err, &skipped) {
		t.Fatalf("expected IntentSkippedError, got %v", err)
	}
	if c.calls != 1 {
		t.Errorf("classifier should be called once, got %d", c.calls)
	}
	if len(rec.rows) != 1 || rec.rows[0].Status != models.PromptRequestStatusSkippedIntent {
		t.Fatalf("expected one skipped_intent row, got %+v", rec.rows)
	}
	if rec.rows[0].EstimatedCostUSD != 0 {
		t.Errorf("skipped request should cost 0, got %v", rec.rows[0].EstimatedCostUSD)
	}
}

func TestGuardrail_ClarifySkipsExecution(t *testing.T) {
	rec := &recordingRecorder{}
	c := &stubClassifier{classification: models.IntentClassification{Decision: IntentDecisionClarify, Reason: "temporal_signal_without_clear_request"}}
	s := newTestPromptService(c, rec)

	_, _, _, _, err := s.CreateJobFromPrompt(context.Background(), 7, "Next Friday at 9am.", nil, nil, nil, nil, "", "")

	var skipped *IntentSkippedError
	if !errors.As(err, &skipped) {
		t.Fatalf("expected IntentSkippedError, got %v", err)
	}
	if len(rec.rows) != 1 {
		t.Fatalf("expected a skipped row, got %+v", rec.rows)
	}
}

func TestGuardrail_FailOpenOnClassifierError(t *testing.T) {
	rec := &recordingRecorder{}
	c := &stubClassifier{err: errors.New("classifier unreachable")}
	// No executors configured: after failing open, execution proceeds and then fails with the
	// "no prompt executors configured" path — importantly NOT an IntentSkippedError.
	s := newTestPromptService(c, rec)

	_, _, _, _, err := s.CreateJobFromPrompt(context.Background(), 7, "Remind me every Monday at 9am.", nil, nil, nil, nil, "", "")

	var skipped *IntentSkippedError
	if errors.As(err, &skipped) {
		t.Fatalf("fail-open path must not skip on classifier error")
	}
	if c.calls != 1 {
		t.Errorf("classifier should have been called once, got %d", c.calls)
	}
	// It proceeded past the guardrail (no skipped_intent row recorded).
	for _, row := range rec.rows {
		if row.Status == models.PromptRequestStatusSkippedIntent {
			t.Errorf("should not record skipped_intent when failing open")
		}
	}
}

func TestGuardrail_SkippedForNonEnglishLocale(t *testing.T) {
	rec := &recordingRecorder{}
	// A classifier that would reject if consulted — it must NOT be called for a non-en locale.
	c := &stubClassifier{classification: models.IntentClassification{Decision: IntentDecisionReject, Reason: "not_a_schedule_request"}}
	s := newTestPromptService(c, rec)

	_, _, _, _, err := s.CreateJobFromPrompt(context.Background(), 7, "recuérdame mañana a las 9", nil, nil, nil, nil, "", "es-ES")

	var skipped *IntentSkippedError
	if errors.As(err, &skipped) {
		t.Fatalf("non-English locale must bypass the intent guardrail, got IntentSkippedError")
	}
	if c.calls != 0 {
		t.Errorf("classifier must not be called for non-English locale, got %d calls", c.calls)
	}
	for _, row := range rec.rows {
		if row.Status == models.PromptRequestStatusSkippedIntent {
			t.Errorf("should not record skipped_intent for a non-English locale")
		}
	}
}

func TestIsEnglishLocale(t *testing.T) {
	for _, tc := range []struct {
		locale string
		want   bool
	}{
		{"", true}, {"en", true}, {"en-US", true}, {"EN_GB", true}, {" en ", true},
		{"es", false}, {"es-ES", false}, {"fr", false}, {"de-DE", false},
	} {
		if got := IsEnglishLocale(tc.locale); got != tc.want {
			t.Errorf("IsEnglishLocale(%q) = %v, want %v", tc.locale, got, tc.want)
		}
	}
}

func TestNewHTTPIntentClassifier_DisabledWhenNoURL(t *testing.T) {
	if NewHTTPIntentClassifier("", hclog.NewNullLogger()) != nil {
		t.Error("classifier should be nil (disabled) when no URL is configured")
	}
	if NewHTTPIntentClassifier("http://localhost:9000", hclog.NewNullLogger()) == nil {
		t.Error("classifier should be constructed when a URL is provided")
	}
}
