package workload

import (
	"context"
	"encoding/json"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/workload"
)

// inspectStacksCommand writes stacks as the API answers them.
type inspectStacksCommand struct {
	cli   *command.Cli
	flags *console.FlagSet

	format   string
	allUsers bool
}

func newInspectStacksCommand(cli *command.Cli) *inspectStacksCommand {
	return &inspectStacksCommand{cli: cli}
}

func (c *inspectStacksCommand) Name() string {
	return "inspect"
}

func (c *inspectStacksCommand) Description() string {
	return "displays detailed information on one or more stacks."
}

func (c *inspectStacksCommand) Usage() string {
	return command.Name + " stack inspect [OPTIONS] STACK [STACK...]"
}

func (c *inspectStacksCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.format, console.Long("format"), "formats the output with a Go template.", console.Short("f"))
	defineAllUsers(flags, &c.allUsers)
}

func (c *inspectStacksCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *inspectStacksCommand) run(ctx context.Context) error {
	if err := command.MinArgs(c.flags, 1); err != nil {
		return err
	}

	client, err := clientOf(ctx, c.cli, c.allUsers)
	if err != nil {
		return err
	}

	stacks, err := client.Stacks(ctx, "")
	if err != nil {
		return err
	}

	return inspectEach(c.cli, c.flags.Args(), c.format, func(reference string) (json.RawMessage, error) {
		stack, err := workload.MatchStack(stacks, reference)

		return stack.Raw(), err
	})
}
