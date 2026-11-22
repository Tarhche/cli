//go:build windows

package remote

import (
	"context"
	"time"
)

// pollEvery is how often the size of a Windows console is looked at, which
// says nothing when it is resized.
const pollEvery = 250 * time.Millisecond

// resized tells, until ctx is done, every pollEvery, that the console may
// have been resized: whoever is told compares its size with the last. Ticks
// that come while one is not yet told are told once.
func resized(ctx context.Context) <-chan struct{} {
	changes := make(chan struct{})

	go func() {
		defer close(changes)

		ticker := time.NewTicker(pollEvery)
		defer ticker.Stop()

		var pending chan<- struct{}

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				pending = changes
			case pending <- struct{}{}:
				pending = nil
			}
		}
	}()

	return changes
}
