package executor

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"scheduler0/pkg/alerts"
	"scheduler0/pkg/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingPublisher struct {
	mu     sync.Mutex
	alerts []alerts.Alert
	notify chan struct{}
}

func newRecordingPublisher() *recordingPublisher {
	return &recordingPublisher{notify: make(chan struct{}, 16)}
}

func (r *recordingPublisher) Publish(_ context.Context, a alerts.Alert) error {
	r.mu.Lock()
	r.alerts = append(r.alerts, a)
	r.mu.Unlock()
	r.notify <- struct{}{}
	return nil
}

func (r *recordingPublisher) waitFor(t *testing.T, n int) []alerts.Alert {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		r.mu.Lock()
		if len(r.alerts) >= n {
			out := append([]alerts.Alert(nil), r.alerts...)
			r.mu.Unlock()
			return out
		}
		r.mu.Unlock()
		select {
		case <-r.notify:
		case <-deadline:
			t.Fatalf("expected %d alerts, got %d", n, len(r.alerts))
		}
	}
}

func TestNotifyJobFailureAsync_PublishesJobExecutionFailedForAnyAccount(t *testing.T) {
	je := newJobFailureNotifyExecutor()
	pub := newRecordingPublisher()
	je.SetAlertPublisher(pub)
	je.SetNotifyJobFailureCallback(func(models.Job, uint64, uint64) error { return nil })

	execID := uint64(7)
	je.notifyJobFailureAsync(models.Job{ID: 42, AccountId: 9, ProjectID: 3, RetryMax: 3, Spec: "@every 1h", ExecutorId: &execID}, 3, 2)

	got := pub.waitFor(t, 1)
	// Webhook succeeded, so exactly one alert; give a moment to be sure no
	// platform_notify_failed follows.
	time.Sleep(50 * time.Millisecond)
	got = pub.waitFor(t, 1)
	require.Len(t, got, 1)

	a := got[0]
	assert.Equal(t, alerts.EventJobExecutionFailed, a.Event)
	assert.Equal(t, alerts.SeverityWarn, a.Severity)
	assert.Equal(t, "job_execution_failed:42", a.ThrottleKey, "throttled per job, not per event")
	assert.Equal(t, uint64(42), a.Details["jobId"])
	assert.Equal(t, uint64(9), a.Details["accountId"], "non-system accounts alert too")
	assert.Equal(t, uint64(3), a.Details["failCount"])
	assert.Equal(t, uint64(2), a.Details["executionVersion"])
	assert.Equal(t, uint64(7), a.Details["executorId"])
	assert.NotContains(t, a.Details, "data", "customer payload must not be included")
}

func TestNotifyJobFailureAsync_PublishesPlatformNotifyFailedAfterWebhookError(t *testing.T) {
	je := newJobFailureNotifyExecutor()
	pub := newRecordingPublisher()
	je.SetAlertPublisher(pub)
	je.SetNotifyJobFailureCallback(func(models.Job, uint64, uint64) error { return errors.New("unexpected status 403") })

	je.notifyJobFailureAsync(models.Job{ID: 5, AccountId: 1}, 1, 1)

	got := pub.waitFor(t, 2)
	require.Len(t, got, 2)
	assert.Equal(t, alerts.EventJobExecutionFailed, got[0].Event, "customer-path outcome does not suppress the ops alert")
	assert.Equal(t, alerts.EventPlatformNotifyFailed, got[1].Event)
	assert.Equal(t, alerts.SeverityError, got[1].Severity)
	assert.Contains(t, got[1].Summary, "unexpected status 403")
	assert.EqualError(t, got[1].Details["error"].(error), "unexpected status 403")
}

func TestNotifyJobFailureAsync_PublishesWithoutWebhookCallback(t *testing.T) {
	// No platform webhook configured at all: the ops alert must still go out.
	je := newJobFailureNotifyExecutor()
	pub := newRecordingPublisher()
	je.SetAlertPublisher(pub)

	je.notifyJobFailureAsync(models.Job{ID: 8, AccountId: 4}, 2, 1)

	got := pub.waitFor(t, 1)
	assert.Equal(t, alerts.EventJobExecutionFailed, got[0].Event)
}

func TestNotifyJobFailureAsync_SkipsUnsetAccountEvenWithPublisher(t *testing.T) {
	je := newJobFailureNotifyExecutor()
	pub := newRecordingPublisher()
	je.SetAlertPublisher(pub)

	je.notifyJobFailureAsync(models.Job{ID: 1, AccountId: 0}, 1, 1)
	time.Sleep(50 * time.Millisecond)

	pub.mu.Lock()
	defer pub.mu.Unlock()
	assert.Empty(t, pub.alerts)
}
