package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"scheduler0-private/pkg/models"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
)

// Intent classifier decisions returned by scheduler0-edge-classifier.
const (
	IntentDecisionAllow   = "allow"
	IntentDecisionClarify = "clarify"
	IntentDecisionReject  = "reject"
)

// IsEnglishLocale reports whether a BCP-47 locale string is English (e.g. "en",
// "en-US", "en_GB"). An empty locale is treated as English since the AI endpoints
// default to "en". The intent classifier is English-only, so this gates both the
// /ai/prompt/classify rejection and the /ai/prompt guardrail so they agree.
func IsEnglishLocale(locale string) bool {
	locale = strings.TrimSpace(strings.ToLower(locale))
	return locale == "" || strings.HasPrefix(locale, "en")
}

// IntentClassifier decides whether a natural-language prompt is a scheduling request, so the
// prompt service can skip (cost-saving) model calls for prompts that clearly are not.
type IntentClassifier interface {
	Classify(ctx context.Context, text string) (models.IntentClassification, error)
}

// httpIntentClassifier calls the scheduler0-edge-classifier FastAPI service.
type httpIntentClassifier struct {
	client  *http.Client
	logger  hclog.Logger
	baseURL string
}

// NewHTTPIntentClassifier returns a classifier client, or nil when no URL is configured
// (which disables the guardrail entirely — the service then runs every prompt).
func NewHTTPIntentClassifier(baseURL string, logger hclog.Logger) IntentClassifier {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil
	}
	return &httpIntentClassifier{
		// Short timeout: the guardrail must not add meaningful latency, and callers fail open.
		client:  &http.Client{Timeout: 3 * time.Second},
		logger:  logger.Named("intent-classifier"),
		baseURL: baseURL,
	}
}

// classifierResponse mirrors the JSON payload returned by the edge classifier.
type classifierResponse struct {
	Text     string `json:"text"`
	Decision string `json:"decision"`
	Reason   string `json:"reason"`
}

func (c *httpIntentClassifier) Classify(ctx context.Context, text string) (models.IntentClassification, error) {
	body, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		return models.IntentClassification{}, fmt.Errorf("intent classifier: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/intents/classify", bytes.NewReader(body))
	if err != nil {
		return models.IntentClassification{}, fmt.Errorf("intent classifier: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return models.IntentClassification{}, fmt.Errorf("intent classifier: request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return models.IntentClassification{}, fmt.Errorf("intent classifier: read response: %w", err)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return models.IntentClassification{}, fmt.Errorf("intent classifier: status %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var parsed classifierResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return models.IntentClassification{}, fmt.Errorf("intent classifier: unmarshal response: %w", err)
	}

	result := models.IntentClassification{
		Text:     parsed.Text,
		Decision: strings.ToLower(strings.TrimSpace(parsed.Decision)),
		Reason:   parsed.Reason,
	}

	return result, nil
}

// IntentSkippedError is returned when the guardrail skips model execution because the prompt
// is not a scheduling request. Callers can use errors.As to surface a friendly response.
type IntentSkippedError struct {
	Decision       string
	Reason         string
	Classification *models.IntentClassification
}

func (e *IntentSkippedError) Error() string {
	reason := e.Reason
	if reason == "" {
		reason = "the prompt does not look like a scheduling request"
	}
	return fmt.Sprintf("prompt skipped (%s): %s", e.Decision, reason)
}
