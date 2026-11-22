//go:build !windows

package remote

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// resized tells, until ctx is done, that the process got a SIGWINCH: the
// terminal it runs in was resized. Signals that come while one is not yet
// told are told once.
func resized(ctx context.Context) <-chan struct{} {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGWINCH)

	changes := make(chan struct{})

	go func() {
		defer close(changes)
		defer signal.Stop(signals)

		var pending chan<- struct{}

		for {
			select {
			case <-ctx.Done():
				return
			case <-signals:
				pending = changes
			case pending <- struct{}{}:
				pending = nil
			}
		}
	}()

	return changes
}
