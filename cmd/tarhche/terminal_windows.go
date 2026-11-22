//go:build windows

package main

import (
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/term"
)

var (
	kernel32              = windows.NewLazySystemDLL("kernel32.dll")
	getConsoleProcessList = kernel32.NewProc("GetConsoleProcessList")
	freeConsole           = kernel32.NewProc("FreeConsole")
)

// startedByDesktop reports whether the binary writes to a console of its
// own, which Windows opens for a console program Explorer, a shortcut or the
// Start menu starts, rather than to one it shares with the shell that
// started it.
func startedByDesktop(_, out, _ *os.File) bool {
	return ownConsole() && term.IsTerminal(int(out.Fd()))
}

// ownConsole reports whether the binary is the only process attached to its
// console.
func ownConsole() bool {
	processes := make([]uint32, 2)
	count, _, _ := getConsoleProcessList.Call(uintptr(unsafe.Pointer(&processes[0])), uintptr(len(processes)))

	return count == 1
}

// leaveOwnConsole closes a console Windows opened for the binary alone, which
// the desktop app has no use for: one it shares with a shell stays.
func leaveOwnConsole() {
	if ownConsole() {
		_, _, _ = freeConsole.Call()
	}
}
