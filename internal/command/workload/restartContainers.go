package workload

import (
	"context"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/workload"
)

// restartContainersCommand stops containers and starts them again.
type restartContainersCommand struct {
	cli   *command.Cli
	flags *console.FlagSet

	vm       string
	allUsers bool
}

func newRestartContainersCommand(cli *command.Cli) *restartContainersCommand {
	return &restartContainersCommand{cli: cli}
}

func (c *restartContainersCommand) Name() string {
	return "restart"
}

func (c *restartContainersCommand) Description() string {
	return "restarts one or more containers."
}

func (c *restartContainersCommand) Usage() string {
	return command.Name + " container restart [OPTIONS] CONTAINER [CONTAINER...]"
}

func (c *restartContainersCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.vm, console.Long("vm"), "looks for the containers in this Docker VM alone.")
	defineAllUsers(flags, &c.allUsers)
}

func (c *restartContainersCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *restartContainersCommand) run(ctx context.Context) error {
	if err := command.MinArgs(c.flags, 1); err != nil {
		return err
	}

	return actOnContainers(ctx, c.cli, c.allUsers, c.vm, c.flags.Args(), func(client *workload.Client, container workload.ContainerManifest) error {
		return client.RestartContainer(ctx, container.VMUUID, container.Metadata.UUID)
	})
}
