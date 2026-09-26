package models

import "time"

// RescheduleJobEntry represents a job that should be considered for rescheduling,
// along with its next execution time and the state that led to this reschedule.
type RescheduleJobEntry struct {
	Job               Job
	State             JobExecutionLogState
	NextExecutionTime time.Time
}

// RescheduleBucket groups jobs by a cutoff time derived from their next execution times.
type RescheduleBucket struct {
	Cutoff    time.Time
	BucketKey string
}

// RescheduleBucketHeap is a min-heap of reschedule buckets ordered by their cutoff time.
type RescheduleBucketHeap []RescheduleBucket

func (h RescheduleBucketHeap) Len() int { return len(h) }

func (h RescheduleBucketHeap) Less(i, j int) bool {
	return h[i].Cutoff.Before(h[j].Cutoff)
}

func (h RescheduleBucketHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h *RescheduleBucketHeap) Push(x interface{}) {
	*h = append(*h, x.(RescheduleBucket))
}

func (h *RescheduleBucketHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

func (h *RescheduleBucketHeap) Peek() (RescheduleBucket, bool) {
	if len(*h) == 0 {
		return RescheduleBucket{}, false
	}
	return (*h)[0], true
}


