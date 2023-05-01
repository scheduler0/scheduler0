package utils

import (
	"testing"
	"time"
)

func TestGetSchedulerTime(t *testing.T) {
	schedulerTime1 := GetSchedulerTime()
	schedulerTime2 := GetSchedulerTime()
	if schedulerTime1 != schedulerTime2 {
		t.Errorf("Expected GetSchedulerTime to return the same instance, but got different instances.")
	}

	if schedulerTime1.loc != nil {
		t.Errorf("Expected GetSchedulerTime to return an instance with a nil loc field, but got a non-nil loc field.")
	}
}

func TestSetTimezone(t *testing.T) {
	schedulerTime := GetSchedulerTime()

	// Test valid timezone string
	err := schedulerTime.SetTimezone("America/New_York")
	if err != nil {
		t.Errorf("Expected SetTimezone to succeed with a valid timezone string, but got an error: %v", err)
	}
	expectedLocation, _ := time.LoadLocation("America/New_York")
	if schedulerTime.loc.String() != expectedLocation.String() {
		t.Errorf("Expected SetTimezone to set the loc field to the correct time.Location object, but got %v", schedulerTime.loc)
	}

	// Test invalid timezone string
	err = schedulerTime.SetTimezone("invalid/timezone")
	if err == nil {
		t.Errorf("Expected SetTimezone to return an error with an invalid timezone string, but got no error.")
	}

	// Test setting timezone twice with different values
	schedulerTime.SetTimezone("America/New_York")
	err = schedulerTime.SetTimezone("Asia/Tokyo")
	if err != nil {
		t.Errorf("Expected SetTimezone to succeed with a valid timezone string, but got an error: %v", err)
	}
	expectedLocation, _ = time.LoadLocation("Asia/Tokyo")
	if schedulerTime.loc.String() != expectedLocation.String() {
		t.Errorf("Expected SetTimezone to update the loc field with the new timezone, but got %v", schedulerTime.loc)
	}
}

func TestGetTime(t *testing.T) {
	schedulerTime := GetSchedulerTime()

	// Test with valid time and timezone
	inputTime := time.Date(2023, time.May, 1, 12, 0, 0, 0, time.UTC)
	schedulerTime.SetTimezone("America/New_York")
	expectedTime := time.Date(2023, time.May, 1, 8, 0, 0, 0, schedulerTime.loc)
	outputTime := schedulerTime.GetTime(inputTime)
	if !outputTime.Equal(expectedTime) {
		t.Errorf("Expected GetTime to return %v, but got %v", expectedTime, outputTime)
	}

	// Test with nil loc field
	inputTime = time.Date(2023, time.May, 1, 12, 0, 0, 0, time.UTC)
	schedulerTime.loc = nil
	expectedTime = inputTime
	outputTime = schedulerTime.GetTime(inputTime)
	if !outputTime.Equal(expectedTime) {
		t.Errorf("Expected GetTime with a nil loc field to return the input time, but got %v", outputTime)
	}
}
