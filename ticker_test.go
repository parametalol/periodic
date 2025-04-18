package periodic

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewTestTicker(t *testing.T) {
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

func TestNewTicker(t *testing.T) {
	t.Run("test first tick", func(t *testing.T) {
		ticker := NewTicker(time.Hour)
		var i int
		wg := sync.WaitGroup{}
		wg.Add(1)
		ticked := make(chan bool)
		go func() {
			for range ticker.TickChan() {
				i++
				ticked <- true
			}
			wg.Done()
		}()
		<-ticked
		ticker.Destroy()
		wg.Wait()
		assert.Equal(t, 1, i)
	})

	t.Run("test timed tick", func(t *testing.T) {
		ticker := NewTicker(time.Second)
		i := atomic.Int32{}
		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			for range ticker.TickChan() {
				i.Add(1)
			}
			wg.Done()
		}()
		time.Sleep(2500 * time.Millisecond)
		ticker.Destroy()
		wg.Wait()
		assert.GreaterOrEqual(t, i.Load(), int32(3))
	})
}
