package utils

import (
	"context"
	"scheduler0/models"
)

type Worker struct {
	ctx         context.Context
	WorkerPool  chan chan models.Work
	WorkerQueue chan models.Work
	callback    func(effector func(successChannel chan any, errorChannel chan any), successChannel chan any, errorChannel chan any)
	quit        chan bool
}

func NewWorker(ctx context.Context, workerPool chan chan models.Work, callback func(effector func(successChannel chan any, errorChannel chan any), successChannel chan any, errorChannel chan any)) Worker {
	return Worker{
		ctx:         ctx,
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
			case <-worker.ctx.Done():
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
