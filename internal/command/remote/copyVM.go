package remote

import (
	"context"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/transfer"
)

// copyVMCommand copies files and directories between a VM and here, by tar
// and a shell in the VM.
type copyVMCommand struct {
	remote *remote
	flags  *console.FlagSet

	archive    bool
	followLink bool
	quiet      bool
}

var _ console.Command = (*copyVMCommand)(nil)

func newCopyVMCommand(r *remote) *copyVMCommand {
	return &copyVMCommand{remote: r}
}

func (c *copyVMCommand) Name() string {
	return "cp"
}

func (c *copyVMCommand) Description() string {
	return "Copy files/folders between a VM and the local filesystem"
}

func (c *copyVMCommand) Usage() string {
	return "cp [OPTIONS] VM:SRC_PATH DEST_PATH|-\n  cp [OPTIONS] SRC_PATH|- VM:DEST_PATH"
}

func (c *copyVMCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.archive, console.Long("archive"), archiveUsage, console.Short("a"))
	console.Var(flags, &c.followLink, console.Long("follow-link"), followLinkUsage, console.Short("L"))
	console.Var(flags, &c.quiet, console.Long("quiet"), quietUsage, console.Short("q"))
}

func (c *copyVMCommand) Run(ctx context.Context) console.ExitStatus {
	return c.remote.cli.Exit(c.run(ctx))
}

func (c *copyVMCommand) run(ctx context.Context) error {
	if err := command.ExactArgs(c.flags, 2); err != nil {
		return err
	}

	plan, err := planCopy(c.flags.Arg(0), c.flags.Arg(1), "VM")
	if err != nil {
		return err
	}

	vm, err := c.remote.vm(ctx, plan.reference)
	if err != nil {
		return err
	}

	dialer, err := c.remote.dialer()
	if err != nil {
		return err
	}

	options := transfer.Options{FollowLink: c.followLink, CopyUIDGID: c.archive}

	return c.remote.copy(ctx, plan, transfer.VM(dialer.Runner(vm.UUID)), options, c.quiet, "vm")
}
