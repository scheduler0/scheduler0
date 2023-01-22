package workers

type Worker struct {
	WorkerPool  chan chan []interface{}
	WorkerQueue chan []interface{}
	callback    func(effector func(successChannel chan any, errorChannel chan any), successChannel chan any, errorChannel chan any)
	quit        chan bool
}

func NewWorker(workerPool chan chan []interface{}, callback func(effector func(successChannel chan any, errorChannel chan any), successChannel chan any, errorChannel chan any)) Worker {
	return Worker{
		WorkerPool:  workerPool,
		WorkerQueue: make(chan []interface{}),
		quit:        make(chan bool),
		callback:    callback,
	}
}

func (worker Worker) Start() {
	go func() {
		for {
			worker.WorkerPool <- worker.WorkerQueue

			select {
			case work := <-worker.WorkerQueue:
				effector := work[0].(func(sc, ec chan any))
				successChannel := work[1].(chan any)
				errorChannel := work[2].(chan any)
				worker.callback(effector, successChannel, errorChannel)
			case <-worker.quit:
				return
			}
		}
	}()
}

func (worker Worker) Stop() {
	go func() {
		worker.quit <- true
	}()
}
