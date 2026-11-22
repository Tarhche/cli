// Package reach opens connections to the ports of workloads, for the CLI's
// port-forward and the desktop app's alike: a VM's through the forward
// action while it publishes the port, and relayed by nc from inside a Docker
// VM otherwise; a container's relayed by nc from inside its Docker VM to the
// address the VM's dockerd gave it.
package reach

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"slices"
	"strings"
	"sync/atomic"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/portforward"
	"github.com/Tarhche/cli/internal/stream"
	"github.com/Tarhche/cli/internal/version"
	"github.com/Tarhche/cli/internal/workload"
)

const (
	// relayHost is where a Docker VM's own ports are relayed to from inside
	// it.
	relayHost = "127.0.0.1"

	// containerAddresses is how docker inspect is asked for the addresses of
	// a container, one per network it is on.
	containerAddresses = "{{range .NetworkSettings.Networks}}{{.IPAddress}} {{end}}"
)

// VMPort is how a connection to the port of vm, which reference names, is
// opened. A VM that runs no dockerd and does not publish the port has
// nothing that could relay it, which is an error now rather than at every
// connection.
func VMPort(dialer *stream.Dialer, vm workload.VM, reference string, port uint) (portforward.Dial, error) {
	if !vm.Docker && !slices.Contains(vm.Ports, port) {
		return nil, fmt.Errorf("vm %s does not publish port %d, and runs no dockerd to relay it from inside: publish the port to forward it", reference, port)
	}

	p := &vmPort{dialer: dialer, vm: vm, reference: reference, port: port}
	p.relaying.Store(!slices.Contains(vm.Ports, port))

	return p.dial, nil
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

// ContainerAddress is the address the dockerd of a container's VM gave it,
// on the first network it is on.
func ContainerAddress(ctx context.Context, dialer *stream.Dialer, container workload.Container, reference string) (string, error) {
	var inspected bytes.Buffer

	inspect := []string{"docker", "inspect", "--format", containerAddresses, container.DockerID}
	if err := dialer.Runner(container.VM.UUID).Run(ctx, inspect, nil, &inspected); err != nil {
		return "", fmt.Errorf("finding the address of container %s: %w", reference, err)
	}

	found := strings.Fields(inspected.String())
	if len(found) == 0 {
		return "", fmt.Errorf("container %s has no address to forward to: is it running?", reference)
	}

	return found[0], nil
}

// ContainerPort is how a connection to the port of a container at address
// is opened: relayed by nc from inside its Docker VM.
func ContainerPort(dialer *stream.Dialer, container workload.Container, address string, port uint) portforward.Dial {
	return func(ctx context.Context) (io.ReadWriteCloser, error) {
		return dialer.Relay(ctx, container.VM.UUID, address, port)
	}
}

// Dialer is what streams are opened through in the context cli talks to: its
// workload ingress, signed in with its session.
func Dialer(cli *command.Cli) (*stream.Dialer, error) {
	current, err := cli.Context()
	if err != nil {
		return nil, err
	}

	if len(current.Ingress) == 0 {
		return nil, fmt.Errorf("the context %q names no workload ingress: give it one, or name one with --ingress", current.Name)
	}

	tokens, err := cli.Tokens()
	if err != nil {
		return nil, err
	}

	return stream.NewDialer(current.Ingress, tokens, stream.WithUserAgent(version.UserAgent()))
}
