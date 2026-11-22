package workload

import (
	"context"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/formatter"
)

// listSnapshotsCommand lists snapshots.
type listSnapshotsCommand struct {
	cli   *command.Cli
	flags *console.FlagSet

	quiet    bool
	format   string
	vm       string
	allUsers bool
}

func newListSnapshotsCommand(cli *command.Cli) *listSnapshotsCommand {
	return &listSnapshotsCommand{cli: cli}
}

func (c *listSnapshotsCommand) Name() string {
	return "ls"
}

func (c *listSnapshotsCommand) Description() string {
	return "List snapshots"
}

func (c *listSnapshotsCommand) Usage() string {
	return command.Name + " snapshot ls [OPTIONS]"
}

func (c *listSnapshotsCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.quiet, console.Long("quiet"), "only shows snapshot uuids.", console.Short("q"))
	console.Var(flags, &c.format, console.Long("format"), "formats the output: table, json, or a Go template.")
	console.Var(flags, &c.vm, console.Long("vm"), "only shows the snapshots of this VM.")
	defineAllUsers(flags, &c.allUsers)
}

func (c *listSnapshotsCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *listSnapshotsCommand) run(ctx context.Context) error {
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

	snapshots, err := client.Snapshots(ctx, vmUUID)
	if err != nil {
		return err
	}

	rows := make([]snapshotRow, len(snapshots))
	for i, s := range snapshots {
		rows[i] = snapshotRowOf(s)
	}

	return writeRows(c.cli.Out, rows, formatter.Options{
		Format:  c.format,
		Table:   withOwner(snapshotTable, c.allUsers),
		Quiet:   c.quiet,
		ID:      "{{.UUID}}",
		Headers: snapshotHeaders,
	})
}
