package workers

type Dispatcher struct {
	InputQueue chan []interface{}
	WorkerPool chan chan []interface{}
	maxWorkers int64
	callback   func(effector func(successChannel chan any, errorChannel chan any), successChannel chan any, errorChannel chan any)
}

func NewDispatcher(maxWorkers int64, maxQueue int64, callback func(effector func(successChannel chan any, errorChannel chan any), successChannel chan any, errorChannel chan any)) *Dispatcher {
	pool := make(chan chan []interface{}, maxWorkers)
	return &Dispatcher{
		WorkerPool: pool,
		maxWorkers: maxWorkers,
		callback:   callback,
		InputQueue: make(chan []interface{}, maxQueue),
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
			go func(i []interface{}) {
				workerQueue := <-dispatcher.WorkerPool
				workerQueue <- i
			}(input)
		}
	}
}

func (dispatcher *Dispatcher) Queue(effector func(successChannel chan any, errorChannel chan any), successChannel chan any, errorChannel chan any) {
	dispatcher.InputQueue <- []interface{}{effector, successChannel, errorChannel}
}
