package utils

import (
	"context"
	"scheduler0/pkg/models"
)

// dispatcherPendingBufferMin bounds the amount of not-yet-assigned work the
// dispatcher will hold in memory. It is deliberately large so that producers
// (including worker effectors that re-queue work, e.g. webhook execution via
// NoBlockQueue) practically never block, while keeping the total goroutine and
// memory footprint bounded.
const dispatcherPendingBufferMin = 1 << 16 // 65536

type Dispatcher struct {
	ctx        context.Context
	inputQueue chan models.Work
	workerPool chan chan models.Work
	// pending buffers work that has been accepted from inputQueue but not yet
	// handed to a worker. A fixed pool of assigner goroutines drains it, so the
	// number of goroutines stays bounded regardless of load.
	pending    chan models.Work
	maxWorkers int64
}

func NewDispatcher(ctx context.Context, maxWorkers int64, maxQueue int64) *Dispatcher {
	pool := make(chan chan models.Work, maxWorkers)
	pendingBuf := maxQueue
	if pendingBuf < dispatcherPendingBufferMin {
		pendingBuf = dispatcherPendingBufferMin
	}
	return &Dispatcher{
		workerPool: pool,
		ctx:        ctx,
		maxWorkers: maxWorkers,
		inputQueue: make(chan models.Work, maxQueue),
		pending:    make(chan models.Work, pendingBuf),
	}
}

func (dispatcher *Dispatcher) Run() {
	for i := 0; int64(i) < dispatcher.maxWorkers; i++ {
		worker := NewWorker(dispatcher.ctx, dispatcher.workerPool)
		worker.Start()
	}

	// A fixed set of assigner goroutines hand pending work to free workers.
	// Previously dispatch() spawned one goroutine per queued item that blocked
	// on <-workerPool; under sustained load (and the self-amplifying,
	// fire-and-forget NoBlockQueue submissions made from inside worker effectors
	// such as webhook execution) those goroutines accumulated without bound
	// (100k+ observed in production-like load). The unbounded growth starved the
	// Go scheduler and collapsed write throughput, so writes appeared to hang
	// while raft leadership and lock-free reads (healthcheck) stayed healthy; a
	// restart "fixed" it only by discarding the leaked goroutines.
	for i := 0; int64(i) < dispatcher.maxWorkers; i++ {
		go dispatcher.assign()
	}

	go dispatcher.dispatch()
}

// dispatch moves accepted work off inputQueue into the bounded pending buffer.
// Draining inputQueue promptly is what keeps producers (including nested
// fire-and-forget submitters) from blocking; the pending buffer is what bounds
// memory instead of an unbounded number of goroutines.
func (dispatcher *Dispatcher) dispatch() {
	for {
		select {
		case input := <-dispatcher.inputQueue:
			select {
			case dispatcher.pending <- input:
			case <-dispatcher.ctx.Done():
				return
			}
		case <-dispatcher.ctx.Done():
			return
		}
	}
}

// assign takes buffered work and hands it to the next free worker. There are
// maxWorkers assigners, so the goroutine count is fixed.
func (dispatcher *Dispatcher) assign() {
	for {
		select {
		case work := <-dispatcher.pending:
			select {
			case workerQueue := <-dispatcher.workerPool:
				workerQueue <- work
			case <-dispatcher.ctx.Done():
				return
			}
		case <-dispatcher.ctx.Done():
			return
		}
	}
}

func (dispatcher *Dispatcher) BlockQueue(effector func(successChannel chan any, errorChannel chan any)) (successData any, errorData any) {
	successChannel := make(chan any)
	errorChannel := make(chan any)

	dispatcher.inputQueue <- models.Work{
		Effector:       effector,
		SuccessChannel: successChannel,
		ErrorChannel:   errorChannel,
	}

	for {
		select {
		case data := <-successChannel:
			return data, nil
		case err := <-errorChannel:
			return nil, err
		}
	}
}

func (dispatcher *Dispatcher) NoBlockQueue(effector func(successChannel chan any, errorChannel chan any)) {
	successChannel := make(chan any)
	errorChannel := make(chan any)

	dispatcher.inputQueue <- models.Work{
		Effector:       effector,
		SuccessChannel: successChannel,
		ErrorChannel:   errorChannel,
	}
}
