package workers

type Worker struct {
	WorkerPool  chan chan any
	WorkerQueue chan any
	callback    func(args ...any)
	quit        chan bool
}

func NewWorker(workerPool chan chan any, callback func(args ...any)) Worker {
	return Worker{
		WorkerPool:  workerPool,
		WorkerQueue: make(chan any),
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
				worker.callback(work)
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
