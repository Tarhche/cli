package remote

import (
	"context"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/transfer"
)

// copyContainerCommand copies files and directories between a container and
// here, by docker cp in its Docker VM.
type copyContainerCommand struct {
	remote *remote
	flags  *console.FlagSet

	archive    bool
	followLink bool
	quiet      bool
}

var _ console.Command = (*copyContainerCommand)(nil)

func newCopyContainerCommand(r *remote) *copyContainerCommand {
	return &copyContainerCommand{remote: r}
}

func (c *copyContainerCommand) Name() string {
	return "cp"
}

func (c *copyContainerCommand) Description() string {
	return "Copy files/folders between a container and the local filesystem"
}

func (c *copyContainerCommand) Usage() string {
	return "cp [OPTIONS] CONTAINER:SRC_PATH DEST_PATH|-\n  cp [OPTIONS] SRC_PATH|- CONTAINER:DEST_PATH"
}

func (c *copyContainerCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.archive, console.Long("archive"), archiveUsage, console.Short("a"))
	console.Var(flags, &c.followLink, console.Long("follow-link"), followLinkUsage, console.Short("L"))
	console.Var(flags, &c.quiet, console.Long("quiet"), quietUsage, console.Short("q"))
}

func (c *copyContainerCommand) Run(ctx context.Context) console.ExitStatus {
	return c.remote.cli.Exit(c.run(ctx))
}

func (c *copyContainerCommand) run(ctx context.Context) error {
	if err := command.ExactArgs(c.flags, 2); err != nil {
		return err
	}

	plan, err := planCopy(c.flags.Arg(0), c.flags.Arg(1), "container")
	if err != nil {
		return err
	}

	container, err := c.remote.container(ctx, plan.reference)
	if err != nil {
		return err
	}

	dialer, err := c.remote.dialer()
	if err != nil {
		return err
	}

	options := transfer.Options{FollowLink: c.followLink, CopyUIDGID: c.archive}
	remote := transfer.Container(dialer.Runner(container.VM.UUID), container.DockerID)

	return c.remote.copy(ctx, plan, remote, options, c.quiet, "container")
}
