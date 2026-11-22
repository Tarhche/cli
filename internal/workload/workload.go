// Package workload is the workload as the CLI reaches into it: which VM or
// container a name means, so that a command, a terminal or a forwarded port
// can be opened in it.
package workload

import (
	"context"
	"errors"

	"github.com/Tarhche/cli/internal/api"
)

// ErrAmbiguous is a reference that names more than one thing.
var ErrAmbiguous = errors.New("ambiguous reference")

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

// NewResolver is a resolver asking the API through client.
func NewResolver(client *api.Client) Resolver {
	return unresolved{}
}

type unresolved struct{}

func (unresolved) VM(context.Context, string) (VM, error) {
	return VM{}, errors.New("workload: resolving a vm is not implemented yet")
}

func (unresolved) Container(context.Context, string) (Container, error) {
	return Container{}, errors.New("workload: resolving a container is not implemented yet")
}
