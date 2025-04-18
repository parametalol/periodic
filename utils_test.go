package periodic

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSeqIgnoreErr(t *testing.T) {
	i := 2
	inc := func() {
		i++
	}
	mul := func(_ context.Context) error {
		i *= 2
		return errors.New("error")
	}
	assert.NoError(t, Seq(Adapt(inc), IgnoreErr(mul))(context.Background()))
	assert.Equal(t, 6, i)

	assert.Error(t, Seq(mul, Adapt(inc))(context.Background()))
	assert.Equal(t, 12, i)
}

type arr []string

func (a *arr) Info(args ...any) {
	*a = append(*a, fmt.Sprint(args...))
}

func (a *arr) Error(args ...any) {
	*a = append(*a, fmt.Sprint(args...))
}

func TestWithLog(t *testing.T) {
	var a arr = make([]string, 0, 2)
	err := WithLog(&a, func() error { return errors.New("test") })(context.Background())
	assert.Error(t, err)
	assert.Equal(t, []string{
		"Calling task<nil>",
		"Task<nil>failed with error:test",
	}, ([]string)(a))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = WithLog(&a, func(context.Context) {})(ctx)
	assert.NoError(t, err)
	assert.Equal(t, []string{
		"Calling task<nil>",
		"Execution cancelled for task<nil>",
	}, ([]string)(a[2:]))
}

func TestWithTimeout(t *testing.T) {
	var deadline time.Time
	var ok bool
	now := time.Now()
	err := WithTimeout(0, func(ctx context.Context) error {
		deadline, ok = ctx.Deadline()
		return ctx.Err()
	})(context.Background())
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.True(t, ok)
	assert.True(t, time.Since(now) >= time.Since(deadline))
}

func TestNoOverlap(t *testing.T) {
	var i atomic.Int32
	testCh := make(chan bool)
	task := func() {
		i.Add(1)
		testCh <- true
		testCh <- true
	}
	fn := NoOverlap(Adapt(task))
	go fn(context.Background())
	<-testCh
	_ = fn(context.Background())
	_ = fn(context.Background())
	_ = fn(context.Background())
	<-testCh
	assert.Equal(t, int32(1), i.Load())
}

func TestWithRetry(t *testing.T) {
	t.Run("with error", func(t *testing.T) {
		var i int
		task := func() error {
			i++
			return errors.New("test")
		}
		err := WithRetry(task, SimpleRetryPolicy(3))(context.Background())
		assert.Error(t, err)
		assert.Equal(t, 3, i)
	})
	t.Run("without error", func(t *testing.T) {
		var i int
		task := func() {
			i++
		}
		err := WithRetry(task, SimpleRetryPolicy(3))(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, 1, i)
	})
	t.Run("with cancelled context", func(t *testing.T) {
		var i int
		task := func() {
			i++
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := WithRetry(task, SimpleRetryPolicy(3))(ctx)
		assert.NoError(t, err)
		assert.Equal(t, 1, i)
	})
	t.Run("with exponential backoff", func(t *testing.T) {
		var i int
		task := func() error {
			i++
			return errors.New("test")
		}
		err := WithRetry(task, ExponentialBackoffPolicy(3, time.Millisecond))(context.Background())
		assert.Error(t, err)
		assert.Equal(t, 3, i)
	})
}

func TestRoutine(t *testing.T) {
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
		var err = Routine(ticks, ctx, func() {
			testCh <- i.Add(1)
		})

		assert.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, int32(3), i.Load())
	})

	t.Run("cancel with no ticks", func(t *testing.T) {
		ticks := make(chan time.Time)
		close(ticks)

		var i atomic.Int32
		var err = Routine(ticks, context.Background(), func() {
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
		var err = Routine(ticks, context.Background(), func() {
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
		var err = Routine(ticks, context.Background(), func() error {
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
		err := Routine(ticks, context.Background(), func(ctx context.Context) {
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
