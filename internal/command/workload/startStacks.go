package workload

import (
	"context"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/workload"
)

// startStacksCommand starts the containers of stopped stacks.
type startStacksCommand struct {
	cli   *command.Cli
	flags *console.FlagSet

	allUsers bool
}

func newStartStacksCommand(cli *command.Cli) *startStacksCommand {
	return &startStacksCommand{cli: cli}
}

func (c *startStacksCommand) Name() string {
	return "start"
}

func (c *startStacksCommand) Description() string {
	return "Start the containers of one or more stacks"
}

func (c *startStacksCommand) Usage() string {
	return command.Name + " stack start [OPTIONS] STACK [STACK...]"
}

func (c *startStacksCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	defineAllUsers(flags, &c.allUsers)
}

func (c *startStacksCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *startStacksCommand) run(ctx context.Context) error {
	if err := command.MinArgs(c.flags, 1); err != nil {
		return err
	}

	return actOnStacks(ctx, c.cli, c.allUsers, c.flags.Args(), func(client *workload.Client, stack workload.StackManifest) error {
		return client.StartStack(ctx, stack.Metadata.UUID)
	})
}
