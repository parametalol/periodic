package utils

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/parametalol/curry/assert"
)

func TestSeqIgnoreErr(t *testing.T) {
	i := 2
	inc := func() {
		i++
	}
	mul := func(_ context.Context, _ any) error {
		i *= 2
		return errors.New("error")
	}
	assert.That(t,
		assert.ErrorIs(Seq(Adapt[any](inc), IgnoreErr[any](mul))(context.Background(), 0), nil),
		assert.Equal(6, i),
		assert.Not(assert.Equal(nil, Seq(mul, Adapt[any](inc))(context.Background(), 0))),
		assert.Equal(12, i))
}

type arr []string

func (a *arr) Write(data []byte) (int, error) {
	*a = append(*a, string(data))
	return len(data), nil
}

func TestWithLog(t *testing.T) {
	t.Run("error", func(t *testing.T) {
		var a = &arr{}
		err := Log[any](a, a, "test", func() error {
			return errors.New("test")
		})(context.Background(), nil)

		assert.That(t,
			assert.Not(assert.NoError(err)),
			assert.EqualSlices(arr{
				"Calling test\n",
				"Execution of test failed with error: test\n",
			}, *a))
	})

	t.Run("stopped", func(t *testing.T) {
		var a = &arr{}
		err := Log[any](a, a, "test", func() error {
			return ErrStopped
		})(context.Background(), nil)

		assert.That(t,
			assert.ErrorIs(err, ErrStopped),
			assert.EqualSlices(arr{
				"Calling test\n",
				"Execution of test stopped with error: stopped\n",
			}, *a))
	})

	t.Run("with cancel", func(t *testing.T) {
		var a = &arr{}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := Log[any](a, a, "test", func() {})(ctx, nil)
		assert.That(t,
			assert.NoError(err),
			assert.EqualSlices(arr{
				"Calling test\n",
				"Execution cancelled for test\n",
			}, (*a)))
	})

	t.Run("with deadline", func(t *testing.T) {
		var a = &arr{}
		ctx, cancel := context.WithDeadline(context.Background(), time.Now())
		defer cancel()
		err := Log[any](a, a, "test", func() {})(ctx, nil)
		assert.That(t,
			assert.NoError(err),
			assert.EqualSlices(arr{
				"Calling test\n",
				"Execution deadline exceeded for test\n",
			}, (*a)))
	})

	t.Run("attempt", func(t *testing.T) {
		var a = &arr{}
		err := Retry[any](SimpleRetryPolicy(2),
			Log[any](a, a, "test",
				func() error {
					return errors.New("test")
				}))(context.Background(), nil)
		assert.That(t,
			assert.Not(assert.NoError(err)),
			assert.EqualSlices(arr{
				"Calling test\n",
				"Execution of test failed after the first attempt with error: test\n",
				"Retry 1 of test\n",
				"Execution of test failed after retry 1 with error: test\n",
			}, (*a)))
	})

	t.Run("attempt stopped", func(t *testing.T) {
		var a = &arr{}
		err := Retry[any](SimpleRetryPolicy(2),
			Log[any](a, a, "test",
				func() error {
					return ErrStopped
				}))(context.Background(), nil)
		assert.That(t,
			assert.ErrorIs(err, ErrStopped),
			assert.EqualSlices(arr{
				"Calling test\n",
				"Execution of test stopped after the first attempt with error: stopped\n",
			}, (*a)))
	})

	t.Run("retry stopped", func(t *testing.T) {
		var a = &arr{}
		attempt := 0
		err := Retry[any](SimpleRetryPolicy(2),
			Log[any](a, a, "test",
				func() error {
					if attempt == 0 {
						attempt++
						return errors.New("test")
					}
					return ErrStopped
				}))(context.Background(), nil)
		assert.That(t,
			assert.ErrorIs(err, ErrStopped),
			assert.EqualSlices(arr{
				"Calling test\n",
				"Execution of test failed after the first attempt with error: test\n",
				"Retry 1 of test\n",
				"Execution of test stopped after retry 1 with error: stopped\n",
			}, (*a)))
	})
}

func TestWithTimeout(t *testing.T) {
	var deadline time.Time
	var ok bool
	now := time.Now()
	err := Timeout[any](0, func(ctx context.Context) error {
		deadline, ok = ctx.Deadline()
		return ctx.Err()
	})(context.Background(), 0)
	assert.That(t,
		assert.ErrorIs(err, context.DeadlineExceeded),
		assert.True(ok),
		assert.True(time.Since(now) >= time.Since(deadline)))
}

func TestNoOverlap(t *testing.T) {
	var i atomic.Int32
	testCh := make(chan bool)
	task := func() {
		i.Add(1)
		testCh <- true
		testCh <- true
	}
	fn := NoOverlap[any](task)
	go func() {
		_ = fn(context.Background(), 0)
	}()
	<-testCh
	_ = fn(context.Background(), 0)
	_ = fn(context.Background(), 0)
	_ = fn(context.Background(), 0)
	<-testCh
	assert.That(t, assert.Equal(int32(1), i.Load()))
}

func TestWithRetry(t *testing.T) {
	t.Run("with error", func(t *testing.T) {
		var i int
		task := func() error {
			i++
			return errors.New("test")
		}
		err := Retry[any](SimpleRetryPolicy(3), task)(context.Background(), 0)
		assert.That(t,
			assert.Not(assert.NoError(err)),
			assert.Equal(3, i))
	})
	t.Run("without error", func(t *testing.T) {
		var i int
		task := func() {
			i++
		}
		err := Retry[any](SimpleRetryPolicy(3), task)(context.Background(), 0)
		assert.That(t,
			assert.NoError(err),
			assert.Equal(1, i))
	})
	t.Run("with cancelled context", func(t *testing.T) {
		var i int
		task := func() {
			i++
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := Retry[any](SimpleRetryPolicy(3), task)(ctx, 0)
		assert.That(t,
			assert.NoError(err),
			assert.Equal(1, i))
	})
	t.Run("with exponential backoff", func(t *testing.T) {
		var i int
		task := func() error {
			i++
			return errors.New("test")
		}
		err := Retry[any](ExponentialBackoffPolicy(3, time.Millisecond), task)(context.Background(), 0)
		assert.That(t,
			assert.Not(assert.NoError(err)),
			assert.Equal(3, i))
	})
	t.Run("cancel with exponential backoff", func(t *testing.T) {
		var i int
		task := func() {
			i++
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := Retry[any](ExponentialBackoffPolicy(3, time.Millisecond), task)(ctx, 0)
		assert.That(t,
			assert.NoError(err),
			assert.Equal(1, i))
	})
}

func (a *arr) Lock() {
	_, _ = a.Write([]byte("locked\n"))
}
func (a *arr) Unlock() {
	_, _ = a.Write([]byte("unlocked\n"))
}

func TestSync(t *testing.T) {
	loglock := &arr{}

	_ = Sync[any](loglock, Log[any](loglock, loglock, "test",
		func() {}))(context.Background(), 0)

	assert.That(t,
		assert.EqualSlices(arr{
			"locked\n",
			"Calling test\n",
			"unlocked\n",
		}, (*loglock)))
}
