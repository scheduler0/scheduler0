package executor

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"scheduler0-private/pkg/models"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
)

func newJobFailureNotifyExecutor() *jobExecutor {
	return &jobExecutor{
		logger: hclog.NewNullLogger(),
	}
}

func TestNotifyJobFailureAsync_SkipsUnsetAccount(t *testing.T) {
	je := newJobFailureNotifyExecutor()
	var calls atomic.Int32
	je.SetNotifyJobFailureCallback(func(job models.Job, failCount uint64, executionVersion uint64) error {
		calls.Add(1)
		return nil
	})

	je.notifyJobFailureAsync(models.Job{ID: 1, AccountId: 0}, 3, 1)
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, int32(0), calls.Load())
}

func TestNotifyJobFailureAsync_InvokesCallbackForSystemAccount(t *testing.T) {
	je := newJobFailureNotifyExecutor()
	done := make(chan struct{})
	je.SetNotifyJobFailureCallback(func(job models.Job, failCount uint64, executionVersion uint64) error {
		close(done)
		return nil
	})

	je.notifyJobFailureAsync(models.Job{ID: 5, AccountId: 1}, 2, 3)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("callback was not invoked for system account")
	}
}

func TestNotifyJobFailureAsync_InvokesCallback(t *testing.T) {
	je := newJobFailureNotifyExecutor()
	var (
		mu   sync.Mutex
		got  models.Job
		fc   uint64
		ev   uint64
		done = make(chan struct{})
	)
	je.SetNotifyJobFailureCallback(func(job models.Job, failCount uint64, executionVersion uint64) error {
		mu.Lock()
		got = job
		fc = failCount
		ev = executionVersion
		mu.Unlock()
		close(done)
		return nil
	})

	je.notifyJobFailureAsync(models.Job{ID: 99, AccountId: 2, ProjectID: 5}, 3, 7)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("callback was not invoked")
	}

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, uint64(99), got.ID)
	assert.Equal(t, uint64(2), got.AccountId)
	assert.Equal(t, uint64(3), fc)
	assert.Equal(t, uint64(7), ev)
}

func TestNotifyJobFailureAsync_NoopWithoutCallback(t *testing.T) {
	je := newJobFailureNotifyExecutor()
	// Should not panic when callback is unset.
	je.notifyJobFailureAsync(models.Job{ID: 1, AccountId: 2}, 1, 1)
}
