package workload

import (
	"context"
	"fmt"
	"time"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
)

// createSnapshotCommand takes a snapshot of one's own VM's disk.
type createSnapshotCommand struct {
	cli   *command.Cli
	flags *console.FlagSet

	name string

	// now is when the snapshot is taken, which names one given no name.
	now func() time.Time
}

func newCreateSnapshotCommand(cli *command.Cli) *createSnapshotCommand {
	return &createSnapshotCommand{cli: cli, now: time.Now}
}

func (c *createSnapshotCommand) Name() string {
	return "create"
}

func (c *createSnapshotCommand) Description() string {
	return "Take a snapshot of a running or stopped VM's disk"
}

func (c *createSnapshotCommand) Usage() string {
	return command.Name + " snapshot create [OPTIONS] VM"
}

func (c *createSnapshotCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.name, console.Long("name"), "the snapshot's name; the VM's and when it was taken when it is not given.")
}

func (c *createSnapshotCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *createSnapshotCommand) run(ctx context.Context) error {
	if err := command.ExactArgs(c.flags, 1); err != nil {
		return err
	}

	client, err := clientOf(ctx, c.cli, false)
	if err != nil {
		return err
	}

	vm, err := client.FindVM(ctx, c.flags.Arg(0))
	if err != nil {
		return err
	}

	name := c.name
	if len(name) == 0 {
		name = orDefault(vm.Metadata.Name, "snapshot") + "-" + c.now().UTC().Format("20060102-150405")
	}

	snapshot, err := client.CreateSnapshot(ctx, vm.Metadata.UUID, name)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintln(c.cli.Out, snapshot.Metadata.UUID)

	return err
}
