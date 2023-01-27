package workers

type Dispatcher struct {
	inputQueue chan []interface{}
	workerPool chan chan []interface{}
	maxWorkers int64
	callback   func(effector func(successChannel chan any, errorChannel chan any), successChannel chan any, errorChannel chan any)
}

func NewDispatcher(maxWorkers int64, maxQueue int64, callback func(effector func(successChannel chan any, errorChannel chan any), successChannel chan any, errorChannel chan any)) *Dispatcher {
	pool := make(chan chan []interface{}, maxWorkers)
	return &Dispatcher{
		workerPool: pool,
		maxWorkers: maxWorkers,
		callback:   callback,
		inputQueue: make(chan []interface{}, maxQueue),
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
			go func(i []interface{}) {
				workerQueue := <-dispatcher.workerPool
				workerQueue <- i
			}(input)
		}
	}
}

func (dispatcher *Dispatcher) BlockQueue(effector func(successChannel chan any, errorChannel chan any)) (successData any, errorData any) {
	successChannel := make(chan any)
	errorChannel := make(chan any)

	dispatcher.inputQueue <- []interface{}{effector, successChannel, errorChannel}

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

	dispatcher.inputQueue <- []interface{}{effector, successChannel, errorChannel}
}
