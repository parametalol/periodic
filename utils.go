package periodic

import (
	"context"
	"sync"
	"time"
)

func Seq(tasks ...TaskFunc) TaskFunc {
	return func(ctx context.Context) error {
		for _, task := range tasks {
			if err := task(ctx); err != nil {
				return err
			}
		}
		return nil
	}
}

func IgnoreErr(task TaskFunc) TaskFunc {
	return func(ctx context.Context) error {
		_ = task(ctx)
		return nil
	}
}

func Sync(locker sync.Locker, task TaskFunc) TaskFunc {
	return func(ctx context.Context) error {
		locker.Lock()
		defer locker.Unlock()
		return task(ctx)
	}
}

func WithTimeout(timeout time.Duration, task TaskFunc) TaskFunc {
	return func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		return task(ctx)
	}
}

func WithLog(log interface {
	Info(...any)
	Error(...any)
}, task TaskFunc) TaskFunc {
	return func(ctx context.Context) error {
		log.Info("Calling task", ctx.Value(TaskNameKey{}))
		if err := task(ctx); err != nil {
			log.Error("Execution stopped for task", ctx.Value(TaskNameKey{}), "with error:", err)
		}
		return nil
	}
}
