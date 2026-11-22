package workload

import (
	"context"
	"net/http"

	"github.com/Tarhche/cli/internal/api"
)

// StackManifest is a stack, services of a compose-like file deployed into a
// Docker VM, in its kind's manifest shape, and beside it what the VM it is
// in is called now and whose it is.
type StackManifest struct {
	Kind     string      `json:"kind"`
	Metadata Metadata    `json:"metadata"`
	Spec     StackSpec   `json:"spec"`
	Status   StackStatus `json:"status"`

	VMName string `json:"vm_name"`
	Owner  *Owner `json:"owner,omitempty"`

	kept
}

// UnmarshalJSON reads a stack, keeping it as it was answered.
func (m *StackManifest) UnmarshalJSON(data []byte) error {
	type plain StackManifest

	k, err := keep(data, (*plain)(m))
	m.kept = k

	return err
}

// StackSpec is the VM a stack went into and its file, as it was given.
type StackSpec struct {
	VM      VMChoice `json:"vm"`
	Compose string   `json:"compose"`
}

// StackStatus is what a stack is doing, and each of its services.
type StackStatus struct {
	Status

	Services []Service `json:"services,omitempty"`
	Failure  *Failure  `json:"failure,omitempty"`
}

// Service is one service of a stack's file, and the containers made of it.
type Service struct {
	Name       string             `json:"name"`
	State      string             `json:"state"`
	Containers []ServiceContainer `json:"containers,omitempty"`
}

// ServiceContainer is a container of a service: its uuid, its Docker name,
// and its state in docker's word.
type ServiceContainer struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	State string `json:"state"`
}

// CreateStackRequest is a stack to deploy, which is always one's own, into
// the Docker VM VMUUID names, a new one VM describes, or, with neither, a new
// one with the defaults.
type CreateStackRequest struct {
	Name string `json:"name"`

	// Compose is its file, in the shape of a compose file.
	Compose string `json:"compose"`

	VMUUID string       `json:"vm_uuid,omitempty"`
	VM     *NewDockerVM `json:"vm,omitempty"`
}

// CreatedStack is a stack deployed, and the Docker VM it went into.
type CreatedStack struct {
	VM    ChosenVM      `json:"vm"`
	Stack StackManifest `json:"stack"`
}

// Stacks are every stack, or those deployed into the VM of vmUUID alone
// when it is not empty.
func (c *Client) Stacks(ctx context.Context, vmUUID string) ([]StackManifest, error) {
	return api.All[StackManifest](ctx, c.api, c.path("stacks"), narrowed(vmUUID))
}

// Stack is the stack of uuid.
func (c *Client) Stack(ctx context.Context, uuid string) (StackManifest, error) {
	var stack StackManifest
	if err := c.get(ctx, c.path("stacks", uuid), nil, &stack); err != nil {
		return StackManifest{}, err
	}

	return stack, nil
}

// CreateStack deploys a stack, which its graph makes in its own time.
func (c *Client) CreateStack(ctx context.Context, request CreateStackRequest) (CreatedStack, error) {
	var created CreatedStack
	if err := c.post(ctx, creation("stacks"), request, &created); err != nil {
		return CreatedStack{}, err
	}

	return created, nil
}

// DeleteStack asks for the stack of uuid to be removed, with its containers
// and its networks; its volumes and its VM are kept.
func (c *Client) DeleteStack(ctx context.Context, uuid string) error {
	return c.act(ctx, http.MethodDelete, c.path("stacks", uuid), nil, nil)
}

// StartStack asks for the containers of the stack of uuid to be started.
func (c *Client) StartStack(ctx context.Context, uuid string) error {
	return c.act(ctx, http.MethodPost, c.path("stacks", uuid, "start"), nil, nil)
}

// StopStack asks for the containers of the stack of uuid to be stopped.
func (c *Client) StopStack(ctx context.Context, uuid string) error {
	return c.act(ctx, http.MethodPost, c.path("stacks", uuid, "stop"), nil, nil)
}

// RestartStack asks for the containers of the stack of uuid to be restarted.
func (c *Client) RestartStack(ctx context.Context, uuid string) error {
	return c.act(ctx, http.MethodPost, c.path("stacks", uuid, "restart"), nil, nil)
}
