package workload

import (
	"context"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/workload"
)

// restartVMsCommand stops VMs and starts them again in place.
type restartVMsCommand struct {
	cli   *command.Cli
	flags *console.FlagSet

	allUsers bool
}

func newRestartVMsCommand(cli *command.Cli) *restartVMsCommand {
	return &restartVMsCommand{cli: cli}
}

func (c *restartVMsCommand) Name() string {
	return "restart"
}

func (c *restartVMsCommand) Description() string {
	return "Restart one or more VMs"
}

func (c *restartVMsCommand) Usage() string {
	return command.Name + " vm restart [OPTIONS] VM [VM...]"
}

func (c *restartVMsCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	defineAllUsers(flags, &c.allUsers)
}

func (c *restartVMsCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *restartVMsCommand) run(ctx context.Context) error {
	if err := command.MinArgs(c.flags, 1); err != nil {
		return err
	}

	return actOnVMs(ctx, c.cli, c.allUsers, c.flags.Args(), func(client *workload.Client, vm workload.VMManifest) error {
		return client.RestartVM(ctx, vm.Metadata.UUID)
	})
}
