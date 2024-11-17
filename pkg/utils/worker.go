package utils

import (
	"context"
	"scheduler0/pkg/models"
)

type Worker struct {
	ctx         context.Context
	WorkerPool  chan chan models.Work
	WorkerQueue chan models.Work
	quit        chan bool
}

func NewWorker(ctx context.Context, workerPool chan chan models.Work) Worker {
	return Worker{
		ctx:         ctx,
		WorkerPool:  workerPool,
		WorkerQueue: make(chan models.Work),
		quit:        make(chan bool),
	}
}

func (worker Worker) Start() {
	go func() {
		for {
			worker.WorkerPool <- worker.WorkerQueue
			select {
			case work := <-worker.WorkerQueue:
				work.Effector(work.SuccessChannel, work.ErrorChannel)
			case <-worker.ctx.Done():
				return
			}
		}
	}()
}
