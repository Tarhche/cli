package remote

import (
	"context"
	"fmt"
	"io"
	"slices"
	"sync/atomic"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/portforward"
	"github.com/Tarhche/cli/internal/stream"
	"github.com/Tarhche/cli/internal/workload"
)

// relayHost is where a VM's own port is relayed to from inside it.
const relayHost = "127.0.0.1"

// forwardVMCommand forwards local ports to a VM's: through the forward
// action to a port it publishes, else relayed by nc inside it when it runs
// a dockerd.
type forwardVMCommand struct {
	remote *remote
	flags  *console.FlagSet

	address string
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

	vm, err := c.remote.vm(ctx, reference)
	if err != nil {
		return err
	}

	for _, port := range ports {
		if !vm.Docker && !slices.Contains(vm.Ports, port.remote) {
			return fmt.Errorf("vm %s does not publish port %d, and runs no dockerd to relay it from inside: publish the port to forward it", reference, port.remote)
		}
	}

	dialer, err := c.remote.dialer()
	if err != nil {
		return err
	}

	return c.remote.forward(ctx, addresses, ports, func(remote uint) portforward.Dial {
		port := &vmPort{dialer: dialer, vm: vm, reference: reference, port: remote}
		port.relaying.Store(!slices.Contains(vm.Ports, remote))

		return port.dial
	})
}

// vmPort is a port of a VM, reached through the forward action while the VM
// publishes it. A Docker VM's port it does not publish, or that the forward
// action is refused for (its ingress may be denied), is relayed by nc from
// inside it from then on.
type vmPort struct {
	dialer    *stream.Dialer
	vm        workload.VM
	reference string
	port      uint
	relaying  atomic.Bool
}

// dial opens a connection to the port.
func (p *vmPort) dial(ctx context.Context) (io.ReadWriteCloser, error) {
	if !p.relaying.Load() {
		conn, err := p.dialer.Forward(ctx, p.vm.UUID, p.port)

		switch {
		case err == nil:
			return conn, nil
		case !stream.IsNotFound(err):
			return nil, err
		case !p.vm.Docker:
			return nil, fmt.Errorf("vm %s does not publish port %d, or its ingress is denied: %w", p.reference, p.port, err)
		}

		p.relaying.Store(true)
	}

	return p.dialer.Relay(ctx, p.vm.UUID, relayHost, p.port)
}
