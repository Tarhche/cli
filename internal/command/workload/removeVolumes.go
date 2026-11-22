package workload

import (
	"context"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/workload"
)

// removeVolumesCommand removes volumes from the Docker VMs they are in.
type removeVolumesCommand struct {
	cli   *command.Cli
	flags *console.FlagSet

	force    bool
	cascade  bool
	vm       string
	allUsers bool
}

func newRemoveVolumesCommand(cli *command.Cli) *removeVolumesCommand {
	return &removeVolumesCommand{cli: cli}
}

func (c *removeVolumesCommand) Name() string {
	return "rm"
}

func (c *removeVolumesCommand) Description() string {
	return "removes one or more volumes."
}

func (c *removeVolumesCommand) Usage() string {
	return command.Name + " volume rm [OPTIONS] VOLUME [VOLUME...]"
}

func (c *removeVolumesCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.force, console.Long("force"), "removes a volume a container uses too.", console.Short("f"))
	console.Var(flags, &c.cascade, console.Long("cascade"), "first removes the stacks and the containers that need it.")
	console.Var(flags, &c.vm, console.Long("vm"), "looks for the volumes in this Docker VM alone.")
	defineAllUsers(flags, &c.allUsers)
}

func (c *removeVolumesCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *removeVolumesCommand) run(ctx context.Context) error {
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

	volumes, err := client.VolumesIn(ctx, vms)
	if err != nil {
		return err
	}

	return eachReference(c.cli, c.flags.Args(), func(reference string) error {
		volume, err := workload.MatchVolume(volumes, reference)
		if err != nil {
			return err
		}

		removal, err := client.DeleteVolume(ctx, volume.VM.Metadata.UUID, volume.Item.DockerName(), c.force, c.cascade)
		if err != nil {
			return err
		}

		return warnRemoval(c.cli.Err, reference, removal)
	})
}
