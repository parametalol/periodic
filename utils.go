package periodic

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// Seq executes a sequence of tasks in order.
// If one of the tasks fails, the execution stops and returns the error.
func Seq(tasks ...fullTaskFunc) fullTaskFunc {
	return func(ctx context.Context) error {
		for _, task := range tasks {
			if err := task(ctx); err != nil {
				return err
			}
		}
		return nil
	}
}

// IgnoreErr wraps a task and ignores its error.
func IgnoreErr[T TaskFunc](task T) fullTaskFunc {
	adaptedTask := Adapt(task)
	return func(ctx context.Context) error {
		_ = adaptedTask(ctx)
		return nil
	}
}

// Sync wraps a task in a mutex lock to avoid concurrent execution.
func Sync[T TaskFunc](locker sync.Locker, task T) fullTaskFunc {
	adaptedTask := Adapt(task)
	return func(ctx context.Context) error {
		locker.Lock()
		defer locker.Unlock()
		return adaptedTask(ctx)
	}
}

// WithTimeout sets a timeout for the task.
// If the task does not finish before the timeout, the context will be
// cancelled.
func WithTimeout[T TaskFunc](timeout time.Duration, task T) fullTaskFunc {
	adaptedTask := Adapt(task)
	return func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		return adaptedTask(ctx)
	}
}

// WithLog adds logging to the task.
// It will log the task name on every invocation, and the error if it occurs.
func WithLog[T TaskFunc](log interface {
	Info(...any)
	Error(...any)
}, task T) fullTaskFunc {
	adaptedTask := Adapt(task)
	return func(ctx context.Context) error {
		log.Info("Calling task", ctx.Value(TaskNameKey{}))
		err := adaptedTask(ctx)
		if err != nil && err != context.Canceled {
			log.Error("Task", ctx.Value(TaskNameKey{}), "failed with error:", err)
		} else if ctx.Err() != nil {
			log.Error("Execution cancelled for task", ctx.Value(TaskNameKey{}))
		}
		return err
	}
}

// NoOverlap prevents the task from running concurrently.
// It will skip the task if it is already running.
func NoOverlap[T TaskFunc](task T) fullTaskFunc {
	adaptedTask := Adapt(task)
	var running atomic.Int32
	return func(ctx context.Context) error {
		if !running.CompareAndSwap(0, 1) {
			return nil
		}
		defer running.Store(0)
		return adaptedTask(ctx)
	}
}

// RetryPolicy is a function that defines the retry policy.
// It takes the task context, the current 0-based attempt number and the error
// returned by the task.
// It should return true if the task should be retried, and false otherwise.
type RetryPolicy func(context.Context, int, error) bool

// SimpleRetryPolicy returns the retry policy, that attempts to run
// the task the specified number of times.
func SimpleRetryPolicy(attempts int) RetryPolicy {
	return func(ctx context.Context, i int, err error) bool {
		return i < attempts-1 && err != nil && ctx.Err() == nil
	}
}

// ExponentialBackoffPolicy returns a retry policy that uses exponential
// backoff.
// It will retry to run the task the specified number of times.
func ExponentialBackoffPolicy(attempts int, duration time.Duration) RetryPolicy {
	return func(ctx context.Context, i int, err error) bool {
		if err != nil && ctx.Err() == nil {
			time.Sleep(time.Duration(i+1) * duration)
			return i < attempts-1
		}
		return false
	}
}

// WithRetry retries the task if it returns an error.
// It will retry to run the task according to the policy function.
func WithRetry[T TaskFunc](task T, policy RetryPolicy) fullTaskFunc {
	adaptedTask := Adapt(task)
	return func(ctx context.Context) error {
		var err error
		for i := 0; ; i++ {
			err = adaptedTask(ctx)
			if !policy(ctx, i, err) {
				break
			}
		}
		return err
	}
}

// Adapt the task to a function that takes a context and returns an error.
func Adapt[T TaskFunc](task T) fullTaskFunc {
	switch t := any(task).(type) {
	case fullTaskFunc:
		return t
	case func():
		return func(_ context.Context) error {
			t()
			return nil
		}
	case func() error:
		return func(_ context.Context) error {
			return t()
		}
	case func(context.Context):
		return func(ctx context.Context) error {
			t(ctx)
			return nil
		}
	}
	return nil
}

// Routine calls go fn() on every tick.
func Routine(wg *sync.WaitGroup, ticks <-chan time.Time, fn func()) {
	defer wg.Done()
	for range ticks {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fn()
		}()
	}
}
