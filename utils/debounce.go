package utils

import (
	"context"
	"sync"
	"time"
)

type debounce struct {
	ticker    time.Ticker
	once      sync.Once
	threshold time.Time
}

type Debounce interface {
	Debounce(ctx context.Context, delay int64, effector func())
}

func NewDebounce() *debounce {
	return &debounce{
		once: sync.Once{},
	}
}

func (d *debounce) Debounce(ctx context.Context, delay int64, effector func()) {
	d.threshold = time.Now().Add(time.Duration(delay) * time.Millisecond)
	d.once.Do(func() {
		go func() {
			for {
				select {
				case <-d.ticker.C:
					if time.Now().After(d.threshold) {
						effector()
						d.ticker.Stop()
						d.once = sync.Once{}
					}
				case <-ctx.Done():
					return
				}
			}
		}()
	})
}
