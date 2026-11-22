package workload

import (
	"context"
	"encoding/json"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/workload"
)

// inspectSnapshotsCommand writes snapshots as the API answers them.
type inspectSnapshotsCommand struct {
	cli   *command.Cli
	flags *console.FlagSet

	format   string
	allUsers bool
}

func newInspectSnapshotsCommand(cli *command.Cli) *inspectSnapshotsCommand {
	return &inspectSnapshotsCommand{cli: cli}
}

func (c *inspectSnapshotsCommand) Name() string {
	return "inspect"
}

func (c *inspectSnapshotsCommand) Description() string {
	return "Display detailed information on one or more snapshots"
}

func (c *inspectSnapshotsCommand) Usage() string {
	return command.Name + " snapshot inspect [OPTIONS] SNAPSHOT [SNAPSHOT...]"
}

func (c *inspectSnapshotsCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.format, console.Long("format"), "formats the output with a Go template.", console.Short("f"))
	defineAllUsers(flags, &c.allUsers)
}

func (c *inspectSnapshotsCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *inspectSnapshotsCommand) run(ctx context.Context) error {
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

	return inspectEach(c.cli, c.flags.Args(), c.format, func(reference string) (json.RawMessage, error) {
		snapshot, err := workload.MatchSnapshot(snapshots, reference)

		return snapshot.Raw(), err
	})
}
