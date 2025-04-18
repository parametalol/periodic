package periodic

import (
	"sync"
	"time"
)

type Ticker interface {
	Destroy()
	TickChan() <-chan time.Time
}

type timeTicker struct {
	t    *time.Ticker
	ch   chan time.Time
	stop chan bool

	stopMux sync.Mutex
	stopped bool
}

func NewTicker(d time.Duration) Ticker {
	ticker := &timeTicker{
		t:    time.NewTicker(d),
		ch:   make(chan time.Time, 1),
		stop: make(chan bool),
	}

	go func() {
		ticker.stopMux.Lock()
		if !ticker.stopped {
			ticker.ch <- time.Now() // Send the first tick.
		}
		ticker.stopMux.Unlock()

		for {
			select {
			case tick := <-ticker.t.C:
				ticker.stopMux.Lock()
				if !ticker.stopped {
					ticker.ch <- tick
				}
				ticker.stopMux.Unlock()
			case <-ticker.stop:
				return
			}
		}
	}()
	return ticker
}

func (tt *timeTicker) Destroy() {
	tt.stopMux.Lock()
	defer tt.stopMux.Unlock()
	tt.stopped = true

	close(tt.stop)
	tt.t.Stop()
	close(tt.ch)
}

func (tt *timeTicker) TickChan() <-chan time.Time {
	return tt.ch
}

// region TestTicker

// TestTicker is a [Ticker] implementation that just wraps a time.Time channel.
// It could be used to test [Ticker] clients by sending ticks explicitly.
type TestTicker chan time.Time

var _ Ticker = (*TestTicker)(nil)

func NewTestTicker(time.Duration) Ticker {
	return make(TestTicker, 1)
}
func (tt TestTicker) Destroy()                   { close(tt) }
func (tt TestTicker) TickChan() <-chan time.Time { return tt }
