package periodic

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_testTicker(t *testing.T) {
	var ticker = NewTestTicker(0).(TestTicker)
	i := atomic.Int32{}
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		for range ticker {
			i.Add(1)
		}
		wg.Done()
	}()
	ticker <- time.Now()
	ticker <- time.Now()
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
