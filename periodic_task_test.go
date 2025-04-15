package periodic

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func makeTestPeriodicTask(t *testing.T) *periodicTask {
	pt := NewTask("test", time.Hour, func(ctx context.Context) error { return nil })
	pt.SetLog((*testLogWrapper)(t))
	pt.tickerConstructor = newTestTicker
	return pt
}

type testLogWrapper testing.T

//log: logging.CreateLogger(			logging.ModuleForName("Periodic "+name), 1),

func (t *testLogWrapper) Info(args ...any) {
	(*testing.T)(t).Log(args...)
}
func (t *testLogWrapper) Error(args ...any) {
	(*testing.T)(t).Log(args...)
}

func TestNewTask(t *testing.T) {
	var counter atomic.Int32
	var pt Task = NewTask("test", time.Hour,
		func(ctx context.Context) error {
			counter.Add(1)
			return nil
		})
	for range 5 {
		pt.Start()
		assert.NoError(t, pt.Error())
		pt.Stop()
		pt.Wait()
		assert.ErrorIs(t, pt.Error(), ErrStopped)
	}
	assert.Equal(t, int32(5), counter.Load())

	assert.Panics(t, func() { _ = NewTask("", 0, nil) })
}

func Test_periodicTask(t *testing.T) {
	pt := makeTestPeriodicTask(t)
	taskSyncCh := make(chan int32, 5)
	var counter atomic.Int32
	pt.task = func(ctx context.Context) error {
		taskSyncCh <- counter.Add(1)
		return nil
	}

	tick := time.Now()

	t.Run("start start and stop stop ", func(t *testing.T) {
		counter.Store(0)
		pt.Start()
		assert.NoError(t, pt.Error())
		pt.Start()
		assert.NoError(t, pt.Error())
		pt.ticker.(testTicker) <- tick
		pt.Stop()
		assert.ErrorIs(t, pt.Error(), ErrStopped)
		pt.Stop()
		assert.ErrorIs(t, pt.Error(), ErrStopped)

		assert.Equal(t, int32(1), <-taskSyncCh)
	})

	t.Run("ticker", func(t *testing.T) {
		counter.Store(0)
		pt.Start()
		for i := range int32(5) {
			pt.ticker.(testTicker) <- tick
			assert.Equal(t, i+1, <-taskSyncCh)
		}
		pt.Stop()
		assert.Equal(t, int32(5), counter.Load())
	})
}

func Test_stopOnError(t *testing.T) {
	pt := makeTestPeriodicTask(t)
	taskSyncChIn := make(chan int32, 5)
	taskSyncChOut := make(chan int32, 5)

	err := errors.New("test error")
	pt.task = func(ctx context.Context) error {
		select {
		case x := <-taskSyncChIn:
			taskSyncChOut <- x
			if x == 5 {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
		return nil
	}

	tick := time.Now()

	assert.NoError(t, pt.Error())
	pt.Start()

	for i := range int32(5) {
		pt.ticker.(testTicker) <- tick
		taskSyncChIn <- i // No error.
		assert.Equal(t, i, <-taskSyncChOut)
	}
	pt.ticker.(testTicker) <- tick
	taskSyncChIn <- 5 // Error that triggers Stop.
	assert.Equal(t, int32(5), <-taskSyncChOut)

	// No way to wait for the internal run goroutine, so use Eventually.
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.ErrorIs(c, pt.Error(), err)
	}, time.Minute, time.Second,
		"should be stopped eventually and return the right error")
}

func Test_cancelPeriodicTask(t *testing.T) {
	pt := makeTestPeriodicTask(t)

	taskSyncChIn := make(chan int32, 5)
	taskSyncChOut := make(chan bool)
	var i atomic.Int32

	var testCtxCause error
	pt.task = func(ctx context.Context) error {
		testCtxCause = nil
		taskSyncChOut <- true
		select {
		case x := <-taskSyncChIn:
			i.Store(x)
			if x == 0 {
				return errors.New("test error")
			}
		case <-ctx.Done():
			testCtxCause = context.Cause(ctx)
			taskSyncChOut <- true
			return ctx.Err()
		}
		return nil
	}

	tick := time.Now()

	t.Run("cancel context on start", func(t *testing.T) {
		i.Store(0)
		pt.Start()
		pt.ticker.(testTicker) <- tick
		<-taskSyncChOut
		assert.NoError(t, pt.Error())
		pt.Stop()
		<-taskSyncChOut

		assert.Equal(t, int32(0), i.Load())
		assert.ErrorIs(t, pt.Error(), ErrStopped)
		assert.ErrorIs(t, testCtxCause, ErrStopped)
	})

	t.Run("cancel context on tick", func(t *testing.T) {
		i.Store(0)
		pt.Start()
		pt.ticker.(testTicker) <- tick
		<-taskSyncChOut
		taskSyncChIn <- 42 // Skip the first run.
		assert.NoError(t, pt.Error())

		pt.ticker.(testTicker) <- tick
		<-taskSyncChOut
		pt.Stop()
		<-taskSyncChOut

		assert.ErrorIs(t, pt.Error(), ErrStopped)
		assert.ErrorIs(t, testCtxCause, ErrStopped)
	})

	t.Run("cancel a real timer on start", func(t *testing.T) {
		testCtxCause = nil
		taskSyncCh := make(chan bool)
		pt := NewTask("test", 100*time.Hour,
			func(ctx context.Context) error {

				taskSyncCh <- true
				<-ctx.Done()
				testCtxCause = context.Cause(ctx)
				taskSyncCh <- true
				return ctx.Err()
			})
		pt.Start()
		<-taskSyncCh
		assert.NoError(t, pt.Error())
		pt.Stop()
		<-taskSyncCh

		assert.ErrorIs(t, pt.Error(), ErrStopped)
		assert.ErrorIs(t, testCtxCause, ErrStopped)
	})

	t.Run("task returns an error on stop", func(t *testing.T) {
		testCtxCause = nil
		taskSyncCh := make(chan bool)
		pt := NewTask("test", 100*time.Hour,
			func(ctx context.Context) error {
				taskSyncCh <- true
				<-ctx.Done()
				testCtxCause = context.Cause(ctx)
				taskSyncCh <- true
				return errors.New("some ignored error")
			})
		pt.Start()
		<-taskSyncCh
		assert.NoError(t, pt.Error())
		pt.Stop()
		<-taskSyncCh

		assert.ErrorIs(t, pt.Error(), ErrStopped)
		assert.ErrorIs(t, testCtxCause, ErrStopped)
	})
}
