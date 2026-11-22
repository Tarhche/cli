package workload

import (
	"context"
	"fmt"
	"strings"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
)

// showContainerPortsCommand lists the ports a container publishes on its VM,
// as docker's port does.
type showContainerPortsCommand struct {
	cli   *command.Cli
	flags *console.FlagSet

	vm       string
	allUsers bool
}

func newShowContainerPortsCommand(cli *command.Cli) *showContainerPortsCommand {
	return &showContainerPortsCommand{cli: cli}
}

func (c *showContainerPortsCommand) Name() string {
	return "port"
}

func (c *showContainerPortsCommand) Description() string {
	return "List the ports a container publishes on its VM, or what one of them is published as"
}

func (c *showContainerPortsCommand) Usage() string {
	return command.Name + " container port [OPTIONS] CONTAINER [PRIVATE_PORT[/PROTO]]"
}

func (c *showContainerPortsCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.vm, console.Long("vm"), "looks for the container in this Docker VM alone.")
	defineAllUsers(flags, &c.allUsers)
}

func (c *showContainerPortsCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *showContainerPortsCommand) run(ctx context.Context) error {
	if err := command.MinArgs(c.flags, 1); err != nil {
		return err
	}

	if err := command.MaxArgs(c.flags, 2); err != nil {
		return err
	}

	client, err := clientOf(ctx, c.cli, c.allUsers)
	if err != nil {
		return err
	}

	vmUUID, err := vmUUIDOf(ctx, client, c.vm)
	if err != nil {
		return err
	}

	container, err := client.FindContainer(ctx, vmUUID, c.flags.Arg(0))
	if err != nil {
		return err
	}

	bindings := bindingsOf(container)

	if c.flags.NArg() == 1 {
		for _, binding := range bindings {
			if binding.HostPort == 0 {
				continue
			}

			if _, err := fmt.Fprintf(c.cli.Out, "%s -> %s\n", privatePort(binding), publicPort(binding)); err != nil {
				return err
			}
		}

		return nil
	}

	private := c.flags.Arg(1)
	if !strings.Contains(private, "/") {
		private += "/tcp"
	}

	found := false

	for _, binding := range bindings {
		if binding.HostPort == 0 || privatePort(binding) != private {
			continue
		}

		if _, err := fmt.Fprintln(c.cli.Out, publicPort(binding)); err != nil {
			return err
		}

		found = true
	}

	if !found {
		return fmt.Errorf("no public port %q published for %s", private, container.DockerName())
	}

	return nil
}
