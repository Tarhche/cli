package system

import (
	"context"
	"fmt"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
)

// useContextCommand is docker's context use: the context commands talk to
// unless told otherwise.
type useContextCommand struct {
	cli   *command.Cli
	flags *console.FlagSet
}

func newUseContextCommand(cli *command.Cli) *useContextCommand {
	return &useContextCommand{cli: cli}
}

func (c *useContextCommand) Name() string {
	return "use"
}

func (c *useContextCommand) Description() string {
	return "Set the current context"
}

func (c *useContextCommand) Usage() string {
	return command.Name + " context use CONTEXT"
}

func (c *useContextCommand) Configure(flags *console.FlagSet) {
	c.flags = flags
}

func (c *useContextCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *useContextCommand) run(context.Context) error {
	if err := command.ExactArgs(c.flags, 1); err != nil {
		return err
	}

	name := c.flags.Arg(0)

	cfg, err := c.cli.Config()
	if err != nil {
		return err
	}

	if err := cfg.Use(name); err != nil {
		return err
	}

	if err := cfg.Save(); err != nil {
		return err
	}

	if _, err := fmt.Fprintln(c.cli.Out, name); err != nil {
		return err
	}

	_, err = fmt.Fprintf(c.cli.Err, "Current context is now %q\n", name)

	return err
}
