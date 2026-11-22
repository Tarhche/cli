package workload

import (
	"context"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
)

// connectContainerCommand puts a container on another network of its VM.
type connectContainerCommand struct {
	cli   *command.Cli
	flags *console.FlagSet

	aliases  list
	vm       string
	allUsers bool
}

func newConnectContainerCommand(cli *command.Cli) *connectContainerCommand {
	return &connectContainerCommand{cli: cli}
}

func (c *connectContainerCommand) Name() string {
	return "connect"
}

func (c *connectContainerCommand) Description() string {
	return "Connect a container to a network of its VM"
}

func (c *connectContainerCommand) Usage() string {
	return command.Name + " container connect [OPTIONS] CONTAINER NETWORK"
}

func (c *connectContainerCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.aliases, console.Long("alias"), "a name the container is reached by on the network, besides its own.")
	console.Var(flags, &c.vm, console.Long("vm"), "looks for the container in this Docker VM alone.")
	defineAllUsers(flags, &c.allUsers)
}

func (c *connectContainerCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *connectContainerCommand) run(ctx context.Context) error {
	if err := command.ExactArgs(c.flags, 2); err != nil {
		return err
	}

	return connect(ctx, c.cli, c.allUsers, c.vm, c.flags.Arg(0), c.flags.Arg(1), c.aliases)
}

// connect puts the container reference names on network, a network of its
// VM named by its name or its id, where it is reached by aliases too.
func connect(ctx context.Context, cli *command.Cli, allUsers bool, vm string, reference string, network string, aliases []string) error {
	client, err := clientOf(ctx, cli, allUsers)
	if err != nil {
		return err
	}

	vmUUID, err := vmUUIDOf(ctx, client, vm)
	if err != nil {
		return err
	}

	container, err := client.FindContainer(ctx, vmUUID, reference)
	if err != nil {
		return err
	}

	return client.ConnectContainer(ctx, container.VMUUID, container.Metadata.UUID, network, aliases)
}

// disconnect takes the container reference names off network, a network of
// its VM named by its name or its id.
func disconnect(ctx context.Context, cli *command.Cli, allUsers bool, vm string, reference string, network string) error {
	client, err := clientOf(ctx, cli, allUsers)
	if err != nil {
		return err
	}

	vmUUID, err := vmUUIDOf(ctx, client, vm)
	if err != nil {
		return err
	}

	container, err := client.FindContainer(ctx, vmUUID, reference)
	if err != nil {
		return err
	}

	return client.DisconnectContainer(ctx, container.VMUUID, container.Metadata.UUID, network)
}
