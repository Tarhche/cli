package workload

import (
	"context"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
)

// disconnectContainerCommand takes a container off a network of its VM.
type disconnectContainerCommand struct {
	cli   *command.Cli
	flags *console.FlagSet

	vm       string
	allUsers bool
}

func newDisconnectContainerCommand(cli *command.Cli) *disconnectContainerCommand {
	return &disconnectContainerCommand{cli: cli}
}

func (c *disconnectContainerCommand) Name() string {
	return "disconnect"
}

func (c *disconnectContainerCommand) Description() string {
	return "Disconnect a container from a network of its VM"
}

func (c *disconnectContainerCommand) Usage() string {
	return command.Name + " container disconnect [OPTIONS] CONTAINER NETWORK"
}

func (c *disconnectContainerCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.vm, console.Long("vm"), "looks for the container in this Docker VM alone.")
	defineAllUsers(flags, &c.allUsers)
}

func (c *disconnectContainerCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *disconnectContainerCommand) run(ctx context.Context) error {
	if err := command.ExactArgs(c.flags, 2); err != nil {
		return err
	}

	return disconnect(ctx, c.cli, c.allUsers, c.vm, c.flags.Arg(0), c.flags.Arg(1))
}
