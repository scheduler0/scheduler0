package utils

import (
	"context"
	"testing"
	"time"
)

func TestDebounce(t *testing.T) {
	effectorCalled := false
	effector := func() {
		effectorCalled = true
	}

	debounce := NewDebounce()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	delay := uint64(1000) // Delay of 100 milliseconds

	// Call Debounce with a short delay
	debounce.Debounce(ctx, delay, effector)

	// Wait for a duration shorter than the delay
	time.Sleep(time.Duration(delay/2) * time.Millisecond)
	// Assert that effector has not been called yet
	if effectorCalled {
		t.Error("Effector was called too early")
	}

	// Wait for a duration longer than the delay
	time.Sleep(time.Duration(delay) * time.Millisecond)
	// Assert that effector has been called
	if !effectorCalled {
		t.Error("Effector was not called")
	}

	// Reset the flag
	effectorCalled = false

	// Call Debounce again
	debounce.Debounce(ctx, delay, effector)

	// Cancel the context to stop the debounce goroutine
	cancel()

	// Wait for a duration longer than the delay
	select {
	case <-time.After(time.Duration(delay) * time.Millisecond):
		// Assert that effector has not been called due to canceled context
		if effectorCalled {
			t.Error("Effector was called after canceling the context")
		}
	}
}
