package models

import (
	"encoding/json"
	"testing"
	"time"
)

func TestJobStatusConstants(t *testing.T) {
	if JobStatusActive != "active" {
		t.Errorf("Expected JobStatusActive to be 'active', got %s", JobStatusActive)
	}

	if JobStatusInactive != "inactive" {
		t.Errorf("Expected JobStatusInactive to be 'inactive', got %s", JobStatusInactive)
	}
}

func TestJobDefaultStatus(t *testing.T) {
	job := Job{
		ID:          1,
		ProjectID:   1,
		Spec:        "* * * * *",
		Timezone:    "UTC",
		AccountId:   1,
		DateCreated: time.Now(),
	}

	// Status should be empty by default
	if job.Status != "" {
		t.Errorf("Expected job status to be empty by default, got %s", job.Status)
	}
}

func TestJobWithStatus(t *testing.T) {
	job := Job{
		ID:          1,
		ProjectID:   1,
		Spec:        "* * * * *",
		Timezone:    "UTC",
		AccountId:   1,
		DateCreated: time.Now(),
		Status:      JobStatusActive,
	}

	if job.Status != JobStatusActive {
		t.Errorf("Expected job status to be 'active', got %s", job.Status)
	}
}

func TestJobToJSON(t *testing.T) {
	job := Job{
		ID:          1,
		ProjectID:   1,
		Spec:        "* * * * *",
		Timezone:    "UTC",
		AccountId:   1,
		DateCreated: time.Now(),
		Status:      JobStatusActive,
	}

	jsonData, err := job.ToJSON()
	if err != nil {
		t.Errorf("Expected ToJSON to succeed, got error: %v", err)
	}

	if len(jsonData) == 0 {
		t.Errorf("Expected ToJSON to return non-empty JSON data")
	}

	// Verify it's valid JSON
	var decodedJob Job
	if err := json.Unmarshal(jsonData, &decodedJob); err != nil {
		t.Errorf("Expected ToJSON to return valid JSON, got error: %v", err)
	}

	if decodedJob.ID != job.ID {
		t.Errorf("Expected decoded job ID to be %d, got %d", job.ID, decodedJob.ID)
	}
}

func TestJobFromJSON(t *testing.T) {
	job := Job{
		ID:          1,
		ProjectID:   1,
		Spec:        "* * * * *",
		Timezone:    "UTC",
		AccountId:   100,
		DateCreated: time.Now(),
		Status:      JobStatusActive,
	}

	jsonData, _ := json.Marshal(job)

	var decodedJob Job
	err := decodedJob.FromJSON(jsonData)
	if err != nil {
		t.Errorf("Expected FromJSON to succeed, got error: %v", err)
	}

	// AccountId should be reset to 0
	if decodedJob.AccountId != 0 {
		t.Errorf("Expected FromJSON to reset AccountId to 0, got %d", decodedJob.AccountId)
	}

	if decodedJob.ID != job.ID {
		t.Errorf("Expected decoded job ID to be %d, got %d", job.ID, decodedJob.ID)
	}

	if decodedJob.Status != job.Status {
		t.Errorf("Expected decoded job status to be %s, got %s", job.Status, decodedJob.Status)
	}
}

func TestJobFromJSONInvalid(t *testing.T) {
	var job Job
	invalidJSON := []byte("{invalid json}")

	err := job.FromJSON(invalidJSON)
	if err == nil {
		t.Errorf("Expected FromJSON to return error for invalid JSON, got nil")
	}
}

func TestJobGetNextExecutionTime_OneTimeJob_NoStartDate(t *testing.T) {
	job := Job{
		ID:          1,
		ProjectID:   1,
		Spec:        "", // One-time job
		Timezone:    "UTC",
		DateCreated: time.Now(),
	}

	nextTime, err := job.GetNextExecutionTime()
	if err == nil {
		t.Errorf("Expected GetNextExecutionTime to return error for one-time job without startDate, got nil")
	}
	if nextTime != nil {
		t.Errorf("Expected GetNextExecutionTime to return nil time for error case, got %v", nextTime)
	}
}

func TestJobGetNextExecutionTime_OneTimeJob_AlreadyExecuted(t *testing.T) {
	startDate := time.Now().Add(-2 * time.Hour)
	lastExecution := time.Now().Add(-1 * time.Hour)

	job := Job{
		ID:                1,
		ProjectID:         1,
		Spec:              "", // One-time job
		StartDate:         startDate,
		LastExecutionDate: lastExecution,
		Timezone:          "UTC",
		DateCreated:       time.Now(),
	}

	nextTime, err := job.GetNextExecutionTime()
	if err != nil {
		t.Errorf("Expected GetNextExecutionTime to succeed, got error: %v", err)
	}
	if nextTime == nil {
		t.Errorf("Expected GetNextExecutionTime to return a time pointer, got nil")
	} else if !nextTime.IsZero() {
		t.Errorf("Expected GetNextExecutionTime to return zero time for already executed one-time job, got %v", *nextTime)
	}
}

func TestJobGetNextExecutionTime_OneTimeJob_NotYetExecuted(t *testing.T) {
	startDate := time.Now().Add(1 * time.Hour)

	job := Job{
		ID:          1,
		ProjectID:   1,
		Spec:        "", // One-time job
		StartDate:   startDate,
		Timezone:    "UTC",
		DateCreated: time.Now(),
	}

	nextTime, err := job.GetNextExecutionTime()
	if err != nil {
		t.Errorf("Expected GetNextExecutionTime to succeed, got error: %v", err)
	}
	if nextTime == nil {
		t.Errorf("Expected GetNextExecutionTime to return a time pointer, got nil")
	} else if nextTime.IsZero() {
		t.Errorf("Expected GetNextExecutionTime to return non-zero time for one-time job, got zero time")
	}
}

func TestJobGetNextExecutionTime_RecurringJob_InvalidCronSpec(t *testing.T) {
	job := Job{
		ID:          1,
		ProjectID:   1,
		Spec:        "invalid cron spec",
		Timezone:    "UTC",
		DateCreated: time.Now(),
	}

	nextTime, err := job.GetNextExecutionTime()
	if err == nil {
		t.Errorf("Expected GetNextExecutionTime to return error for invalid cron spec, got nil")
	}
	if nextTime != nil {
		t.Errorf("Expected GetNextExecutionTime to return nil time for error case, got %v", nextTime)
	}
}

func TestJobGetNextExecutionTime_RecurringJob_InvalidTimezone(t *testing.T) {
	job := Job{
		ID:          1,
		ProjectID:   1,
		Spec:        "* * * * *",
		Timezone:    "Invalid/Timezone",
		DateCreated: time.Now(),
	}

	nextTime, err := job.GetNextExecutionTime()
	if err == nil {
		t.Errorf("Expected GetNextExecutionTime to return error for invalid timezone, got nil")
	}
	if nextTime != nil {
		t.Errorf("Expected GetNextExecutionTime to return nil time for error case, got %v", nextTime)
	}
}

func TestJobGetNextExecutionTime_RecurringJob_ValidCronSpec(t *testing.T) {
	job := Job{
		ID:          1,
		ProjectID:   1,
		Spec:        "0 * * * *", // Every hour
		Timezone:    "UTC",
		DateCreated: time.Now().Add(-2 * time.Hour),
	}

	nextTime, err := job.GetNextExecutionTime()
	if err != nil {
		t.Errorf("Expected GetNextExecutionTime to succeed, got error: %v", err)
	}
	if nextTime == nil {
		t.Errorf("Expected GetNextExecutionTime to return a time pointer, got nil")
	} else if nextTime.IsZero() {
		t.Errorf("Expected GetNextExecutionTime to return non-zero time, got zero time")
	}
}

func TestJobGetNextExecutionTime_RecurringJob_StartDateInFuture(t *testing.T) {
	futureStartDate := time.Now().Add(2 * time.Hour)

	job := Job{
		ID:          1,
		ProjectID:   1,
		Spec:        "0 * * * *", // Every hour
		StartDate:   futureStartDate,
		Timezone:    "UTC",
		DateCreated: time.Now(),
	}

	nextTime, err := job.GetNextExecutionTime()
	if err != nil {
		t.Errorf("Expected GetNextExecutionTime to succeed, got error: %v", err)
	}
	if nextTime == nil {
		t.Errorf("Expected GetNextExecutionTime to return a time pointer, got nil")
	} else if nextTime.IsZero() {
		t.Errorf("Expected GetNextExecutionTime to return non-zero time, got zero time")
	}
}

func TestJobGetNextExecutionTime_RecurringJob_StartDateInPast(t *testing.T) {
	pastStartDate := time.Now().Add(-2 * time.Hour)

	job := Job{
		ID:          1,
		ProjectID:   1,
		Spec:        "0 * * * *", // Every hour
		StartDate:   pastStartDate,
		Timezone:    "UTC",
		DateCreated: time.Now().Add(-3 * time.Hour),
	}

	nextTime, err := job.GetNextExecutionTime()
	if err != nil {
		t.Errorf("Expected GetNextExecutionTime to succeed, got error: %v", err)
	}
	if nextTime == nil {
		t.Errorf("Expected GetNextExecutionTime to return a time pointer, got nil")
	} else if nextTime.IsZero() {
		t.Errorf("Expected GetNextExecutionTime to return non-zero time, got zero time")
	}
}

func TestJobGetNextExecutionTime_RecurringJob_WithLastExecutionDate(t *testing.T) {
	lastExecution := time.Now().Add(-30 * time.Minute)

	job := Job{
		ID:                1,
		ProjectID:         1,
		Spec:              "0 * * * *", // Every hour
		LastExecutionDate: lastExecution,
		Timezone:          "UTC",
		DateCreated:       time.Now().Add(-2 * time.Hour),
	}

	nextTime, err := job.GetNextExecutionTime()
	if err != nil {
		t.Errorf("Expected GetNextExecutionTime to succeed, got error: %v", err)
	}
	if nextTime == nil {
		t.Errorf("Expected GetNextExecutionTime to return a time pointer, got nil")
	} else if nextTime.IsZero() {
		t.Errorf("Expected GetNextExecutionTime to return non-zero time, got zero time")
	}
	// Next execution should be after the last execution
	if !nextTime.After(lastExecution) {
		t.Errorf("Expected next execution time to be after last execution, got %v (last: %v)", *nextTime, lastExecution)
	}
}

func TestJobConvertTimeToJobTimezone_ValidTimezone(t *testing.T) {
	job := Job{
		Timezone: "America/New_York",
	}

	testTime := time.Now().UTC()
	convertedTime, err := job.ConvertTimeToJobTimezone(testTime)
	if err != nil {
		t.Errorf("Expected ConvertTimeToJobTimezone to succeed, got error: %v", err)
	}
	if convertedTime == nil {
		t.Errorf("Expected ConvertTimeToJobTimezone to return a time pointer, got nil")
	} else {
		// Verify the timezone is correct
		expectedLoc, _ := time.LoadLocation("America/New_York")
		if convertedTime.Location().String() != expectedLoc.String() {
			t.Errorf("Expected converted time to be in America/New_York timezone, got %s", convertedTime.Location().String())
		}
	}
}

func TestJobConvertTimeToJobTimezone_InvalidTimezone(t *testing.T) {
	job := Job{
		Timezone: "Invalid/Timezone",
	}

	testTime := time.Now()
	convertedTime, err := job.ConvertTimeToJobTimezone(testTime)
	if err == nil {
		t.Errorf("Expected ConvertTimeToJobTimezone to return error for invalid timezone, got nil")
	}
	if convertedTime != nil {
		t.Errorf("Expected ConvertTimeToJobTimezone to return nil time for error case, got %v", convertedTime)
	}
}

func TestJobGetNextExecutionId(t *testing.T) {
	startDate := time.Now().Add(1 * time.Hour)
	job := Job{
		ID:          1,
		ProjectID:   1,
		Spec:        "", // One-time job
		StartDate:   startDate,
		Timezone:    "UTC",
		DateCreated: time.Now(),
	}

	executionId, err := job.GetNextExecutionId()
	if err != nil {
		t.Errorf("Expected GetNextExecutionId to succeed, got error: %v", err)
	}
	if executionId == "" {
		t.Errorf("Expected GetNextExecutionId to return non-empty string, got empty string")
	}

	// Call again to verify it's deterministic (or at least returns a value)
	executionId2, err2 := job.GetNextExecutionId()
	if err2 != nil {
		t.Errorf("Expected GetNextExecutionId to succeed on second call, got error: %v", err2)
	}
	if executionId2 == "" {
		t.Errorf("Expected GetNextExecutionId to return non-empty string on second call, got empty string")
	}
}

func TestJobGetNextExecutionId_WithError(t *testing.T) {
	job := Job{
		ID:          1,
		ProjectID:   1,
		Spec:        "", // One-time job without startDate - will cause error
		Timezone:    "UTC",
		DateCreated: time.Now(),
	}

	executionId, _ := job.GetNextExecutionId()
	// Note: GetNextExecutionId returns empty string and nil error when GetNextExecutionTime fails
	// This is based on the implementation: if err != nil { return "", nil }
	if executionId != "" {
		t.Errorf("Expected GetNextExecutionId to return empty string when GetNextExecutionTime fails, got %s", executionId)
	}
}

func TestJobHasJobEnded_NoEndDate(t *testing.T) {
	job := Job{
		ID:          1,
		ProjectID:   1,
		Spec:        "* * * * *",
		Timezone:    "UTC",
		DateCreated: time.Now(),
		// EndDate is zero
	}

	hasEnded, err := job.HasJobEnded()
	if err != nil {
		t.Errorf("Expected HasJobEnded to succeed, got error: %v", err)
	}
	if hasEnded {
		t.Errorf("Expected HasJobEnded to return false when no end date is set, got true")
	}
}

func TestJobHasJobEnded_EndDateInPast(t *testing.T) {
	pastEndDate := time.Now().Add(-1 * time.Hour)

	job := Job{
		ID:          1,
		ProjectID:   1,
		Spec:        "* * * * *",
		EndDate:     pastEndDate,
		Timezone:    "UTC",
		DateCreated: time.Now(),
	}

	hasEnded, err := job.HasJobEnded()
	if err != nil {
		t.Errorf("Expected HasJobEnded to succeed, got error: %v", err)
	}
	if !hasEnded {
		t.Errorf("Expected HasJobEnded to return true when end date is in the past, got false")
	}
}

func TestJobHasJobEnded_EndDateInFuture(t *testing.T) {
	futureEndDate := time.Now().Add(1 * time.Hour)

	job := Job{
		ID:          1,
		ProjectID:   1,
		Spec:        "* * * * *",
		EndDate:     futureEndDate,
		Timezone:    "UTC",
		DateCreated: time.Now(),
	}

	hasEnded, err := job.HasJobEnded()
	if err != nil {
		t.Errorf("Expected HasJobEnded to succeed, got error: %v", err)
	}
	if hasEnded {
		t.Errorf("Expected HasJobEnded to return false when end date is in the future, got true")
	}
}

func TestJobHasJobEnded_InvalidTimezone(t *testing.T) {
	pastEndDate := time.Now().Add(-1 * time.Hour)

	job := Job{
		ID:          1,
		ProjectID:   1,
		Spec:        "* * * * *",
		EndDate:     pastEndDate,
		Timezone:    "Invalid/Timezone",
		DateCreated: time.Now(),
	}

	hasEnded, err := job.HasJobEnded()
	if err == nil {
		t.Errorf("Expected HasJobEnded to return error for invalid timezone, got nil")
	}
	// When error occurs, it returns false
	if hasEnded {
		t.Errorf("Expected HasJobEnded to return false when error occurs, got true")
	}
}

func TestJobHasJobEnded_WithDifferentTimezone(t *testing.T) {
	// Test with a timezone that has offset
	pastEndDate := time.Now().Add(-1 * time.Hour)

	job := Job{
		ID:          1,
		ProjectID:   1,
		Spec:        "* * * * *",
		EndDate:     pastEndDate,
		Timezone:    "America/New_York",
		DateCreated: time.Now(),
	}

	hasEnded, err := job.HasJobEnded()
	if err != nil {
		t.Errorf("Expected HasJobEnded to succeed, got error: %v", err)
	}
	// The result depends on the current time, but should not error
	// hasEnded value depends on current time, so we just verify no error occurred
	_ = hasEnded
}
