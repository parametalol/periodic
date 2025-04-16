package periodic

import (
	"context"
	"errors"
	"fmt"
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
		"Execution stopped for task<nil>with error:test",
	}, ([]string)(a))
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
