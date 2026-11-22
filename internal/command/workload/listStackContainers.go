package workload

import (
	"context"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/formatter"
	"github.com/Tarhche/cli/internal/workload"
)

// listStackContainersCommand lists the containers a stack's services were
// made into, as docker's stack ps lists a stack's tasks.
type listStackContainersCommand struct {
	cli   *command.Cli
	flags *console.FlagSet

	quiet       bool
	format      string
	untruncated bool
	allUsers    bool
}

func newListStackContainersCommand(cli *command.Cli) *listStackContainersCommand {
	return &listStackContainersCommand{cli: cli}
}

func (c *listStackContainersCommand) Name() string {
	return "ps"
}

func (c *listStackContainersCommand) Description() string {
	return "lists the containers of a stack."
}

func (c *listStackContainersCommand) Usage() string {
	return command.Name + " stack ps [OPTIONS] STACK"
}

func (c *listStackContainersCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.quiet, console.Long("quiet"), "only shows container ids.", console.Short("q"))
	console.Var(flags, &c.format, console.Long("format"), "formats the output: table, json, or a Go template.")
	console.Var(flags, &c.untruncated, console.Long("no-trunc"), "does not truncate the output.")
	defineAllUsers(flags, &c.allUsers)
}

func (c *listStackContainersCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *listStackContainersCommand) run(ctx context.Context) error {
	if err := command.ExactArgs(c.flags, 1); err != nil {
		return err
	}

	client, err := clientOf(ctx, c.cli, c.allUsers)
	if err != nil {
		return err
	}

	stack, err := client.FindStack(ctx, c.flags.Arg(0))
	if err != nil {
		return err
	}

	containers, err := client.Containers(ctx, stack.Spec.VM.UUID)
	if err != nil {
		return err
	}

	var made []workload.ContainerManifest

	for _, container := range containers {
		if container.Metadata.Labels[workload.LabelStack] == stack.Metadata.UUID || container.Status.Labels[workload.LabelStack] == stack.Metadata.UUID {
			made = append(made, container)
		}
	}

	return writeRows(c.cli.Out, containerRows(made, c.untruncated), formatter.Options{
		Format:  c.format,
		Table:   stackContainersTable,
		Quiet:   c.quiet,
		ID:      "{{.ID}}",
		Headers: containerHeaders,
	})
}
