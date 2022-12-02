package workers

type Dispatcher struct {
	InputQueue chan any
	WorkerPool chan chan any
	maxWorkers int64
	callback   func(args ...any)
}

func NewDispatcher(maxWorkers int64, maxQueue int64, callback func(args ...any)) *Dispatcher {
	pool := make(chan chan any, maxWorkers)
	return &Dispatcher{
		WorkerPool: pool,
		maxWorkers: maxWorkers,
		callback:   callback,
		InputQueue: make(chan any, maxQueue),
	}
}

func (dispatcher *Dispatcher) Run() {
	for i := 0; int64(i) < dispatcher.maxWorkers; i++ {
		worker := NewWorker(dispatcher.WorkerPool, dispatcher.callback)
		worker.Start()
	}

	go dispatcher.dispatch()
}

func (dispatcher *Dispatcher) dispatch() {
	for {
		select {
		case input := <-dispatcher.InputQueue:
			go func(i any) {
				workerQueue := <-dispatcher.WorkerPool
				workerQueue <- i
			}(input)
		}
	}
}
