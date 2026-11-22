package workload

import (
	"context"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/formatter"
)

// listImagesCommand lists the images one's Docker VMs hold.
type listImagesCommand struct {
	cli   *command.Cli
	flags *console.FlagSet

	quiet       bool
	format      string
	untruncated bool
	vm          string
	allUsers    bool
}

func newListImagesCommand(cli *command.Cli) *listImagesCommand {
	return &listImagesCommand{cli: cli}
}

func (c *listImagesCommand) Name() string {
	return "ls"
}

func (c *listImagesCommand) Description() string {
	return "List images"
}

func (c *listImagesCommand) Usage() string {
	return command.Name + " image ls [OPTIONS]"
}

func (c *listImagesCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.quiet, console.Long("quiet"), "only shows image ids.", console.Short("q"))
	console.Var(flags, &c.format, console.Long("format"), "formats the output: table, json, or a Go template.")
	console.Var(flags, &c.untruncated, console.Long("no-trunc"), "does not truncate the output.")
	console.Var(flags, &c.vm, console.Long("vm"), "only shows the images of this Docker VM.")
	defineAllUsers(flags, &c.allUsers)
}

func (c *listImagesCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *listImagesCommand) run(ctx context.Context) error {
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

	images, err := client.ImagesIn(ctx, vms)
	if err != nil {
		return err
	}

	return writeRows(c.cli.Out, imageRows(images, c.untruncated), formatter.Options{
		Format:  c.format,
		Table:   imageTable,
		Quiet:   c.quiet,
		ID:      "{{.ID}}",
		Headers: imageHeaders,
	})
}
