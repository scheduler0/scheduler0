package executor

import (
	"context"
	"sort"
	"scheduler0/pkg/mocks"
	"scheduler0/pkg/models"
	"scheduler0/pkg/utils"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// These tests exercise ListenForJobsToInvokeV1 in isolation, using lightweight
// mocks for the job/executor repositories and a recording webhook executor.
// The recording executor never invokes the success/failure callbacks, so the
// reschedule pipeline never runs. That keeps each test focused purely on the
// behaviour of the scheduling loop and invokeJobsV1: draining due jobs, skipping
// jobs missing from the cache, grouping by executor, and (for executors with
// payload aggregation enabled) sub-grouping by fire time.

type recordedInvocation struct {
	kind       string // "single" or "batch"
	executorID uint64
	jobIDs     []uint64
}

// recordingWebhookExecutor implements webhook.WebhookExecutor. Instead of making
// real HTTP calls it records each invocation and publishes it on a channel so the
// test can assert on what was dispatched. Callbacks are intentionally ignored.
type recordingWebhookExecutor struct {
	events chan recordedInvocation
}

func newRecordingWebhookExecutor() *recordingWebhookExecutor {
	return &recordingWebhookExecutor{events: make(chan recordedInvocation, 128)}
}

func (r *recordingWebhookExecutor) ExecuteWebhookJob(
	executor models.JobExecutor,
	pendingJob models.JobInvocationPayload,
	successCallback func(job models.Job),
	errorCallback func(job models.Job),
) {
	r.events <- recordedInvocation{
		kind:       "single",
		executorID: executor.ID,
		jobIDs:     []uint64{pendingJob.Job.ID},
	}
}

func (r *recordingWebhookExecutor) ExecuteWebhookJobBatch(
	executor models.JobExecutor,
	pendingJobs models.AggregatedJobInvocationPayload,
	successCallback func(jobs []models.Job),
	errorCallback func(jobs []models.Job),
) {
	ids := make([]uint64, 0, len(pendingJobs.Jobs))
	for _, entry := range pendingJobs.Jobs {
		ids = append(ids, entry.Job.ID)
	}
	r.events <- recordedInvocation{
		kind:       "batch",
		executorID: executor.ID,
		jobIDs:     ids,
	}
}

func newWebhookJob(id, executorID uint64) models.Job {
	eid := executorID
	return models.Job{
		ID:         id,
		ExecutorId: &eid,
		Status:     models.JobStatusActive,
		AccountId:  1,
	}
}

func newWebhookExecutorModel(id uint64, aggregate bool) models.JobExecutor {
	return models.JobExecutor{
		ID:                 id,
		Type:               string(models.ExecutorTypeWebhookUrl),
		PayloadAggregation: aggregate,
	}
}

// newTestJobExecutor builds a jobExecutor wired with the minimal set of
// dependencies that ListenForJobsToInvokeV1 -> invokeJobsV1 touch. The returned
// cancel func stops the loop goroutine and the dispatcher workers.
func newTestJobExecutor(
	t *testing.T,
	rec *recordingWebhookExecutor,
	jobs []models.Job,
	executors []models.JobExecutor,
) (*jobExecutor, context.CancelFunc) {
	t.Helper()

	logger := hclog.NewNullLogger()
	ctx, cancel := context.WithCancel(context.Background())

	dispatcher := utils.NewDispatcher(ctx, int64(4), int64(64))
	dispatcher.Run()

	jobByID := make(map[uint64]models.Job, len(jobs))
	for _, j := range jobs {
		jobByID[j.ID] = j
	}
	jobRepo := mocks.NewMockJobRepo(t)
	jobRepo.On("BatchGetJobsByID", mock.Anything).Return(
		func(ids []uint64) ([]models.Job, *utils.GenericError) {
			out := make([]models.Job, 0, len(ids))
			for _, id := range ids {
				if j, ok := jobByID[id]; ok {
					out = append(out, j)
				}
			}
			return out, nil
		},
	).Maybe()

	execByID := make(map[uint64]models.JobExecutor, len(executors))
	for _, e := range executors {
		execByID[e.ID] = e
	}
	executorRepo := mocks.NewMockJobExecutorRepo(t)
	executorRepo.On("BatchGetByIds", mock.Anything).Return(
		func(ids []uint64) ([]models.JobExecutor, *utils.GenericError) {
			out := make([]models.JobExecutor, 0, len(ids))
			for _, id := range ids {
				if e, ok := execByID[id]; ok {
					out = append(out, e)
				}
			}
			return out, nil
		},
	).Maybe()

	je := &jobExecutor{
		logger:                  logger,
		context:                 ctx,
		cancelReq:               cancel,
		dispatcher:              dispatcher,
		jobRepo:                 jobRepo,
		jobExecutorRepo:         executorRepo,
		webhookExecutionHandler: rec,
		scheduleQueue:           models.NewScheduleQueue(),
		jobAddedChan:            make(chan struct{}, 1),
	}
	return je, cancel
}

// seedScheduledJob places a job in both the in-memory execution cache (keyed by
// fire time, used for aggregation grouping) and the schedule queue (keyed by the
// time it becomes due).
func seedScheduledJob(je *jobExecutor, id uint64, dueAt, fireTime time.Time) {
	je.jobExecutionsCache.Store(id, &models.JobSchedule{
		Job: models.Job{ID: id},
		MemExecution: models.MemJobExecution{
			LastState:             models.ExecutionLogScheduleState,
			NextExecutionDatetime: fireTime,
		},
	})
	je.scheduleQueue.AddJob(models.JobScheduleKey{JobId: id, ExecutionTime: dueAt})
}

func collectInvocations(t *testing.T, rec *recordingWebhookExecutor, want int, timeout time.Duration) []recordedInvocation {
	t.Helper()
	got := make([]recordedInvocation, 0, want)
	deadline := time.After(timeout)
	for len(got) < want {
		select {
		case ev := <-rec.events:
			got = append(got, ev)
		case <-deadline:
			t.Fatalf("timed out waiting for %d invocations, got %d: %+v", want, len(got), got)
		}
	}
	return got
}

func expectNoFurtherInvocation(t *testing.T, rec *recordingWebhookExecutor, within time.Duration) {
	t.Helper()
	select {
	case ev := <-rec.events:
		t.Fatalf("expected no further invocation, but got %+v", ev)
	case <-time.After(within):
	}
}

func sortedIDs(ids []uint64) []uint64 {
	out := append([]uint64(nil), ids...)
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func TestListenForJobsToInvokeV1_SingleDueJobIsInvoked(t *testing.T) {
	rec := newRecordingWebhookExecutor()
	je, cancel := newTestJobExecutor(t, rec,
		[]models.Job{newWebhookJob(1, 100)},
		[]models.JobExecutor{newWebhookExecutorModel(100, false)},
	)
	defer cancel()

	past := time.Now().Add(-1 * time.Second)
	seedScheduledJob(je, 1, past, past)

	go je.ListenForJobsToInvokeV1()

	got := collectInvocations(t, rec, 1, 3*time.Second)
	assert.Equal(t, "single", got[0].kind)
	assert.Equal(t, uint64(100), got[0].executorID)
	assert.Equal(t, []uint64{1}, got[0].jobIDs)

	// The job was popped and never rescheduled (no callback), so it must not fire again.
	expectNoFurtherInvocation(t, rec, 500*time.Millisecond)
}

func TestListenForJobsToInvokeV1_FutureJobIsNotInvoked(t *testing.T) {
	rec := newRecordingWebhookExecutor()
	je, cancel := newTestJobExecutor(t, rec,
		[]models.Job{newWebhookJob(1, 100)},
		[]models.JobExecutor{newWebhookExecutorModel(100, false)},
	)
	defer cancel()

	future := time.Now().Add(30 * time.Second)
	seedScheduledJob(je, 1, future, future)

	go je.ListenForJobsToInvokeV1()

	expectNoFurtherInvocation(t, rec, 1*time.Second)
}

func TestListenForJobsToInvokeV1_DrainsMultipleDueJobsWithoutAggregation(t *testing.T) {
	rec := newRecordingWebhookExecutor()
	je, cancel := newTestJobExecutor(t, rec,
		[]models.Job{newWebhookJob(1, 100), newWebhookJob(2, 100), newWebhookJob(3, 100)},
		[]models.JobExecutor{newWebhookExecutorModel(100, false)},
	)
	defer cancel()

	past := time.Now().Add(-1 * time.Second)
	seedScheduledJob(je, 1, past, past)
	seedScheduledJob(je, 2, past, past)
	seedScheduledJob(je, 3, past, past)

	go je.ListenForJobsToInvokeV1()

	got := collectInvocations(t, rec, 3, 3*time.Second)

	invokedIDs := make([]uint64, 0, 3)
	for _, ev := range got {
		assert.Equal(t, "single", ev.kind)
		assert.Equal(t, uint64(100), ev.executorID)
		assert.Len(t, ev.jobIDs, 1)
		invokedIDs = append(invokedIDs, ev.jobIDs[0])
	}
	assert.Equal(t, []uint64{1, 2, 3}, sortedIDs(invokedIDs))

	expectNoFurtherInvocation(t, rec, 500*time.Millisecond)
}

func TestListenForJobsToInvokeV1_AggregatesJobsWithSameFireTime(t *testing.T) {
	rec := newRecordingWebhookExecutor()
	je, cancel := newTestJobExecutor(t, rec,
		[]models.Job{newWebhookJob(1, 100), newWebhookJob(2, 100), newWebhookJob(3, 100)},
		[]models.JobExecutor{newWebhookExecutorModel(100, true)},
	)
	defer cancel()

	past := time.Now().Add(-1 * time.Second)
	fireTime := time.Now().Add(-1 * time.Second)
	seedScheduledJob(je, 1, past, fireTime)
	seedScheduledJob(je, 2, past, fireTime)
	seedScheduledJob(je, 3, past, fireTime)

	go je.ListenForJobsToInvokeV1()

	got := collectInvocations(t, rec, 1, 3*time.Second)
	assert.Equal(t, "batch", got[0].kind)
	assert.Equal(t, uint64(100), got[0].executorID)
	assert.Equal(t, []uint64{1, 2, 3}, sortedIDs(got[0].jobIDs))

	expectNoFurtherInvocation(t, rec, 500*time.Millisecond)
}

func TestListenForJobsToInvokeV1_AggregationSubgroupsByFireTime(t *testing.T) {
	rec := newRecordingWebhookExecutor()
	je, cancel := newTestJobExecutor(t, rec,
		[]models.Job{
			newWebhookJob(1, 100), newWebhookJob(2, 100),
			newWebhookJob(3, 100), newWebhookJob(4, 100),
		},
		[]models.JobExecutor{newWebhookExecutorModel(100, true)},
	)
	defer cancel()

	past := time.Now().Add(-1 * time.Second)
	fireTimeA := time.Now().Add(-1 * time.Second)
	fireTimeB := time.Now().Add(1 * time.Minute)
	// All four are due now, but jobs 1,2 share one fire time while 3,4 share another,
	// so aggregation must produce two separate batches.
	seedScheduledJob(je, 1, past, fireTimeA)
	seedScheduledJob(je, 2, past, fireTimeA)
	seedScheduledJob(je, 3, past, fireTimeB)
	seedScheduledJob(je, 4, past, fireTimeB)

	go je.ListenForJobsToInvokeV1()

	got := collectInvocations(t, rec, 2, 3*time.Second)

	batches := make(map[string][]uint64)
	for _, ev := range got {
		assert.Equal(t, "batch", ev.kind)
		assert.Equal(t, uint64(100), ev.executorID)
		sorted := sortedIDs(ev.jobIDs)
		if len(sorted) == 2 && sorted[0] == 1 {
			batches["a"] = sorted
		} else {
			batches["b"] = sorted
		}
	}
	assert.Equal(t, []uint64{1, 2}, batches["a"])
	assert.Equal(t, []uint64{3, 4}, batches["b"])

	expectNoFurtherInvocation(t, rec, 500*time.Millisecond)
}

func TestListenForJobsToInvokeV1_SkipsJobMissingFromCache(t *testing.T) {
	rec := newRecordingWebhookExecutor()
	je, cancel := newTestJobExecutor(t, rec,
		[]models.Job{newWebhookJob(1, 100), newWebhookJob(2, 100)},
		[]models.JobExecutor{newWebhookExecutorModel(100, false)},
	)
	defer cancel()

	// Job 1 is fully seeded; job 2 is only in the queue (no cache entry) and must be
	// dropped during the drain rather than invoked.
	seedScheduledJob(je, 1, time.Now().Add(-2*time.Second), time.Now().Add(-2*time.Second))
	je.scheduleQueue.AddJob(models.JobScheduleKey{JobId: 2, ExecutionTime: time.Now().Add(-1 * time.Second)})

	go je.ListenForJobsToInvokeV1()

	got := collectInvocations(t, rec, 1, 3*time.Second)
	assert.Equal(t, "single", got[0].kind)
	assert.Equal(t, []uint64{1}, got[0].jobIDs)

	expectNoFurtherInvocation(t, rec, 500*time.Millisecond)
}

func TestListenForJobsToInvokeV1_WakesWhenJobAdded(t *testing.T) {
	rec := newRecordingWebhookExecutor()
	je, cancel := newTestJobExecutor(t, rec,
		[]models.Job{newWebhookJob(1, 100)},
		[]models.JobExecutor{newWebhookExecutorModel(100, false)},
	)
	defer cancel()

	// A far-future job keeps the loop sleeping for ~1 hour. If the jobAddedChan wake
	// works, adding a due job interrupts that sleep and it fires almost immediately.
	farFuture := time.Now().Add(1 * time.Hour)
	seedScheduledJob(je, 2, farFuture, farFuture)

	go je.ListenForJobsToInvokeV1()

	// Give the loop a moment to enter its long sleep.
	time.Sleep(200 * time.Millisecond)

	past := time.Now().Add(-1 * time.Second)
	je.jobExecutionsCache.Store(uint64(1), &models.JobSchedule{
		Job: models.Job{ID: 1},
		MemExecution: models.MemJobExecution{
			LastState:             models.ExecutionLogScheduleState,
			NextExecutionDatetime: past,
		},
	})
	je.addJobToScheduleQueue(models.JobScheduleKey{JobId: 1, ExecutionTime: past})

	got := collectInvocations(t, rec, 1, 3*time.Second)
	assert.Equal(t, "single", got[0].kind)
	assert.Equal(t, []uint64{1}, got[0].jobIDs)
}

func TestListenForJobsToInvokeV1_GroupsByExecutor(t *testing.T) {
	rec := newRecordingWebhookExecutor()
	je, cancel := newTestJobExecutor(t, rec,
		[]models.Job{
			newWebhookJob(1, 100), newWebhookJob(2, 100),
			newWebhookJob(3, 200),
		},
		[]models.JobExecutor{
			newWebhookExecutorModel(100, true),  // aggregation on
			newWebhookExecutorModel(200, false), // aggregation off
		},
	)
	defer cancel()

	past := time.Now().Add(-1 * time.Second)
	fireTime := time.Now().Add(-1 * time.Second)
	seedScheduledJob(je, 1, past, fireTime)
	seedScheduledJob(je, 2, past, fireTime)
	seedScheduledJob(je, 3, past, fireTime)

	go je.ListenForJobsToInvokeV1()

	got := collectInvocations(t, rec, 2, 3*time.Second)

	var batch, single *recordedInvocation
	for i := range got {
		switch got[i].kind {
		case "batch":
			batch = &got[i]
		case "single":
			single = &got[i]
		}
	}

	if assert.NotNil(t, batch, "expected an aggregated batch for executor 100") {
		assert.Equal(t, uint64(100), batch.executorID)
		assert.Equal(t, []uint64{1, 2}, sortedIDs(batch.jobIDs))
	}
	if assert.NotNil(t, single, "expected a single invocation for executor 200") {
		assert.Equal(t, uint64(200), single.executorID)
		assert.Equal(t, []uint64{3}, single.jobIDs)
	}

	expectNoFurtherInvocation(t, rec, 500*time.Millisecond)
}
