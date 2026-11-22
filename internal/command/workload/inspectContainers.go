package workload

import (
	"context"
	"encoding/json"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/workload"
)

// inspectContainersCommand writes containers as the API answers them.
type inspectContainersCommand struct {
	cli   *command.Cli
	flags *console.FlagSet

	format   string
	vm       string
	allUsers bool
}

func newInspectContainersCommand(cli *command.Cli) *inspectContainersCommand {
	return &inspectContainersCommand{cli: cli}
}

func (c *inspectContainersCommand) Name() string {
	return "inspect"
}

func (c *inspectContainersCommand) Description() string {
	return "Display detailed information on one or more containers"
}

func (c *inspectContainersCommand) Usage() string {
	return command.Name + " container inspect [OPTIONS] CONTAINER [CONTAINER...]"
}

func (c *inspectContainersCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.format, console.Long("format"), "formats the output with a Go template.", console.Short("f"))
	console.Var(flags, &c.vm, console.Long("vm"), "looks for the containers in this Docker VM alone.")
	defineAllUsers(flags, &c.allUsers)
}

func (c *inspectContainersCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *inspectContainersCommand) run(ctx context.Context) error {
	if err := command.MinArgs(c.flags, 1); err != nil {
		return err
	}

	client, err := clientOf(ctx, c.cli, c.allUsers)
	if err != nil {
		return err
	}

	containers, err := containersOf(ctx, client, c.vm)
	if err != nil {
		return err
	}

	return inspectEach(c.cli, c.flags.Args(), c.format, func(reference string) (json.RawMessage, error) {
		container, err := workload.MatchContainer(containers, reference)

		return container.Raw(), err
	})
}
