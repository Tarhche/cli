package workload

import (
	"context"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/formatter"
)

// listNetworksCommand lists the networks of one's Docker VMs.
type listNetworksCommand struct {
	cli   *command.Cli
	flags *console.FlagSet

	quiet       bool
	format      string
	untruncated bool
	vm          string
	allUsers    bool
}

func newListNetworksCommand(cli *command.Cli) *listNetworksCommand {
	return &listNetworksCommand{cli: cli}
}

func (c *listNetworksCommand) Name() string {
	return "ls"
}

func (c *listNetworksCommand) Description() string {
	return "lists networks."
}

func (c *listNetworksCommand) Usage() string {
	return command.Name + " network ls [OPTIONS]"
}

func (c *listNetworksCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.quiet, console.Long("quiet"), "only shows network ids.", console.Short("q"))
	console.Var(flags, &c.format, console.Long("format"), "formats the output: table, json, or a Go template.")
	console.Var(flags, &c.untruncated, console.Long("no-trunc"), "does not truncate the output.")
	console.Var(flags, &c.vm, console.Long("vm"), "only shows the networks of this Docker VM.")
	defineAllUsers(flags, &c.allUsers)
}

func (c *listNetworksCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *listNetworksCommand) run(ctx context.Context) error {
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

	networks, err := client.NetworksIn(ctx, vms)
	if err != nil {
		return err
	}

	return writeRows(c.cli.Out, networkRows(networks, c.untruncated), formatter.Options{
		Format:  c.format,
		Table:   networkTable,
		Quiet:   c.quiet,
		ID:      "{{.ID}}",
		Headers: networkHeaders,
	})
}
