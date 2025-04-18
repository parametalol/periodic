package periodic

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTickLoop(t *testing.T) {
	t.Run("ticks with cancel", func(t *testing.T) {
		ticks := make(chan time.Time)
		testCh := make(chan int32)

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			for range 3 {
				ticks <- time.Now()
				<-testCh
			}
			cancel()
		}()

		var i atomic.Int32
		var err = TickLoop(ticks, ctx, func() {
			testCh <- i.Add(1)
		})

		assert.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, int32(3), i.Load())
	})

	t.Run("cancel with no ticks", func(t *testing.T) {
		ticks := make(chan time.Time)
		close(ticks)

		var i atomic.Int32
		var err = TickLoop(ticks, context.Background(), func() {
			i.Add(1)
		})

		assert.ErrorIs(t, err, ErrStopped)
		assert.Equal(t, int32(0), i.Load())
	})

	t.Run("close channel after ticks", func(t *testing.T) {
		ticks := make(chan time.Time)
		testCh := make(chan int32)
		go func() {
			for range 3 {
				ticks <- time.Now()
				<-testCh
			}
			close(ticks)
		}()

		var i atomic.Int32
		var err = TickLoop(ticks, context.Background(), func() {
			testCh <- i.Add(1)
		})

		assert.ErrorIs(t, err, ErrStopped)
		assert.Equal(t, int32(3), i.Load())
	})

	t.Run("ticks stopped by an error", func(t *testing.T) {
		ticks := make(chan time.Time, 3)
		testCh := make(chan int32)
		go func() {
			for range 3 {
				ticks <- time.Now()
				<-testCh
			}
		}()

		var i atomic.Int32
		testError := errors.New("test")
		var err = TickLoop(ticks, context.Background(), func() error {
			testCh <- i.Add(1)
			return testError
		})
		testCh <- 0
		testCh <- 0

		assert.ErrorIs(t, err, testError)
		assert.Equal(t, int32(1), i.Load())
	})

	t.Run("cancellation cause", func(t *testing.T) {
		ticks := make(chan time.Time, 1)
		testCh := make(chan int32)
		ticks <- time.Now()
		close(ticks)

		var i atomic.Int32
		err := TickLoop(ticks, context.Background(), func(ctx context.Context) {
			<-ctx.Done()
			if errors.Is(ctx.Err(), context.Canceled) {
				testCh <- i.Add(1)
			}
			if errors.Is(context.Cause(ctx), ErrStopped) {
				testCh <- i.Add(2)
			}
		})

		assert.ErrorIs(t, err, ErrStopped)
		assert.Equal(t, int32(1), <-testCh)
		assert.Equal(t, int32(3), <-testCh)
		assert.Equal(t, int32(3), i.Load())
	})
}
