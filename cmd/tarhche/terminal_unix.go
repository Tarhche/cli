//go:build !windows

package main

import (
	"os"

	"golang.org/x/term"
)

// noTerminal reports whether none of files is a terminal.
func noTerminal(files ...*os.File) bool {
	for _, file := range files {
		if term.IsTerminal(int(file.Fd())) {
			return false
		}
	}

	return true
}

// leaveOwnConsole lets go of a console opened for the binary alone, of which
// there is none but on Windows.
func leaveOwnConsole() {}
