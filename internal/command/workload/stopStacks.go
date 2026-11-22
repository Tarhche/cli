package workload

import (
	"context"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/workload"
)

// stopStacksCommand stops the containers of stacks.
type stopStacksCommand struct {
	cli   *command.Cli
	flags *console.FlagSet

	allUsers bool
}

func newStopStacksCommand(cli *command.Cli) *stopStacksCommand {
	return &stopStacksCommand{cli: cli}
}

func (c *stopStacksCommand) Name() string {
	return "stop"
}

func (c *stopStacksCommand) Description() string {
	return "stops the containers of one or more stacks."
}

func (c *stopStacksCommand) Usage() string {
	return command.Name + " stack stop [OPTIONS] STACK [STACK...]"
}

func (c *stopStacksCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	defineAllUsers(flags, &c.allUsers)
}

func (c *stopStacksCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *stopStacksCommand) run(ctx context.Context) error {
	if err := command.MinArgs(c.flags, 1); err != nil {
		return err
	}

	return actOnStacks(ctx, c.cli, c.allUsers, c.flags.Args(), func(client *workload.Client, stack workload.StackManifest) error {
		return client.StopStack(ctx, stack.Metadata.UUID)
	})
}
