package system

import (
	"context"
	"fmt"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
)

// showContextCommand is docker's context show: the name of the context the
// CLI talks to.
type showContextCommand struct {
	cli   *command.Cli
	flags *console.FlagSet
}

func newShowContextCommand(cli *command.Cli) *showContextCommand {
	return &showContextCommand{cli: cli}
}

func (c *showContextCommand) Name() string {
	return "show"
}

func (c *showContextCommand) Description() string {
	return "Print the name of the current context"
}

func (c *showContextCommand) Usage() string {
	return command.Name + " context show"
}

func (c *showContextCommand) Configure(flags *console.FlagSet) {
	c.flags = flags
}

func (c *showContextCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *showContextCommand) run(context.Context) error {
	if err := command.NoArgs(c.flags); err != nil {
		return err
	}

	name, err := c.cli.ContextName()
	if err != nil {
		return err
	}

	_, err = fmt.Fprintln(c.cli.Out, name)

	return err
}
