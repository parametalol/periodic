package periodic

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type testTicker chan time.Time

var _ ticker = (*testTicker)(nil)

func newTestTicker(time.Duration) ticker {
	return make(testTicker, 1)
}
func (tt testTicker) Destroy()                   { close(tt) }
func (tt testTicker) TickChan() <-chan time.Time { return tt }

func Test_testTicker(t *testing.T) {
	ticker := newTestTicker(0)
	i := atomic.Int32{}
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		for range ticker.(testTicker) {
			i.Add(1)
		}
		wg.Done()
	}()
	ticker.(testTicker) <- time.Now()
	ticker.(testTicker) <- time.Now()
	ticker.Destroy()
	wg.Wait()
	assert.Equal(t, int32(2), i.Load())
}

func Test_timeTicker(t *testing.T) {
	ticker := NewTicker(time.Hour)
	i := atomic.Int32{}
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		for range ticker.TickChan() {
			i.Add(1)
		}
		wg.Done()
	}()
	ticker.Destroy()
	wg.Wait()
	assert.Equal(t, int32(1), i.Load())
}
