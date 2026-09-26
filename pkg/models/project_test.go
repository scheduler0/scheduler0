package models

import (
	"encoding/json"
	"testing"
	"time"
)

func TestProjectToJSON(t *testing.T) {
	project := Project{
		ID:          1,
		Name:        "Test Project",
		Description: "Test Description",
		AccountId:   1,
		DateCreated: time.Now(),
	}

	jsonData, err := project.ToJSON()
	if err != nil {
		t.Errorf("Expected ToJSON to succeed, got error: %v", err)
	}

	if len(jsonData) == 0 {
		t.Errorf("Expected ToJSON to return non-empty JSON data")
	}

	// Verify it's valid JSON
	var decodedProject Project
	if err := json.Unmarshal(jsonData, &decodedProject); err != nil {
		t.Errorf("Expected ToJSON to return valid JSON, got error: %v", err)
	}

	if decodedProject.ID != project.ID {
		t.Errorf("Expected decoded project ID to be %d, got %d", project.ID, decodedProject.ID)
	}
}

func TestProjectFromJSON(t *testing.T) {
	project := Project{
		ID:          1,
		Name:        "Test Project",
		Description: "Test Description",
		AccountId:   100,
		DateCreated: time.Now(),
	}

	jsonData, _ := json.Marshal(project)

	var decodedProject Project
	err := decodedProject.FromJSON(jsonData)
	if err != nil {
		t.Errorf("Expected FromJSON to succeed, got error: %v", err)
	}

	// AccountId should be reset to 0
	if decodedProject.AccountId != 0 {
		t.Errorf("Expected FromJSON to reset AccountId to 0, got %d", decodedProject.AccountId)
	}

	if decodedProject.ID != project.ID {
		t.Errorf("Expected decoded project ID to be %d, got %d", project.ID, decodedProject.ID)
	}

	if decodedProject.Name != project.Name {
		t.Errorf("Expected decoded project name to be %s, got %s", project.Name, decodedProject.Name)
	}
}

func TestProjectFromJSONInvalid(t *testing.T) {
	var project Project
	invalidJSON := []byte("{invalid json}")

	err := project.FromJSON(invalidJSON)
	if err == nil {
		t.Errorf("Expected FromJSON to return error for invalid JSON, got nil")
	}
}

