//go:build windows

package desktop

import (
	"context"
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// startDetached starts name with args with no console, which Windows would
// otherwise open a window of for tarhche, a console program, and does not
// wait for it.
func startDetached(name string, args ...string) error {
	command := exec.CommandContext(context.Background(), name, args...)
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP}

	if err := command.Start(); err != nil {
		return err
	}

	return command.Process.Release()
}
