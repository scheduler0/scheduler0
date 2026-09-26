package alerts

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingSNS struct {
	mu     sync.Mutex
	inputs []*sns.PublishInput
	err    error
	// block, when non-nil, is waited on before returning so tests can assert
	// the publisher's own timeout applies.
	block chan struct{}
}

func (r *recordingSNS) Publish(ctx context.Context, in *sns.PublishInput, _ ...func(*sns.Options)) (*sns.PublishOutput, error) {
	if r.block != nil {
		select {
		case <-r.block:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.inputs = append(r.inputs, in)
	return &sns.PublishOutput{}, r.err
}

func (r *recordingSNS) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.inputs)
}

const topic = "arn:aws:sns:us-east-1:748201447723:production-scheduler0-alerts"

func newTestPublisher(rec *recordingSNS, now *time.Time) *snsPublisher {
	return newSNSPublisher(rec, topic, "", 1, hclog.NewNullLogger(), 15*time.Minute, func() time.Time { return *now })
}

func TestEnvFromTopicARN(t *testing.T) {
	assert.Equal(t, "production", EnvFromTopicARN(topic))
	assert.Equal(t, "staging", EnvFromTopicARN("arn:aws:sns:us-east-1:1:staging-scheduler0-alerts"))
	assert.Equal(t, "unknown", EnvFromTopicARN("arn:aws:sns:us-east-1:1:something-else"))
	assert.Equal(t, "unknown", EnvFromTopicARN(""))
}

func TestPublish_MessageContract(t *testing.T) {
	now := time.Date(2026, 9, 13, 1, 30, 52, 0, time.UTC)
	rec := &recordingSNS{}
	p := newTestPublisher(rec, &now)

	err := p.Publish(context.Background(), Alert{
		Event:    EventPlatformNotifyFailed,
		Severity: SeverityError,
		Summary:  "job failure notification to app failed: 403 Forbidden",
		Details: map[string]any{
			"jobId":     uint64(5),
			"accountId": uint64(1),
			"error":     errors.New("unexpected status 403"),
			"when":      now,
			"payload":   map[string]any{"jobType": "welcome_email"}, // must be dropped
			"tags":      []string{"a"},                              // must be dropped
			"long":      strings.Repeat("x", 2000),                  // must be truncated
		},
	})
	require.NoError(t, err)
	require.Equal(t, 1, rec.count())

	in := rec.inputs[0]
	assert.Equal(t, topic, *in.TopicArn)
	assert.Equal(t, "[production] scheduler0-private: platform_notify_failed", *in.Subject)
	assert.Equal(t, "ERROR", *in.MessageAttributes["severity"].StringValue)
	assert.Equal(t, EventPlatformNotifyFailed, *in.MessageAttributes["event"].StringValue)
	assert.Equal(t, "scheduler0-private", *in.MessageAttributes["source"].StringValue)

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(*in.Message), &got))
	assert.Equal(t, float64(1), got["version"])
	assert.Equal(t, "scheduler0-private", got["source"])
	assert.Equal(t, "production", got["env"])
	assert.Equal(t, float64(1), got["nodeId"])
	assert.Equal(t, EventPlatformNotifyFailed, got["event"])
	assert.Equal(t, "ERROR", got["severity"])
	assert.Equal(t, "2026-09-13T01:30:52Z", got["time"])
	assert.Contains(t, got["runbook"], "ec2-private-node-troubleshooting.md")

	details := got["details"].(map[string]any)
	assert.Equal(t, float64(5), details["jobId"])
	assert.Equal(t, "unexpected status 403", details["error"])
	assert.Equal(t, "2026-09-13T01:30:52Z", details["when"])
	assert.Equal(t, "<omitted map[string]interface {}>", details["payload"])
	assert.Equal(t, "<omitted []string>", details["tags"])
	assert.Len(t, details["long"], maxDetailLen)
	assert.NotContains(t, *in.Message, "welcome_email", "customer payload must never reach SNS")
}

func TestPublish_DefaultsSeverityToError(t *testing.T) {
	now := time.Now()
	rec := &recordingSNS{}
	p := newTestPublisher(rec, &now)
	require.NoError(t, p.Publish(context.Background(), Alert{Event: EventBackupFailed, Summary: "x"}))
	assert.Equal(t, "ERROR", *rec.inputs[0].MessageAttributes["severity"].StringValue)
}

func TestPublish_ThrottlesPerKeyAndReleasesOnError(t *testing.T) {
	now := time.Now()
	rec := &recordingSNS{}
	p := newTestPublisher(rec, &now)
	ctx := context.Background()

	// Same event, different jobs -> different throttle keys -> both publish.
	require.NoError(t, p.Publish(ctx, Alert{Event: EventJobExecutionFailed, ThrottleKey: "job_execution_failed:1"}))
	require.NoError(t, p.Publish(ctx, Alert{Event: EventJobExecutionFailed, ThrottleKey: "job_execution_failed:2"}))
	assert.Equal(t, 2, rec.count())

	// Same key again inside the window -> suppressed, no error.
	require.NoError(t, p.Publish(ctx, Alert{Event: EventJobExecutionFailed, ThrottleKey: "job_execution_failed:1"}))
	assert.Equal(t, 2, rec.count())

	// Key defaults to Event.
	require.NoError(t, p.Publish(ctx, Alert{Event: EventBackupFailed}))
	require.NoError(t, p.Publish(ctx, Alert{Event: EventBackupFailed}))
	assert.Equal(t, 3, rec.count())

	// After the window it fires again.
	now = now.Add(16 * time.Minute)
	require.NoError(t, p.Publish(ctx, Alert{Event: EventJobExecutionFailed, ThrottleKey: "job_execution_failed:1"}))
	assert.Equal(t, 4, rec.count())

	// A failed publish releases the throttle so the next occurrence retries.
	rec.err = errors.New("sns down")
	err := p.Publish(ctx, Alert{Event: EventPlatformNotifierUnconfigured})
	require.Error(t, err)
	rec.err = nil
	require.NoError(t, p.Publish(ctx, Alert{Event: EventPlatformNotifierUnconfigured}))
	assert.Equal(t, 6, rec.count())
}

func TestPublish_UsesOwnTimeoutAndIgnoresCancelledCaller(t *testing.T) {
	now := time.Now()
	rec := &recordingSNS{block: make(chan struct{})}
	p := newTestPublisher(rec, &now)

	// Caller context already cancelled (typical fire-and-forget goroutine
	// after its parent returned): publish must still be attempted.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan error, 1)
	go func() { done <- p.Publish(ctx, Alert{Event: EventBackupFailed}) }()

	// Unblock the fake SNS; the publish should complete successfully.
	close(rec.block)
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("publish did not complete")
	}
	assert.Equal(t, 1, rec.count())
}

func TestPublish_RecoversFromPanic(t *testing.T) {
	now := time.Now()
	p := newTestPublisher(nil, &now) // nil client -> nil pointer dereference inside Publish
	err := p.Publish(context.Background(), Alert{Event: EventBackupFailed})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "panicked")
}

func TestNoopPublisher(t *testing.T) {
	p := NewNoopPublisher(hclog.NewNullLogger())
	assert.NoError(t, p.Publish(context.Background(), Alert{Event: EventBackupFailed, Details: map[string]any{"x": 1}}))
}
