package workload

import (
	"context"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
)

// renameSnapshotCommand renames a snapshot, which is all that changes of
// one.
type renameSnapshotCommand struct {
	cli   *command.Cli
	flags *console.FlagSet

	allUsers bool
}

func newRenameSnapshotCommand(cli *command.Cli) *renameSnapshotCommand {
	return &renameSnapshotCommand{cli: cli}
}

func (c *renameSnapshotCommand) Name() string {
	return "rename"
}

func (c *renameSnapshotCommand) Description() string {
	return "renames a snapshot."
}

func (c *renameSnapshotCommand) Usage() string {
	return command.Name + " snapshot rename [OPTIONS] SNAPSHOT NEW_NAME"
}

func (c *renameSnapshotCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	defineAllUsers(flags, &c.allUsers)
}

func (c *renameSnapshotCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *renameSnapshotCommand) run(ctx context.Context) error {
	if err := command.ExactArgs(c.flags, 2); err != nil {
		return err
	}

	client, err := clientOf(ctx, c.cli, c.allUsers)
	if err != nil {
		return err
	}

	snapshot, err := client.FindSnapshot(ctx, c.flags.Arg(0))
	if err != nil {
		return err
	}

	_, err = client.RenameSnapshot(ctx, snapshot.Metadata.UUID, c.flags.Arg(1))

	return err
}
