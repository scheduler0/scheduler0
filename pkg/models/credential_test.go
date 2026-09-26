package models

import (
	"encoding/json"
	"testing"
	"time"
)

func TestCredentialToJSON(t *testing.T) {
	credential := Credential{
		ID:          1,
		ApiKey:      "test-api-key",
		ApiSecret:   "test-api-secret",
		Archived:    false,
		AccountId:   1,
		DateCreated: time.Now(),
	}

	jsonData, err := credential.ToJSON()
	if err != nil {
		t.Errorf("Expected ToJSON to succeed, got error: %v", err)
	}

	if len(jsonData) == 0 {
		t.Errorf("Expected ToJSON to return non-empty JSON data")
	}

	// Verify it's valid JSON
	var decodedCredential Credential
	if err := json.Unmarshal(jsonData, &decodedCredential); err != nil {
		t.Errorf("Expected ToJSON to return valid JSON, got error: %v", err)
	}

	if decodedCredential.ID != credential.ID {
		t.Errorf("Expected decoded credential ID to be %d, got %d", credential.ID, decodedCredential.ID)
	}
}

func TestCredentialFromJSON(t *testing.T) {
	credential := Credential{
		ID:          1,
		ApiKey:      "test-api-key",
		ApiSecret:   "test-api-secret",
		Archived:    false,
		AccountId:   100,
		DateCreated: time.Now(),
	}

	jsonData, _ := json.Marshal(credential)

	var decodedCredential Credential
	err := decodedCredential.FromJSON(jsonData)
	if err != nil {
		t.Errorf("Expected FromJSON to succeed, got error: %v", err)
	}

	// AccountId should be reset to 0
	if decodedCredential.AccountId != 0 {
		t.Errorf("Expected FromJSON to reset AccountId to 0, got %d", decodedCredential.AccountId)
	}

	if decodedCredential.ID != credential.ID {
		t.Errorf("Expected decoded credential ID to be %d, got %d", credential.ID, decodedCredential.ID)
	}

	if decodedCredential.ApiKey != credential.ApiKey {
		t.Errorf("Expected decoded credential ApiKey to be %s, got %s", credential.ApiKey, decodedCredential.ApiKey)
	}
}

func TestCredentialFromJSONInvalid(t *testing.T) {
	var credential Credential
	invalidJSON := []byte("{invalid json}")

	err := credential.FromJSON(invalidJSON)
	if err == nil {
		t.Errorf("Expected FromJSON to return error for invalid JSON, got nil")
	}
}

