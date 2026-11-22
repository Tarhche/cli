package remote

import (
	"context"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/portforward"
	"github.com/Tarhche/cli/internal/reach"
)

// forwardVMCommand forwards local ports to a VM's: through the forward
// action to a port it publishes, else relayed by nc inside it when it runs
// a dockerd.
type forwardVMCommand struct {
	remote *remote
	flags  *console.FlagSet

	address  string
	allUsers bool
}

var _ console.Command = (*forwardVMCommand)(nil)

func newForwardVMCommand(r *remote) *forwardVMCommand {
	return &forwardVMCommand{remote: r}
}

func (c *forwardVMCommand) Name() string {
	return "port-forward"
}

func (c *forwardVMCommand) Description() string {
	return "Forward one or more local ports to a VM"
}

func (c *forwardVMCommand) Usage() string {
	return "port-forward [OPTIONS] VM [LOCAL_PORT:]REMOTE_PORT [...[LOCAL_PORT_N:]REMOTE_PORT_N]"
}

func (c *forwardVMCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.address, console.Long("address"), addressUsage, console.Default(portforward.DefaultAddress))
	defineAllUsers(flags, &c.allUsers, "VM")
}

func (c *forwardVMCommand) Run(ctx context.Context) console.ExitStatus {
	return c.remote.cli.Exit(c.run(ctx))
}

func (c *forwardVMCommand) run(ctx context.Context) error {
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

	vm, err := c.remote.vm(ctx, audience, reference)
	if err != nil {
		return err
	}

	dialer, err := c.remote.dialer(audience)
	if err != nil {
		return err
	}

	dials := make(map[uint]portforward.Dial, len(ports))
	for _, port := range ports {
		if dials[port.remote], err = reach.VMPort(dialer, vm, reference, port.remote); err != nil {
			return err
		}
	}

	return c.remote.forward(ctx, addresses, ports, func(remote uint) portforward.Dial {
		return dials[remote]
	})
}
