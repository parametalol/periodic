package periodic

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrStopped is the error set by the [Task.Stop] method.
var ErrStopped = errors.New("stopped")

type Task interface {
	// Start or restart the periodic task execution. No-op on started instance.
	Start()
	// Stop the periodic task execution. No-op on stopped instance.
	Stop()
	// Wait for the tasks to terminate.
	Wait()
	// Error returns the reason why the task execution has been stopped.
	// Returns [ErrStopped] if the instance has been stopped with [Stop].
	Error() error
}

type TaskFunc func(context.Context) error

type periodicTask struct {
	period time.Duration
	task   TaskFunc
	name   string

	wg       sync.WaitGroup
	stateMux sync.RWMutex
	ticker   ticker
	err      error

	// Used for testing.
	tickerConstructor func(time.Duration) ticker
}

var _ Task = (*periodicTask)(nil)

type TaskNameKey struct{}

// NewTask constructs a stopped instance of a named periodic task, that calls
// the provided function on start, and then periodically at the p period.
// The periodic execution will stop if task returns an error.
func NewTask(name string, p time.Duration, task TaskFunc) *periodicTask {
	if task == nil {
		panic("no function provided for " + name + " task")
	}
	return &periodicTask{
		period:            p,
		task:              task,
		name:              name,
		tickerConstructor: NewTicker,
	}
}

func (pt *periodicTask) Start() {
	pt.stateMux.Lock()
	defer pt.stateMux.Unlock()

	if pt.ticker != nil {
		return
	}
	pt.wg.Add(1)
	pt.err = nil
	pt.ticker = pt.tickerConstructor(pt.period)
	go pt.loop(pt.ticker.TickChan())
}

// Stop could be called explicitly by the client code, or after the task
// returned an error: go Start -> go loop -> go run -> go Stop.
func (pt *periodicTask) Stop() {
	pt.stateMux.Lock()
	defer pt.stateMux.Unlock()
	if pt.ticker == nil {
		return
	}
	pt.ticker.Destroy()
	pt.ticker = nil

	if pt.err == nil {
		pt.err = ErrStopped
	}
}

func (pt *periodicTask) Wait() {
	pt.wg.Wait()
}

func (pt *periodicTask) Error() error {
	pt.stateMux.RLock()
	defer pt.stateMux.RUnlock()
	return pt.err
}

func (pt *periodicTask) loop(ticks <-chan time.Time) {
	defer pt.wg.Done()

	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(ErrStopped)

	ctx = context.WithValue(ctx, TaskNameKey{}, pt.name)

	for range ticks {
		pt.wg.Add(1)
		go pt.run(ctx)
	}
}

func (pt *periodicTask) run(ctx context.Context) {
	defer pt.wg.Done()

	// task calls are not synchronized.
	if err := pt.task(ctx); err != nil && ctx.Err() == nil {
		pt.stateMux.Lock()
		defer pt.stateMux.Unlock()
		pt.err = err
		// Stop if the task returned non-context error.
		go pt.Stop()
	}
}
