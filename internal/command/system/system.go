// Package system holds the commands about the CLI itself and the estate it
// talks to: signing in and out, its contexts, its version, and who it talks
// to as whom.
package system

import (
	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
)

// Register registers the system commands and the context group on c.
func Register(c *console.Console, cli *command.Cli) {}
