package workload

import (
	"context"
	"fmt"
	"time"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/units"
	"github.com/Tarhche/cli/internal/workload"
)

// updateVMCommand changes a VM: its name, its lifetime, its ports, its
// network and its resources. A change to its ports, its network or its
// resources restarts it when it is not stopped, and its disk only grows.
type updateVMCommand struct {
	cli   *command.Cli
	flags *console.FlagSet

	name     string
	lifetime time.Duration
	ports    portList
	noPorts  bool
	ingress  string
	egress   string
	cpus     uint
	memory   units.Size
	disk     units.Size
	allUsers bool
}

func newUpdateVMCommand(cli *command.Cli) *updateVMCommand {
	return &updateVMCommand{cli: cli}
}

func (c *updateVMCommand) Name() string {
	return "update"
}

func (c *updateVMCommand) Description() string {
	return "Change a VM's name, lifetime, ports, network or resources"
}

func (c *updateVMCommand) Usage() string {
	return command.Name + " vm update [OPTIONS] VM"
}

func (c *updateVMCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.name, console.Long("name"), "its new name.")
	console.Var(flags, &c.lifetime, console.Long("lifetime"), "how long it is kept from now, as 2h; 0 keeps it until it is deleted.")
	console.Var(flags, &c.ports, console.Long("publish"), "a port it publishes through the ingress, in place of those it does; given again for more.", console.Short("p"))
	console.Var(flags, &c.noPorts, console.Long("no-ports"), "publishes no port any more.")
	console.Var(flags, &c.ingress, console.Long("ingress"), "whether the ingress reaches its ports: allow or deny.")
	console.Var(flags, &c.egress, console.Long("egress"), "whether it reaches the internet: allow or deny.")
	console.Var(flags, &c.cpus, console.Long("cpus"), "its whole vCPUs.")
	console.Var(flags, &c.memory, console.Long("memory"), "its memory, as 512m.", console.Short("m"))
	console.Var(flags, &c.disk, console.Long("disk"), "its disk, as 4g, which only grows.")
	defineAllUsers(flags, &c.allUsers)
}

func (c *updateVMCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *updateVMCommand) run(ctx context.Context) error {
	if err := command.ExactArgs(c.flags, 1); err != nil {
		return err
	}

	if c.noPorts && len(c.ports) > 0 {
		return usageError(c.flags, "--publish and --no-ports cannot be given together")
	}

	client, err := clientOf(ctx, c.cli, c.allUsers)
	if err != nil {
		return err
	}

	vm, err := client.FindVM(ctx, c.flags.Arg(0))
	if err != nil {
		return err
	}

	request := c.request(vm)
	if request == (workload.UpdateVMRequest{}) {
		return usageError(c.flags, "nothing to change: give what changes, as --name or --memory")
	}

	if _, err := client.UpdateVM(ctx, vm.Metadata.UUID, request); err != nil {
		return err
	}

	_, err = fmt.Fprintln(c.cli.Out, c.flags.Arg(0))

	return err
}

// request is what the flags change of vm. The network and the resources are
// asked for whole, so what of them the flags leave out is vm's as it is.
func (c *updateVMCommand) request(vm workload.VMManifest) workload.UpdateVMRequest {
	var request workload.UpdateVMRequest

	if c.given("name") {
		request.Name = &c.name
	}

	if c.given("lifetime") {
		seconds := int64(c.lifetime / time.Second)
		request.LifetimeSeconds = &seconds
	}

	switch {
	case c.noPorts:
		request.Ports = &[]uint{}
	case len(c.ports) > 0:
		ports := []uint(c.ports)
		request.Ports = &ports
	}

	if c.given("ingress") || c.given("egress") {
		network := vm.Spec.Network

		if c.given("ingress") {
			network.Ingress = c.ingress
		}

		if c.given("egress") {
			network.Egress = c.egress
		}

		request.Network = &network
	}

	if c.given("cpus") || c.given("memory") || c.given("disk") {
		resources := vm.Spec.Resources

		if c.given("cpus") {
			resources.CPUs = c.cpus
		}

		if c.given("memory") {
			resources.Memory = uint64(c.memory)
		}

		if c.given("disk") {
			resources.Disk = uint64(c.disk)
		}

		request.Resources = &resources
	}

	return request
}

// given reports whether the flag of name was given.
func (c *updateVMCommand) given(name string) bool {
	flag := c.flags.Lookup(name)

	return flag != nil && flag.Provided()
}
