// Package workload is the workload as the CLI reaches it: a client of the
// API's routes for VMs and what lives in Docker VMs (containers, images,
// networks, volumes and stacks) and the snapshots of their disks, both one's
// own and anybody's, and which VM or container a name means, so that a
// command, a terminal or a forwarded port can be opened in it.
package workload

import (
	"context"
	"fmt"

	"github.com/Tarhche/cli/internal/api"
)

// VM is a VM as the CLI reaches into it.
type VM struct {
	UUID string
	Name string
	Slug string

	// Docker says it runs a dockerd, which containers live in.
	Docker bool

	State string

	// Ports are the ports it publishes, which a forward reaches without
	// anything inside it.
	Ports []uint
}

// Container is a container in a Docker VM, as the CLI reaches into it.
type Container struct {
	UUID string

	// DockerID is what the VM's dockerd calls it.
	DockerID string

	Name  string
	State string

	// VM is the Docker VM it lives in.
	VM VM
}

// Resolver finds what a reference names: a uuid, a name, a slug, or a
// container's Docker id.
type Resolver interface {
	VM(ctx context.Context, reference string) (VM, error)
	Container(ctx context.Context, reference string) (Container, error)
}

// NewResolver is a resolver of one's own VMs and containers, asking the API
// through client.
func NewResolver(client *api.Client) Resolver {
	return NewClient(client, Mine).Resolver()
}

// Resolver is a resolver of the VMs and the containers of the client's
// audience.
func (c *Client) Resolver() Resolver {
	return resolver{client: c}
}

type resolver struct {
	client *Client
}

// VM is the VM reference names: its uuid, its slug or its name.
func (r resolver) VM(ctx context.Context, reference string) (VM, error) {
	vm, err := r.client.FindVM(ctx, reference)
	if err != nil {
		return VM{}, err
	}

	return vm.Reach(), nil
}

// Container is the container reference names, its uuid, its Docker id, its
// name or the beginning of its Docker id, and the VM it lives in.
func (r resolver) Container(ctx context.Context, reference string) (Container, error) {
	container, err := r.client.FindContainer(ctx, "", reference)
	if err != nil {
		return Container{}, err
	}

	vm, err := r.client.VM(ctx, container.VMUUID)
	if err != nil {
		return Container{}, fmt.Errorf("the vm of container %s: %w", reference, err)
	}

	return container.Reach(vm.Reach()), nil
}
