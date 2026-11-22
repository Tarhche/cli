//go:build !windows

package remote

import (
	"context"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestResized(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	changes := resized(ctx)

	assert.Eventually(t, func() bool {
		_ = syscall.Kill(syscall.Getpid(), syscall.SIGWINCH)

		select {
		case <-changes:
			return true
		case <-time.After(10 * time.Millisecond):
			return false
		}
	}, 5*time.Second, time.Millisecond)

	cancel()

	for range changes {
	}
}
