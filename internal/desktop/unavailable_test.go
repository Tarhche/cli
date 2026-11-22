package desktop_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Tarhche/cli/internal/desktop"
)

func TestUnavailable(t *testing.T) {
	t.Parallel()

	t.Run("nothing reaches into a workload yet", func(t *testing.T) {
		t.Parallel()

		var reach desktop.Unavailable

		_, terminalErr := reach.Open(context.Background(), nil, desktop.TerminalRequest{})
		_, forwardErr := reach.Forward(context.Background(), nil, desktop.ForwardRequest{})
		copyInErr := reach.CopyIn(context.Background(), nil, desktop.CopyRequest{})
		copyOutErr := reach.CopyOut(context.Background(), nil, desktop.CopyRequest{})

		assert.EqualError(t, terminalErr, "terminals are not available yet")
		assert.EqualError(t, forwardErr, "port forwards are not available yet")
		assert.EqualError(t, copyInErr, "copying files is not available yet")
		assert.EqualError(t, copyOutErr, "copying files is not available yet")

		for _, err := range []error{terminalErr, forwardErr, copyInErr, copyOutErr} {
			assert.ErrorIs(t, err, desktop.ErrUnavailable)
		}
	})
}
