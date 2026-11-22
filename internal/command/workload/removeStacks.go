package workload

import (
	"context"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/workload"
)

// removeStacksCommand removes stacks, with their containers and networks;
// their volumes and their VMs are kept.
type removeStacksCommand struct {
	cli   *command.Cli
	flags *console.FlagSet

	allUsers bool
}

func newRemoveStacksCommand(cli *command.Cli) *removeStacksCommand {
	return &removeStacksCommand{cli: cli}
}

func (c *removeStacksCommand) Name() string {
	return "rm"
}

func (c *removeStacksCommand) Description() string {
	return "removes one or more stacks, with their containers and networks."
}

func (c *removeStacksCommand) Usage() string {
	return command.Name + " stack rm [OPTIONS] STACK [STACK...]"
}

func (c *removeStacksCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	defineAllUsers(flags, &c.allUsers)
}

func (c *removeStacksCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *removeStacksCommand) run(ctx context.Context) error {
	if err := command.MinArgs(c.flags, 1); err != nil {
		return err
	}

	return actOnStacks(ctx, c.cli, c.allUsers, c.flags.Args(), func(client *workload.Client, stack workload.StackManifest) error {
		return client.DeleteStack(ctx, stack.Metadata.UUID)
	})
}
