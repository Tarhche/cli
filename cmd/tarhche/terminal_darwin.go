package main

import "os"

// launchd is the process every app Finder, the Dock or open(1) opens is
// started by.
const launchd = 1

// startedByDesktop reports whether launchd started the binary with no
// terminal to read from or write to, as it starts an app bundle.
func startedByDesktop(in, out, errOut *os.File) bool {
	return os.Getppid() == launchd && noTerminal(in, out, errOut)
}
