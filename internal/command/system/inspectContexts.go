package system

import (
	"context"
	"errors"
	"strings"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/formatter"
)

// inspectContextsCommand is docker's context inspect: contexts in detail, as
// JSON, the current one when none is named.
type inspectContextsCommand struct {
	cli   *command.Cli
	flags *console.FlagSet

	format string
}

func newInspectContextsCommand(cli *command.Cli) *inspectContextsCommand {
	return &inspectContextsCommand{cli: cli}
}

func (c *inspectContextsCommand) Name() string {
	return "inspect"
}

func (c *inspectContextsCommand) Description() string {
	return "Display detailed information on one or more contexts"
}

func (c *inspectContextsCommand) Usage() string {
	return command.Name + " context inspect [OPTIONS] [CONTEXT...]"
}

func (c *inspectContextsCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.format, console.Long("format"), "Format output using a custom template: 'json' prints each as JSON, 'TEMPLATE' prints each through the given Go template", console.Short("f"))
}

func (c *inspectContextsCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *inspectContextsCommand) run(context.Context) error {
	cfg, err := c.cli.Config()
	if err != nil {
		return err
	}

	current, err := c.cli.ContextName()
	if err != nil {
		return err
	}

	names := c.flags.Args()
	if len(names) == 0 {
		names = []string{current}
	}

	var (
		items    []any
		failures []error
	)

	for _, name := range names {
		found, err := cfg.Context(name)
		if err != nil {
			failures = append(failures, err)

			continue
		}

		items = append(items, inspectionOf(found, current))
	}

	if len(strings.TrimSpace(c.format)) == 0 {
		if items == nil {
			items = []any{}
		}

		err = formatter.WriteJSON(c.cli.Out, items)
	} else {
		err = formatter.Write(c.cli.Out, items, formatter.Options{Format: c.format})
	}

	return errors.Join(append(failures, err)...)
}
