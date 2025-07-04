package utils

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/parametalol/curry"
)

var ErrStopped = errors.New("stopped")

type attemptNumberCtxKey struct{}

var AttemptNumber attemptNumberCtxKey

type Func[TickType any] interface {
	curry.Func2R[context.Context, TickType, error]
}

func Adapt[TickType any, Fn Func[TickType]](task Fn) func(context.Context, TickType) error {
	return curry.Adapt2R[context.Context, TickType, error](task)
}

// AdaptT is a [time.Time] specialization of [Adapt].
func AdaptT[Fn Func[time.Time]](task Fn) func(context.Context, time.Time) error {
	return Adapt[time.Time](task)
}

// Seq executes a sequence of tasks in order.
// If one of the tasks fails, the execution stops and returns the error.
func Seq[TickType any](tasks ...func(context.Context, TickType) error) func(context.Context, TickType) error {
	return func(ctx context.Context, tick TickType) error {
		for _, task := range tasks {
			if err := task(ctx, tick); err != nil {
				return err
			}
		}
		return nil
	}
}

// IgnoreErr wraps a task and ignores its error.
func IgnoreErr[TickType any, Fn Func[TickType]](task Fn) func(context.Context, TickType) error {
	adaptedTask := Adapt[TickType](task)
	return func(ctx context.Context, tick TickType) error {
		_ = adaptedTask(ctx, tick)
		return nil
	}
}

// Sync wraps a task in a mutex lock to avoid concurrent execution.
func Sync[TickType any, Fn Func[TickType]](locker sync.Locker, task Fn) func(context.Context, TickType) error {
	adaptedTask := Adapt[TickType](task)
	return func(ctx context.Context, tick TickType) error {
		locker.Lock()
		defer locker.Unlock()
		return adaptedTask(ctx, tick)
	}
}

// Timeout sets a timeout for the task.
// If the task does not finish before the timeout, the context will be
// cancelled.
func Timeout[TickType any, Fn Func[TickType]](timeout time.Duration, task Fn) func(context.Context, TickType) error {
	adaptedTask := Adapt[TickType](task)
	return func(ctx context.Context, tick TickType) error {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		return adaptedTask(ctx, tick)
	}
}

func getAttemptNumber(ctx context.Context) (int, bool) {
	attempt, ok := ctx.Value(AttemptNumber).(int)
	return attempt, ok
}

// Log adds logging to the task.
// It will log the task name on every invocation, and the error if it occurs.
func Log[TickType any, Fn Func[TickType]](outW io.Writer, errW io.Writer, name string, task Fn) func(context.Context, TickType) error {
	adaptedTask := Adapt[TickType](task)
	return func(ctx context.Context, tick TickType) error {
		attempt, ok := getAttemptNumber(ctx)
		if attempt > 0 {
			_, _ = fmt.Fprintln(outW, "Retry", attempt, "of", name)
		} else {
			_, _ = fmt.Fprintln(outW, "Calling", name)
		}
		err := adaptedTask(ctx, tick)
		switch {
		case err != nil && ctx.Err() == nil:
			if errors.Is(err, ErrStopped) {
				if attempt > 0 {
					_, _ = fmt.Fprintln(errW, "Execution of", name, "stopped after retry", attempt, "with error:", err.Error())
				} else if ok {
					_, _ = fmt.Fprintln(errW, "Execution of", name, "stopped after the first attempt with error:", err.Error())
				} else {
					_, _ = fmt.Fprintln(errW, "Execution of", name, "stopped with error:", err.Error())
				}
			} else {
				if attempt > 0 {
					_, _ = fmt.Fprintln(errW, "Execution of", name, "failed after retry", attempt, "with error:", err.Error())
				} else if ok {
					_, _ = fmt.Fprintln(errW, "Execution of", name, "failed after the first attempt with error:", err.Error())
				} else {
					_, _ = fmt.Fprintln(errW, "Execution of", name, "failed with error:", err.Error())
				}
			}
		case ctx.Err() == context.Canceled:
			_, _ = fmt.Fprintln(errW, "Execution cancelled for", name)
		case ctx.Err() == context.DeadlineExceeded:
			_, _ = fmt.Fprintln(errW, "Execution deadline exceeded for", name)
		}
		return err
	}
}

// NoOverlap prevents the task from running concurrently.
// It will skip the task if it is already running.
func NoOverlap[TickType any, Fn Func[TickType]](task Fn) func(context.Context, TickType) error {
	adaptedTask := Adapt[TickType](task)
	var running atomic.Int32
	return func(ctx context.Context, tick TickType) error {
		if !running.CompareAndSwap(0, 1) {
			return nil
		}
		defer running.Store(0)
		return adaptedTask(ctx, tick)
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

// Retry retries the task if it returns an error.
// It will retry to run the task according to the policy function.
func Retry[TickType any, Fn Func[TickType]](policy RetryPolicy, task Fn) func(context.Context, TickType) error {
	adaptedTask := Adapt[TickType](task)
	return func(ctx context.Context, tick TickType) error {
		var err error
		for i := 0; ; i++ {
			ctx = context.WithValue(ctx, AttemptNumber, i)
			err = adaptedTask(ctx, tick)
			if errors.Is(err, ErrStopped) || !policy(ctx, i, err) {
				break
			}
		}
		return err
	}
}
