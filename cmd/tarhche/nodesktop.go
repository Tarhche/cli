//go:build nodesktop

package main

import (
	"context"
	"errors"

	"github.com/Tarhche/cli/internal/command"
)

// errNoDesktop is a build of the CLI alone, as its image is: one that never
// loads a web view is a static binary, which runs where no C library is.
var errNoDesktop = errors.New("this build of tarhche has no desktop app: use a release binary for --desktop")

// openDesktop fails: this build has no desktop app.
func openDesktop(context.Context, *command.Cli) error {
	return unavailableError{err: errNoDesktop}
}
