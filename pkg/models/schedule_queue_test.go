package models

import (
	"testing"
	"time"
)

func TestNewScheduleQueue(t *testing.T) {
	queue := NewScheduleQueue()
	if queue == nil {
		t.Errorf("Expected NewScheduleQueue to return a non-nil queue")
	}

	if queue.Len() != 0 {
		t.Errorf("Expected new queue to have length 0, got %d", queue.Len())
	}
}

func TestScheduleQueue_AddJob(t *testing.T) {
	queue := NewScheduleQueue()
	now := time.Now()

	job := JobScheduleKey{
		JobId:         1,
		ExecutionTime: now.Add(1 * time.Hour),
	}

	queue.AddJob(job)

	if queue.Len() != 1 {
		t.Errorf("Expected queue to have length 1 after adding job, got %d", queue.Len())
	}
}

func TestScheduleQueue_AddJob_ZeroJobId(t *testing.T) {
	queue := NewScheduleQueue()
	now := time.Now()

	job := JobScheduleKey{
		JobId:         0, // Invalid
		ExecutionTime: now.Add(1 * time.Hour),
	}

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected AddJob to panic when JobId is 0")
		}
	}()

	queue.AddJob(job)
}

func TestScheduleQueue_AddJob_ZeroExecutionTime(t *testing.T) {
	queue := NewScheduleQueue()

	job := JobScheduleKey{
		JobId:         1,
		ExecutionTime: time.Time{}, // Zero time
	}

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected AddJob to panic when ExecutionTime is zero")
		}
	}()

	queue.AddJob(job)
}

func TestScheduleQueue_Pop(t *testing.T) {
	queue := NewScheduleQueue()
	now := time.Now()

	job1 := JobScheduleKey{
		JobId:         1,
		ExecutionTime: now.Add(2 * time.Hour),
	}
	job2 := JobScheduleKey{
		JobId:         2,
		ExecutionTime: now.Add(1 * time.Hour),
	}

	queue.AddJob(job1)
	queue.AddJob(job2)

	// Pop should return the job with earliest execution time
	popped := queue.Pop()
	if popped.JobId != job2.JobId {
		t.Errorf("Expected Pop to return job with earliest execution time (job2), got job %d", popped.JobId)
	}

	if queue.Len() != 1 {
		t.Errorf("Expected queue to have length 1 after pop, got %d", queue.Len())
	}

	// Pop the remaining job
	popped = queue.Pop()
	if popped.JobId != job1.JobId {
		t.Errorf("Expected Pop to return remaining job (job1), got job %d", popped.JobId)
	}

	// Pop from empty queue should return zero value
	popped = queue.Pop()
	if popped.JobId != 0 {
		t.Errorf("Expected Pop from empty queue to return zero value, got job %d", popped.JobId)
	}
}

func TestScheduleQueue_Peek(t *testing.T) {
	queue := NewScheduleQueue()
	now := time.Now()

	job1 := JobScheduleKey{
		JobId:         1,
		ExecutionTime: now.Add(2 * time.Hour),
	}
	job2 := JobScheduleKey{
		JobId:         2,
		ExecutionTime: now.Add(1 * time.Hour),
	}

	queue.AddJob(job1)
	queue.AddJob(job2)

	// Peek should return the job with earliest execution time without removing it
	peeked := queue.Peek()
	if peeked.JobId != job2.JobId {
		t.Errorf("Expected Peek to return job with earliest execution time (job2), got job %d", peeked.JobId)
	}

	// Length should remain the same
	if queue.Len() != 2 {
		t.Errorf("Expected queue length to remain 2 after peek, got %d", queue.Len())
	}

	// Peek from empty queue should return zero value
	queue.Clear()
	peeked = queue.Peek()
	if peeked.JobId != 0 {
		t.Errorf("Expected Peek from empty queue to return zero value, got job %d", peeked.JobId)
	}
}

func TestScheduleQueue_RemoveJob(t *testing.T) {
	queue := NewScheduleQueue()
	now := time.Now()

	job1 := JobScheduleKey{
		JobId:         1,
		ExecutionTime: now.Add(1 * time.Hour),
	}
	job2 := JobScheduleKey{
		JobId:         2,
		ExecutionTime: now.Add(2 * time.Hour),
	}
	job3 := JobScheduleKey{
		JobId:         3,
		ExecutionTime: now.Add(3 * time.Hour),
	}

	queue.AddJob(job1)
	queue.AddJob(job2)
	queue.AddJob(job3)

	if queue.Len() != 3 {
		t.Errorf("Expected queue to have length 3, got %d", queue.Len())
	}

	// Remove job2
	queue.RemoveJob(job2)

	if queue.Len() != 2 {
		t.Errorf("Expected queue to have length 2 after removing job, got %d", queue.Len())
	}

	// Verify job2 is removed
	for queue.Len() > 0 {
		popped := queue.Pop()
		if popped.JobId == job2.JobId {
			t.Errorf("Expected job2 to be removed, but it was found in queue")
		}
	}
}

func TestScheduleQueue_RemoveJob_NonExistent(t *testing.T) {
	queue := NewScheduleQueue()
	now := time.Now()

	job1 := JobScheduleKey{
		JobId:         1,
		ExecutionTime: now.Add(1 * time.Hour),
	}
	job2 := JobScheduleKey{
		JobId:         2,
		ExecutionTime: now.Add(2 * time.Hour),
	}

	queue.AddJob(job1)

	// Try to remove non-existent job
	queue.RemoveJob(job2)

	if queue.Len() != 1 {
		t.Errorf("Expected queue to still have length 1 after removing non-existent job, got %d", queue.Len())
	}
}

func TestScheduleQueue_Clear(t *testing.T) {
	queue := NewScheduleQueue()
	now := time.Now()

	job1 := JobScheduleKey{
		JobId:         1,
		ExecutionTime: now.Add(1 * time.Hour),
	}
	job2 := JobScheduleKey{
		JobId:         2,
		ExecutionTime: now.Add(2 * time.Hour),
	}

	queue.AddJob(job1)
	queue.AddJob(job2)

	if queue.Len() != 2 {
		t.Errorf("Expected queue to have length 2, got %d", queue.Len())
	}

	queue.Clear()

	if queue.Len() != 0 {
		t.Errorf("Expected queue to have length 0 after clear, got %d", queue.Len())
	}
}

func TestScheduleQueue_Ordering(t *testing.T) {
	queue := NewScheduleQueue()
	now := time.Now()

	// Add jobs in random order
	jobs := []JobScheduleKey{
		{JobId: 3, ExecutionTime: now.Add(3 * time.Hour)},
		{JobId: 1, ExecutionTime: now.Add(1 * time.Hour)},
		{JobId: 5, ExecutionTime: now.Add(5 * time.Hour)},
		{JobId: 2, ExecutionTime: now.Add(2 * time.Hour)},
		{JobId: 4, ExecutionTime: now.Add(4 * time.Hour)},
	}

	for _, job := range jobs {
		queue.AddJob(job)
	}

	// Pop all jobs and verify they come out in order
	var lastTime time.Time
	firstPop := true
	for queue.Len() > 0 {
		popped := queue.Pop()
		if !firstPop && popped.ExecutionTime.Before(lastTime) {
			t.Errorf("Expected jobs to be popped in ascending order of execution time")
		}
		lastTime = popped.ExecutionTime
		firstPop = false
	}
}

