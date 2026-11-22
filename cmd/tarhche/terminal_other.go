//go:build !darwin && !windows

package main

import "os"

// startedByDesktop reports whether the binary has no terminal to read from
// or write to, as when a desktop entry or a file manager starts it.
func startedByDesktop(in, out, errOut *os.File) bool {
	return noTerminal(in, out, errOut)
}
