package models

import (
	"container/heap"
	"sync"
)

type scheduleQueue struct {
	jobs *jobHeap
	mtx  sync.Mutex
}

type jobHeap []JobScheduleKey

func (h jobHeap) Len() int { return len(h) }

func (h jobHeap) Less(i, j int) bool {
	iNextTime := h[i].ExecutionTime
	jNextTime := h[j].ExecutionTime

	// Compare times in UTC to ensure reliable ordering across different timezones
	return iNextTime.UTC().Before(jNextTime.UTC())
}

func (h jobHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h *jobHeap) Push(x interface{}) {
	*h = append(*h, x.(JobScheduleKey))
}

func (h *jobHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

func (h *jobHeap) Peek() JobScheduleKey {
	if len(*h) == 0 {
		return JobScheduleKey{}
	}
	return (*h)[0]
}

type ScheduleQueue interface {
	AddJob(job JobScheduleKey)
	RemoveJob(job JobScheduleKey)
	Pop() JobScheduleKey
	Peek() JobScheduleKey
	Len() int
	Clear()
	GetAllItems() []JobScheduleKey
}

func NewScheduleQueue() ScheduleQueue {
	h := &jobHeap{}
	heap.Init(h)
	return &scheduleQueue{
		jobs: h,
	}
}

func (q *scheduleQueue) AddJob(job JobScheduleKey) {
	if job.JobId == 0 {
		panic("job id is 0")
	}

	if job.ExecutionTime.IsZero() {
		panic("execution time is zero")
	}
	heap.Push(q.jobs, job)
}

func (q *scheduleQueue) RemoveJob(job JobScheduleKey) {
	// Since we can't efficiently remove an arbitrary element from a heap,
	// we'll need to rebuild the heap without the job to remove
	newHeap := &jobHeap{}
	heap.Init(newHeap)

	for q.jobs.Len() > 0 {
		j := heap.Pop(q.jobs).(JobScheduleKey)
		if j.JobId != job.JobId {
			heap.Push(newHeap, j)
		}
	}

	q.jobs = newHeap
}

func (q *scheduleQueue) Pop() JobScheduleKey {
	if q.jobs.Len() == 0 {
		return JobScheduleKey{}
	}

	return heap.Pop(q.jobs).(JobScheduleKey)
}

func (q *scheduleQueue) Len() int {
	return q.jobs.Len()
}

func (q *scheduleQueue) Peek() JobScheduleKey {
	return q.jobs.Peek()
}

func (q *scheduleQueue) Clear() {
	q.jobs = &jobHeap{}
	heap.Init(q.jobs)
}

func (q *scheduleQueue) GetAllItems() []JobScheduleKey {
	// Extract all items from the heap without modifying it
	// We'll create a copy of the heap and pop all items
	tempHeap := make(jobHeap, len(*q.jobs))
	copy(tempHeap, *q.jobs)
	heap.Init(&tempHeap)

	result := make([]JobScheduleKey, 0, tempHeap.Len())
	for tempHeap.Len() > 0 {
		result = append(result, heap.Pop(&tempHeap).(JobScheduleKey))
	}
	return result
}
