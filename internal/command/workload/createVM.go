package workload

import (
	"context"
	"crypto/rand"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/units"
	"github.com/Tarhche/cli/internal/workload"
)

// What a VM is given when it is not told otherwise: the least the workload
// gives one of its kind, which is more for a Docker VM, for its dockerd and
// the images it pulls.
const (
	defaultCPUs         = 1
	defaultMemory       = 128 << 20
	defaultDisk         = 1 << 30
	defaultDockerMemory = 512 << 20
	defaultDockerDisk   = 4 << 30
)

// createVMCommand creates a VM: a machine booting an image, or a Docker VM,
// which containers and stacks run in.
type createVMCommand struct {
	cli   *command.Cli
	flags *console.FlagSet

	name           string
	image          string
	docker         bool
	cpus           uint
	memory         units.Size
	disk           units.Size
	ports          portList
	ingress        string
	egress         string
	lifetime       time.Duration
	persistentDisk bool
	snapshot       string
}

func newCreateVMCommand(cli *command.Cli) *createVMCommand {
	return &createVMCommand{cli: cli}
}

func (c *createVMCommand) Name() string {
	return "create"
}

func (c *createVMCommand) Description() string {
	return "Create a VM: a machine booting an image, or a Docker VM"
}

func (c *createVMCommand) Usage() string {
	return command.Name + " vm create [OPTIONS]"
}

func (c *createVMCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.name, console.Long("name"), "the VM's name; one is made up when it is not given.")
	console.Var(flags, &c.image, console.Long("image"), "the image a machine boots, its kind's default when it is not given.")
	console.Var(flags, &c.docker, console.Long("docker"), "creates a Docker VM, which containers and stacks run in.")
	console.Var(flags, &c.cpus, console.Long("cpus"), "its whole vCPUs.", console.Default(defaultCPUs))
	console.Var(flags, &c.memory, console.Long("memory"), "its memory, as 512m (default 128m, or 512m for a Docker VM).", console.Short("m"))
	console.Var(flags, &c.disk, console.Long("disk"), "its disk, as 4g (default 1g, or 4g for a Docker VM, or the snapshot's).")
	console.Var(flags, &c.ports, console.Long("publish"), "a port it publishes through the ingress; given again for more.", console.Short("p"))
	console.Var(flags, &c.ingress, console.Long("ingress"), "whether the ingress reaches its ports: allow or deny.", console.Default(workload.Allow))
	console.Var(flags, &c.egress, console.Long("egress"), "whether it reaches the internet: allow or deny.", console.Default(workload.Allow))
	console.Var(flags, &c.lifetime, console.Long("lifetime"), "how long it is kept before it is deleted, as 2h; kept until it is deleted when it is not given.")
	console.Var(flags, &c.persistentDisk, console.Long("persistent-disk"), "keeps what is written to its disk across a stop and a start.", console.Default(true))
	console.Var(flags, &c.snapshot, console.Long("from-snapshot"), "makes it from one of one's snapshots, whose kind and image it takes.")
}

func (c *createVMCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *createVMCommand) run(ctx context.Context) error {
	if err := command.NoArgs(c.flags); err != nil {
		return err
	}

	client, err := clientOf(ctx, c.cli, false)
	if err != nil {
		return err
	}

	request := workload.CreateVMRequest{
		Name:            c.name,
		Kind:            workload.KindMachine,
		Image:           c.image,
		Ports:           c.ports,
		Network:         workload.Network{Ingress: c.ingress, Egress: c.egress},
		PersistentDisk:  c.persistentDisk,
		LifetimeSeconds: int64(c.lifetime / time.Second),
	}

	if c.docker {
		request.Kind = workload.KindDocker
	}

	if len(c.snapshot) > 0 {
		snapshot, err := client.FindSnapshot(ctx, c.snapshot)
		if err != nil {
			return err
		}

		request.SnapshotUUID = snapshot.Metadata.UUID
		request.Kind = workload.KindMachine

		if snapshot.Docker {
			request.Kind = workload.KindDocker
		}
	}

	request.Resources = c.resources(request.Kind, len(request.SnapshotUUID) > 0)

	if len(request.Name) == 0 {
		request.Name = madeUpName(request.Kind, request.Image)
	}

	if request.Ports == nil {
		request.Ports = []uint{}
	}

	vm, err := client.CreateVM(ctx, request)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintln(c.cli.Out, vm.Metadata.UUID)

	return err
}

// resources are what the flags give a VM of kind, and what its kind is
// given when they say nothing: a VM made from a snapshot is given the
// snapshot's disk unless it is given more.
func (c *createVMCommand) resources(kind string, fromSnapshot bool) workload.Resources {
	resources := workload.Resources{CPUs: c.cpus, Memory: defaultMemory, Disk: defaultDisk}

	if kind == workload.KindDocker {
		resources.Memory, resources.Disk = defaultDockerMemory, defaultDockerDisk
	}

	if fromSnapshot {
		resources.Disk = 0
	}

	if c.memory > 0 {
		resources.Memory = uint64(c.memory)
	}

	if c.disk > 0 {
		resources.Disk = uint64(c.disk)
	}

	return resources
}

// madeUpName is a name for a VM that was given none: what it is, and a few
// random letters, as docker-4f2k or ubuntu-x7pq.
func madeUpName(kind, image string) string {
	base := kind
	if kind == workload.KindMachine && len(image) > 0 {
		base, _, _ = strings.Cut(path.Base(strings.Split(image, "@")[0]), ":")
	}

	return base + "-" + strings.ToLower(rand.Text()[:4])
}
