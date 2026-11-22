package system

import (
	"context"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/formatter"
)

// listContextsCommand is docker's context ls: every context, the one the CLI
// talks to marked.
type listContextsCommand struct {
	cli   *command.Cli
	flags *console.FlagSet

	quiet  bool
	format string
}

func newListContextsCommand(cli *command.Cli) *listContextsCommand {
	return &listContextsCommand{cli: cli}
}

func (c *listContextsCommand) Name() string {
	return "ls"
}

func (c *listContextsCommand) Description() string {
	return "List contexts"
}

func (c *listContextsCommand) Usage() string {
	return command.Name + " context ls [OPTIONS]"
}

func (c *listContextsCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.quiet, console.Long("quiet"), "Only show context names", console.Short("q"))
	console.Var(flags, &c.format, console.Long("format"), "Format output using a custom template: 'table' (the default), 'table TEMPLATE', 'json', or 'TEMPLATE'")
}

func (c *listContextsCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *listContextsCommand) run(context.Context) error {
	if err := command.NoArgs(c.flags); err != nil {
		return err
	}

	cfg, err := c.cli.Config()
	if err != nil {
		return err
	}

	current, err := c.cli.ContextName()
	if err != nil {
		return err
	}

	var items []any

	for _, name := range cfg.Names() {
		found, err := cfg.Context(name)
		if err != nil {
			return err
		}

		items = append(items, viewOf(found, current))
	}

	return formatter.Write(c.cli.Out, items, formatter.Options{Format: c.format, Table: contextTable, Quiet: c.quiet, ID: "{{.Name}}"})
}
