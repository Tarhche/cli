package workload

import (
	"context"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/workload"
)

// removeSnapshotsCommand removes snapshots and the archives they kept.
type removeSnapshotsCommand struct {
	cli   *command.Cli
	flags *console.FlagSet

	allUsers bool
}

func newRemoveSnapshotsCommand(cli *command.Cli) *removeSnapshotsCommand {
	return &removeSnapshotsCommand{cli: cli}
}

func (c *removeSnapshotsCommand) Name() string {
	return "rm"
}

func (c *removeSnapshotsCommand) Description() string {
	return "Remove one or more snapshots"
}

func (c *removeSnapshotsCommand) Usage() string {
	return command.Name + " snapshot rm [OPTIONS] SNAPSHOT [SNAPSHOT...]"
}

func (c *removeSnapshotsCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	defineAllUsers(flags, &c.allUsers)
}

func (c *removeSnapshotsCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *removeSnapshotsCommand) run(ctx context.Context) error {
	if err := command.MinArgs(c.flags, 1); err != nil {
		return err
	}

	client, err := clientOf(ctx, c.cli, c.allUsers)
	if err != nil {
		return err
	}

	snapshots, err := client.Snapshots(ctx, "")
	if err != nil {
		return err
	}

	return eachReference(c.cli, c.flags.Args(), func(reference string) error {
		snapshot, err := workload.MatchSnapshot(snapshots, reference)
		if err != nil {
			return err
		}

		return client.DeleteSnapshot(ctx, snapshot.Metadata.UUID)
	})
}
