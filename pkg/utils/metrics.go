package utils

import (
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// TCPServerMetrics tracks metrics for TCP server operations, specifically
// uncommitted logs fetch requests (phase 2) to monitor timeout vs. success rates
type TCPServerMetrics struct {
	// Counters
	uncommittedLogsFetchTotal           int64
	uncommittedLogsFetchSuccess         int64
	uncommittedLogsFetchConnectionReset int64
	uncommittedLogsFetchOtherErrors     int64

	// Duration tracking (using atomic for min/max, mutex for sum/count for averages)
	waitDurationSum   int64 // nanoseconds
	waitDurationCount int64
	waitDurationMin   int64 // nanoseconds
	waitDurationMax   int64 // nanoseconds

	writeDurationSum   int64 // nanoseconds
	writeDurationCount int64
	writeDurationMin   int64 // nanoseconds
	writeDurationMax   int64 // nanoseconds

	totalDurationSum   int64 // nanoseconds
	totalDurationCount int64
	totalDurationMin   int64 // nanoseconds
	totalDurationMax   int64 // nanoseconds

	mu sync.RWMutex // For reading snapshot safely
}

// NewTCPServerMetrics creates a new metrics instance
func NewTCPServerMetrics() *TCPServerMetrics {
	return &TCPServerMetrics{
		waitDurationMin:  int64(^uint64(0) >> 1), // Max int64
		writeDurationMin: int64(^uint64(0) >> 1),
		totalDurationMin: int64(^uint64(0) >> 1),
	}
}

// RecordTotal increments the total request counter
func (m *TCPServerMetrics) RecordTotal() {
	atomic.AddInt64(&m.uncommittedLogsFetchTotal, 1)
}

// RecordSuccess increments the success counter
func (m *TCPServerMetrics) RecordSuccess() {
	atomic.AddInt64(&m.uncommittedLogsFetchSuccess, 1)
}

// RecordError records an error, categorizing it as connection reset or other
func (m *TCPServerMetrics) RecordError(err error) {
	if err == nil {
		return
	}
	if isConnectionReset(err) {
		atomic.AddInt64(&m.uncommittedLogsFetchConnectionReset, 1)
	} else {
		atomic.AddInt64(&m.uncommittedLogsFetchOtherErrors, 1)
	}
}

// RecordWaitDuration records the duration spent waiting for async task completion
func (m *TCPServerMetrics) RecordWaitDuration(duration time.Duration) {
	nanos := int64(duration)
	atomic.AddInt64(&m.waitDurationSum, nanos)
	atomic.AddInt64(&m.waitDurationCount, 1)

	// Update min
	for {
		currentMin := atomic.LoadInt64(&m.waitDurationMin)
		if nanos >= currentMin {
			break
		}
		if atomic.CompareAndSwapInt64(&m.waitDurationMin, currentMin, nanos) {
			break
		}
	}

	// Update max
	for {
		currentMax := atomic.LoadInt64(&m.waitDurationMax)
		if nanos <= currentMax {
			break
		}
		if atomic.CompareAndSwapInt64(&m.waitDurationMax, currentMax, nanos) {
			break
		}
	}
}

// RecordWriteDuration records the duration of the write operation
func (m *TCPServerMetrics) RecordWriteDuration(duration time.Duration) {
	nanos := int64(duration)
	atomic.AddInt64(&m.writeDurationSum, nanos)
	atomic.AddInt64(&m.writeDurationCount, 1)

	// Update min
	for {
		currentMin := atomic.LoadInt64(&m.writeDurationMin)
		if nanos >= currentMin {
			break
		}
		if atomic.CompareAndSwapInt64(&m.writeDurationMin, currentMin, nanos) {
			break
		}
	}

	// Update max
	for {
		currentMax := atomic.LoadInt64(&m.writeDurationMax)
		if nanos <= currentMax {
			break
		}
		if atomic.CompareAndSwapInt64(&m.writeDurationMax, currentMax, nanos) {
			break
		}
	}
}

// RecordTotalDuration records the total duration (wait + write)
func (m *TCPServerMetrics) RecordTotalDuration(duration time.Duration) {
	nanos := int64(duration)
	atomic.AddInt64(&m.totalDurationSum, nanos)
	atomic.AddInt64(&m.totalDurationCount, 1)

	// Update min
	for {
		currentMin := atomic.LoadInt64(&m.totalDurationMin)
		if nanos >= currentMin {
			break
		}
		if atomic.CompareAndSwapInt64(&m.totalDurationMin, currentMin, nanos) {
			break
		}
	}

	// Update max
	for {
		currentMax := atomic.LoadInt64(&m.totalDurationMax)
		if nanos <= currentMax {
			break
		}
		if atomic.CompareAndSwapInt64(&m.totalDurationMax, currentMax, nanos) {
			break
		}
	}
}

// MetricsSnapshot represents a snapshot of current metrics
type MetricsSnapshot struct {
	Total           int64
	Success         int64
	ConnectionReset int64
	OtherErrors     int64

	WaitDurationAvg   time.Duration
	WaitDurationMin   time.Duration
	WaitDurationMax   time.Duration
	WaitDurationCount int64

	WriteDurationAvg   time.Duration
	WriteDurationMin   time.Duration
	WriteDurationMax   time.Duration
	WriteDurationCount int64

	TotalDurationAvg   time.Duration
	TotalDurationMin   time.Duration
	TotalDurationMax   time.Duration
	TotalDurationCount int64

	SuccessRate float64
	TimeoutRate float64
	ErrorRate   float64
}

// GetMetrics returns a snapshot of current metrics
func (m *TCPServerMetrics) GetMetrics() MetricsSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	total := atomic.LoadInt64(&m.uncommittedLogsFetchTotal)
	success := atomic.LoadInt64(&m.uncommittedLogsFetchSuccess)
	connectionReset := atomic.LoadInt64(&m.uncommittedLogsFetchConnectionReset)
	otherErrors := atomic.LoadInt64(&m.uncommittedLogsFetchOtherErrors)

	waitSum := atomic.LoadInt64(&m.waitDurationSum)
	waitCount := atomic.LoadInt64(&m.waitDurationCount)
	waitMin := atomic.LoadInt64(&m.waitDurationMin)
	waitMax := atomic.LoadInt64(&m.waitDurationMax)

	writeSum := atomic.LoadInt64(&m.writeDurationSum)
	writeCount := atomic.LoadInt64(&m.writeDurationCount)
	writeMin := atomic.LoadInt64(&m.writeDurationMin)
	writeMax := atomic.LoadInt64(&m.writeDurationMax)

	totalSum := atomic.LoadInt64(&m.totalDurationSum)
	totalCount := atomic.LoadInt64(&m.totalDurationCount)
	totalMin := atomic.LoadInt64(&m.totalDurationMin)
	totalMax := atomic.LoadInt64(&m.totalDurationMax)

	snapshot := MetricsSnapshot{
		Total:           total,
		Success:         success,
		ConnectionReset: connectionReset,
		OtherErrors:     otherErrors,
	}

	// Calculate wait duration stats
	if waitCount > 0 {
		snapshot.WaitDurationAvg = time.Duration(waitSum / waitCount)
		snapshot.WaitDurationMin = time.Duration(waitMin)
		snapshot.WaitDurationMax = time.Duration(waitMax)
		snapshot.WaitDurationCount = waitCount
	} else {
		snapshot.WaitDurationMin = 0
		snapshot.WaitDurationMax = 0
	}

	// Calculate write duration stats
	if writeCount > 0 {
		snapshot.WriteDurationAvg = time.Duration(writeSum / writeCount)
		snapshot.WriteDurationMin = time.Duration(writeMin)
		snapshot.WriteDurationMax = time.Duration(writeMax)
		snapshot.WriteDurationCount = writeCount
	} else {
		snapshot.WriteDurationMin = 0
		snapshot.WriteDurationMax = 0
	}

	// Calculate total duration stats
	if totalCount > 0 {
		snapshot.TotalDurationAvg = time.Duration(totalSum / totalCount)
		snapshot.TotalDurationMin = time.Duration(totalMin)
		snapshot.TotalDurationMax = time.Duration(totalMax)
		snapshot.TotalDurationCount = totalCount
	} else {
		snapshot.TotalDurationMin = 0
		snapshot.TotalDurationMax = 0
	}

	// Calculate rates
	if total > 0 {
		snapshot.SuccessRate = float64(success) / float64(total) * 100
		snapshot.TimeoutRate = float64(connectionReset) / float64(total) * 100
		snapshot.ErrorRate = float64(connectionReset+otherErrors) / float64(total) * 100
	}

	return snapshot
}

// Reset resets all metrics (useful for testing)
func (m *TCPServerMetrics) Reset() {
	atomic.StoreInt64(&m.uncommittedLogsFetchTotal, 0)
	atomic.StoreInt64(&m.uncommittedLogsFetchSuccess, 0)
	atomic.StoreInt64(&m.uncommittedLogsFetchConnectionReset, 0)
	atomic.StoreInt64(&m.uncommittedLogsFetchOtherErrors, 0)

	atomic.StoreInt64(&m.waitDurationSum, 0)
	atomic.StoreInt64(&m.waitDurationCount, 0)
	atomic.StoreInt64(&m.waitDurationMin, int64(^uint64(0)>>1))
	atomic.StoreInt64(&m.waitDurationMax, 0)

	atomic.StoreInt64(&m.writeDurationSum, 0)
	atomic.StoreInt64(&m.writeDurationCount, 0)
	atomic.StoreInt64(&m.writeDurationMin, int64(^uint64(0)>>1))
	atomic.StoreInt64(&m.writeDurationMax, 0)

	atomic.StoreInt64(&m.totalDurationSum, 0)
	atomic.StoreInt64(&m.totalDurationCount, 0)
	atomic.StoreInt64(&m.totalDurationMin, int64(^uint64(0)>>1))
	atomic.StoreInt64(&m.totalDurationMax, 0)
}

// isConnectionReset checks if an error is a connection reset error
func isConnectionReset(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "connection reset by peer") ||
		strings.Contains(errStr, "broken pipe") ||
		strings.Contains(errStr, "use of closed network connection") ||
		strings.Contains(errStr, "write: connection reset") ||
		strings.Contains(errStr, "read: connection reset")
}
