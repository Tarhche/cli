package workload

import (
	"context"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/workload"
)

// restartStacksCommand restarts the containers of stacks.
type restartStacksCommand struct {
	cli   *command.Cli
	flags *console.FlagSet

	allUsers bool
}

func newRestartStacksCommand(cli *command.Cli) *restartStacksCommand {
	return &restartStacksCommand{cli: cli}
}

func (c *restartStacksCommand) Name() string {
	return "restart"
}

func (c *restartStacksCommand) Description() string {
	return "restarts the containers of one or more stacks."
}

func (c *restartStacksCommand) Usage() string {
	return command.Name + " stack restart [OPTIONS] STACK [STACK...]"
}

func (c *restartStacksCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	defineAllUsers(flags, &c.allUsers)
}

func (c *restartStacksCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *restartStacksCommand) run(ctx context.Context) error {
	if err := command.MinArgs(c.flags, 1); err != nil {
		return err
	}

	return actOnStacks(ctx, c.cli, c.allUsers, c.flags.Args(), func(client *workload.Client, stack workload.StackManifest) error {
		return client.RestartStack(ctx, stack.Metadata.UUID)
	})
}
