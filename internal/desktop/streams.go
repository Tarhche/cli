package desktop

import (
	"context"
	"errors"
	"io"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/portforward"
	"github.com/Tarhche/cli/internal/reach"
	"github.com/Tarhche/cli/internal/stream"
	"github.com/Tarhche/cli/internal/transfer"
	"github.com/Tarhche/cli/internal/workload"
)

// localhost is where a forwarded port listens on the host.
const localhost = "127.0.0.1"

// containerShell is what a terminal in a container runs when it is asked to
// run nothing: bash where the container has one, sh where it does not.
var containerShell = []string{"sh", "-c", "if command -v bash >/dev/null 2>&1; then exec bash; else exec sh; fi"}

// Streams reaches into workloads through the workload ingress of the context
// a Cli talks to, as the CLI's attach, exec, port-forward and cp do: a VM
// through its own streams, and a container through its Docker VM's.
type Streams struct {
	forwarder *portforward.Forwarder
}

var (
	_ TerminalOpener = (*Streams)(nil)
	_ Forwarder      = (*Streams)(nil)
	_ Copier         = (*Streams)(nil)
)

// NewStreams is what reaches into workloads.
func NewStreams() *Streams {
	return &Streams{forwarder: portforward.New()}
}

// Open opens a terminal: a VM's own shell (its attach stream), a command run
// on a terminal in a VM, or one run in a container with docker exec.
func (s *Streams) Open(ctx context.Context, cli *command.Cli, request TerminalRequest) (Terminal, error) {
	dialer, resolver, err := reachOf(ctx, cli)
	if err != nil {
		return nil, err
	}

	if len(request.Target.Container) > 0 {
		container, err := resolver.Container(ctx, request.Target.Container)
		if err != nil {
			return nil, err
		}

		command := request.Command
		if len(command) == 0 {
			command = containerShell
		}

		return execute(ctx, dialer, container.VM.UUID, append([]string{"docker", "exec", "-i", "-t", container.DockerID}, command...), request.Size)
	}

	vm, err := resolver.VM(ctx, request.Target.VM)
	if err != nil {
		return nil, err
	}

	if len(request.Command) > 0 {
		return execute(ctx, dialer, vm.UUID, request.Command, request.Size)
	}

	terminal, err := dialer.Attach(ctx, stream.KindVMs, vm.UUID)
	if err != nil {
		return nil, err
	}

	attached := attachedTerminal{terminal: terminal}
	if err := attached.Resize(request.Size); err != nil {
		_ = terminal.Close()

		return nil, err
	}

	return attached, nil
}

// Forward forwards a port of a VM or a container to the host, until it is
// closed or ctx is done.
func (s *Streams) Forward(ctx context.Context, cli *command.Cli, request ForwardRequest) (PortForward, error) {
	dialer, resolver, err := reachOf(ctx, cli)
	if err != nil {
		return nil, err
	}

	var dial portforward.Dial

	if len(request.Target.Container) > 0 {
		container, err := resolver.Container(ctx, request.Target.Container)
		if err != nil {
			return nil, err
		}

		address, err := reach.ContainerAddress(ctx, dialer, container, request.Target.Container)
		if err != nil {
			return nil, err
		}

		dial = reach.ContainerPort(dialer, container, address, uint(request.Port))
	} else {
		vm, err := resolver.VM(ctx, request.Target.VM)
		if err != nil {
			return nil, err
		}

		if dial, err = reach.VMPort(dialer, vm, request.Target.VM, uint(request.Port)); err != nil {
			return nil, err
		}
	}

	forward, err := s.forwarder.Start(portforward.Spec{Address: localhost, Port: request.LocalPort, Dial: dial})
	if err != nil {
		return nil, err
	}

	return forwardedPort{forward: forward, stop: context.AfterFunc(ctx, func() { _ = forward.Close() })}, nil
}

// CopyIn copies what is at the request's local path into the workload, at
// its path, as docker cp does.
func (s *Streams) CopyIn(ctx context.Context, cli *command.Cli, request CopyRequest) error {
	files, err := filesOf(ctx, cli, request.Target)
	if err != nil {
		return err
	}

	_, err = transfer.Upload(ctx, files, request.LocalPath, request.Path, nil, transfer.Options{})

	return err
}

// CopyOut copies what is at the request's path in the workload to its local
// path, as docker cp does.
func (s *Streams) CopyOut(ctx context.Context, cli *command.Cli, request CopyRequest) error {
	files, err := filesOf(ctx, cli, request.Target)
	if err != nil {
		return err
	}

	_, err = transfer.Download(ctx, files, request.Path, request.LocalPath, nil, transfer.Options{})

	return err
}

// reachOf is what reaches into the workloads of the context cli talks to:
// the streams of its ingress, and what its references name.
func reachOf(ctx context.Context, cli *command.Cli) (*stream.Dialer, workload.Resolver, error) {
	dialer, err := reach.Dialer(cli)
	if err != nil {
		return nil, nil, err
	}

	client, err := cli.Client(ctx)
	if err != nil {
		return nil, nil, err
	}

	return dialer, workload.NewResolver(client), nil
}

// filesOf are the files of a VM, or of a container in its Docker VM.
func filesOf(ctx context.Context, cli *command.Cli, target Target) (transfer.Remote, error) {
	dialer, resolver, err := reachOf(ctx, cli)
	if err != nil {
		return nil, err
	}

	if len(target.Container) > 0 {
		container, err := resolver.Container(ctx, target.Container)
		if err != nil {
			return nil, err
		}

		return transfer.Container(dialer.Runner(container.VM.UUID), container.DockerID), nil
	}

	vm, err := resolver.VM(ctx, target.VM)
	if err != nil {
		return nil, err
	}

	return transfer.VM(dialer.Runner(vm.UUID)), nil
}

// execute runs command on a terminal of size in vm.
func execute(ctx context.Context, dialer *stream.Dialer, vm string, command []string, size TerminalSize) (Terminal, error) {
	exec, err := dialer.Exec(ctx, vm, stream.ExecOptions{Command: command, TTY: true, Rows: uint(size.Rows), Cols: uint(size.Cols)})
	if err != nil {
		return nil, err
	}

	return executedTerminal{exec: exec}, nil
}

// attachedTerminal is a VM's own shell.
type attachedTerminal struct {
	terminal *stream.Terminal
}

func (t attachedTerminal) Read(p []byte) (int, error) {
	return t.terminal.Read(p)
}

func (t attachedTerminal) Write(p []byte) (int, error) {
	return t.terminal.Write(p)
}

// Resize tells the shell its terminal's size.
func (t attachedTerminal) Resize(size TerminalSize) error {
	return t.terminal.Resize(uint(size.Rows), uint(size.Cols))
}

func (t attachedTerminal) Close() error {
	return t.terminal.Close()
}

// executedTerminal is a command run on a terminal, which ends with the
// status the command exited with.
type executedTerminal struct {
	exec *stream.Exec
}

// Read reads what the command says, and is io.EOF once it exited with 0 and
// a command.StatusError once it exited with anything else.
func (t executedTerminal) Read(p []byte) (int, error) {
	n, err := t.exec.Read(p)
	if !errors.Is(err, io.EOF) {
		return n, err
	}

	code, err := t.exec.Wait()

	switch {
	case err != nil:
		return n, err
	case code != 0:
		return n, command.StatusError{Code: code}
	}

	return n, io.EOF
}

func (t executedTerminal) Write(p []byte) (int, error) {
	return t.exec.Write(p)
}

// Resize tells the command its terminal's size.
func (t executedTerminal) Resize(size TerminalSize) error {
	return t.exec.Resize(uint(size.Rows), uint(size.Cols))
}

func (t executedTerminal) Close() error {
	return t.exec.Close()
}

// forwardedPort is a port forwarded until it is closed, or until what it was
// forwarded for is done.
type forwardedPort struct {
	forward *portforward.Forward
	stop    func() bool
}

// Address is where it listens on the host.
func (f forwardedPort) Address() string {
	return f.forward.Addr().String()
}

// Wait waits until it stops, and is why when it stopped on its own.
func (f forwardedPort) Wait() error {
	<-f.forward.Done()

	return f.forward.Err()
}

// Close stops it.
func (f forwardedPort) Close() error {
	f.stop()

	return f.forward.Close()
}
