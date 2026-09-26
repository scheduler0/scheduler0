// Package alerts publishes operator-facing alerts from the private node to the
// per-environment SNS topic ({env}-scheduler0-alerts) that CloudWatch alarms
// and app.scheduler0.com already fan out through. Customer-facing
// notifications are NOT routed here; they go through pkg/platform to the app.
//
// Design: docs/specs/ops-alerts-sns.md.
package alerts

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"
	"github.com/hashicorp/go-hclog"
)

// Event names. snake_case, stable: subscribers filter on them.
const (
	EventJobExecutionFailed           = "job_execution_failed"
	EventPlatformNotifyFailed         = "platform_notify_failed"
	EventPlatformNotifierUnconfigured = "platform_notifier_unconfigured"
	EventBackupFailed                 = "backup_failed"
)

const (
	SeverityWarn  = "WARN"
	SeverityError = "ERROR"
)

const (
	source           = "scheduler0"
	messageVersion   = 1
	snsSubjectMaxLen = 100
	// maxDetailLen bounds every string in Details so error text or a URL can
	// never balloon the message (same bound the webhook executor uses).
	maxDetailLen = 512
	// DefaultThrottle is the minimum gap between two publishes with the same
	// throttle key. A stuck dependency must not turn into one SNS message per
	// failing job.
	DefaultThrottle = 15 * time.Minute
	publishTimeout  = 5 * time.Second
	runbook         = "scheduler0-terraform/docs/ec2-private-node-troubleshooting.md#alerts"
)

// Alert is one operator-facing event.
type Alert struct {
	Event    string
	Severity string
	// Summary is one human-first line; it becomes the top of the email.
	Summary string
	// Details are scalars only. Never put job Data (customer payload),
	// secrets, or request/response bodies here.
	Details map[string]any
	// ThrottleKey defaults to Event. Set it to e.g. "job_execution_failed:42"
	// to throttle per job rather than per event type.
	ThrottleKey string
}

// Publisher is the operator-alert sink. Implementations must be safe for
// concurrent use and must not block the caller for more than a few seconds.
// The returned error is advisory: callers log it and move on.
type Publisher interface {
	Publish(ctx context.Context, a Alert) error
}

// snsAPI is the subset of *sns.Client used, so tests can substitute a recorder.
type snsAPI interface {
	Publish(ctx context.Context, params *sns.PublishInput, optFns ...func(*sns.Options)) (*sns.PublishOutput, error)
}

type message struct {
	Version  int            `json:"version"`
	Source   string         `json:"source"`
	Env      string         `json:"env"`
	NodeID   uint64         `json:"nodeId"`
	Event    string         `json:"event"`
	Severity string         `json:"severity"`
	Time     string         `json:"time"`
	Summary  string         `json:"summary"`
	Details  map[string]any `json:"details,omitempty"`
	Runbook  string         `json:"runbook"`
}

type snsPublisher struct {
	client   snsAPI
	topicARN string
	env      string
	nodeID   uint64
	logger   hclog.Logger
	throttle time.Duration
	now      func() time.Time

	mu       sync.Mutex
	lastSent map[string]time.Time
}

// NewSNSPublisher returns a Publisher backed by SNS. env may be empty, in
// which case it is derived from the topic ARN ("...:{env}-scheduler0-alerts").
func NewSNSPublisher(client snsAPI, topicARN string, env string, nodeID uint64, logger hclog.Logger) Publisher {
	return newSNSPublisher(client, topicARN, env, nodeID, logger, DefaultThrottle, time.Now)
}

func newSNSPublisher(client snsAPI, topicARN, env string, nodeID uint64, logger hclog.Logger, throttle time.Duration, now func() time.Time) *snsPublisher {
	if strings.TrimSpace(env) == "" {
		env = EnvFromTopicARN(topicARN)
	}
	return &snsPublisher{
		client:   client,
		topicARN: topicARN,
		env:      env,
		nodeID:   nodeID,
		logger:   logger.Named("ops-alerts"),
		throttle: throttle,
		now:      now,
		lastSent: map[string]time.Time{},
	}
}

// EnvFromTopicARN extracts "{env}" from "...:{env}-scheduler0-alerts"; returns
// "unknown" when the ARN does not follow that convention.
func EnvFromTopicARN(topicARN string) string {
	name := topicARN[strings.LastIndex(topicARN, ":")+1:]
	if env, ok := strings.CutSuffix(name, "-scheduler0-alerts"); ok && env != "" {
		return env
	}
	return "unknown"
}

func (p *snsPublisher) Publish(ctx context.Context, a Alert) (err error) {
	// The scheduler's hot path must never be taken down by an alert.
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("alert publisher panicked: %v", r)
			p.logger.Error("ops alert publish panicked", "event", a.Event, "panic", r)
		}
	}()

	if a.Severity == "" {
		a.Severity = SeverityError
	}
	key := a.ThrottleKey
	if key == "" {
		key = a.Event
	}
	if !p.claim(key) {
		p.logger.Debug("ops alert suppressed by throttle", "event", a.Event, "throttleKey", key, "throttle", p.throttle.String())
		return nil
	}

	body, err := json.Marshal(message{
		Version:  messageVersion,
		Source:   source,
		Env:      p.env,
		NodeID:   p.nodeID,
		Event:    a.Event,
		Severity: a.Severity,
		Time:     p.now().UTC().Format(time.RFC3339),
		Summary:  truncate(a.Summary, maxDetailLen),
		Details:  sanitizeDetails(a.Details),
		Runbook:  runbook,
	})
	if err != nil {
		p.release(key)
		return fmt.Errorf("marshal alert: %w", err)
	}

	subject := truncate(fmt.Sprintf("[%s] %s: %s", p.env, source, a.Event), snsSubjectMaxLen)

	// Own budget, detached from the caller's context: callers are usually
	// fire-and-forget goroutines whose context may already be cancelled.
	pctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), publishTimeout)
	defer cancel()

	_, err = p.client.Publish(pctx, &sns.PublishInput{
		TopicArn: aws.String(p.topicARN),
		Subject:  aws.String(subject),
		Message:  aws.String(string(body)),
		MessageAttributes: map[string]snstypes.MessageAttributeValue{
			"severity": {DataType: aws.String("String"), StringValue: aws.String(a.Severity)},
			"event":    {DataType: aws.String("String"), StringValue: aws.String(a.Event)},
			"source":   {DataType: aws.String("String"), StringValue: aws.String(source)},
		},
	})
	if err != nil {
		// Let the next occurrence try again instead of swallowing the whole
		// throttle window because of a transient SNS error.
		p.release(key)
		p.logger.Error("ops alert publish failed", "event", a.Event, "topicArn", p.topicARN, "error", err)
		return fmt.Errorf("publish alert %s: %w", a.Event, err)
	}

	p.logger.Info("ops alert published", "event", a.Event, "severity", a.Severity, "topicArn", p.topicARN)
	return nil
}

func (p *snsPublisher) claim(key string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := p.now()
	if last, ok := p.lastSent[key]; ok && now.Sub(last) < p.throttle {
		return false
	}
	p.lastSent[key] = now
	return true
}

func (p *snsPublisher) release(key string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.lastSent, key)
}

// sanitizeDetails keeps only scalar values and bounds string length. Anything
// else (maps, slices, structs) is replaced by its type name so a caller cannot
// accidentally ship a customer payload.
func sanitizeDetails(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		switch t := v.(type) {
		case nil:
			out[k] = nil
		case string:
			out[k] = truncate(t, maxDetailLen)
		case error:
			out[k] = truncate(t.Error(), maxDetailLen)
		case bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
			out[k] = t
		case time.Time:
			out[k] = t.UTC().Format(time.RFC3339)
		case fmt.Stringer:
			out[k] = truncate(t.String(), maxDetailLen)
		default:
			out[k] = fmt.Sprintf("<omitted %T>", v)
		}
	}
	return out
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

type noopPublisher struct {
	logger hclog.Logger
}

// NewNoopPublisher returns a Publisher that only logs, for when no topic ARN is
// configured. Alerts are logged at WARN with their full body so CloudWatch
// still captures them.
func NewNoopPublisher(logger hclog.Logger) Publisher {
	return &noopPublisher{logger: logger.Named("ops-alerts")}
}

func (p *noopPublisher) Publish(_ context.Context, a Alert) error {
	p.logger.Warn("ops alert not delivered: ALERTS_SNS_TOPIC_ARN is not configured",
		"event", a.Event, "severity", a.Severity, "summary", a.Summary, "details", sanitizeDetails(a.Details))
	return nil
}
