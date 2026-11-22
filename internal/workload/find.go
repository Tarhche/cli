package workload

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/Tarhche/cli/internal/api"
)

const (
	// minimumPrefix is the fewest characters of a Docker id that name it by
	// its beginning, as the workload itself resolves one: fewer would too
	// readily name the wrong one.
	minimumPrefix = 4

	// mostAtOnce is how many VMs are asked at once for what lives in them.
	mostAtOnce = 8
)

// way is one way a reference names an item.
type way[T any] func(item T, reference string) bool

// finder finds the one item of a kind a reference names.
type finder[T any] struct {
	kind string

	// identify is what tells items apart: items of one identity are one, as
	// the manifests of every reference one image is held under are.
	identify func(T) string

	// describe is how an item is told apart from the others a reference
	// names.
	describe func(T) string

	// ways are tried in turn, what names one surely, its uuid, before what
	// may name several, a name.
	ways []way[T]
}

// find is the one item of items reference names.
func (f finder[T]) find(reference string, items []T) (T, error) {
	var none T

	if len(reference) == 0 {
		return none, &NotFoundError{Kind: f.kind, Reference: reference}
	}

	for _, named := range f.ways {
		matched := f.matching(named, reference, items)

		switch len(matched) {
		case 0:
			continue
		case 1:
			return matched[0], nil
		}

		described := make([]string, len(matched))
		for i, item := range matched {
			described[i] = f.describe(item)
		}

		return none, &AmbiguousError{Kind: f.kind, Reference: reference, Matches: described}
	}

	return none, &NotFoundError{Kind: f.kind, Reference: reference}
}

// matching are the items reference names in one way, each identity once.
func (f finder[T]) matching(named way[T], reference string, items []T) []T {
	var (
		matched []T
		seen    = make(map[string]bool)
	)

	for _, item := range items {
		identity := f.identify(item)
		if !named(item, reference) || seen[identity] {
			continue
		}

		seen[identity] = true
		matched = append(matched, item)
	}

	return matched
}

// named is "name (id)", or the id alone for what has no name.
func named(name, id string) string {
	if len(name) == 0 {
		return id
	}

	return fmt.Sprintf("%s (%s)", name, id)
}

// inVM is what describes something, in the VM it lives in.
func inVM(described string, vm VMManifest) string {
	return fmt.Sprintf("%s in vm %s", described, named(vm.Metadata.Name, vm.Metadata.UUID))
}

// prefixes reports whether reference is long enough to name id by its
// beginning, and does.
func prefixes(id, reference string) bool {
	return len(reference) >= minimumPrefix && len(id) > 0 && strings.HasPrefix(id, reference)
}

// withoutAlgorithm is a Docker id without the algorithm an image's carries.
func withoutAlgorithm(id string) string {
	return strings.TrimPrefix(id, "sha256:")
}

var vmFinder = finder[VMManifest]{
	kind:     "vm",
	identify: func(m VMManifest) string { return m.Metadata.UUID },
	describe: func(m VMManifest) string { return named(m.Metadata.Name, m.Metadata.UUID) },
	ways: []way[VMManifest]{
		func(m VMManifest, reference string) bool { return m.Metadata.UUID == reference },
		func(m VMManifest, reference string) bool { return m.Metadata.Slug == reference },
		func(m VMManifest, reference string) bool { return m.Metadata.Name == reference },
	},
}

var containerFinder = finder[ContainerManifest]{
	kind:     "container",
	identify: func(m ContainerManifest) string { return m.Metadata.UUID },
	describe: func(m ContainerManifest) string {
		return fmt.Sprintf("%s in vm %s", named(m.DockerName(), m.Metadata.UUID), named(m.VMName, m.VMUUID))
	},
	ways: []way[ContainerManifest]{
		func(m ContainerManifest, reference string) bool { return m.Metadata.UUID == reference },
		func(m ContainerManifest, reference string) bool { return m.Status.ID == reference },
		func(m ContainerManifest, reference string) bool {
			return strings.TrimPrefix(m.Status.Name, "/") == reference || m.Spec.Name == reference || m.Metadata.Name == reference
		},
		func(m ContainerManifest, reference string) bool { return prefixes(m.Status.ID, reference) },
	},
}

var imageFinder = finder[Placed[ImageManifest]]{
	kind: "image",
	identify: func(p Placed[ImageManifest]) string {
		if id := p.Item.Status.ID; len(id) > 0 {
			return p.VM.Metadata.UUID + "/" + withoutAlgorithm(id)
		}

		return p.VM.Metadata.UUID + "/" + p.Item.Metadata.UUID
	},
	describe: func(p Placed[ImageManifest]) string {
		return inVM(named(p.Item.Reference(), shortened(withoutAlgorithm(p.Item.Status.ID), p.Item.Metadata.UUID)), p.VM)
	},
	ways: []way[Placed[ImageManifest]]{
		func(p Placed[ImageManifest], reference string) bool { return p.Item.Metadata.UUID == reference },
		func(p Placed[ImageManifest], reference string) bool {
			id := withoutAlgorithm(p.Item.Status.ID)
			return len(id) > 0 && id == withoutAlgorithm(reference)
		},
		func(p Placed[ImageManifest], reference string) bool {
			return slices.Contains(p.Item.References(), NormalizedReference(reference))
		},
		func(p Placed[ImageManifest], reference string) bool {
			return prefixes(withoutAlgorithm(p.Item.Status.ID), withoutAlgorithm(reference))
		},
	},
}

var networkFinder = finder[Placed[NetworkManifest]]{
	kind:     "network",
	identify: func(p Placed[NetworkManifest]) string { return p.VM.Metadata.UUID + "/" + p.Item.Metadata.UUID },
	describe: func(p Placed[NetworkManifest]) string {
		return inVM(named(p.Item.DockerName(), p.Item.Metadata.UUID), p.VM)
	},
	ways: []way[Placed[NetworkManifest]]{
		func(p Placed[NetworkManifest], reference string) bool { return p.Item.Metadata.UUID == reference },
		func(p Placed[NetworkManifest], reference string) bool { return p.Item.Status.ID == reference },
		func(p Placed[NetworkManifest], reference string) bool {
			return p.Item.Status.Name == reference || p.Item.Spec.Name == reference || p.Item.Metadata.Name == reference
		},
		func(p Placed[NetworkManifest], reference string) bool { return prefixes(p.Item.Status.ID, reference) },
	},
}

var volumeFinder = finder[Placed[VolumeManifest]]{
	kind:     "volume",
	identify: func(p Placed[VolumeManifest]) string { return p.VM.Metadata.UUID + "/" + p.Item.Metadata.UUID },
	describe: func(p Placed[VolumeManifest]) string {
		return inVM(named(p.Item.DockerName(), p.Item.Metadata.UUID), p.VM)
	},
	ways: []way[Placed[VolumeManifest]]{
		func(p Placed[VolumeManifest], reference string) bool { return p.Item.Metadata.UUID == reference },
		func(p Placed[VolumeManifest], reference string) bool {
			return p.Item.Status.Name == reference || p.Item.Spec.Name == reference || p.Item.Metadata.Name == reference
		},
	},
}

var stackFinder = finder[StackManifest]{
	kind:     "stack",
	identify: func(m StackManifest) string { return m.Metadata.UUID },
	describe: func(m StackManifest) string { return named(m.Metadata.Name, m.Metadata.UUID) },
	ways: []way[StackManifest]{
		func(m StackManifest, reference string) bool { return m.Metadata.UUID == reference },
		func(m StackManifest, reference string) bool { return m.Metadata.Slug == reference },
		func(m StackManifest, reference string) bool { return m.Metadata.Name == reference },
	},
}

var snapshotFinder = finder[SnapshotManifest]{
	kind:     "snapshot",
	identify: func(m SnapshotManifest) string { return m.Metadata.UUID },
	describe: func(m SnapshotManifest) string { return named(m.Metadata.Name, m.Metadata.UUID) },
	ways: []way[SnapshotManifest]{
		func(m SnapshotManifest, reference string) bool { return m.Metadata.UUID == reference },
		func(m SnapshotManifest, reference string) bool { return m.Metadata.Name == reference },
	},
}

// shortened is a Docker id as docker shortens it, or fallback when there is
// none.
func shortened(id, fallback string) string {
	switch {
	case len(id) == 0:
		return fallback
	case len(id) > 12:
		return id[:12]
	default:
		return id
	}
}

// FindVM is the VM reference names: its uuid, its slug or its name. A
// reference that is a uuid is asked for as it is first.
func (c *Client) FindVM(ctx context.Context, reference string) (VMManifest, error) {
	if IsUUID(reference) {
		vm, err := c.VM(ctx, reference)
		if err == nil || !api.IsNotFound(err) {
			return vm, err
		}
	}

	vms, err := c.VMs(ctx)
	if err != nil {
		return VMManifest{}, err
	}

	return MatchVM(vms, reference)
}

// MatchVM is the VM of vms reference names: its uuid, its slug or its name.
func MatchVM(vms []VMManifest, reference string) (VMManifest, error) {
	return vmFinder.find(reference, vms)
}

// MatchContainer is the container of containers reference names: its uuid,
// its Docker id, its name, or the beginning of its Docker id.
func MatchContainer(containers []ContainerManifest, reference string) (ContainerManifest, error) {
	return containerFinder.find(reference, containers)
}

// MatchImage is the image of images reference names: its uuid, its id, one
// of its references, or the beginning of its id.
func MatchImage(images []Placed[ImageManifest], reference string) (Placed[ImageManifest], error) {
	return imageFinder.find(reference, images)
}

// MatchNetwork is the network of networks reference names: its uuid, its id,
// its name, or the beginning of its id.
func MatchNetwork(networks []Placed[NetworkManifest], reference string) (Placed[NetworkManifest], error) {
	return networkFinder.find(reference, networks)
}

// MatchVolume is the volume of volumes reference names: its uuid or its
// name.
func MatchVolume(volumes []Placed[VolumeManifest], reference string) (Placed[VolumeManifest], error) {
	return volumeFinder.find(reference, volumes)
}

// MatchStack is the stack of stacks reference names: its uuid, its slug or
// its name.
func MatchStack(stacks []StackManifest, reference string) (StackManifest, error) {
	return stackFinder.find(reference, stacks)
}

// MatchSnapshot is the snapshot of snapshots reference names: its uuid or
// its name.
func MatchSnapshot(snapshots []SnapshotManifest, reference string) (SnapshotManifest, error) {
	return snapshotFinder.find(reference, snapshots)
}

// FindContainer is the container reference names: its uuid, its Docker id,
// its name, or the beginning of its Docker id, across every Docker VM, or
// in the VM of vmUUID alone when it is not empty. It says which VM it is in.
func (c *Client) FindContainer(ctx context.Context, vmUUID string, reference string) (ContainerManifest, error) {
	containers, err := c.Containers(ctx, vmUUID)
	if err != nil {
		return ContainerManifest{}, err
	}

	return MatchContainer(containers, reference)
}

// FindImage is the image reference names in one of vms: its uuid, its id,
// one of its references, or the beginning of its id.
func (c *Client) FindImage(ctx context.Context, vms []VMManifest, reference string) (Placed[ImageManifest], error) {
	images, err := c.ImagesIn(ctx, vms)
	if err != nil {
		return Placed[ImageManifest]{}, err
	}

	return MatchImage(images, reference)
}

// FindNetwork is the network reference names in one of vms: its uuid, its
// id, its name, or the beginning of its id.
func (c *Client) FindNetwork(ctx context.Context, vms []VMManifest, reference string) (Placed[NetworkManifest], error) {
	networks, err := c.NetworksIn(ctx, vms)
	if err != nil {
		return Placed[NetworkManifest]{}, err
	}

	return MatchNetwork(networks, reference)
}

// FindVolume is the volume reference names in one of vms: its uuid or its
// name.
func (c *Client) FindVolume(ctx context.Context, vms []VMManifest, reference string) (Placed[VolumeManifest], error) {
	volumes, err := c.VolumesIn(ctx, vms)
	if err != nil {
		return Placed[VolumeManifest]{}, err
	}

	return MatchVolume(volumes, reference)
}

// FindStack is the stack reference names: its uuid, its slug or its name.
func (c *Client) FindStack(ctx context.Context, reference string) (StackManifest, error) {
	if IsUUID(reference) {
		stack, err := c.Stack(ctx, reference)
		if err == nil || !api.IsNotFound(err) {
			return stack, err
		}
	}

	stacks, err := c.Stacks(ctx, "")
	if err != nil {
		return StackManifest{}, err
	}

	return MatchStack(stacks, reference)
}

// FindSnapshot is the snapshot reference names: its uuid or its name.
func (c *Client) FindSnapshot(ctx context.Context, reference string) (SnapshotManifest, error) {
	if IsUUID(reference) {
		snapshot, err := c.Snapshot(ctx, reference)
		if err == nil || !api.IsNotFound(err) {
			return snapshot, err
		}
	}

	snapshots, err := c.Snapshots(ctx, "")
	if err != nil {
		return SnapshotManifest{}, err
	}

	return MatchSnapshot(snapshots, reference)
}

// ImagesIn are the images every one of vms holds.
func (c *Client) ImagesIn(ctx context.Context, vms []VMManifest) ([]Placed[ImageManifest], error) {
	return across(ctx, vms, c.Images)
}

// NetworksIn are the networks of every one of vms.
func (c *Client) NetworksIn(ctx context.Context, vms []VMManifest) ([]Placed[NetworkManifest], error) {
	return across(ctx, vms, c.Networks)
}

// VolumesIn are the volumes of every one of vms.
func (c *Client) VolumesIn(ctx context.Context, vms []VMManifest) ([]Placed[VolumeManifest], error) {
	return across(ctx, vms, c.Volumes)
}

// across is what list says lives in each of vms, a few of them asked at
// once, in the order of vms. A VM gone since it was listed holds nothing.
func across[T any](ctx context.Context, vms []VMManifest, list func(ctx context.Context, vmUUID string) ([]T, error)) ([]Placed[T], error) {
	var (
		found    = make([][]T, len(vms))
		failures = make([]error, len(vms))
		slots    = make(chan struct{}, mostAtOnce)
		group    sync.WaitGroup
	)

	for i, vm := range vms {
		group.Go(func() {
			slots <- struct{}{}
			defer func() { <-slots }()

			items, err := list(ctx, vm.Metadata.UUID)
			if api.IsNotFound(err) {
				return
			}

			found[i], failures[i] = items, err
		})
	}

	group.Wait()

	for _, err := range failures {
		if err != nil {
			return nil, err
		}
	}

	var placed []Placed[T]

	for i, vm := range vms {
		for _, item := range found[i] {
			placed = append(placed, Placed[T]{VM: vm, Item: item})
		}
	}

	return placed, nil
}

// IsUUID reports whether s is written as a uuid is: 32 hexadecimal digits
// in groups of 8, 4, 4, 4 and 12.
func IsUUID(s string) bool {
	if len(s) != 36 {
		return false
	}

	for i, r := range s {
		switch i {
		case 8, 13, 18, 23:
			if r != '-' {
				return false
			}
		default:
			if !strings.ContainsRune("0123456789abcdefABCDEF", r) {
				return false
			}
		}
	}

	return true
}
