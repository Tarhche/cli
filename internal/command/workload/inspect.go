package workload

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"sync"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/workload"
)

// inspectedKinds are what a reference is looked for as, in turn, when inspect
// is not told what it names.
var inspectedKinds = []string{"container", "vm", "image", "network", "volume", "stack", "snapshot"}

// inspectCommand writes what references name as the API answers them,
// whatever they are, as docker's inspect does.
type inspectCommand struct {
	cli   *command.Cli
	flags *console.FlagSet

	kind     string
	format   string
	allUsers bool
}

func newInspectCommand(cli *command.Cli) *inspectCommand {
	return &inspectCommand{cli: cli}
}

func (c *inspectCommand) Name() string {
	return "inspect"
}

func (c *inspectCommand) Description() string {
	return "displays detailed information on workloads of any kind."
}

func (c *inspectCommand) Usage() string {
	return command.Name + " inspect [OPTIONS] NAME|ID [NAME|ID...]"
}

func (c *inspectCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.kind, console.Long("type"), "what to look for: "+strings.Join(inspectedKinds, ", ")+".")
	console.Var(flags, &c.format, console.Long("format"), "formats the output with a Go template.", console.Short("f"))
	defineAllUsers(flags, &c.allUsers)
}

func (c *inspectCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *inspectCommand) run(ctx context.Context) error {
	if err := command.MinArgs(c.flags, 1); err != nil {
		return err
	}

	kinds := inspectedKinds

	if len(c.kind) > 0 {
		if !slices.Contains(inspectedKinds, c.kind) {
			return usageError(c.flags, "%q is not a type: give one of %s", c.kind, strings.Join(inspectedKinds, ", "))
		}

		kinds = []string{c.kind}
	}

	client, err := clientOf(ctx, c.cli, c.allUsers)
	if err != nil {
		return err
	}

	found := newWorkloads(ctx, client)

	return inspectEach(c.cli, c.flags.Args(), c.format, func(reference string) (json.RawMessage, error) {
		for _, kind := range kinds {
			raw, err := found.find(kind, reference)
			if !errors.Is(err, workload.ErrNotFound) {
				return raw, err
			}
		}

		if len(c.kind) > 0 {
			return nil, &workload.NotFoundError{Kind: c.kind, Reference: reference}
		}

		return nil, &workload.NotFoundError{Kind: "object", Reference: reference}
	})
}

// workloads are every kind of workload, each read once, when a reference is
// first looked for as one of its kind.
type workloads struct {
	vms        func() ([]workload.VMManifest, error)
	dockerVMs  func() ([]workload.VMManifest, error)
	containers func() ([]workload.ContainerManifest, error)
	images     func() ([]workload.Placed[workload.ImageManifest], error)
	networks   func() ([]workload.Placed[workload.NetworkManifest], error)
	volumes    func() ([]workload.Placed[workload.VolumeManifest], error)
	stacks     func() ([]workload.StackManifest, error)
	snapshots  func() ([]workload.SnapshotManifest, error)
}

func newWorkloads(ctx context.Context, client *workload.Client) *workloads {
	w := &workloads{}

	w.vms = sync.OnceValues(func() ([]workload.VMManifest, error) { return client.VMs(ctx) })
	w.dockerVMs = sync.OnceValues(func() ([]workload.VMManifest, error) {
		vms, err := w.vms()

		return slices.DeleteFunc(slices.Clone(vms), func(vm workload.VMManifest) bool { return !vm.Docker }), err
	})
	w.containers = sync.OnceValues(func() ([]workload.ContainerManifest, error) { return client.Containers(ctx, "") })
	w.images = sync.OnceValues(func() ([]workload.Placed[workload.ImageManifest], error) {
		return across(w.dockerVMs, func(vms []workload.VMManifest) ([]workload.Placed[workload.ImageManifest], error) {
			return client.ImagesIn(ctx, vms)
		})
	})
	w.networks = sync.OnceValues(func() ([]workload.Placed[workload.NetworkManifest], error) {
		return across(w.dockerVMs, func(vms []workload.VMManifest) ([]workload.Placed[workload.NetworkManifest], error) {
			return client.NetworksIn(ctx, vms)
		})
	})
	w.volumes = sync.OnceValues(func() ([]workload.Placed[workload.VolumeManifest], error) {
		return across(w.dockerVMs, func(vms []workload.VMManifest) ([]workload.Placed[workload.VolumeManifest], error) {
			return client.VolumesIn(ctx, vms)
		})
	})
	w.stacks = sync.OnceValues(func() ([]workload.StackManifest, error) { return client.Stacks(ctx, "") })
	w.snapshots = sync.OnceValues(func() ([]workload.SnapshotManifest, error) { return client.Snapshots(ctx, "") })

	return w
}

// across is what list says lives in the Docker VMs vms reads.
func across[T any](vms func() ([]workload.VMManifest, error), list func([]workload.VMManifest) ([]T, error)) ([]T, error) {
	read, err := vms()
	if err != nil {
		return nil, err
	}

	return list(read)
}

// find is the workload of kind reference names, as the API answered it.
func (w *workloads) find(kind, reference string) (json.RawMessage, error) {
	switch kind {
	case "container":
		return matched(w.containers, reference, workload.MatchContainer, workload.ContainerManifest.Raw)
	case "vm":
		return matched(w.vms, reference, workload.MatchVM, workload.VMManifest.Raw)
	case "image":
		return matched(w.images, reference, workload.MatchImage, func(p workload.Placed[workload.ImageManifest]) json.RawMessage { return p.Item.Raw() })
	case "network":
		return matched(w.networks, reference, workload.MatchNetwork, func(p workload.Placed[workload.NetworkManifest]) json.RawMessage { return p.Item.Raw() })
	case "volume":
		return matched(w.volumes, reference, workload.MatchVolume, func(p workload.Placed[workload.VolumeManifest]) json.RawMessage { return p.Item.Raw() })
	case "stack":
		return matched(w.stacks, reference, workload.MatchStack, workload.StackManifest.Raw)
	default:
		return matched(w.snapshots, reference, workload.MatchSnapshot, workload.SnapshotManifest.Raw)
	}
}

// matched is what reference names among what read reads, as the API
// answered it.
func matched[T any](read func() ([]T, error), reference string, match func([]T, string) (T, error), raw func(T) json.RawMessage) (json.RawMessage, error) {
	items, err := read()
	if err != nil {
		return nil, err
	}

	item, err := match(items, reference)
	if err != nil {
		return nil, err
	}

	return raw(item), nil
}
