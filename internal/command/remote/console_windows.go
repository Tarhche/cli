//go:build windows

package remote

import (
	"golang.org/x/sys/windows"
)

// virtualTerminal has the console out is understand a remote terminal's
// escape sequences, and is what puts it back.
func virtualTerminal(out any) func() error {
	fd, ok := descriptor(out)
	if !ok {
		return func() error { return nil }
	}

	handle := windows.Handle(fd)

	var mode uint32
	if err := windows.GetConsoleMode(handle, &mode); err != nil {
		return func() error { return nil }
	}

	if err := windows.SetConsoleMode(handle, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING); err != nil {
		return func() error { return nil }
	}

	return func() error { return windows.SetConsoleMode(handle, mode) }
}
