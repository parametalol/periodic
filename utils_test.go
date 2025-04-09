package periodic

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSeqIgnoreErr(t *testing.T) {
	i := 2
	inc := func(_ context.Context) error {
		i++
		return nil
	}
	mul := func(_ context.Context) error {
		i *= 2
		return errors.New("error")
	}
	assert.NoError(t, Seq(inc, IgnoreErr(mul))(context.Background()))
	assert.Equal(t, 6, i)

	assert.Error(t, Seq(mul, inc)(context.Background()))
	assert.Equal(t, 12, i)
}
