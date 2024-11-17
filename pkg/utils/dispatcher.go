package utils

import (
	"context"
	"scheduler0/pkg/models"
)

type Dispatcher struct {
	ctx        context.Context
	inputQueue chan models.Work
	workerPool chan chan models.Work
	maxWorkers int64
}

func NewDispatcher(ctx context.Context, maxWorkers int64, maxQueue int64) *Dispatcher {
	pool := make(chan chan models.Work, maxWorkers)
	return &Dispatcher{
		workerPool: pool,
		ctx:        ctx,
		maxWorkers: maxWorkers,
		inputQueue: make(chan models.Work, maxQueue),
	}
}

func (dispatcher *Dispatcher) Run() {
	for i := 0; int64(i) < dispatcher.maxWorkers; i++ {
		worker := NewWorker(dispatcher.ctx, dispatcher.workerPool)
		worker.Start()
	}

	go dispatcher.dispatch()
}

func (dispatcher *Dispatcher) dispatch() {
	for {
		select {
		case input := <-dispatcher.inputQueue:
			go func(i models.Work) {
				workerQueue := <-dispatcher.workerPool
				workerQueue <- i
			}(input)
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
