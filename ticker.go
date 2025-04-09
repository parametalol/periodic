package periodic

import (
	"sync/atomic"
	"time"
)

type ticker interface {
	Destroy()
	TickChan() <-chan time.Time
}

type timeTicker struct {
	t       *time.Ticker
	ch      chan time.Time
	stop    chan bool
	stopped atomic.Bool
}

func NewTicker(d time.Duration) ticker {
	ch := make(chan time.Time, 1)
	ticker := &timeTicker{
		t:    time.NewTicker(d),
		ch:   ch,
		stop: make(chan bool),
	}
	ticker.ch <- time.Now()
	go func() {
		for {
			select {
			case tick := <-ticker.t.C:
				if !ticker.stopped.Load() {
					ticker.ch <- tick
				}
			case <-ticker.stop:
				return
			}
		}
	}()
	return ticker
}

func (tt *timeTicker) Destroy() {
	tt.stopped.Store(true)
	close(tt.stop)
	tt.t.Stop()
	close(tt.ch)
}

func (tt *timeTicker) TickChan() <-chan time.Time {
	return tt.ch
}
