package workload

import (
	"context"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/workload"
)

// removeContainersCommand removes containers.
type removeContainersCommand struct {
	cli   *command.Cli
	flags *console.FlagSet

	force    bool
	vm       string
	allUsers bool
}

func newRemoveContainersCommand(cli *command.Cli) *removeContainersCommand {
	return &removeContainersCommand{cli: cli}
}

func (c *removeContainersCommand) Name() string {
	return "rm"
}

func (c *removeContainersCommand) Description() string {
	return "Remove one or more containers"
}

func (c *removeContainersCommand) Usage() string {
	return command.Name + " container rm [OPTIONS] CONTAINER [CONTAINER...]"
}

func (c *removeContainersCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.force, console.Long("force"), "removes a running container too.", console.Short("f"))
	console.Var(flags, &c.vm, console.Long("vm"), "looks for the containers in this Docker VM alone.")
	defineAllUsers(flags, &c.allUsers)
}

func (c *removeContainersCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *removeContainersCommand) run(ctx context.Context) error {
	if err := command.MinArgs(c.flags, 1); err != nil {
		return err
	}

	return actOnContainers(ctx, c.cli, c.allUsers, c.vm, c.flags.Args(), func(client *workload.Client, container workload.ContainerManifest) error {
		return client.DeleteContainer(ctx, container.VMUUID, container.Metadata.UUID, c.force)
	})
}
