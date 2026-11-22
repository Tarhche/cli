package remote

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/portforward"
)

// containerAddresses is the template docker inspect says a container's
// addresses with, one on each of its networks.
const containerAddresses = "{{range .NetworkSettings.Networks}}{{.IPAddress}} {{end}}"

// forwardContainerCommand forwards local ports to a container's, relayed by
// nc in its Docker VM to the container's address there.
type forwardContainerCommand struct {
	remote *remote
	flags  *console.FlagSet

	address string
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

	container, err := c.remote.container(ctx, reference)
	if err != nil {
		return err
	}

	dialer, err := c.remote.dialer()
	if err != nil {
		return err
	}

	var inspected bytes.Buffer

	inspect := []string{"docker", "inspect", "--format", containerAddresses, container.DockerID}
	if err := dialer.Runner(container.VM.UUID).Run(ctx, inspect, nil, &inspected); err != nil {
		return fmt.Errorf("finding the address of container %s: %w", reference, err)
	}

	found := strings.Fields(inspected.String())
	if len(found) == 0 {
		return fmt.Errorf("container %s has no address to forward to: is it running?", reference)
	}

	return c.remote.forward(ctx, addresses, ports, func(remote uint) portforward.Dial {
		return func(ctx context.Context) (io.ReadWriteCloser, error) {
			return dialer.Relay(ctx, container.VM.UUID, found[0], remote)
		}
	})
}
