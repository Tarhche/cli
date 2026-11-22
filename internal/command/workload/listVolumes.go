package workload

import (
	"context"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/formatter"
)

// listVolumesCommand lists the volumes of one's Docker VMs.
type listVolumesCommand struct {
	cli   *command.Cli
	flags *console.FlagSet

	quiet    bool
	format   string
	vm       string
	allUsers bool
}

func newListVolumesCommand(cli *command.Cli) *listVolumesCommand {
	return &listVolumesCommand{cli: cli}
}

func (c *listVolumesCommand) Name() string {
	return "ls"
}

func (c *listVolumesCommand) Description() string {
	return "List volumes"
}

func (c *listVolumesCommand) Usage() string {
	return command.Name + " volume ls [OPTIONS]"
}

func (c *listVolumesCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.quiet, console.Long("quiet"), "only shows volume names.", console.Short("q"))
	console.Var(flags, &c.format, console.Long("format"), "formats the output: table, json, or a Go template.")
	console.Var(flags, &c.vm, console.Long("vm"), "only shows the volumes of this Docker VM.")
	defineAllUsers(flags, &c.allUsers)
}

func (c *listVolumesCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *listVolumesCommand) run(ctx context.Context) error {
	if err := command.NoArgs(c.flags); err != nil {
		return err
	}

	client, err := clientOf(ctx, c.cli, c.allUsers)
	if err != nil {
		return err
	}

	vms, err := dockerVMsOf(ctx, client, c.vm)
	if err != nil {
		return err
	}

	volumes, err := client.VolumesIn(ctx, vms)
	if err != nil {
		return err
	}

	return writeRows(c.cli.Out, volumeRows(volumes), formatter.Options{
		Format:  c.format,
		Table:   volumeTable,
		Quiet:   c.quiet,
		ID:      "{{.Name}}",
		Headers: volumeHeaders,
	})
}
