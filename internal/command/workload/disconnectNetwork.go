package workload

import (
	"context"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
)

// disconnectNetworkCommand takes a container off a network, as docker's
// network disconnect does.
type disconnectNetworkCommand struct {
	cli   *command.Cli
	flags *console.FlagSet

	vm       string
	allUsers bool
}

func newDisconnectNetworkCommand(cli *command.Cli) *disconnectNetworkCommand {
	return &disconnectNetworkCommand{cli: cli}
}

func (c *disconnectNetworkCommand) Name() string {
	return "disconnect"
}

func (c *disconnectNetworkCommand) Description() string {
	return "Disconnect a container from a network of its VM"
}

func (c *disconnectNetworkCommand) Usage() string {
	return command.Name + " network disconnect [OPTIONS] NETWORK CONTAINER"
}

func (c *disconnectNetworkCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.vm, console.Long("vm"), "looks for the container in this Docker VM alone.")
	defineAllUsers(flags, &c.allUsers)
}

func (c *disconnectNetworkCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *disconnectNetworkCommand) run(ctx context.Context) error {
	if err := command.ExactArgs(c.flags, 2); err != nil {
		return err
	}

	return disconnect(ctx, c.cli, c.allUsers, c.vm, c.flags.Arg(1), c.flags.Arg(0))
}
