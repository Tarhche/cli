//go:build !windows

package desktop

import (
	"context"
	"os/exec"
	"syscall"
)

// startDetached starts name with args in a session of its own, so that
// nothing that ends the app's ends it, and does not wait for it.
func startDetached(name string, args ...string) error {
	command := exec.CommandContext(context.Background(), name, args...)
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := command.Start(); err != nil {
		return err
	}

	return command.Process.Release()
}
