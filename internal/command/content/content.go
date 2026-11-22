// Package content holds a group of commands for every resource the
// dashboard manages that is not a workload: articles, comments, users,
// roles and the rest, each built from its description in package resource.
package content

import (
	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
)

// Register registers a group of commands for every content resource on c.
func Register(c *console.Console, cli *command.Cli) {}
