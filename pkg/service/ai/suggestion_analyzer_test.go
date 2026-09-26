package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"scheduler0/pkg/models"

	"github.com/hashicorp/go-hclog"
)

func TestNewHTTPSuggestionAnalyzer_DisabledWhenNoURL(t *testing.T) {
	if NewHTTPSuggestionAnalyzer("", hclog.NewNullLogger()) != nil {
		t.Error("analyzer should be nil (disabled) when no URL is configured")
	}
	if NewHTTPSuggestionAnalyzer("http://localhost:9000", hclog.NewNullLogger()) == nil {
		t.Error("analyzer should be constructed when a URL is provided")
	}
}

func TestSuggestionAnalyzer_AnalyzeRoundTrip(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/suggestions/analyze" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		// Echo a minimal, realistic analyzer response including an extra field
		// that must be ignored on the Go side.
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"request_id": "req_abc",
			"conversation_id": "conv_1",
			"analyzed_at": "2026-07-17T10:02:01-04:00",
			"suggestions": [{"id":"sug_001","type":"COMMITMENT","status":"OPEN","confidence":0.92}],
			"obligations": [{"id":"obl_001","status":"OPEN","suggestion_id":"sug_001"}],
			"warnings": [],
			"engine": {"engine_version":"1.0.0"},
			"unexpected_extra_field": true
		}`))
	}))
	defer server.Close()

	analyzer := NewHTTPSuggestionAnalyzer(server.URL, hclog.NewNullLogger())
	if analyzer == nil {
		t.Fatal("expected a configured analyzer")
	}

	speaker, _ := json.Marshal(map[string]string{"id": "u1", "display_name": "Victor"})
	req := models.SuggestionAnalyzeRequest{
		ConversationID: "conv_1",
		Messages: []models.SuggestionMessage{
			{ID: "m1", Speaker: speaker, Timestamp: "2026-07-17T10:00:00-04:00", Message: "I'll send the proposal tomorrow."},
		},
	}

	result, err := analyzer.Analyze(context.Background(), req)
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	if result.RequestID != "req_abc" {
		t.Errorf("request_id = %q, want req_abc", result.RequestID)
	}
	if len(result.Suggestions) != 1 || result.Suggestions[0]["type"] != "COMMITMENT" {
		t.Errorf("unexpected suggestions: %+v", result.Suggestions)
	}
	if len(result.Obligations) != 1 || result.Obligations[0]["status"] != "OPEN" {
		t.Errorf("unexpected obligations: %+v", result.Obligations)
	}
}

func TestSuggestionAnalyzer_ErrorOnBadStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"detail":{"code":"UNSUPPORTED_LOCALE"}}`))
	}))
	defer server.Close()

	analyzer := NewHTTPSuggestionAnalyzer(server.URL, hclog.NewNullLogger())
	_, err := analyzer.Analyze(context.Background(), models.SuggestionAnalyzeRequest{
		Messages: []models.SuggestionMessage{{Timestamp: "2026-07-17T10:00:00-04:00", Message: "hi"}},
	})
	if err == nil {
		t.Fatal("expected an error for a 400 response")
	}
}
