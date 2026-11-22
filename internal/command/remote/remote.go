// Package remote holds the commands that reach into a workload over the
// workload ingress: a terminal (attach), a command (exec), files copied in
// and out (cp), and ports forwarded to the host (port-forward).
package remote

import (
	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
)

// ContainerCommands are the commands that reach into a container. Each call
// is a fresh set, so that they can be registered in more than one place.
func ContainerCommands(cli *command.Cli) []console.Command {
	return nil
}

// VMCommands are the commands that reach into a VM. Each call is a fresh
// set.
func VMCommands(cli *command.Cli) []console.Command {
	return nil
}
