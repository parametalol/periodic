package periodic

import (
	"sync/atomic"
	"time"
)

type Ticker interface {
	Destroy()
	TickChan() <-chan time.Time
}

type timeTicker struct {
	t       *time.Ticker
	ch      chan time.Time
	stop    chan bool
	stopped atomic.Bool
}

func NewTicker(d time.Duration) Ticker {
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

// region TestTicker

// TestTicker is a [periodic.ticker] implementation that just wraps a
// time.Time channel. It could be used as to test ticker consumers by sending
// ticks explicitly.
type TestTicker chan time.Time

var _ Ticker = (*TestTicker)(nil)

func NewTestTicker(time.Duration) Ticker {
	return make(TestTicker, 1)
}
func (tt TestTicker) Destroy()                   { close(tt) }
func (tt TestTicker) TickChan() <-chan time.Time { return tt }
