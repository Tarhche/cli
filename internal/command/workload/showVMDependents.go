package workload

import (
	"context"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/formatter"
)

// showVMDependentsCommand lists what needs a VM, as deleting it would be
// answered with them now, and whether each makes it again once it is gone.
type showVMDependentsCommand struct {
	cli   *command.Cli
	flags *console.FlagSet

	quiet    bool
	format   string
	allUsers bool
}

func newShowVMDependentsCommand(cli *command.Cli) *showVMDependentsCommand {
	return &showVMDependentsCommand{cli: cli}
}

func (c *showVMDependentsCommand) Name() string {
	return "dependents"
}

func (c *showVMDependentsCommand) Description() string {
	return "lists the stacks and the containers that need a VM."
}

func (c *showVMDependentsCommand) Usage() string {
	return command.Name + " vm dependents [OPTIONS] VM"
}

func (c *showVMDependentsCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.quiet, console.Long("quiet"), "only shows their uuids.", console.Short("q"))
	console.Var(flags, &c.format, console.Long("format"), "formats the output: table, json, or a Go template.")
	defineAllUsers(flags, &c.allUsers)
}

func (c *showVMDependentsCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *showVMDependentsCommand) run(ctx context.Context) error {
	if err := command.ExactArgs(c.flags, 1); err != nil {
		return err
	}

	client, err := clientOf(ctx, c.cli, c.allUsers)
	if err != nil {
		return err
	}

	vm, err := client.FindVM(ctx, c.flags.Arg(0))
	if err != nil {
		return err
	}

	removal, err := client.VMDependents(ctx, vm.Metadata.UUID)
	if err != nil {
		return err
	}

	return writeRows(c.cli.Out, dependentRows(removal), formatter.Options{
		Format: c.format,
		Table:  dependentTable,
		Quiet:  c.quiet,
		ID:     "{{.UUID}}",
	})
}
