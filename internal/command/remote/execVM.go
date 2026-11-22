package remote

import (
	"context"
	"errors"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/stream"
	"github.com/Tarhche/cli/internal/workload"
)

// errNotTTY is a terminal asked for with input that is not one.
var errNotTTY = errors.New("the input device is not a TTY")

// execVMCommand runs a command in a VM, and exits with its status.
type execVMCommand struct {
	remote *remote
	flags  *console.FlagSet

	interactive bool
	tty         bool
	env         list
	workdir     string
	allUsers    bool
}

var _ console.Command = (*execVMCommand)(nil)

func newExecVMCommand(r *remote) *execVMCommand {
	return &execVMCommand{remote: r}
}

func (c *execVMCommand) Name() string {
	return "exec"
}

func (c *execVMCommand) Description() string {
	return "Execute a command in a running VM"
}

func (c *execVMCommand) Usage() string {
	return "exec [OPTIONS] VM COMMAND [ARG...]"
}

func (c *execVMCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.interactive, console.Long("interactive"), "Keep STDIN open even if not attached", console.Short("i"))
	console.Var(flags, &c.tty, console.Long("tty"), "Allocate a pseudo-TTY", console.Short("t"))
	flags.Var(&c.env, console.Long("env"), "Set environment variables", console.Short("e"))
	console.Var(flags, &c.workdir, console.Long("workdir"), "Working directory inside the VM", console.Short("w"))
	defineAllUsers(flags, &c.allUsers, "VM")
}

func (c *execVMCommand) Run(ctx context.Context) console.ExitStatus {
	return c.remote.cli.Exit(c.run(ctx))
}

func (c *execVMCommand) run(ctx context.Context) error {
	if err := command.MinArgs(c.flags, 2); err != nil {
		return err
	}

	env, err := c.remote.environment(c.env)
	if err != nil {
		return err
	}

	if c.tty && c.interactive && !c.remote.terminal.IsTerminal(c.remote.cli.In) {
		return errNotTTY
	}

	audience := audienceOf(c.allUsers)

	vm, err := c.remote.vm(ctx, audience, c.flags.Arg(0))
	if err != nil {
		return err
	}

	options := stream.ExecOptions{Command: c.flags.Args()[1:], TTY: c.tty, Env: env, WorkingDir: c.workdir, Stderr: c.remote.cli.Err}

	return c.remote.exec(ctx, audience, vm.UUID, options, c.interactive)
}

// exec runs a command in the VM uuid, one of the VMs of audience, attached
// to the local terminal, its input the local input when interactive says
// so, and is how it exited.
func (r *remote) exec(ctx context.Context, audience workload.Audience, vm string, options stream.ExecOptions, interactive bool) error {
	dialer, err := r.dialer(audience)
	if err != nil {
		return err
	}

	if options.TTY {
		options.Rows, options.Cols = r.size()
	}

	session, err := dialer.Exec(ctx, vm, options)
	if err != nil {
		return err
	}

	defer func() { _ = session.Close() }()

	if !interactive {
		_ = session.CloseWrite()
	}

	attached := attachment{tty: options.TTY, rows: options.Rows, cols: options.Cols, input: interactive, end: session.CloseWrite}
	if err := r.attach(ctx, session, attached); err != nil {
		return err
	}

	return exitStatus(session.Wait())
}
