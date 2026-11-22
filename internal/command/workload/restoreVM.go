package workload

import (
	"context"
	"fmt"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
)

// restoreVMCommand replaces a VM's disk with a snapshot's.
type restoreVMCommand struct {
	cli   *command.Cli
	flags *console.FlagSet

	snapshot string
	allUsers bool
}

func newRestoreVMCommand(cli *command.Cli) *restoreVMCommand {
	return &restoreVMCommand{cli: cli}
}

func (c *restoreVMCommand) Name() string {
	return "restore"
}

func (c *restoreVMCommand) Description() string {
	return "Replace a VM's disk with a snapshot's"
}

func (c *restoreVMCommand) Usage() string {
	return command.Name + " vm restore [OPTIONS] --snapshot SNAPSHOT VM"
}

func (c *restoreVMCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.snapshot, console.Long("snapshot"), "the snapshot whose disk replaces the VM's.")
	defineAllUsers(flags, &c.allUsers)
}

func (c *restoreVMCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *restoreVMCommand) run(ctx context.Context) error {
	if err := command.ExactArgs(c.flags, 1); err != nil {
		return err
	}

	if len(c.snapshot) == 0 {
		return usageError(c.flags, "a snapshot is required: give it with --snapshot")
	}

	client, err := clientOf(ctx, c.cli, c.allUsers)
	if err != nil {
		return err
	}

	vm, err := client.FindVM(ctx, c.flags.Arg(0))
	if err != nil {
		return err
	}

	snapshot, err := client.FindSnapshot(ctx, c.snapshot)
	if err != nil {
		return err
	}

	if err := client.RestoreVM(ctx, vm.Metadata.UUID, snapshot.Metadata.UUID); err != nil {
		return err
	}

	_, err = fmt.Fprintln(c.cli.Out, c.flags.Arg(0))

	return err
}
