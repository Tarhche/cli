package workload

import (
	"context"
	"encoding/json"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/workload"
)

// inspectNetworksCommand writes networks as the API answers them.
type inspectNetworksCommand struct {
	cli   *command.Cli
	flags *console.FlagSet

	format   string
	vm       string
	allUsers bool
}

func newInspectNetworksCommand(cli *command.Cli) *inspectNetworksCommand {
	return &inspectNetworksCommand{cli: cli}
}

func (c *inspectNetworksCommand) Name() string {
	return "inspect"
}

func (c *inspectNetworksCommand) Description() string {
	return "displays detailed information on one or more networks."
}

func (c *inspectNetworksCommand) Usage() string {
	return command.Name + " network inspect [OPTIONS] NETWORK [NETWORK...]"
}

func (c *inspectNetworksCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.format, console.Long("format"), "formats the output with a Go template.", console.Short("f"))
	console.Var(flags, &c.vm, console.Long("vm"), "looks for the networks in this Docker VM alone.")
	defineAllUsers(flags, &c.allUsers)
}

func (c *inspectNetworksCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *inspectNetworksCommand) run(ctx context.Context) error {
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

	networks, err := client.NetworksIn(ctx, vms)
	if err != nil {
		return err
	}

	return inspectEach(c.cli, c.flags.Args(), c.format, func(reference string) (json.RawMessage, error) {
		network, err := workload.MatchNetwork(networks, reference)

		return network.Item.Raw(), err
	})
}
