package utils

import "scheduler0/models"

type Dispatcher struct {
	inputQueue chan models.Work
	workerPool chan chan models.Work
	maxWorkers int64
	callback   func(effector func(successChannel chan any, errorChannel chan any), successChannel chan any, errorChannel chan any)
}

func NewDispatcher(maxWorkers int64, maxQueue int64, callback func(effector func(successChannel chan any, errorChannel chan any), successChannel chan any, errorChannel chan any)) *Dispatcher {
	pool := make(chan chan models.Work, maxWorkers)
	return &Dispatcher{
		workerPool: pool,
		maxWorkers: maxWorkers,
		callback:   callback,
		inputQueue: make(chan models.Work, maxQueue),
	}
}

func (dispatcher *Dispatcher) Run() {
	for i := 0; int64(i) < dispatcher.maxWorkers; i++ {
		worker := NewWorker(dispatcher.workerPool, dispatcher.callback)
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
