package ai

import (
	"strings"
	"testing"
)

func TestGenerateSystemPrompt_DefaultsLocaleToEnglish(t *testing.T) {
	prompt := GenerateSystemPrompt(SystemPromptConfig{})
	if !strings.Contains(prompt, "Locale: en.") {
		t.Fatalf("expected prompt to default locale to en, got: %s", prompt)
	}
}

func TestGenerateSystemPrompt_IncludesGivenLocale(t *testing.T) {
	prompt := GenerateSystemPrompt(SystemPromptConfig{Locale: "es-ES"})
	if !strings.Contains(prompt, "Locale: es-ES.") {
		t.Fatalf("expected prompt to include the given locale, got: %s", prompt)
	}
}
