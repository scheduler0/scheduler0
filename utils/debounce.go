package utils

import (
	"context"
	"sync"
	"time"
)

type Debounce struct {
	ticker    *time.Ticker
	once      sync.Once
	threshold time.Time
	mtx       sync.Mutex
}

func NewDebounce() *Debounce {
	return &Debounce{
		once: sync.Once{},
	}
}

func (d *Debounce) Debounce(ctx context.Context, delay uint64, effector func()) {
	d.mtx.Lock()
	defer d.mtx.Unlock()

	if delay < 500 {
		panic("delay cannot be less than 500 secs")
	}

	d.threshold = time.Now().Add(time.Duration(delay) * time.Millisecond)
	d.once.Do(func() {
		go func() {
			defer func() {
				d.mtx.Lock()
				defer d.mtx.Unlock()

				d.ticker.Stop()
				d.once = sync.Once{}
			}()
			d.ticker = time.NewTicker(time.Duration(500) * time.Millisecond)

			for {
				select {
				case <-d.ticker.C:
					if time.Now().After(d.threshold) {
						effector()
						return
					}
				case <-ctx.Done():
					return
				}
			}
		}()
	})
}
