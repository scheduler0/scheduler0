package utils

import (
	"runtime"
	"testing"
	"time"
)

func TestNewMemoryLimitChecker(t *testing.T) {
	memStats := &runtime.MemStats{}
	panicCh := make(chan bool)
	checkInterval := 500 * time.Millisecond

	memoryLimitChecker := NewMemoryLimitChecker(100, memStats, panicCh, checkInterval)
	if memoryLimitChecker == nil {
		t.Fatalf("Failed to create a new MemoryLimitChecker instance")
	}
}

func TestMemoryLimitChecker_CheckMemoryUsage(t *testing.T) {
	memStats := &runtime.MemStats{}
	panicCh := make(chan bool)
	checkInterval := 500 * time.Millisecond

	memoryLimitChecker := NewMemoryLimitChecker(100, memStats, panicCh, checkInterval)

	// Set maxMem to an artificially low value for testing purposes
	memoryLimitChecker.maxMem = 1

	go func() {
		<-panicCh
		t.Log("Memory usage exceeded the limit")
	}()

	memoryLimitChecker.CheckMemoryUsage()
}

func TestMemoryLimitChecker_StartAndStopMemoryUsageChecker(t *testing.T) {
	memStats := &runtime.MemStats{}
	panicCh := make(chan bool)
	checkInterval := 500 * time.Millisecond

	memoryLimitChecker := NewMemoryLimitChecker(100, memStats, panicCh, checkInterval)

	// Set maxMem to an artificially high value for testing purposes
	memoryLimitChecker.maxMem = 1 << 62

	stopTest := make(chan bool)
	go func() {
		<-panicCh
		t.Fatal("Memory usage should not exceed the limit")
	}()

	go memoryLimitChecker.StartMemoryUsageChecker()

	select {
	case <-time.After(1 * time.Second):
		t.Log("Memory usage checker is running")
	case <-stopTest:
		t.Fatal("Memory usage checker failed to start")
	}

	memoryLimitChecker.StopMemoryUsageChecker()
}
