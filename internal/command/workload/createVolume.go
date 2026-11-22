package workload

import (
	"context"
	"crypto/rand"
	"fmt"
	"strings"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/workload"
)

// createVolumeCommand creates a volume in a Docker VM, on its disk.
type createVolumeCommand struct {
	cli   *command.Cli
	flags *console.FlagSet

	driver   string
	labels   list
	vm       string
	allUsers bool
}

func newCreateVolumeCommand(cli *command.Cli) *createVolumeCommand {
	return &createVolumeCommand{cli: cli}
}

func (c *createVolumeCommand) Name() string {
	return "create"
}

func (c *createVolumeCommand) Description() string {
	return "Create a volume in a Docker VM"
}

func (c *createVolumeCommand) Usage() string {
	return command.Name + " volume create [OPTIONS] [VOLUME]"
}

func (c *createVolumeCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.driver, console.Long("driver"), "the driver that manages it: local.", console.Short("d"))
	console.Var(flags, &c.labels, console.Long("label"), "sets a label on it: key=value.")
	console.Var(flags, &c.vm, console.Long("vm"), "the Docker VM to create it in, which has to be named when there is more than one.")
	defineAllUsers(flags, &c.allUsers)
}

func (c *createVolumeCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *createVolumeCommand) run(ctx context.Context) error {
	if err := command.MaxArgs(c.flags, 1); err != nil {
		return err
	}

	name := c.flags.Arg(0)
	if len(name) == 0 {
		name = strings.ToLower(rand.Text())
	}

	client, err := clientOf(ctx, c.cli, c.allUsers)
	if err != nil {
		return err
	}

	vm, err := dockerVMOf(ctx, client, c.vm)
	if err != nil {
		return err
	}

	volume, err := client.CreateVolume(ctx, vm.Metadata.UUID, workload.CreateVolumeRequest{
		Name:   name,
		Driver: c.driver,
		Labels: labelsOf(c.labels),
	})
	if err != nil {
		return err
	}

	_, err = fmt.Fprintln(c.cli.Out, orDefault(volume.DockerName(), name))

	return err
}
