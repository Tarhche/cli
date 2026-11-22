//go:build nodesktop

package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOpenDesktop(t *testing.T) {
	t.Parallel()

	t.Run("this build has none to open", func(t *testing.T) {
		t.Parallel()

		err := openDesktop(context.Background(), nil)

		assert.True(t, desktopUnavailable(err))
		assert.ErrorIs(t, err, errNoDesktop)
	})
}
