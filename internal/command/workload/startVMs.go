package workload

import (
	"context"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/workload"
)

// startVMsCommand starts stopped VMs.
type startVMsCommand struct {
	cli   *command.Cli
	flags *console.FlagSet

	allUsers bool
}

func newStartVMsCommand(cli *command.Cli) *startVMsCommand {
	return &startVMsCommand{cli: cli}
}

func (c *startVMsCommand) Name() string {
	return "start"
}

func (c *startVMsCommand) Description() string {
	return "Start one or more stopped VMs"
}

func (c *startVMsCommand) Usage() string {
	return command.Name + " vm start [OPTIONS] VM [VM...]"
}

func (c *startVMsCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	defineAllUsers(flags, &c.allUsers)
}

func (c *startVMsCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *startVMsCommand) run(ctx context.Context) error {
	if err := command.MinArgs(c.flags, 1); err != nil {
		return err
	}

	return actOnVMs(ctx, c.cli, c.allUsers, c.flags.Args(), func(client *workload.Client, vm workload.VMManifest) error {
		return client.StartVM(ctx, vm.Metadata.UUID)
	})
}
