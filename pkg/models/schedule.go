package models

// Schedule DTOs for POST /api/v1/ai/schedule. This endpoint combines the prompt
// pipeline (natural-language -> job configs + intent guardrail) with project and
// job creation: it resolves/creates a project, picks the executor whose
// description/tags best match the prompt, and creates the jobs synchronously.

// ScheduleProjectInput lets the caller create-or-reuse a project by name. When
// ProjectId is supplied on the request instead, this is ignored.
type ScheduleProjectInput struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

// SchedulePromptRequest is the request body for POST /api/v1/ai/schedule. It carries
// the same natural-language hints as POST /api/v1/ai/prompt plus the target project
// and executor selection.
type SchedulePromptRequest struct {
	Prompt     string   `json:"prompt,omitempty"`
	Purposes   []string `json:"purposes,omitempty"`
	Events     []string `json:"events,omitempty"`
	Recipients []string `json:"recipients,omitempty"`
	Channels   []string `json:"channels,omitempty"`
	Timezone   string   `json:"timezone,omitempty"`
	Locale     string   `json:"locale,omitempty"`
	// ProjectId reuses an existing project. Takes precedence over Project.
	ProjectId *uint64 `json:"projectId,omitempty"`
	// Project creates-or-reuses a project by name. Used when ProjectId is absent.
	Project *ScheduleProjectInput `json:"project,omitempty"`
	// ExecutorId pins a specific executor and skips LLM matching.
	ExecutorId *uint64 `json:"executorId,omitempty"`
	// CreatedBy is required; it is stamped on the created project and jobs.
	CreatedBy string `json:"createdBy,omitempty"`
}

// Executor match strategies reported back to the caller.
const (
	ExecutorMatchedByPinned = "pinned"
	ExecutorMatchedByOnly   = "only"
	ExecutorMatchedByLLM    = "llm"
)

// SchedulePromptResult is the response for POST /api/v1/ai/schedule.
type SchedulePromptResult struct {
	Classification *IntentClassification `json:"classification,omitempty"`
	Project        Project               `json:"project"`
	ProjectCreated bool                  `json:"projectCreated"`
	Executor       JobExecutor           `json:"executor"`
	// ExecutorMatchedBy is one of pinned|only|llm.
	ExecutorMatchedBy string `json:"executorMatchedBy"`
	// ExecutorMatchReason is the model's rationale when matched by llm.
	ExecutorMatchReason string `json:"executorMatchReason,omitempty"`
	Jobs                []Job  `json:"jobs"`
	Provider            string `json:"provider,omitempty"`
	Model               string `json:"model,omitempty"`
}
