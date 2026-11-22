package workload

import (
	"context"
	"time"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/formatter"
	"github.com/Tarhche/cli/internal/workload"
)

// showVMStatsCommand shows what a VM used over a span, a step at a time:
// the last hour unless it is told otherwise.
type showVMStatsCommand struct {
	cli   *command.Cli
	flags *console.FlagSet

	since    string
	until    string
	format   string
	allUsers bool
}

func newShowVMStatsCommand(cli *command.Cli) *showVMStatsCommand {
	return &showVMStatsCommand{cli: cli}
}

func (c *showVMStatsCommand) Name() string {
	return "stats"
}

func (c *showVMStatsCommand) Description() string {
	return "shows what a VM used, a step at a time."
}

func (c *showVMStatsCommand) Usage() string {
	return command.Name + " vm stats [OPTIONS] VM"
}

func (c *showVMStatsCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.since, console.Long("since"), "shows what was used since a time (2026-11-22T10:00:00Z) or a duration ago (6h); the hour before --until when it is not given.")
	console.Var(flags, &c.until, console.Long("until"), "shows what was used until a time or a duration ago; now when it is not given.")
	console.Var(flags, &c.format, console.Long("format"), "formats the output: table, json, or a Go template.")
	defineAllUsers(flags, &c.allUsers)
}

func (c *showVMStatsCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *showVMStatsCommand) run(ctx context.Context) error {
	if err := command.ExactArgs(c.flags, 1); err != nil {
		return err
	}

	now := time.Now()

	since, err := momentOf(c.since, now)
	if err != nil {
		return usageError(c.flags, "%s", err)
	}

	until, err := momentOf(c.until, now)
	if err != nil {
		return usageError(c.flags, "%s", err)
	}

	client, err := clientOf(ctx, c.cli, c.allUsers)
	if err != nil {
		return err
	}

	vm, err := client.FindVM(ctx, c.flags.Arg(0))
	if err != nil {
		return err
	}

	usage, err := client.VMUsage(ctx, vm.Metadata.UUID, workload.Span{From: since, To: until})
	if err != nil {
		return err
	}

	rows := make([]usageRow, len(usage.Points))
	for i, point := range usage.Points {
		rows[i] = usageRowOf(point)
	}

	return writeRows(c.cli.Out, rows, formatter.Options{Format: c.format, Table: usageTable, Headers: usageHeaders})
}
