package workload

import (
	"context"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/workload"
)

// stopContainersCommand stops running containers.
type stopContainersCommand struct {
	cli   *command.Cli
	flags *console.FlagSet

	vm       string
	allUsers bool
}

func newStopContainersCommand(cli *command.Cli) *stopContainersCommand {
	return &stopContainersCommand{cli: cli}
}

func (c *stopContainersCommand) Name() string {
	return "stop"
}

func (c *stopContainersCommand) Description() string {
	return "Stop one or more running containers"
}

func (c *stopContainersCommand) Usage() string {
	return command.Name + " container stop [OPTIONS] CONTAINER [CONTAINER...]"
}

func (c *stopContainersCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.vm, console.Long("vm"), "looks for the containers in this Docker VM alone.")
	defineAllUsers(flags, &c.allUsers)
}

func (c *stopContainersCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *stopContainersCommand) run(ctx context.Context) error {
	if err := command.MinArgs(c.flags, 1); err != nil {
		return err
	}

	return actOnContainers(ctx, c.cli, c.allUsers, c.vm, c.flags.Args(), func(client *workload.Client, container workload.ContainerManifest) error {
		return client.StopContainer(ctx, container.VMUUID, container.Metadata.UUID)
	})
}
