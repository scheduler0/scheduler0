package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"scheduler0-private/pkg/models"
	"strings"
)

// ExecutorCandidate is the minimal, non-secret view of an executor that is shown to the
// model when it selects the best match for a prompt.
type ExecutorCandidate struct {
	ID          uint64   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Type        string   `json:"type,omitempty"`
}

// ExecutorSelection is the parsed result of an executor-selection model call.
type ExecutorSelection struct {
	// ExecutorID is the chosen executor, or 0 when the model could not confidently match.
	ExecutorID uint64
	Reason     string
}

const executorSelectorSystemPrompt = `You are a routing assistant. Your only job is to map one scheduling request to the single executor that should run it.

You are given:
- A list of executors, each with an "id", "name", "description", "tags", and "type". Treat the description and tags as the source of truth for what an executor does; the name is only a hint.
- A request describing the work to schedule, including its purpose(s) and delivery channel(s).

Selection rules:
- Choose exactly ONE executor whose description and tags best fit the request.
- Weight the delivery channel most heavily (e.g. an "email" request should go to an executor whose tags/description cover email), then the purpose, then any other signals in the request text.
- You MUST pick an "id" that appears verbatim in the provided list. Never invent, guess, or modify an id.
- Be conservative: if no executor is a clearly appropriate match, or two or more are equally plausible with no way to decide, return null rather than guessing. A wrong routing is worse than abstaining.

Output:
- Respond with a single strict JSON object and nothing else — no prose, no markdown, no code fences.
- Shape: {"executorId": <number or null>, "reason": "<one short sentence, <=160 chars>"}.
- Set "executorId" to null when you abstain, and use "reason" to briefly say why (e.g. "no executor covers SMS" or "email tag matches two executors equally").`

// SelectExecutorWithSettings asks the account's configured model (BYOK, falling back to the
// global config) to pick the executor that best matches the prompt's purpose and channels.
// It returns ExecutorID == 0 when the model abstains or returns an id that is not among the
// candidates. An error is only returned when the model call itself fails; callers treat both
// an error and a zero id as "no confident match" and apply their fallback rule.
func (s *PromptService) SelectExecutorWithSettings(ctx context.Context, settings *models.AccountAISettings, prompt string, purposes []string, channels []string, candidates []ExecutorCandidate) (ExecutorSelection, error) {
	if len(candidates) == 0 {
		return ExecutorSelection{}, nil
	}

	executors := s.resolveExecutors(settings)
	if len(executors) == 0 {
		return ExecutorSelection{}, fmt.Errorf("no model executors configured for executor selection")
	}

	candidatesJSON, err := json.Marshal(candidates)
	if err != nil {
		return ExecutorSelection{}, fmt.Errorf("failed to marshal executor candidates: %w", err)
	}

	userPrompt := buildExecutorSelectionUserPrompt(prompt, purposes, channels, string(candidatesJSON))

	// Try executors in order (primary first, then fallbacks) so BYOK failover still applies.
	var lastErr error
	validIDs := make(map[uint64]bool, len(candidates))
	for _, c := range candidates {
		validIDs[c.ID] = true
	}

	for _, executor := range executors {
		result, execErr := executor.Complete(ctx, executorSelectorSystemPrompt, userPrompt)
		if execErr != nil {
			lastErr = execErr
			s.logger.Warn("executor selection model call failed, trying next", "provider", executor.ProviderName(), "error", execErr)
			continue
		}
		selection := parseExecutorSelection(result.Text)
		if selection.ExecutorID != 0 && !validIDs[selection.ExecutorID] {
			// Model hallucinated an id that is not owned by the account; treat as abstain.
			s.logger.Warn("executor selection returned unknown id, treating as no match", "id", selection.ExecutorID)
			selection.ExecutorID = 0
		}
		return selection, nil
	}

	return ExecutorSelection{}, lastErr
}

func buildExecutorSelectionUserPrompt(prompt string, purposes []string, channels []string, candidatesJSON string) string {
	var b strings.Builder
	b.WriteString("Request:\n")
	b.WriteString(strings.TrimSpace(prompt))
	b.WriteString("\n\n")
	if len(purposes) > 0 {
		b.WriteString("Purposes: ")
		b.WriteString(strings.Join(purposes, ", "))
		b.WriteString("\n")
	}
	if len(channels) > 0 {
		b.WriteString("Channels: ")
		b.WriteString(strings.Join(channels, ", "))
		b.WriteString("\n")
	}
	b.WriteString("\nExecutors:\n")
	b.WriteString(candidatesJSON)
	return b.String()
}

var executorSelectionObjectRe = regexp.MustCompile(`(?s)\{.*\}`)

// parseExecutorSelection tolerantly extracts {"executorId": <n|null>, "reason": "..."} from
// the model's text output. Malformed or absent ids yield a zero ExecutorID (abstain).
func parseExecutorSelection(text string) ExecutorSelection {
	text = strings.TrimSpace(text)
	stripFences := func(v string) string {
		v = strings.TrimSpace(v)
		v = strings.TrimPrefix(v, "```json")
		v = strings.TrimPrefix(v, "```JSON")
		v = strings.TrimPrefix(v, "```")
		v = strings.TrimSuffix(v, "```")
		return strings.TrimSpace(v)
	}
	clean := stripFences(text)

	var parsed struct {
		ExecutorID *uint64 `json:"executorId"`
		Reason     string  `json:"reason"`
	}
	if err := json.Unmarshal([]byte(clean), &parsed); err != nil {
		// Last resort: pull the first {...} object out of a larger blob.
		if m := executorSelectionObjectRe.FindString(clean); m != "" {
			_ = json.Unmarshal([]byte(m), &parsed)
		}
	}

	selection := ExecutorSelection{Reason: strings.TrimSpace(parsed.Reason)}
	if parsed.ExecutorID != nil {
		selection.ExecutorID = *parsed.ExecutorID
	}
	return selection
}
