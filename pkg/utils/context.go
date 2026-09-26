package utils

import (
	"context"
	"fmt"
	"log"

	"github.com/hashicorp/go-hclog"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const (
	requestIDKey  contextKey = "scheduler0.RequestID"
	accountIDKey  contextKey = "scheduler0.AccountID"
	credentialKey contextKey = "scheduler0.Credential"
)

// RequestIDContextKey returns the context key for request ID.
// This is exported so middleware can use it to set request IDs in context.
func RequestIDContextKey() contextKey {
	return requestIDKey
}

// AccountIDContextKey returns the context key for account ID.
// This is exported so middleware can use it to set account IDs in context.
func AccountIDContextKey() contextKey {
	return accountIDKey
}

// CredentialContextKey returns the context key for the resolved credential.
// The auth middleware stores the validated credential in context so downstream
// handlers and log lines can reference it.
func CredentialContextKey() contextKey {
	return credentialKey
}

// GetRequestID extracts the request ID from context.
// Returns empty string if request ID is not found in context.
func GetRequestID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	requestID, ok := ctx.Value(requestIDKey).(string)
	if !ok {
		return ""
	}
	return requestID
}

// GetAccountID extracts the account ID from context.
// Returns the account ID and a boolean indicating if it was found.
// Returns (0, false) if account ID is not found in context.
func GetAccountID(ctx context.Context) (uint64, bool) {
	if ctx == nil {
		return 0, false
	}
	accountID, ok := ctx.Value(accountIDKey).(uint64)
	if !ok {
		return 0, false
	}
	return accountID, true
}

// LogWithRequestID logs a message with request ID prefix for standard log.Logger.
// If format is empty, treats args as a pre-formatted message (concatenates all args).
// If format is provided, uses it as a format string with args.
func LogWithRequestID(logger *log.Logger, requestID string, format string, args ...interface{}) {
	var message string

	if format == "" {
		// No format string provided, concatenate all args as a pre-formatted message
		if len(args) > 0 {
			message = fmt.Sprint(args...)
		} else {
			message = ""
		}
	} else {
		// Format string provided, use it with args
		message = fmt.Sprintf(format, args...)
	}

	// Prepend request ID prefix if available
	if requestID != "" {
		prefix := "[request_id=" + requestID + "] "
		if message != "" {
			logger.Println(prefix + message)
		} else {
			logger.Println(prefix)
		}
	} else {
		// No request ID, log message as-is
		if message != "" {
			logger.Println(message)
		}
	}
}

// LogWithRequestIDHCLog logs a message with request ID for hclog.Logger.
// This uses structured logging with request_id as a key-value pair.
func LogWithRequestIDHCLog(logger hclog.Logger, level string, msg string, requestID string, additionalKeysAndValues ...interface{}) {
	keysAndValues := []interface{}{"request_id", requestID}
	keysAndValues = append(keysAndValues, additionalKeysAndValues...)

	switch level {
	case "trace":
		logger.Trace(msg, keysAndValues...)
	case "debug":
		logger.Debug(msg, keysAndValues...)
	case "info":
		logger.Info(msg, keysAndValues...)
	case "warn":
		logger.Warn(msg, keysAndValues...)
	case "error":
		logger.Error(msg, keysAndValues...)
	default:
		logger.Info(msg, keysAndValues...)
	}
}
