package aischedule

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"scheduler0-private/pkg/models"
	executor_repo "scheduler0-private/pkg/repository/executor"
	"scheduler0-private/pkg/service/ai"
	"scheduler0-private/pkg/service/job"
	"scheduler0-private/pkg/service/project"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
)

// ScheduleError is returned by ScheduleFromPrompt for domain failures that map to a specific
// HTTP status (e.g. no executors -> 409). The controller inspects it with errors.As.
type ScheduleError struct {
	Status  int
	Message string
}

func (e *ScheduleError) Error() string { return e.Message }

func newScheduleError(status int, message string) *ScheduleError {
	return &ScheduleError{Status: status, Message: message}
}

// ScheduleService turns a natural-language prompt into scheduled jobs: it runs the prompt
// pipeline (guardrail + generation), resolves or creates a project, picks the executor whose
// description/tags best match the prompt, and synchronously creates the jobs.
type ScheduleService interface {
	ScheduleFromPrompt(ctx context.Context, requestId string, accountId uint64, settings *models.AccountAISettings, req models.SchedulePromptRequest) (*models.SchedulePromptResult, error)
}

type scheduleService struct {
	promptService   *ai.PromptService
	projectService  project.ProjectService
	jobService      job.JobService
	jobExecutorRepo executor_repo.JobExecutorRepo
	logger          hclog.Logger
}

func NewScheduleService(logger hclog.Logger, promptService *ai.PromptService, projectService project.ProjectService, jobService job.JobService, jobExecutorRepo executor_repo.JobExecutorRepo) ScheduleService {
	return &scheduleService{
		promptService:   promptService,
		projectService:  projectService,
		jobService:      jobService,
		jobExecutorRepo: jobExecutorRepo,
		logger:          logger.Named("ai-schedule-service"),
	}
}

// maxExecutorsForSelection caps how many executors are listed and shown to the model.
const maxExecutorsForSelection = 100

func (s *scheduleService) ScheduleFromPrompt(ctx context.Context, requestId string, accountId uint64, settings *models.AccountAISettings, req models.SchedulePromptRequest) (*models.SchedulePromptResult, error) {
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return nil, newScheduleError(http.StatusBadRequest, "prompt is required")
	}
	createdBy := strings.TrimSpace(req.CreatedBy)
	if createdBy == "" {
		return nil, newScheduleError(http.StatusBadRequest, "createdBy is required")
	}
	timezone := strings.TrimSpace(req.Timezone)
	if timezone == "" {
		timezone = "UTC"
	}
	if _, err := time.LoadLocation(timezone); err != nil {
		return nil, newScheduleError(http.StatusBadRequest, fmt.Sprintf("invalid timezone %q: %s", timezone, err.Error()))
	}
	locale := strings.TrimSpace(req.Locale)
	if locale == "" {
		locale = "en"
	}

	// 1. Run the prompt pipeline. This applies the intent guardrail (which returns an
	// *ai.IntentSkippedError the controller maps to 422) and generates job configs.
	providers, classification, _, _, err := s.promptService.CreateJobFromPromptWithSettings(
		ctx, accountId, settings, prompt, req.Purposes, req.Events, req.Recipients, req.Channels, timezone, locale,
	)
	if err != nil {
		return nil, err
	}

	var promptJobs []models.PromptJobResponse
	var provider, model string
	for _, p := range providers {
		if len(p.Jobs) > 0 {
			promptJobs = p.Jobs
			provider = p.Provider
			model = p.Model
			break
		}
	}
	if len(promptJobs) == 0 {
		return nil, newScheduleError(http.StatusConflict, "the prompt did not produce any schedulable jobs")
	}

	// 2. Resolve the executor (pinned id, the account's only executor, or LLM match).
	executor, matchedBy, matchReason, execErr := s.resolveExecutor(ctx, accountId, settings, req, prompt)
	if execErr != nil {
		return nil, execErr
	}

	// 3. Resolve the project (by id, by name create-or-reuse, or derived from the prompt).
	// Done after executor resolution so we don't create a project we then can't schedule to.
	proj, projectCreated, projErr := s.resolveProject(accountId, createdBy, req, prompt)
	if projErr != nil {
		return nil, projErr
	}

	// 4. Map the generated job configs onto Scheduler0 jobs.
	jobs, mapErr := buildJobs(promptJobs, proj.ID, executor.ID, accountId, createdBy, timezone)
	if mapErr != nil {
		return nil, mapErr
	}

	// 5. Create the jobs synchronously so we can return them in the response.
	createdJobs, createErr := s.jobService.BatchInsertJobsSync(requestId, jobs)
	if createErr != nil {
		return nil, newScheduleError(createErr.Type, createErr.Message)
	}

	return &models.SchedulePromptResult{
		Classification:      classification,
		Project:             *proj,
		ProjectCreated:      projectCreated,
		Executor:            *executor,
		ExecutorMatchedBy:   matchedBy,
		ExecutorMatchReason: matchReason,
		Jobs:                createdJobs,
		Provider:            provider,
		Model:               model,
	}, nil
}

// resolveExecutor returns the executor to schedule to, the strategy used (pinned|only|llm),
// and (for llm) the model's rationale.
func (s *scheduleService) resolveExecutor(ctx context.Context, accountId uint64, settings *models.AccountAISettings, req models.SchedulePromptRequest, prompt string) (*models.JobExecutor, string, string, error) {
	// Pinned executor: validate ownership and use it directly.
	if req.ExecutorId != nil {
		executor, getErr := s.jobExecutorRepo.GetOneByID(*req.ExecutorId, accountId)
		if getErr != nil {
			return nil, "", "", newScheduleError(getErr.Type, getErr.Message)
		}
		if executor == nil || executor.ID == 0 {
			return nil, "", "", newScheduleError(http.StatusNotFound, fmt.Sprintf("executor %d not found", *req.ExecutorId))
		}
		return executor, models.ExecutorMatchedByPinned, "", nil
	}

	executors, listErr := s.jobExecutorRepo.List(0, maxExecutorsForSelection, "id", "asc", accountId)
	if listErr != nil {
		return nil, "", "", newScheduleError(listErr.Type, listErr.Message)
	}
	switch len(executors) {
	case 0:
		return nil, "", "", newScheduleError(http.StatusConflict, "no executors exist for this account; create an executor before scheduling")
	case 1:
		e := executors[0]
		return &e, models.ExecutorMatchedByOnly, "", nil
	}

	// Multiple executors: ask the model to pick the best match by description/tags.
	candidates := make([]ai.ExecutorCandidate, 0, len(executors))
	for _, e := range executors {
		candidates = append(candidates, ai.ExecutorCandidate{
			ID:          e.ID,
			Name:        e.Name,
			Description: e.Description,
			Tags:        e.Tags,
			Type:        e.Type,
		})
	}

	selection, selErr := s.promptService.SelectExecutorWithSettings(ctx, settings, prompt, req.Purposes, req.Channels, candidates)
	if selErr != nil {
		s.logger.Warn("executor selection failed", "accountId", accountId, "error", selErr)
		return nil, "", "", newScheduleError(http.StatusConflict, "could not match an executor to the prompt; pin an executorId or refine executor descriptions/tags")
	}
	if selection.ExecutorID == 0 {
		return nil, "", "", newScheduleError(http.StatusConflict, "could not match an executor to the prompt; pin an executorId or refine executor descriptions/tags")
	}
	for i := range executors {
		if executors[i].ID == selection.ExecutorID {
			return &executors[i], models.ExecutorMatchedByLLM, selection.Reason, nil
		}
	}
	// Should not happen (SelectExecutorWithSettings validates ownership), but guard anyway.
	return nil, "", "", newScheduleError(http.StatusConflict, "could not match an executor to the prompt; pin an executorId or refine executor descriptions/tags")
}

// resolveProject returns the project to schedule to and whether it was newly created.
func (s *scheduleService) resolveProject(accountId uint64, createdBy string, req models.SchedulePromptRequest, prompt string) (*models.Project, bool, error) {
	// Explicit project id: fetch and use.
	if req.ProjectId != nil {
		proj := &models.Project{ID: *req.ProjectId, AccountId: accountId}
		if getErr := s.projectService.GetOneByID(proj); getErr != nil {
			return nil, false, newScheduleError(getErr.Type, getErr.Message)
		}
		if proj.ID == 0 {
			return nil, false, newScheduleError(http.StatusNotFound, fmt.Sprintf("project %d not found", *req.ProjectId))
		}
		return proj, false, nil
	}

	// Otherwise create-or-reuse by name, deriving the name/description from the request or prompt.
	name, description := deriveProject(req, prompt)

	existing := &models.Project{Name: name, AccountId: accountId}
	if getErr := s.projectService.GetOneByName(existing); getErr == nil && existing.ID > 0 {
		return existing, false, nil
	}

	created, createErr := s.projectService.CreateOne(models.Project{
		Name:        name,
		Description: description,
		AccountId:   accountId,
		CreatedBy:   createdBy,
	})
	if createErr != nil {
		return nil, false, newScheduleError(createErr.Type, createErr.Message)
	}
	return created, true, nil
}

// deriveProject computes a project name and description from the request's project hint or,
// failing that, the prompt itself. Both are guaranteed non-empty (the repo requires them).
func deriveProject(req models.SchedulePromptRequest, prompt string) (string, string) {
	name := ""
	description := ""
	if req.Project != nil {
		name = strings.TrimSpace(req.Project.Name)
		description = strings.TrimSpace(req.Project.Description)
	}
	if name == "" {
		if len(req.Purposes) > 0 && strings.TrimSpace(req.Purposes[0]) != "" {
			name = truncate(strings.TrimSpace(req.Purposes[0]), 60)
		} else {
			name = truncate(prompt, 60)
		}
	}
	if name == "" {
		name = "AI Scheduled"
	}
	if description == "" {
		description = truncate(prompt, 240)
	}
	if description == "" {
		description = "Created by the AI schedule endpoint."
	}
	return name, description
}

func truncate(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return strings.TrimSpace(s[:max])
}

// buildJobs maps generated prompt job configs onto Scheduler0 jobs bound to the resolved
// project and executor. A recurrence of "none" produces a one-time job (empty spec + start
// date) instead of a point-in-time cron that would otherwise recur annually.
func buildJobs(promptJobs []models.PromptJobResponse, projectID, executorID, accountID uint64, createdBy, requestTimezone string) ([]models.Job, error) {
	jobs := make([]models.Job, 0, len(promptJobs))
	for _, pj := range promptJobs {
		timezone := strings.TrimSpace(pj.Timezone)
		if timezone == "" {
			timezone = requestTimezone
		}
		if _, err := time.LoadLocation(timezone); err != nil {
			return nil, newScheduleError(http.StatusBadRequest, fmt.Sprintf("invalid timezone %q in generated job: %s", timezone, err.Error()))
		}

		execID := executorID
		job := models.Job{
			ProjectID:      projectID,
			ExecutorId:     &execID,
			AccountId:      accountID,
			CreatedBy:      createdBy,
			Timezone:       timezone,
			TimezoneOffset: timezoneOffsetSeconds(timezone),
			Status:         models.JobStatusActive,
		}

		oneTime := pj.Recurrence == models.PromptJobResponseRecurrenceNone
		if oneTime {
			// One-time job: no cron spec, run once at the computed time.
			if start := firstNonZeroTime(pj.StartDate, pj.NextRunAt); start != nil {
				job.StartDate = *start
			}
		} else {
			job.Spec = strings.TrimSpace(pj.CronExpression)
			if pj.StartDate != nil {
				job.StartDate = *pj.StartDate
			}
			if pj.EndDate != nil {
				job.EndDate = *pj.EndDate
			}
		}

		data, dataErr := buildJobData(pj)
		if dataErr != nil {
			return nil, newScheduleError(http.StatusInternalServerError, fmt.Sprintf("failed to serialize job data: %s", dataErr.Error()))
		}
		job.Data = data

		jobs = append(jobs, job)
	}
	return jobs, nil
}

// buildJobData serializes the semantic fields of a generated job into the job's Data JSON so
// the executor payload carries the intent (purpose, subject, channel, recipients, etc.).
func buildJobData(pj models.PromptJobResponse) (string, error) {
	payload := map[string]any{}
	if pj.Kind != "" {
		payload["kind"] = pj.Kind
	}
	if pj.Purpose != "" {
		payload["purpose"] = pj.Purpose
	}
	if pj.Subject != "" {
		payload["subject"] = pj.Subject
	}
	if pj.Event != "" {
		payload["event"] = pj.Event
	}
	if pj.Delivery != "" {
		payload["delivery"] = pj.Delivery
	}
	if pj.Channel != "" {
		payload["channel"] = pj.Channel
	}
	if len(pj.Recipients) > 0 {
		payload["recipients"] = pj.Recipients
	}
	if pj.Metadata != nil {
		payload["metadata"] = *pj.Metadata
	}
	if len(payload) == 0 {
		return "", nil
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func firstNonZeroTime(times ...*time.Time) *time.Time {
	for _, t := range times {
		if t != nil && !t.IsZero() {
			return t
		}
	}
	return nil
}

// timezoneOffsetSeconds returns the current UTC offset (in seconds) for the given IANA zone.
// It is informational on the job record; scheduling itself uses the timezone name.
func timezoneOffsetSeconds(tz string) int64 {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return 0
	}
	_, offset := time.Now().In(loc).Zone()
	return int64(offset)
}
