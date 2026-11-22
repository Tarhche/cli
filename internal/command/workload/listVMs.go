package workload

import (
	"context"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/formatter"
)

// listVMsCommand lists VMs, machines and Docker VMs alike.
type listVMsCommand struct {
	cli   *command.Cli
	flags *console.FlagSet

	quiet    bool
	format   string
	allUsers bool
}

func newListVMsCommand(cli *command.Cli) *listVMsCommand {
	return &listVMsCommand{cli: cli}
}

func (c *listVMsCommand) Name() string {
	return "ls"
}

func (c *listVMsCommand) Description() string {
	return "List VMs"
}

func (c *listVMsCommand) Usage() string {
	return command.Name + " vm ls [OPTIONS]"
}

func (c *listVMsCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.quiet, console.Long("quiet"), "only shows VM uuids.", console.Short("q"))
	console.Var(flags, &c.format, console.Long("format"), "formats the output: table, json, or a Go template.")
	defineAllUsers(flags, &c.allUsers)
}

func (c *listVMsCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *listVMsCommand) run(ctx context.Context) error {
	if err := command.NoArgs(c.flags); err != nil {
		return err
	}

	client, err := clientOf(ctx, c.cli, c.allUsers)
	if err != nil {
		return err
	}

	vms, err := client.VMs(ctx)
	if err != nil {
		return err
	}

	rows := make([]vmRow, len(vms))
	for i, vm := range vms {
		rows[i] = vmRowOf(vm)
	}

	return writeRows(c.cli.Out, rows, formatter.Options{
		Format:  c.format,
		Table:   withOwner(vmTable, c.allUsers),
		Quiet:   c.quiet,
		ID:      "{{.UUID}}",
		Headers: vmHeaders,
	})
}
