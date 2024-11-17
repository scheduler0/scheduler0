package utils

import (
	"runtime"
	"time"
)

// MemoryLimitChecker is a type that checks if a program's memory usage
// exceeds a specified limit and sends a panic signal on the given channel
// if it does.
type MemoryLimitChecker struct {
	maxMem        uint64        // Maximum memory usage in bytes
	panicCh       chan<- bool   // Channel to send panic signals on
	checkInterval time.Duration // Interval between memory usage checks
	stopCh        chan bool     // Channel to stop the memory usage checker goroutine
	memStats      *runtime.MemStats
}

// NewMemoryLimitChecker returns a new MemoryLimitChecker instance
// with the specified maximum memory usage limit in megabytes.
func NewMemoryLimitChecker(maxMemMB uint64, memStats *runtime.MemStats, panicCh chan<- bool, checkInterval time.Duration) *MemoryLimitChecker {
	return &MemoryLimitChecker{
		maxMem:        maxMemMB * 1024 * 1024,
		panicCh:       panicCh,
		checkInterval: checkInterval,
		stopCh:        make(chan bool),
		memStats:      memStats,
	}
}

// CheckMemoryUsage checks the program's memory usage and sends a panic
// signal on the given channel if it exceeds the maximum memory usage limit.
func (m *MemoryLimitChecker) CheckMemoryUsage() {
	runtime.ReadMemStats(m.memStats)
	if uint64(m.memStats.Sys) >= (m.maxMem*7)/10 {
		m.panicCh <- true
	}
}

// StartMemoryUsageChecker starts a goroutine that periodically checks the program's memory usage
// and sends a panic signal on the given channel if it exceeds the maximum memory usage limit.
// The goroutine runs until the StopMemoryUsageChecker method is called on the memory checker instance.
func (m *MemoryLimitChecker) StartMemoryUsageChecker() {
	ticker := time.NewTicker(m.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.CheckMemoryUsage()
		case <-m.stopCh:
			return
		}
	}
}

// StopMemoryUsageChecker stops the goroutine that periodically checks the program's memory usage.
func (m *MemoryLimitChecker) StopMemoryUsageChecker() {
	close(m.stopCh)
}
