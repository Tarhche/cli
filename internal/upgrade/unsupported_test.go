//go:build nodesktop

package upgrade

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSupported(t *testing.T) {
	t.Parallel()

	t.Run("the image's build cannot upgrade itself", func(t *testing.T) {
		t.Parallel()

		upgrader := New(WithExecutable(t.TempDir()))

		_, err := upgrader.Target()
		assert.ErrorIs(t, err, ErrUnsupported)

		_, err = upgrader.Install(context.Background(), Release{Version: "0.2.0"}, nil)
		assert.ErrorIs(t, err, ErrUnsupported)
	})
}
