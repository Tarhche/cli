package remote

import (
	"context"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/portforward"
	"github.com/Tarhche/cli/internal/reach"
)

// forwardContainerCommand forwards local ports to a container's, relayed by
// nc in its Docker VM to the container's address there.
type forwardContainerCommand struct {
	remote *remote
	flags  *console.FlagSet

	address  string
	allUsers bool
}

var _ console.Command = (*forwardContainerCommand)(nil)

func newForwardContainerCommand(r *remote) *forwardContainerCommand {
	return &forwardContainerCommand{remote: r}
}

func (c *forwardContainerCommand) Name() string {
	return "port-forward"
}

func (c *forwardContainerCommand) Description() string {
	return "Forward one or more local ports to a container"
}

func (c *forwardContainerCommand) Usage() string {
	return "port-forward [OPTIONS] CONTAINER [LOCAL_PORT:]REMOTE_PORT [...[LOCAL_PORT_N:]REMOTE_PORT_N]"
}

func (c *forwardContainerCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.address, console.Long("address"), addressUsage, console.Default(portforward.DefaultAddress))
	defineAllUsers(flags, &c.allUsers, "container")
}

func (c *forwardContainerCommand) Run(ctx context.Context) console.ExitStatus {
	return c.remote.cli.Exit(c.run(ctx))
}

func (c *forwardContainerCommand) run(ctx context.Context) error {
	if err := command.MinArgs(c.flags, 2); err != nil {
		return err
	}

	ports, err := parsePorts(c.flags.Args()[1:])
	if err != nil {
		return err
	}

	addresses, err := parseAddresses(c.address)
	if err != nil {
		return err
	}

	reference := c.flags.Arg(0)
	audience := audienceOf(c.allUsers)

	container, err := c.remote.container(ctx, audience, reference)
	if err != nil {
		return err
	}

	dialer, err := c.remote.dialer(audience)
	if err != nil {
		return err
	}

	address, err := reach.ContainerAddress(ctx, dialer, container, reference)
	if err != nil {
		return err
	}

	return c.remote.forward(ctx, addresses, ports, func(remote uint) portforward.Dial {
		return reach.ContainerPort(dialer, container, address, remote)
	})
}
