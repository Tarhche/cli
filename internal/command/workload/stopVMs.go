package workload

import (
	"context"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/workload"
)

// stopVMsCommand stops running VMs.
type stopVMsCommand struct {
	cli   *command.Cli
	flags *console.FlagSet

	allUsers bool
}

func newStopVMsCommand(cli *command.Cli) *stopVMsCommand {
	return &stopVMsCommand{cli: cli}
}

func (c *stopVMsCommand) Name() string {
	return "stop"
}

func (c *stopVMsCommand) Description() string {
	return "stops one or more running VMs."
}

func (c *stopVMsCommand) Usage() string {
	return command.Name + " vm stop [OPTIONS] VM [VM...]"
}

func (c *stopVMsCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	defineAllUsers(flags, &c.allUsers)
}

func (c *stopVMsCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *stopVMsCommand) run(ctx context.Context) error {
	if err := command.MinArgs(c.flags, 1); err != nil {
		return err
	}

	return actOnVMs(ctx, c.cli, c.allUsers, c.flags.Args(), func(client *workload.Client, vm workload.VMManifest) error {
		return client.StopVM(ctx, vm.Metadata.UUID)
	})
}
