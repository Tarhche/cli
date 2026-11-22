package workload

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/formatter"
	"github.com/Tarhche/cli/internal/workload"
)

const (
	// statsTable is docker's stats.
	statsTable = `{{.ID}}\t{{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.MemPerc}}\t{{.NetIO}}\t{{.BlockIO}}\t{{.PIDs}}`

	// statsEvery is how often stats are sampled again while they stream:
	// sampling a container takes its VM's dockerd a second.
	statsEvery = 2 * time.Second

	// notSampled is what is shown of what could not be sampled.
	notSampled = "--"

	// clearScreen clears a terminal and puts its cursor at the top, as
	// docker's stats does before each sample.
	clearScreen = "\033[2J\033[H"
)

// statsHeaders name the columns docker names otherwise.
var statsHeaders = map[string]string{
	"ID":       "CONTAINER ID",
	"CPUPerc":  "CPU %",
	"MemUsage": "MEM USAGE / LIMIT",
	"MemPerc":  "MEM %",
	"NetIO":    "NET I/O",
	"BlockIO":  "BLOCK I/O",
	"PIDs":     "PIDS",
}

// statsRow is what a container uses, as docker's stats shows it.
type statsRow struct {
	ID       string
	UUID     string
	Name     string
	CPUPerc  string
	MemUsage string
	MemPerc  string
	NetIO    string
	BlockIO  string
	PIDs     string
	VM       string
}

func statsRowOf(c workload.ContainerManifest, stats *workload.ContainerStats, untruncated bool) statsRow {
	row := statsRow{
		ID:       containerIDOf(c, untruncated),
		UUID:     c.Metadata.UUID,
		Name:     c.DockerName(),
		CPUPerc:  notSampled,
		MemUsage: notSampled,
		MemPerc:  notSampled,
		NetIO:    notSampled,
		BlockIO:  notSampled,
		PIDs:     notSampled,
		VM:       orDefault(c.VMName, c.VMUUID),
	}

	if stats == nil {
		return row
	}

	row.CPUPerc = percent(stats.CPUPercent)
	row.MemUsage = formatter.Bytes(stats.MemoryUsed) + " / " + formatter.Bytes(stats.MemoryLimit)
	row.MemPerc = percent(share(stats.MemoryUsed, stats.MemoryLimit))
	row.NetIO = formatter.Bytes(stats.NetworkRx) + " / " + formatter.Bytes(stats.NetworkTx)
	row.BlockIO = formatter.Bytes(stats.BlockRead) + " / " + formatter.Bytes(stats.BlockWrite)
	row.PIDs = strconv.FormatUint(stats.PIDs, 10)

	return row
}

// percent is a share out of a hundred, as docker's stats shows it.
func percent(share float64) string {
	return fmt.Sprintf("%.2f%%", share)
}

// share is how much of total used is, out of a hundred.
func share(used, total uint64) float64 {
	if total == 0 {
		return 0
	}

	return float64(used) / float64(total) * 100
}

// showContainerStatsCommand shows what containers use, sampled again and
// again until it is interrupted, as docker's stats does.
type showContainerStatsCommand struct {
	cli   *command.Cli
	flags *console.FlagSet

	all         bool
	once        bool
	format      string
	untruncated bool
	vm          string
	allUsers    bool

	// every is how often the containers are sampled again.
	every time.Duration
}

func newShowContainerStatsCommand(cli *command.Cli) *showContainerStatsCommand {
	return &showContainerStatsCommand{cli: cli, every: statsEvery}
}

func (c *showContainerStatsCommand) Name() string {
	return "stats"
}

func (c *showContainerStatsCommand) Description() string {
	return "displays a live stream of what containers use."
}

func (c *showContainerStatsCommand) Usage() string {
	return command.Name + " container stats [OPTIONS] [CONTAINER...]"
}

func (c *showContainerStatsCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.all, console.Long("all"), "shows every container (default shows just running).", console.Short("a"))
	console.Var(flags, &c.once, console.Long("no-stream"), "shows one sample and stops.")
	console.Var(flags, &c.format, console.Long("format"), "formats the output: table, json, or a Go template.")
	console.Var(flags, &c.untruncated, console.Long("no-trunc"), "does not truncate the output.")
	console.Var(flags, &c.vm, console.Long("vm"), "only the containers of this Docker VM.")
	defineAllUsers(flags, &c.allUsers)
}

func (c *showContainerStatsCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *showContainerStatsCommand) run(ctx context.Context) error {
	client, err := clientOf(ctx, c.cli, c.allUsers)
	if err != nil {
		return err
	}

	containers, err := c.containers(ctx, client)
	if err != nil {
		return err
	}

	for {
		rows, err := sample(ctx, client, containers, c.untruncated)
		if err != nil {
			return err
		}

		if !c.once && command.IsTerminal(c.cli.Out) {
			if _, err := fmt.Fprint(c.cli.Out, clearScreen); err != nil {
				return err
			}
		}

		if err := writeRows(c.cli.Out, rows, formatter.Options{Format: c.format, Table: statsTable, Headers: statsHeaders}); err != nil {
			return err
		}

		if c.once {
			return nil
		}

		if err := wait(ctx, c.every); err != nil {
			return err
		}
	}
}

// containers are those the arguments name, or every one that runs, or every
// one at all with -a.
func (c *showContainerStatsCommand) containers(ctx context.Context, client *workload.Client) ([]workload.ContainerManifest, error) {
	listed, err := containersOf(ctx, client, c.vm)
	if err != nil {
		return nil, err
	}

	if c.flags.NArg() == 0 {
		if c.all {
			return listed, nil
		}

		return running(listed), nil
	}

	named := make([]workload.ContainerManifest, c.flags.NArg())
	for i, reference := range c.flags.Args() {
		if named[i], err = workload.MatchContainer(listed, reference); err != nil {
			return nil, err
		}
	}

	return named, nil
}

// sample samples what each of containers uses now, a few at once, each that
// could not be sampled shown as such.
func sample(ctx context.Context, client *workload.Client, containers []workload.ContainerManifest, untruncated bool) ([]statsRow, error) {
	var (
		rows     = make([]statsRow, len(containers))
		failures = make([]error, len(containers))
		slots    = make(chan struct{}, 8)
		group    sync.WaitGroup
	)

	for i, container := range containers {
		group.Go(func() {
			slots <- struct{}{}
			defer func() { <-slots }()

			stats, err := client.ContainerStats(ctx, container.VMUUID, container.Metadata.UUID)
			if err != nil {
				rows[i], failures[i] = statsRowOf(container, nil, untruncated), err

				return
			}

			rows[i] = statsRowOf(container, &stats, untruncated)
		})
	}

	group.Wait()

	for _, err := range failures {
		if err != nil && stops(err) {
			return nil, err
		}
	}

	return rows, nil
}
