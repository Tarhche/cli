package remote

import (
	"context"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/stream"
)

// attachVMCommand opens a terminal on a VM: a shell in it.
type attachVMCommand struct {
	remote *remote
	flags  *console.FlagSet
}

var _ console.Command = (*attachVMCommand)(nil)

func newAttachVMCommand(r *remote) *attachVMCommand {
	return &attachVMCommand{remote: r}
}

func (c *attachVMCommand) Name() string {
	return "attach"
}

func (c *attachVMCommand) Description() string {
	return "Attach a terminal to a running VM: a shell in it"
}

func (c *attachVMCommand) Usage() string {
	return "attach VM"
}

func (c *attachVMCommand) Configure(flags *console.FlagSet) {
	c.flags = flags
}

func (c *attachVMCommand) Run(ctx context.Context) console.ExitStatus {
	return c.remote.cli.Exit(c.run(ctx))
}

func (c *attachVMCommand) run(ctx context.Context) error {
	if err := command.ExactArgs(c.flags, 1); err != nil {
		return err
	}

	vm, err := c.remote.vm(ctx, c.flags.Arg(0))
	if err != nil {
		return err
	}

	dialer, err := c.remote.dialer()
	if err != nil {
		return err
	}

	terminal, err := dialer.Attach(ctx, stream.KindVMs, vm.UUID)
	if err != nil {
		return err
	}

	defer func() { _ = terminal.Close() }()

	attached := attachment{tty: true, input: true}

	if !c.remote.terminal.IsTerminal(c.remote.cli.In) {
		attached.end = func() error {
			_, err := terminal.Write([]byte{endOfTransmission})

			return err
		}
	}

	return c.remote.attach(ctx, terminal, attached)
}
