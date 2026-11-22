//go:build !nodesktop

package main

import (
	"context"
	"errors"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/desktop"
)

// openDesktop opens the desktop app until it quits, without a console
// Windows opened for it alone.
func openDesktop(ctx context.Context, cli *command.Cli) error {
	leaveOwnConsole()

	return markNoDisplay(desktop.Run(ctx, cli))
}

// markNoDisplay is err, said of a desktop app that cannot be opened here
// when it is that there is no display to open it on.
func markNoDisplay(err error) error {
	if errors.Is(err, desktop.ErrNoDisplay) {
		return unavailableError{err: err}
	}

	return err
}
