package workload

import (
	"context"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/workload"
)

// startContainersCommand starts containers that are not running.
type startContainersCommand struct {
	cli   *command.Cli
	flags *console.FlagSet

	vm       string
	allUsers bool
}

func newStartContainersCommand(cli *command.Cli) *startContainersCommand {
	return &startContainersCommand{cli: cli}
}

func (c *startContainersCommand) Name() string {
	return "start"
}

func (c *startContainersCommand) Description() string {
	return "starts one or more stopped containers."
}

func (c *startContainersCommand) Usage() string {
	return command.Name + " container start [OPTIONS] CONTAINER [CONTAINER...]"
}

func (c *startContainersCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.vm, console.Long("vm"), "looks for the containers in this Docker VM alone.")
	defineAllUsers(flags, &c.allUsers)
}

func (c *startContainersCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *startContainersCommand) run(ctx context.Context) error {
	if err := command.MinArgs(c.flags, 1); err != nil {
		return err
	}

	return actOnContainers(ctx, c.cli, c.allUsers, c.vm, c.flags.Args(), func(client *workload.Client, container workload.ContainerManifest) error {
		return client.StartContainer(ctx, container.VMUUID, container.Metadata.UUID)
	})
}
