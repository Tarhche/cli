package workload

import (
	"context"
	"fmt"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
)

// pullImageCommand pulls an image into a Docker VM.
type pullImageCommand struct {
	cli   *command.Cli
	flags *console.FlagSet

	vm       string
	allUsers bool
}

func newPullImageCommand(cli *command.Cli) *pullImageCommand {
	return &pullImageCommand{cli: cli}
}

func (c *pullImageCommand) Name() string {
	return "pull"
}

func (c *pullImageCommand) Description() string {
	return "downloads an image from a registry into a Docker VM."
}

func (c *pullImageCommand) Usage() string {
	return command.Name + " image pull [OPTIONS] NAME[:TAG|@DIGEST]"
}

func (c *pullImageCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.vm, console.Long("vm"), "the Docker VM to pull it into, which has to be named when there is more than one.")
	defineAllUsers(flags, &c.allUsers)
}

func (c *pullImageCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *pullImageCommand) run(ctx context.Context) error {
	if err := command.ExactArgs(c.flags, 1); err != nil {
		return err
	}

	client, err := clientOf(ctx, c.cli, c.allUsers)
	if err != nil {
		return err
	}

	vm, err := dockerVMOf(ctx, client, c.vm)
	if err != nil {
		return err
	}

	image, err := client.PullImage(ctx, vm.Metadata.UUID, c.flags.Arg(0))
	if err != nil {
		return err
	}

	_, err = fmt.Fprintln(c.cli.Out, orDefault(image.Reference(), c.flags.Arg(0)))

	return err
}
