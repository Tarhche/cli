//go:build !nodesktop

package main

import (
	"context"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/desktop"
)

// openDesktop opens the desktop app, and returns once it is closed.
func openDesktop(ctx context.Context, cli *command.Cli) error {
	return desktop.Run(ctx, cli)
}
