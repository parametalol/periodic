package utils

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAdapt(t *testing.T) {
	ctx := context.Background()

	testErr := errors.New("test")
	assert.NoError(t, Adapt[int](func() {})(ctx, 0))
	assert.ErrorIs(t, Adapt[int](func() error { return testErr })(ctx, 0), testErr)

	ctx, cancel := context.WithCancel(ctx)
	cancel()
	assert.NoError(t, Adapt[int](func(context.Context) {})(ctx, 0))
	assert.ErrorIs(t, Adapt[int](func(ctx context.Context) error { return ctx.Err() })(ctx, 0), context.Canceled)

	assert.NoError(t, Adapt[int](func(context.Context, int) {})(ctx, 0))
	assert.NoError(t, Adapt[int](func(int) {})(ctx, 0))
}

func TestAdaptT(t *testing.T) {
	f := func() {

	}
	adapted := AdaptT(f)
	assert.NoError(t, adapted(context.Background(), time.Now()))
}
