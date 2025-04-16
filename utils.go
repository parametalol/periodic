package periodic

import (
	"context"
	"sync"
	"time"
)

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

func IgnoreErr[T TaskFunc](task T) fullTaskFunc {
	adaptedTask := Adapt(task)
	return func(ctx context.Context) error {
		_ = adaptedTask(ctx)
		return nil
	}
}

func Sync[T TaskFunc](locker sync.Locker, task T) fullTaskFunc {
	adaptedTask := Adapt(task)
	return func(ctx context.Context) error {
		locker.Lock()
		defer locker.Unlock()
		return adaptedTask(ctx)
	}
}

func WithTimeout[T TaskFunc](timeout time.Duration, task T) fullTaskFunc {
	adaptedTask := Adapt(task)
	return func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		return adaptedTask(ctx)
	}
}

func WithLog[T TaskFunc](log interface {
	Info(...any)
	Error(...any)
}, task T) fullTaskFunc {
	adaptedTask := Adapt(task)
	return func(ctx context.Context) error {
		log.Info("Calling task", ctx.Value(TaskNameKey{}))
		if err := adaptedTask(ctx); err != nil {
			log.Error("Execution stopped for task", ctx.Value(TaskNameKey{}), "with error:", err)
		}
		return nil
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
	case func(_ context.Context):
		return func(ctx context.Context) error {
			t(ctx)
			return nil
		}
	}
	return nil
}
