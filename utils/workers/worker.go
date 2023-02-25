package workers

import "scheduler0/models"

type Worker struct {
	WorkerPool  chan chan models.Work
	WorkerQueue chan models.Work
	callback    func(effector func(successChannel chan any, errorChannel chan any), successChannel chan any, errorChannel chan any)
	quit        chan bool
}

func NewWorker(workerPool chan chan models.Work, callback func(effector func(successChannel chan any, errorChannel chan any), successChannel chan any, errorChannel chan any)) Worker {
	return Worker{
		WorkerPool:  workerPool,
		WorkerQueue: make(chan models.Work),
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
				worker.callback(work.Effector, work.SuccessChannel, work.ErrorChannel)
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
