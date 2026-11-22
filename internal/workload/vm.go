package workload

import (
	"context"
	"net/http"
	"slices"
	"time"

	"github.com/Tarhche/cli/internal/api"
)

// The kinds a VM is created as: a machine, an operating system somebody
// opens a terminal in, or a Docker VM, which containers and stacks run in.
const (
	KindMachine = "machine"
	KindDocker  = "docker"
)

// VMManifest is a VM in its kind's manifest shape, and beside it what the
// API knows of it: whether it is a Docker VM, whose it is, and where its
// ports are served.
type VMManifest struct {
	Kind     string   `json:"kind"`
	Metadata Metadata `json:"metadata"`
	Spec     VMSpec   `json:"spec"`
	Status   VMStatus `json:"status"`

	Docker bool   `json:"docker"`
	Owner  *Owner `json:"owner,omitempty"`
	URLs   []URL  `json:"urls"`

	kept
}

// UnmarshalJSON reads a VM, keeping it as it was answered.
func (m *VMManifest) UnmarshalJSON(data []byte) error {
	type plain VMManifest

	k, err := keep(data, (*plain)(m))
	m.kept = k

	return err
}

// VMSpec is what a VM was asked to be.
type VMSpec struct {
	Image          string    `json:"image,omitempty"`
	Resources      Resources `json:"resources"`
	Ports          []uint    `json:"ports"`
	Network        Network   `json:"network"`
	PersistentDisk bool      `json:"persistent_disk,omitempty"`

	// Source is the snapshot it was made from, when it was.
	Source *VMSource `json:"source,omitempty"`
}

// VMSource is the snapshot a VM was made from.
type VMSource struct {
	Snapshot string `json:"snapshot"`
}

// VMStatus is what a VM is doing.
type VMStatus struct {
	Status

	// Stats is the last sample of what it uses, none while it does not run.
	Stats     *VMStats  `json:"stats"`
	StartedAt time.Time `json:"started_at,omitzero"`
}

// VMStats is one sample of what a VM uses. Sizes are bytes, and network
// counters since it started.
type VMStats struct {
	CPUPercent  float64   `json:"cpu_percent"`
	MemoryUsed  uint64    `json:"memory_used"`
	MemoryLimit uint64    `json:"memory_limit"`
	DiskUsed    uint64    `json:"disk_used"`
	DiskTotal   uint64    `json:"disk_total"`
	NetworkRx   uint64    `json:"network_rx"`
	NetworkTx   uint64    `json:"network_tx"`
	SampledAt   time.Time `json:"sampled_at"`
}

// URL is one of a VM's ports, and the address it is served on.
type URL struct {
	Port uint   `json:"port"`
	URL  string `json:"url"`
}

// Reach is the VM as the CLI reaches into it.
func (m VMManifest) Reach() VM {
	return VM{
		UUID:   m.Metadata.UUID,
		Name:   m.Metadata.Name,
		Slug:   m.Metadata.Slug,
		Docker: m.Docker,
		State:  m.Status.State,
		Ports:  slices.Clone(m.Spec.Ports),
	}
}

// CreateVMRequest is a VM to create, which is always one's own.
type CreateVMRequest struct {
	Name string `json:"name"`

	// Kind is KindMachine or KindDocker.
	Kind string `json:"kind"`

	// Image is what a machine boots; none is its kind's default, and a
	// Docker VM always boots the workload's Docker image.
	Image string `json:"image,omitempty"`

	Resources Resources `json:"resources"`

	// Ports are the ports the ingress serves, while its ingress allows it.
	Ports   []uint  `json:"ports"`
	Network Network `json:"network"`

	// PersistentDisk keeps what is written to its disk across a stop and a
	// start.
	PersistentDisk bool `json:"persistent_disk"`

	// LifetimeSeconds is how long it is kept before it is deleted; zero
	// keeps it until somebody deletes it.
	LifetimeSeconds int64 `json:"lifetime_seconds"`

	// SnapshotUUID makes it from one of one's snapshots, whose kind and
	// image it takes, and whose disk it is given at least.
	SnapshotUUID string `json:"snapshot_uuid,omitempty"`
}

// UpdateVMRequest is what changes of a VM: what it leaves out stays as it
// is. A change to its ports, its network or its resources restarts a VM
// that is not stopped, and its disk only grows.
type UpdateVMRequest struct {
	Name *string `json:"name,omitempty"`

	// LifetimeSeconds counts again from now, and zero keeps it until it is
	// deleted.
	LifetimeSeconds *int64 `json:"lifetime_seconds,omitempty"`

	// Ports replace the ones it has; none exposes none.
	Ports *[]uint `json:"ports,omitempty"`

	// Network and Resources are asked for whole.
	Network   *Network   `json:"network,omitempty"`
	Resources *Resources `json:"resources,omitempty"`
}

// VMs are every VM.
func (c *Client) VMs(ctx context.Context) ([]VMManifest, error) {
	return api.All[VMManifest](ctx, c.api, c.path("vms"), nil)
}

// DockerVMs are every Docker VM, which containers, images, networks,
// volumes and stacks live in.
func (c *Client) DockerVMs(ctx context.Context) ([]VMManifest, error) {
	vms, err := c.VMs(ctx)
	if err != nil {
		return nil, err
	}

	return slices.DeleteFunc(vms, func(vm VMManifest) bool { return !vm.Docker }), nil
}

// VM is the VM of uuid.
func (c *Client) VM(ctx context.Context, uuid string) (VMManifest, error) {
	var vm VMManifest
	if err := c.get(ctx, c.path("vms", uuid), nil, &vm); err != nil {
		return VMManifest{}, err
	}

	return vm, nil
}

// CreateVM creates a VM, which its node boots in its own time.
func (c *Client) CreateVM(ctx context.Context, request CreateVMRequest) (VMManifest, error) {
	var vm VMManifest
	if err := c.post(ctx, creation("vms"), request, &vm); err != nil {
		return VMManifest{}, err
	}

	return vm, nil
}

// UpdateVM changes what request says of the VM of uuid.
func (c *Client) UpdateVM(ctx context.Context, uuid string, request UpdateVMRequest) (VMManifest, error) {
	var vm VMManifest
	if err := c.api.Patch(ctx, c.path("vms", uuid), request, &vm); err != nil {
		return VMManifest{}, err
	}

	return vm, nil
}

// DeleteVM asks for the VM of uuid to be deleted, and with it, when
// cascade says so, the stacks and the containers that would make it again.
// What needs it is what it is answered with.
func (c *Client) DeleteVM(ctx context.Context, uuid string, cascade bool) (Removal, error) {
	var removal Removal
	if err := c.api.Do(ctx, http.MethodDelete, c.path("vms", uuid), flags(map[string]bool{"cascade": cascade}), nil, &removal); err != nil {
		return Removal{}, err
	}

	return removal, nil
}

// VMDependents are what needs the VM of uuid, as its delete would be
// answered with them now.
func (c *Client) VMDependents(ctx context.Context, uuid string) (Removal, error) {
	var removal Removal
	if err := c.get(ctx, c.path("vms", uuid, "dependents"), nil, &removal); err != nil {
		return Removal{}, err
	}

	return removal, nil
}

// StartVM asks for the VM of uuid to be started.
func (c *Client) StartVM(ctx context.Context, uuid string) error {
	return c.act(ctx, http.MethodPost, c.path("vms", uuid, "start"), nil, nil)
}

// StopVM asks for the VM of uuid to be stopped.
func (c *Client) StopVM(ctx context.Context, uuid string) error {
	return c.act(ctx, http.MethodPost, c.path("vms", uuid, "stop"), nil, nil)
}

// RestartVM asks for the VM of uuid to be stopped and started again.
func (c *Client) RestartVM(ctx context.Context, uuid string) error {
	return c.act(ctx, http.MethodPost, c.path("vms", uuid, "restart"), nil, nil)
}

// RestoreVM asks for the disk of the VM of uuid to be replaced by the
// snapshot of snapshotUUID.
func (c *Client) RestoreVM(ctx context.Context, uuid string, snapshotUUID string) error {
	return c.act(ctx, http.MethodPost, c.path("vms", uuid, "restore"), nil, map[string]string{"snapshot_uuid": snapshotUUID})
}

// VMLogs are what the VM of uuid wrote, as query narrows it.
func (c *Client) VMLogs(ctx context.Context, uuid string, query LogsQuery) (Logs, error) {
	var logs Logs
	if err := c.get(ctx, c.path("vms", uuid, "logs"), query.values(), &logs); err != nil {
		return Logs{}, err
	}

	return logs, nil
}

// VMUsage is what the VM of uuid used over span.
func (c *Client) VMUsage(ctx context.Context, uuid string, span Span) (Usage, error) {
	var usage Usage
	if err := c.get(ctx, c.path("vms", uuid, "usage"), span.values(), &usage); err != nil {
		return Usage{}, err
	}

	return usage, nil
}
