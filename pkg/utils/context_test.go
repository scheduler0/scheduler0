package utils

import (
	"bytes"
	"context"
	"log"
	"strings"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
)

func TestGetRequestID(t *testing.T) {
	tests := []struct {
		name     string
		ctx      context.Context
		expected string
	}{
		{
			name:     "nil context",
			ctx:      nil,
			expected: "",
		},
		{
			name:     "context without request ID",
			ctx:      context.Background(),
			expected: "",
		},
		{
			name:     "context with request ID",
			ctx:      context.WithValue(context.Background(), requestIDKey, "test-request-id-123"),
			expected: "test-request-id-123",
		},
		{
			name:     "context with wrong type",
			ctx:      context.WithValue(context.Background(), requestIDKey, 123),
			expected: "",
		},
		{
			name:     "context with empty string request ID",
			ctx:      context.WithValue(context.Background(), requestIDKey, ""),
			expected: "",
		},
		{
			name:     "context with different key",
			ctx:      context.WithValue(context.Background(), contextKey("different-key"), "some-value"),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetRequestID(tt.ctx)
			assert.Equal(t, tt.expected, result, "GetRequestID should return expected value")
		})
	}
}

func TestGetAccountID(t *testing.T) {
	tests := []struct {
		name          string
		ctx           context.Context
		expectedID    uint64
		expectedFound bool
	}{
		{
			name:          "nil context",
			ctx:           nil,
			expectedID:    0,
			expectedFound: false,
		},
		{
			name:          "context without account ID",
			ctx:           context.Background(),
			expectedID:    0,
			expectedFound: false,
		},
		{
			name:          "context with account ID",
			ctx:           context.WithValue(context.Background(), accountIDKey, uint64(12345)),
			expectedID:    12345,
			expectedFound: true,
		},
		{
			name:          "context with account ID zero",
			ctx:           context.WithValue(context.Background(), accountIDKey, uint64(0)),
			expectedID:    0,
			expectedFound: true,
		},
		{
			name:          "context with wrong type (string)",
			ctx:           context.WithValue(context.Background(), accountIDKey, "12345"),
			expectedID:    0,
			expectedFound: false,
		},
		{
			name:          "context with wrong type (int)",
			ctx:           context.WithValue(context.Background(), accountIDKey, 12345),
			expectedID:    0,
			expectedFound: false,
		},
		{
			name:          "context with different key",
			ctx:           context.WithValue(context.Background(), contextKey("different-key"), uint64(12345)),
			expectedID:    0,
			expectedFound: false,
		},
		{
			name:          "context with large account ID",
			ctx:           context.WithValue(context.Background(), accountIDKey, uint64(18446744073709551615)),
			expectedID:    18446744073709551615,
			expectedFound: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, found := GetAccountID(tt.ctx)
			assert.Equal(t, tt.expectedID, id, "GetAccountID should return expected ID")
			assert.Equal(t, tt.expectedFound, found, "GetAccountID should return expected found status")
		})
	}
}

func TestLogWithRequestID(t *testing.T) {
	tests := []struct {
		name           string
		requestID      string
		format         string
		args           []interface{}
		expectedPrefix string
		expectedSuffix string
	}{
		{
			name:           "with request ID and format string",
			requestID:      "req-123",
			format:         "User %s performed action %s",
			args:           []interface{}{"john", "login"},
			expectedPrefix: "[request_id=req-123] ",
			expectedSuffix: "User john performed action login",
		},
		{
			name:           "with request ID and pre-formatted message",
			requestID:      "req-456",
			format:         "",
			args:           []interface{}{"This is a pre-formatted message"},
			expectedPrefix: "[request_id=req-456] ",
			expectedSuffix: "This is a pre-formatted message",
		},
		{
			name:           "without request ID",
			requestID:      "",
			format:         "Message: %s",
			args:           []interface{}{"test"},
			expectedPrefix: "",
			expectedSuffix: "Message: test",
		},
		{
			name:           "with request ID, empty format, empty args",
			requestID:      "req-789",
			format:         "",
			args:           []interface{}{},
			expectedPrefix: "[request_id=req-789] ",
			expectedSuffix: "",
		},
		{
			name:           "with request ID, empty format, nil args",
			requestID:      "req-999",
			format:         "",
			args:           nil,
			expectedPrefix: "[request_id=req-999] ",
			expectedSuffix: "",
		},
		{
			name:           "with request ID, multiple args",
			requestID:      "req-multi",
			format:         "",
			args:           []interface{}{"arg1", "arg2", 123},
			expectedPrefix: "[request_id=req-multi] ",
			expectedSuffix: "arg1arg2123", // fmt.Sprint concatenates without spaces
		},
		{
			name:           "without request ID, empty format",
			requestID:      "",
			format:         "",
			args:           []interface{}{"test message"},
			expectedPrefix: "",
			expectedSuffix: "test message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := log.New(&buf, "", 0)

			LogWithRequestID(logger, tt.requestID, tt.format, tt.args...)

			output := buf.String()
			if tt.expectedPrefix != "" {
				assert.True(t, strings.HasPrefix(output, tt.expectedPrefix), "Output should have expected prefix: %s, got: %s", tt.expectedPrefix, output)
			}
			if tt.expectedSuffix != "" {
				assert.True(t, strings.Contains(output, tt.expectedSuffix), "Output should contain expected suffix: %s, got: %s", tt.expectedSuffix, output)
			}
			if tt.expectedPrefix == "" && tt.expectedSuffix == "" {
				// Should be empty or just newline
				assert.True(t, output == "" || output == "\n", "Output should be empty or just newline, got: %s", output)
			}
		})
	}
}

func TestLogWithRequestIDHCLog(t *testing.T) {
	tests := []struct {
		name                 string
		level                string
		msg                  string
		requestID            string
		additionalKeysValues []interface{}
		expectedLevel        string
	}{
		{
			name:                 "trace level",
			level:                "trace",
			msg:                  "trace message",
			requestID:            "req-trace",
			additionalKeysValues: []interface{}{"key1", "value1"},
			expectedLevel:        "trace",
		},
		{
			name:                 "debug level",
			level:                "debug",
			msg:                  "debug message",
			requestID:            "req-debug",
			additionalKeysValues: []interface{}{"key2", "value2"},
			expectedLevel:        "debug",
		},
		{
			name:                 "info level",
			level:                "info",
			msg:                  "info message",
			requestID:            "req-info",
			additionalKeysValues: []interface{}{"key3", "value3"},
			expectedLevel:        "info",
		},
		{
			name:                 "warn level",
			level:                "warn",
			msg:                  "warn message",
			requestID:            "req-warn",
			additionalKeysValues: []interface{}{"key4", "value4"},
			expectedLevel:        "warn",
		},
		{
			name:                 "error level",
			level:                "error",
			msg:                  "error message",
			requestID:            "req-error",
			additionalKeysValues: []interface{}{"key5", "value5"},
			expectedLevel:        "error",
		},
		{
			name:                 "default level (unknown)",
			level:                "unknown",
			msg:                  "unknown level message",
			requestID:            "req-unknown",
			additionalKeysValues: []interface{}{"key6", "value6"},
			expectedLevel:        "info", // defaults to info
		},
		{
			name:                 "empty level",
			level:                "",
			msg:                  "empty level message",
			requestID:            "req-empty",
			additionalKeysValues: []interface{}{},
			expectedLevel:        "info", // defaults to info
		},
		{
			name:                 "with empty request ID",
			level:                "info",
			msg:                  "message with empty request ID",
			requestID:            "",
			additionalKeysValues: []interface{}{"key7", "value7"},
			expectedLevel:        "info",
		},
		{
			name:                 "no additional keys",
			level:                "info",
			msg:                  "message without additional keys",
			requestID:            "req-no-keys",
			additionalKeysValues: nil,
			expectedLevel:        "info",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := hclog.New(&hclog.LoggerOptions{
				Output: &buf,
				Level:  hclog.Trace,
			})

			LogWithRequestIDHCLog(logger, tt.level, tt.msg, tt.requestID, tt.additionalKeysValues...)

			output := buf.String()
			// Verify the message is in the output
			assert.True(t, strings.Contains(output, tt.msg), "Output should contain message: %s, got: %s", tt.msg, output)
			// Verify request_id is in the output (hclog includes key-value pairs)
			if tt.requestID != "" {
				assert.True(t, strings.Contains(output, tt.requestID), "Output should contain request_id: %s, got: %s", tt.requestID, output)
			}
		})
	}
}

func TestContextKeys(t *testing.T) {
	// Test that the context keys are properly defined and can be used
	ctx := context.Background()
	ctx = context.WithValue(ctx, requestIDKey, "test-id")
	ctx = context.WithValue(ctx, accountIDKey, uint64(999))

	requestID := GetRequestID(ctx)
	accountID, found := GetAccountID(ctx)

	assert.Equal(t, "test-id", requestID, "Request ID should be extracted correctly")
	assert.Equal(t, uint64(999), accountID, "Account ID should be extracted correctly")
	assert.True(t, found, "Account ID should be found")
}

func TestGetRequestID_Integration(t *testing.T) {
	// Test that GetRequestID works with nested contexts
	parentCtx := context.Background()
	childCtx := context.WithValue(parentCtx, requestIDKey, "child-request-id")

	// Should get the value from child context
	result := GetRequestID(childCtx)
	assert.Equal(t, "child-request-id", result, "Should get request ID from child context")
}

func TestGetAccountID_Integration(t *testing.T) {
	// Test that GetAccountID works with nested contexts
	parentCtx := context.Background()
	childCtx := context.WithValue(parentCtx, accountIDKey, uint64(777))

	// Should get the value from child context
	id, found := GetAccountID(childCtx)
	assert.Equal(t, uint64(777), id, "Should get account ID from child context")
	assert.True(t, found, "Account ID should be found in child context")
}
