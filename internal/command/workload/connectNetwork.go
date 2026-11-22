package workload

import (
	"context"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
)

// connectNetworkCommand puts a container on a network, as docker's network
// connect does.
type connectNetworkCommand struct {
	cli   *command.Cli
	flags *console.FlagSet

	aliases  list
	vm       string
	allUsers bool
}

func newConnectNetworkCommand(cli *command.Cli) *connectNetworkCommand {
	return &connectNetworkCommand{cli: cli}
}

func (c *connectNetworkCommand) Name() string {
	return "connect"
}

func (c *connectNetworkCommand) Description() string {
	return "Connect a container to a network of its VM"
}

func (c *connectNetworkCommand) Usage() string {
	return command.Name + " network connect [OPTIONS] NETWORK CONTAINER"
}

func (c *connectNetworkCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.aliases, console.Long("alias"), "a name the container is reached by on the network, besides its own.")
	console.Var(flags, &c.vm, console.Long("vm"), "looks for the container in this Docker VM alone.")
	defineAllUsers(flags, &c.allUsers)
}

func (c *connectNetworkCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *connectNetworkCommand) run(ctx context.Context) error {
	if err := command.ExactArgs(c.flags, 2); err != nil {
		return err
	}

	return connect(ctx, c.cli, c.allUsers, c.vm, c.flags.Arg(1), c.flags.Arg(0), c.aliases)
}
