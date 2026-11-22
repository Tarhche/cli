package workload

import (
	"context"
	"time"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/workload"
)

// showContainerLogsCommand writes what a container wrote, and follows it.
type showContainerLogsCommand struct {
	cli   *command.Cli
	flags *console.FlagSet

	logs     logFlags
	vm       string
	allUsers bool

	// every is how often what it writes is read again while it is followed.
	every time.Duration
}

func newShowContainerLogsCommand(cli *command.Cli) *showContainerLogsCommand {
	return &showContainerLogsCommand{cli: cli, every: followEvery}
}

func (c *showContainerLogsCommand) Name() string {
	return "logs"
}

func (c *showContainerLogsCommand) Description() string {
	return "fetches the logs of a container."
}

func (c *showContainerLogsCommand) Usage() string {
	return command.Name + " container logs [OPTIONS] CONTAINER"
}

func (c *showContainerLogsCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	c.logs.define(flags)
	console.Var(flags, &c.vm, console.Long("vm"), "looks for the container in this Docker VM alone.")
	defineAllUsers(flags, &c.allUsers)
}

func (c *showContainerLogsCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *showContainerLogsCommand) run(ctx context.Context) error {
	if err := command.ExactArgs(c.flags, 1); err != nil {
		return err
	}

	query, hidden, err := c.logs.query(c.flags, time.Now())
	if err != nil {
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

	container, err := client.FindContainer(ctx, vmUUID, c.flags.Arg(0))
	if err != nil {
		return err
	}

	end := &containerEnd{client: client, vmUUID: container.VMUUID, uuid: container.Metadata.UUID}

	follower := &logFollower{
		read: func(ctx context.Context, query workload.LogsQuery) (workload.Logs, error) {
			return client.ContainerLogs(ctx, container.VMUUID, container.Metadata.UUID, query)
		},
		ended: end.ended,
		write: lineWriter(c.cli.Out, c.cli.Err, c.logs.timestamps),
		every: c.every,
	}

	return follower.run(ctx, query, hidden, c.logs.follow)
}
