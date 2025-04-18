package periodic

import (
	"context"
	"sync/atomic"
	"time"
)

// TickLoop calls the task in a goroutine on every tick and returns:
//   - if the task returns an error: the error;
//   - if the tick channel is closed: the [ErrStopped];
//   - if the context is cancelled: [context.Cancelled].
//
// For the latter two cases, if the task is still running, it may observe the
// cancelled context with [context.Cause] to be set to [ErrStopped].
//
// It is possible for several tasks to be running concurrently, but only the
// first error will be returned to the caller. Consider wrapping the tasks with
// [NoOverlap] to avoid this situation.
func TickLoop[Fn TaskFunc](ticks <-chan time.Time, ctx context.Context, task Fn) error {
	adaptedTask := Adapt(task)
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(ErrStopped)

	errCh := make(chan error)
	defer close(errCh)
	var closed atomic.Bool
	defer closed.Store(true)

	for {
		select {
		case _, ok := <-ticks:
			if !ok || closed.Load() {
				return ErrStopped
			}
			go func() {
				if err := adaptedTask(ctx); err != nil && !closed.Swap(true) {
					errCh <- err
				}
			}()
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errCh:
			return err
		}
	}
}
