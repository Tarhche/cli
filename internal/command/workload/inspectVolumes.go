package workload

import (
	"context"
	"encoding/json"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/workload"
)

// inspectVolumesCommand writes volumes as the API answers them.
type inspectVolumesCommand struct {
	cli   *command.Cli
	flags *console.FlagSet

	format   string
	vm       string
	allUsers bool
}

func newInspectVolumesCommand(cli *command.Cli) *inspectVolumesCommand {
	return &inspectVolumesCommand{cli: cli}
}

func (c *inspectVolumesCommand) Name() string {
	return "inspect"
}

func (c *inspectVolumesCommand) Description() string {
	return "Display detailed information on one or more volumes"
}

func (c *inspectVolumesCommand) Usage() string {
	return command.Name + " volume inspect [OPTIONS] VOLUME [VOLUME...]"
}

func (c *inspectVolumesCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.format, console.Long("format"), "formats the output with a Go template.", console.Short("f"))
	console.Var(flags, &c.vm, console.Long("vm"), "looks for the volumes in this Docker VM alone.")
	defineAllUsers(flags, &c.allUsers)
}

func (c *inspectVolumesCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *inspectVolumesCommand) run(ctx context.Context) error {
	if err := command.MinArgs(c.flags, 1); err != nil {
		return err
	}

	client, err := clientOf(ctx, c.cli, c.allUsers)
	if err != nil {
		return err
	}

	vms, err := dockerVMsOf(ctx, client, c.vm)
	if err != nil {
		return err
	}

	volumes, err := client.VolumesIn(ctx, vms)
	if err != nil {
		return err
	}

	return inspectEach(c.cli, c.flags.Args(), c.format, func(reference string) (json.RawMessage, error) {
		volume, err := workload.MatchVolume(volumes, reference)

		return volume.Item.Raw(), err
	})
}
