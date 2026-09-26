package models

import (
	"container/heap"
	"testing"
	"time"
)

func TestRescheduleBucketHeap_Len(t *testing.T) {
	h := RescheduleBucketHeap{}
	if h.Len() != 0 {
		t.Errorf("Expected empty heap to have length 0, got %d", h.Len())
	}

	h = RescheduleBucketHeap{
		RescheduleBucket{Cutoff: time.Now()},
		RescheduleBucket{Cutoff: time.Now()},
	}
	if h.Len() != 2 {
		t.Errorf("Expected heap to have length 2, got %d", h.Len())
	}
}

func TestRescheduleBucketHeap_Less(t *testing.T) {
	now := time.Now()
	h := RescheduleBucketHeap{
		RescheduleBucket{Cutoff: now.Add(1 * time.Hour)},
		RescheduleBucket{Cutoff: now},
	}

	// First element has later cutoff, so Less(0, 1) should return false
	if h.Less(0, 1) {
		t.Errorf("Expected Less(0, 1) to return false (first cutoff is after second), got true")
	}

	// Less(1, 0) should return true (second cutoff is before first)
	if !h.Less(1, 0) {
		t.Errorf("Expected Less(1, 0) to return true (second cutoff is before first), got false")
	}
}

func TestRescheduleBucketHeap_Swap(t *testing.T) {
	now := time.Now()
	h := RescheduleBucketHeap{
		RescheduleBucket{Cutoff: now, BucketKey: "first"},
		RescheduleBucket{Cutoff: now.Add(1 * time.Hour), BucketKey: "second"},
	}

	originalFirst := h[0]
	originalSecond := h[1]

	h.Swap(0, 1)

	if h[0].BucketKey != originalSecond.BucketKey {
		t.Errorf("Expected first element to be swapped, got %s", h[0].BucketKey)
	}
	if h[1].BucketKey != originalFirst.BucketKey {
		t.Errorf("Expected second element to be swapped, got %s", h[1].BucketKey)
	}
}

func TestRescheduleBucketHeap_Push(t *testing.T) {
	h := &RescheduleBucketHeap{}
	heap.Init(h)

	bucket := RescheduleBucket{
		Cutoff:    time.Now(),
		BucketKey: "test-key",
	}

	h.Push(bucket)

	if h.Len() != 1 {
		t.Errorf("Expected heap to have length 1 after push, got %d", h.Len())
	}

	if (*h)[0].BucketKey != "test-key" {
		t.Errorf("Expected pushed bucket to have key 'test-key', got %s", (*h)[0].BucketKey)
	}
}

func TestRescheduleBucketHeap_Pop(t *testing.T) {
	now := time.Now()
	h := &RescheduleBucketHeap{
		RescheduleBucket{Cutoff: now, BucketKey: "first"},
		RescheduleBucket{Cutoff: now.Add(1 * time.Hour), BucketKey: "second"},
	}
	heap.Init(h)

	originalLen := h.Len()
	popped := h.Pop().(RescheduleBucket)

	if h.Len() != originalLen-1 {
		t.Errorf("Expected heap length to decrease by 1 after pop, got %d", h.Len())
	}

	if popped.BucketKey != "first" && popped.BucketKey != "second" {
		t.Errorf("Expected popped bucket to be one of the original buckets, got %s", popped.BucketKey)
	}
}

func TestRescheduleBucketHeap_Peek(t *testing.T) {
	now := time.Now()
	h := &RescheduleBucketHeap{
		RescheduleBucket{Cutoff: now, BucketKey: "first"},
		RescheduleBucket{Cutoff: now.Add(1 * time.Hour), BucketKey: "second"},
	}
	heap.Init(h)

	bucket, ok := h.Peek()
	if !ok {
		t.Errorf("Expected Peek to return true for non-empty heap, got false")
	}

	if bucket.BucketKey != "first" {
		t.Errorf("Expected Peek to return the minimum element (first), got %s", bucket.BucketKey)
	}

	// Test Peek on empty heap
	emptyHeap := &RescheduleBucketHeap{}
	bucket, ok = emptyHeap.Peek()
	if ok {
		t.Errorf("Expected Peek to return false for empty heap, got true")
	}
	if bucket.BucketKey != "" {
		t.Errorf("Expected Peek to return empty bucket for empty heap, got %s", bucket.BucketKey)
	}
}

func TestRescheduleBucketHeap_HeapOrder(t *testing.T) {
	now := time.Now()
	h := &RescheduleBucketHeap{
		RescheduleBucket{Cutoff: now.Add(3 * time.Hour), BucketKey: "third"},
		RescheduleBucket{Cutoff: now, BucketKey: "first"},
		RescheduleBucket{Cutoff: now.Add(2 * time.Hour), BucketKey: "second"},
	}
	heap.Init(h)

	// After heap initialization, the minimum should be at index 0
	first, _ := h.Peek()
	if first.Cutoff.After(now.Add(1 * time.Minute)) {
		t.Errorf("Expected heap to be ordered with minimum cutoff first")
	}

	// Pop all elements and verify they come out in order
	var lastCutoff time.Time
	firstPop := true
	for h.Len() > 0 {
		popped := heap.Pop(h).(RescheduleBucket)
		if !firstPop && popped.Cutoff.Before(lastCutoff) {
			t.Errorf("Expected heap to return elements in ascending order, got %v before %v", popped.Cutoff, lastCutoff)
		}
		lastCutoff = popped.Cutoff
		firstPop = false
	}
}
