package workload

import (
	"context"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/workload"
)

// removeNetworksCommand removes networks from the Docker VMs they are in.
type removeNetworksCommand struct {
	cli   *command.Cli
	flags *console.FlagSet

	cascade  bool
	vm       string
	allUsers bool
}

func newRemoveNetworksCommand(cli *command.Cli) *removeNetworksCommand {
	return &removeNetworksCommand{cli: cli}
}

func (c *removeNetworksCommand) Name() string {
	return "rm"
}

func (c *removeNetworksCommand) Description() string {
	return "removes one or more networks."
}

func (c *removeNetworksCommand) Usage() string {
	return command.Name + " network rm [OPTIONS] NETWORK [NETWORK...]"
}

func (c *removeNetworksCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.cascade, console.Long("cascade"), "first removes the stacks and the containers that need it.")
	console.Var(flags, &c.vm, console.Long("vm"), "looks for the networks in this Docker VM alone.")
	defineAllUsers(flags, &c.allUsers)
}

func (c *removeNetworksCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *removeNetworksCommand) run(ctx context.Context) error {
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

	return eachReference(c.cli, c.flags.Args(), func(reference string) error {
		network, err := workload.MatchNetwork(networks, reference)
		if err != nil {
			return err
		}

		removal, err := client.DeleteNetwork(ctx, network.VM.Metadata.UUID, network.Item.Metadata.UUID, c.cascade)
		if err != nil {
			return err
		}

		return warnRemoval(c.cli.Err, reference, removal)
	})
}
