package main

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/Tarhche/cli/internal/command"
)

// processSerialNumber is what older macOS adds to the arguments of an app it
// starts from Finder or the Dock (-psn_0_12345), which is no flag of the
// CLI's.
const processSerialNumber = "-psn_"

// launch is how the binary was started, and how it opens its desktop app.
type launch struct {
	// asApp reports whether it was started as an app is, rather than from a
	// terminal.
	asApp bool
	// openDesktop opens the desktop app until it quits.
	openDesktop func(context.Context, *command.Cli) error
}

// unavailableError is err, said of a desktop app that cannot be opened here:
// a build without one, or a machine with no display.
type unavailableError struct {
	err error
}

// Error is what err says.
func (e unavailableError) Error() string {
	return e.err.Error()
}

// Unwrap is err.
func (e unavailableError) Unwrap() error {
	return e.err
}

// desktopUnavailable reports whether err says no desktop app can be opened
// here, where a binary started as an app falls back to its command line.
func desktopUnavailable(err error) bool {
	_, isUnavailable := errors.AsType[unavailableError](err)

	return isUnavailable
}

// commandLine is args, but for what macOS added to them as it started the
// binary as an app.
func commandLine(args []string) []string {
	kept := make([]string, 0, len(args))

	for i, arg := range args {
		if i > 0 && strings.HasPrefix(arg, processSerialNumber) {
			continue
		}

		kept = append(kept, arg)
	}

	return kept
}

// startedAsApp reports whether the binary was started as an app is, rather
// than from a terminal: with nothing to run, by what starts apps on this
// platform (Finder or the Dock, Explorer or a shortcut, a desktop entry or a
// file manager), which leaves it no terminal of its own.
func startedAsApp(args []string, in, out, errOut *os.File) bool {
	return len(args) == 1 && startedByDesktop(in, out, errOut)
}
