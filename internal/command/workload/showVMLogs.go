package workload

import (
	"context"
	"time"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/workload"
)

// showVMLogsCommand writes what a VM wrote, its kernel, its main process and
// its execs, and follows it.
type showVMLogsCommand struct {
	cli   *command.Cli
	flags *console.FlagSet

	logs     logFlags
	allUsers bool

	// every is how often what it writes is read again while it is followed.
	every time.Duration
}

func newShowVMLogsCommand(cli *command.Cli) *showVMLogsCommand {
	return &showVMLogsCommand{cli: cli, every: followEvery}
}

func (c *showVMLogsCommand) Name() string {
	return "logs"
}

func (c *showVMLogsCommand) Description() string {
	return "fetches the logs of a VM."
}

func (c *showVMLogsCommand) Usage() string {
	return command.Name + " vm logs [OPTIONS] VM"
}

func (c *showVMLogsCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	c.logs.define(flags)
	defineAllUsers(flags, &c.allUsers)
}

func (c *showVMLogsCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *showVMLogsCommand) run(ctx context.Context) error {
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

	vm, err := client.FindVM(ctx, c.flags.Arg(0))
	if err != nil {
		return err
	}

	follower := &logFollower{
		read: func(ctx context.Context, query workload.LogsQuery) (workload.Logs, error) {
			return client.VMLogs(ctx, vm.Metadata.UUID, query)
		},
		write: lineWriter(c.cli.Out, c.cli.Err, c.logs.timestamps),
		every: c.every,
	}

	return follower.run(ctx, query, hidden, c.logs.follow)
}
