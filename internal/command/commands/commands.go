// Package commands builds the CLI's console: its global flags and every
// group and command it has.
package commands

import (
	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/command/content"
	"github.com/Tarhche/cli/internal/command/system"
	"github.com/Tarhche/cli/internal/command/workload"
)

// Description is what the CLI says it is.
const Description = "A self-sufficient runtime for your Tarhche: manage its content, and run your workloads as docker runs containers."

// New is the console of the CLI, its global flags bound to cli.Globals.
func New(cli *command.Cli) (*console.Console, error) {
	globals, err := console.StructFlags(&cli.Globals)
	if err != nil {
		return nil, err
	}

	c := console.New(command.Name, Description, cli.Out, cli.Err).Flags(globals)

	system.Register(c, cli)
	content.Register(c, cli)
	workload.Register(c, cli)

	return c, nil
}
