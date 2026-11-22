package workload

import (
	"context"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/formatter"
)

// listStacksCommand lists stacks.
type listStacksCommand struct {
	cli   *command.Cli
	flags *console.FlagSet

	quiet    bool
	format   string
	vm       string
	allUsers bool
}

func newListStacksCommand(cli *command.Cli) *listStacksCommand {
	return &listStacksCommand{cli: cli}
}

func (c *listStacksCommand) Name() string {
	return "ls"
}

func (c *listStacksCommand) Description() string {
	return "lists stacks."
}

func (c *listStacksCommand) Usage() string {
	return command.Name + " stack ls [OPTIONS]"
}

func (c *listStacksCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.quiet, console.Long("quiet"), "only shows stack uuids.", console.Short("q"))
	console.Var(flags, &c.format, console.Long("format"), "formats the output: table, json, or a Go template.")
	console.Var(flags, &c.vm, console.Long("vm"), "only shows the stacks deployed into this Docker VM.")
	defineAllUsers(flags, &c.allUsers)
}

func (c *listStacksCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *listStacksCommand) run(ctx context.Context) error {
	if err := command.NoArgs(c.flags); err != nil {
		return err
	}

	client, err := clientOf(ctx, c.cli, c.allUsers)
	if err != nil {
		return err
	}

	vmUUID, err := vmUUIDOf(ctx, client, c.vm)
	if err != nil {
		return err
	}

	stacks, err := client.Stacks(ctx, vmUUID)
	if err != nil {
		return err
	}

	rows := make([]stackRow, len(stacks))
	for i, s := range stacks {
		rows[i] = stackRowOf(s)
	}

	return writeRows(c.cli.Out, rows, formatter.Options{
		Format:  c.format,
		Table:   withOwner(stackTable, c.allUsers),
		Quiet:   c.quiet,
		ID:      "{{.UUID}}",
		Headers: stackHeaders,
	})
}
