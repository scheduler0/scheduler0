package utils

import (
	"context"
	"scheduler0/pkg/models"
	"testing"
)

func Test_Workers(t *testing.T) {

	t.Run("should execute callback when work is queued", func(t *testing.T) {
		pool := make(chan chan models.Work, 1)

		ctx, canceler := context.WithCancel(context.Background())
		defer canceler()

		worker := NewWorker(ctx, pool)

		worker.Start()

		e := make(chan any, 1)
		s := make(chan any, 1)
		c := func(s chan any, e chan any) {
			close(s)
			close(e)
		}

		worker.WorkerQueue <- models.Work{
			SuccessChannel: s,
			ErrorChannel:   e,
			Effector:       c,
		}

		_, eClosed := <-e
		_, sClosed := <-s

		if eClosed || sClosed {
			t.Fatal("should execute callback and close channel")
		}
	})

}
