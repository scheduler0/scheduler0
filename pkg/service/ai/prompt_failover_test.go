package ai

import (
	"context"
	"errors"
	"testing"

	"github.com/hashicorp/go-hclog"
)

// mockExecutor is a simple test double for ModelExecutor.
type mockExecutor struct {
	providerName string
	modelName    string
	result       *ExecutionResult
	err          error
	callCount    int
}

func (m *mockExecutor) ProviderName() string { return m.providerName }
func (m *mockExecutor) ModelName() string    { return m.modelName }
func (m *mockExecutor) ExecutePrompt(_ context.Context, _ SystemPromptConfig, _ string) (*ExecutionResult, error) {
	m.callCount++
	return m.result, m.err
}
func (m *mockExecutor) Complete(_ context.Context, _ string, _ string) (*ExecutionResult, error) {
	m.callCount++
	return m.result, m.err
}

// validResultJSON is a minimal JSON response matching promptJobResponsesSchema.
const validResultJSON = `{"jobs":[{"kind":"REMINDER","purpose":"test","subject":"test","nextRunAt":"2024-01-17T14:00:00-05:00","recurrence":"none","event":"test","delivery":"email","channel":"test","timezone":"America/New_York","recipients":["user@test.com"],"startDate":null,"endDate":null}]}`

func newSuccessExecutor(name string) *mockExecutor {
	return &mockExecutor{
		providerName: name,
		modelName:    "test-model",
		result:       &ExecutionResult{Text: validResultJSON},
	}
}

func newFailExecutor(name string) *mockExecutor {
	return &mockExecutor{
		providerName: name,
		modelName:    "test-model",
		err:          errors.New("simulated executor failure"),
	}
}

// buildTestPromptService returns a PromptService backed by the given executors.
func buildTestPromptService(executors []ModelExecutor) *PromptService {
	return &PromptService{
		globalExecutors: executors,
		globalCfg:       nil,
		logger:          hclog.NewNullLogger(),
	}
}

func TestFailoverMode_StopsAtFirstSuccess(t *testing.T) {
	primary := newFailExecutor("primary")
	fallback1 := newSuccessExecutor("fallback1")
	fallback2 := newSuccessExecutor("fallback2")

	svc := buildTestPromptService(nil)
	executors := []ModelExecutor{primary, fallback1, fallback2}

	_, _, _, _, err := svc.createJobFromPromptWithExecutors(
		context.Background(), 1, executors, true,
		"remind me tomorrow", nil, nil, nil, nil, "UTC", "en",
	)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if primary.callCount != 1 {
		t.Errorf("primary callCount = %d, want 1", primary.callCount)
	}
	if fallback1.callCount != 1 {
		t.Errorf("fallback1 callCount = %d, want 1", fallback1.callCount)
	}
	// fallback2 must NOT be called because fallback1 succeeded.
	if fallback2.callCount != 0 {
		t.Errorf("fallback2 callCount = %d, want 0 (should not be called after fallback1 succeeded)", fallback2.callCount)
	}
}

func TestFailoverMode_AllFail_ReturnsError(t *testing.T) {
	primary := newFailExecutor("primary")
	fallback := newFailExecutor("fallback")

	svc := buildTestPromptService(nil)
	executors := []ModelExecutor{primary, fallback}

	_, _, _, _, err := svc.createJobFromPromptWithExecutors(
		context.Background(), 1, executors, true,
		"remind me tomorrow", nil, nil, nil, nil, "UTC", "en",
	)
	if err == nil {
		t.Fatal("expected error when all executors fail")
	}
	if primary.callCount != 1 {
		t.Errorf("primary callCount = %d, want 1", primary.callCount)
	}
	if fallback.callCount != 1 {
		t.Errorf("fallback callCount = %d, want 1", fallback.callCount)
	}
}

func TestFanoutMode_TriesAllExecutors(t *testing.T) {
	exec1 := newSuccessExecutor("exec1")
	exec2 := newSuccessExecutor("exec2")

	svc := buildTestPromptService(nil)
	executors := []ModelExecutor{exec1, exec2}

	_, _, _, _, err := svc.createJobFromPromptWithExecutors(
		context.Background(), 1, executors, false,
		"remind me tomorrow", nil, nil, nil, nil, "UTC", "en",
	)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if exec1.callCount != 1 {
		t.Errorf("exec1 callCount = %d, want 1", exec1.callCount)
	}
	if exec2.callCount != 1 {
		t.Errorf("exec2 callCount = %d, want 1 (fan-out should call all executors)", exec2.callCount)
	}
}
