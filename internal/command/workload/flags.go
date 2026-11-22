package workload

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/units"
	"github.com/Tarhche/cli/internal/workload"
)

// list is a flag given once for every value it holds.
type list []string

// Set adds value to the list.
func (l *list) Set(value string) error {
	*l = append(*l, value)

	return nil
}

func (l *list) String() string {
	return strings.Join(*l, ",")
}

// Type names what the flag takes in its help.
func (l *list) Type() string {
	return "list"
}

// portList is a flag of ports, given once for each, or with several at once
// separated by commas.
type portList []uint

// Set adds the ports value names.
func (p *portList) Set(value string) error {
	for part := range strings.SplitSeq(value, ",") {
		port, err := strconv.ParseUint(strings.TrimSpace(part), 10, 16)
		if err != nil {
			return fmt.Errorf("%q is not a port", part)
		}

		*p = append(*p, uint(port))
	}

	return nil
}

func (p *portList) String() string {
	ports := make([]string, len(*p))
	for i, port := range *p {
		ports[i] = strconv.FormatUint(uint64(port), 10)
	}

	return strings.Join(ports, ",")
}

// Type names what the flag takes in its help.
func (p *portList) Type() string {
	return "ports"
}

// defineAllUsers defines -A/--all-users, which reaches anybody's workloads
// through the administrator's routes rather than one's own.
func defineAllUsers(flags *console.FlagSet, allUsers *bool) {
	console.Var(flags, allUsers, console.Long("all-users"), "anybody's, through the administrator's routes, rather than one's own.", console.Short("A"))
}

// clientOf is the workload client of whoever's workloads allUsers says.
func clientOf(ctx context.Context, cli *command.Cli, allUsers bool) (*workload.Client, error) {
	client, err := cli.Client(ctx)
	if err != nil {
		return nil, err
	}

	if allUsers {
		return workload.NewClient(client, workload.Everyone), nil
	}

	return workload.NewClient(client, workload.Mine), nil
}

// newVMFlags describe the Docker VM made for a container or a stack when
// --vm names none. What they leave out is the workload's default.
type newVMFlags struct {
	name    string
	cpus    uint
	memory  units.Size
	disk    units.Size
	ports   portList
	ingress string
	egress  string
}

func (n *newVMFlags) define(flags *console.FlagSet) {
	console.Var(flags, &n.name, console.Long("vm-name"), "the name of the Docker VM made for it when --vm names none.")
	console.Var(flags, &n.cpus, console.Long("vm-cpus"), "the vCPUs of the Docker VM made for it.")
	console.Var(flags, &n.memory, console.Long("vm-memory"), "the memory of the Docker VM made for it, as 512m or 1g.")
	console.Var(flags, &n.disk, console.Long("vm-disk"), "the disk of the Docker VM made for it, as 4g.")
	console.Var(flags, &n.ports, console.Long("vm-publish"), "a port the Docker VM made for it publishes through the ingress; given again for more.")
	console.Var(flags, &n.ingress, console.Long("vm-ingress"), "whether the ingress reaches the Docker VM made for it: allow or deny.")
	console.Var(flags, &n.egress, console.Long("vm-egress"), "whether the Docker VM made for it reaches the internet: allow or deny.")
}

// described is the Docker VM the flags describe, or none when they say
// nothing.
func (n *newVMFlags) described() *workload.NewDockerVM {
	vm := workload.NewDockerVM{Name: n.name, Ports: n.ports}

	if n.cpus > 0 || n.memory > 0 || n.disk > 0 {
		vm.Resources = &workload.Resources{CPUs: n.cpus, Memory: uint64(n.memory), Disk: uint64(n.disk)}
	}

	if len(n.ingress) > 0 || len(n.egress) > 0 {
		vm.Network = &workload.Network{Ingress: n.ingress, Egress: n.egress}
	}

	if len(vm.Name) == 0 && vm.Resources == nil && len(vm.Ports) == 0 && vm.Network == nil {
		return nil
	}

	return &vm
}

// usageError is a command given what it cannot take.
func usageError(flags *console.FlagSet, format string, arguments ...any) error {
	return command.UsageError{Command: flags.Name(), Message: fmt.Sprintf(format, arguments...)}
}
