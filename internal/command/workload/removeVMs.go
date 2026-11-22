package workload

import (
	"context"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/workload"
)

// removeVMsCommand deletes VMs, saying what still needs each.
type removeVMsCommand struct {
	cli   *command.Cli
	flags *console.FlagSet

	cascade  bool
	allUsers bool
}

func newRemoveVMsCommand(cli *command.Cli) *removeVMsCommand {
	return &removeVMsCommand{cli: cli}
}

func (c *removeVMsCommand) Name() string {
	return "rm"
}

func (c *removeVMsCommand) Description() string {
	return "Delete one or more VMs"
}

func (c *removeVMsCommand) Usage() string {
	return command.Name + " vm rm [OPTIONS] VM [VM...]"
}

func (c *removeVMsCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.cascade, console.Long("cascade"), "deletes the stacks and the containers in it with it, which would make it again.")
	defineAllUsers(flags, &c.allUsers)
}

func (c *removeVMsCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *removeVMsCommand) run(ctx context.Context) error {
	if err := command.MinArgs(c.flags, 1); err != nil {
		return err
	}

	return actOnVMs(ctx, c.cli, c.allUsers, c.flags.Args(), func(client *workload.Client, vm workload.VMManifest) error {
		removal, err := client.DeleteVM(ctx, vm.Metadata.UUID, c.cascade)
		if err != nil {
			return err
		}

		return warnRemoval(c.cli.Err, orDefault(vm.Metadata.Name, vm.Metadata.UUID), removal)
	})
}
