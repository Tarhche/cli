// Package desktop is the desktop app the CLI opens with --desktop: the same
// estate, contexts and sessions as the commands, through a window.
package desktop

import (
	"context"
	"errors"

	"github.com/Tarhche/cli/internal/command"
)

// Run opens the desktop app and returns once it is closed.
func Run(ctx context.Context, cli *command.Cli) error {
	return errors.New("the desktop app is not built in yet")
}
