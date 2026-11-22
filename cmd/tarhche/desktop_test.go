//go:build !nodesktop

package main

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Tarhche/cli/internal/desktop"
)

func TestMarkNoDisplay(t *testing.T) {
	t.Parallel()

	t.Run("no display is a desktop app that cannot be opened here", func(t *testing.T) {
		t.Parallel()

		err := markNoDisplay(desktop.ErrNoDisplay)

		assert.True(t, desktopUnavailable(err))
		assert.ErrorIs(t, err, desktop.ErrNoDisplay)
	})

	t.Run("anything else is a desktop app that failed", func(t *testing.T) {
		t.Parallel()

		failed := errors.New("boom")

		assert.Same(t, failed, markNoDisplay(failed))
		assert.NoError(t, markNoDisplay(nil))
	})
}
