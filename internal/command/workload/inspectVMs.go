package workload

import (
	"context"
	"encoding/json"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/workload"
)

// inspectVMsCommand writes VMs as the API answers them.
type inspectVMsCommand struct {
	cli   *command.Cli
	flags *console.FlagSet

	format   string
	allUsers bool
}

func newInspectVMsCommand(cli *command.Cli) *inspectVMsCommand {
	return &inspectVMsCommand{cli: cli}
}

func (c *inspectVMsCommand) Name() string {
	return "inspect"
}

func (c *inspectVMsCommand) Description() string {
	return "displays detailed information on one or more VMs."
}

func (c *inspectVMsCommand) Usage() string {
	return command.Name + " vm inspect [OPTIONS] VM [VM...]"
}

func (c *inspectVMsCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.format, console.Long("format"), "formats the output with a Go template.", console.Short("f"))
	defineAllUsers(flags, &c.allUsers)
}

func (c *inspectVMsCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *inspectVMsCommand) run(ctx context.Context) error {
	if err := command.MinArgs(c.flags, 1); err != nil {
		return err
	}

	client, err := clientOf(ctx, c.cli, c.allUsers)
	if err != nil {
		return err
	}

	vms, err := client.VMs(ctx)
	if err != nil {
		return err
	}

	return inspectEach(c.cli, c.flags.Args(), c.format, func(reference string) (json.RawMessage, error) {
		vm, err := workload.MatchVM(vms, reference)

		return vm.Raw(), err
	})
}
