package workload

import (
	"cmp"
	"context"
	"net/http"
	"strings"
	"time"
)

// Labels a stack's containers carry: the stack's uuid and the service the
// container is of.
const (
	LabelStack        = "workload.stack"
	LabelStackService = "workload.stack.service"
)

// The states a container is running in, as docker lists it without -a.
var runningStates = map[string]bool{
	"running":    true,
	"starting":   true,
	"restarting": true,
	"stopping":   true,
}

// ContainerManifest is a container in a Docker VM, in its kind's manifest
// shape, and, as a listing across VMs says it, the VM it is in.
type ContainerManifest struct {
	Kind     string          `json:"kind"`
	Metadata Metadata        `json:"metadata"`
	Spec     ContainerSpec   `json:"spec"`
	Status   ContainerStatus `json:"status"`

	VMUUID string `json:"vm_uuid,omitempty"`
	VMName string `json:"vm_name,omitempty"`

	kept
}

// UnmarshalJSON reads a container, keeping it as it was answered.
func (m *ContainerManifest) UnmarshalJSON(data []byte) error {
	type plain ContainerManifest

	k, err := keep(data, (*plain)(m))
	m.kept = k

	return err
}

// ContainerSpec is what a container was asked to be. What it runs with in
// its environment is never answered, since it may be secret.
type ContainerSpec struct {
	VM         VMChoice            `json:"vm"`
	Name       string              `json:"name,omitempty"`
	Image      string              `json:"image"`
	Command    []string            `json:"command,omitempty"`
	Entrypoint []string            `json:"entrypoint,omitempty"`
	WorkingDir string              `json:"working_dir,omitempty"`
	Ports      []PortBinding       `json:"ports,omitempty"`
	Mounts     []Mount             `json:"mounts,omitempty"`
	Networks   []string            `json:"networks,omitempty"`
	Aliases    map[string][]string `json:"aliases,omitempty"`

	RestartPolicy string  `json:"restart_policy,omitempty"`
	CPUs          float64 `json:"cpus,omitempty"`
	Memory        uint64  `json:"memory,omitempty"`
	PidsLimit     int64   `json:"pids_limit,omitempty"`
	Privileged    bool    `json:"privileged,omitempty"`
	User          string  `json:"user,omitempty"`
	Runtime       string  `json:"runtime,omitempty"`
}

// PortBinding publishes a container's port on its VM. A host port of zero
// lets docker pick one, and an empty host ip is every address.
type PortBinding struct {
	ContainerPort uint   `json:"container_port"`
	HostPort      uint   `json:"host_port"`
	HostIP        string `json:"host_ip,omitempty"`
	Protocol      string `json:"protocol,omitempty"`
}

// Mount is something mounted into a container: a volume of its VM, a path
// of its VM (bind), or a tmpfs.
type Mount struct {
	Type     string `json:"type"`
	Source   string `json:"source,omitempty"`
	Target   string `json:"target"`
	ReadOnly bool   `json:"read_only,omitempty"`
}

// The types of mount.
const (
	MountVolume = "volume"
	MountBind   = "bind"
	MountTmpfs  = "tmpfs"
)

// ContainerStatus is what a container is doing: in its kind's words, and as
// its VM's dockerd last said, once it is made.
type ContainerStatus struct {
	Status
	ContainerDocker

	Failure *Failure `json:"failure,omitempty"`

	// Stats is the last sample of what it uses, none while it does not run.
	Stats *ContainerStats `json:"stats"`
}

// ContainerDocker is a container as its VM's dockerd last had it.
type ContainerDocker struct {
	ID    string `json:"id,omitempty"`
	Name  string `json:"name,omitempty"`
	Image string `json:"image,omitempty"`

	// DockerState and DockerStatus are docker's own words for what it is
	// doing: "running", and "Up 2 hours".
	DockerState  string `json:"docker_state,omitempty"`
	DockerStatus string `json:"docker_status,omitempty"`

	Command       string            `json:"command,omitempty"`
	Ports         []PortBinding     `json:"ports,omitempty"`
	Networks      []string          `json:"networks,omitempty"`
	Mounts        []Mount           `json:"mounts,omitempty"`
	Labels        map[string]string `json:"labels,omitempty"`
	RestartPolicy string            `json:"restart_policy,omitempty"`
	CreatedAt     time.Time         `json:"created_at,omitzero"`
}

// ContainerStats is one sample of what a container uses. Sizes are bytes,
// and network and block counters since it started.
type ContainerStats struct {
	CPUPercent  float64   `json:"cpu_percent"`
	MemoryUsed  uint64    `json:"memory_used"`
	MemoryLimit uint64    `json:"memory_limit"`
	NetworkRx   uint64    `json:"network_rx"`
	NetworkTx   uint64    `json:"network_tx"`
	BlockRead   uint64    `json:"block_read"`
	BlockWrite  uint64    `json:"block_write"`
	PIDs        uint64    `json:"pids"`
	SampledAt   time.Time `json:"sampled_at"`
}

// DockerName is what the container is called: by its VM's dockerd, else as
// it was asked for.
func (m ContainerManifest) DockerName() string {
	return cmp.Or(strings.TrimPrefix(m.Status.Name, "/"), m.Spec.Name, m.Metadata.Name)
}

// Running reports whether the container runs, as docker lists it without
// -a: running, or on its way up, down or round again.
func (m ContainerManifest) Running() bool {
	return runningStates[m.Status.State]
}

// Reach is the container as the CLI reaches into it, in vm.
func (m ContainerManifest) Reach(vm VM) Container {
	return Container{
		UUID:     m.Metadata.UUID,
		DockerID: m.Status.ID,
		Name:     m.DockerName(),
		State:    m.Status.State,
		VM:       vm,
	}
}

// CreateContainerRequest is a container to create and start, which is
// always one's own. It goes in the Docker VM VMUUID names, in a new one VM
// describes, or, with neither, in a new one with the defaults.
type CreateContainerRequest struct {
	VMUUID string       `json:"vm_uuid,omitempty"`
	VM     *NewDockerVM `json:"vm,omitempty"`

	// Name is docker's name for it; none lets docker pick one.
	Name  string `json:"name,omitempty"`
	Image string `json:"image"`

	Command    []string `json:"command,omitempty"`
	Entrypoint []string `json:"entrypoint,omitempty"`

	// Env is its environment, each as KEY=value.
	Env        []string `json:"env,omitempty"`
	WorkingDir string   `json:"working_dir,omitempty"`

	Ports  []PortBinding `json:"ports,omitempty"`
	Mounts []Mount       `json:"mounts,omitempty"`

	// Networks are the VM's networks it is put on beside the default one.
	Networks []string `json:"networks,omitempty"`

	// RestartPolicy is no, always, unless-stopped or on-failure.
	RestartPolicy string `json:"restart_policy,omitempty"`

	// CPUs are cores and Memory bytes; zero is no limit beyond the VM's.
	CPUs      float64 `json:"cpus,omitempty"`
	Memory    uint64  `json:"memory,omitempty"`
	PidsLimit int64   `json:"pids_limit,omitempty"`

	Privileged bool   `json:"privileged,omitempty"`
	User       string `json:"user,omitempty"`
	Runtime    string `json:"runtime,omitempty"`
}

// CreatedContainer is a container created, and the Docker VM it went into.
type CreatedContainer struct {
	VM        ChosenVM          `json:"vm"`
	Container ContainerManifest `json:"container"`
}

// Containers are the containers across every Docker VM, each with the VM it
// is in, or those of the VM of vmUUID alone when it is not empty.
func (c *Client) Containers(ctx context.Context, vmUUID string) ([]ContainerManifest, error) {
	return items[ContainerManifest](ctx, c, c.path("containers"), narrowed(vmUUID))
}

// VMContainers are the containers of the Docker VM of vmUUID, which do not
// say the VM they are in.
func (c *Client) VMContainers(ctx context.Context, vmUUID string) ([]ContainerManifest, error) {
	return items[ContainerManifest](ctx, c, c.path("vms", vmUUID, "containers"), nil)
}

// Container is the container id names in the Docker VM of vmUUID: its uuid,
// its Docker id or its name.
func (c *Client) Container(ctx context.Context, vmUUID string, id string) (ContainerManifest, error) {
	var container ContainerManifest
	if err := c.get(ctx, c.containerPath(vmUUID, id), nil, &container); err != nil {
		return ContainerManifest{}, err
	}

	return container, nil
}

// CreateContainer creates a container and starts it, waiting for its image
// to be pulled and its VM to be booted when they have to be.
func (c *Client) CreateContainer(ctx context.Context, request CreateContainerRequest) (CreatedContainer, error) {
	var created CreatedContainer
	if err := c.patient().post(ctx, creation("containers"), request, &created); err != nil {
		return CreatedContainer{}, err
	}

	return created, nil
}

// DeleteContainer removes a container, one that runs only by force.
func (c *Client) DeleteContainer(ctx context.Context, vmUUID string, id string, force bool) error {
	return c.act(ctx, http.MethodDelete, c.containerPath(vmUUID, id), flags(map[string]bool{"force": force}), nil)
}

// StartContainer starts a container, which runs once this answers.
func (c *Client) StartContainer(ctx context.Context, vmUUID string, id string) error {
	return c.act(ctx, http.MethodPost, c.containerPath(vmUUID, id, "start"), nil, nil)
}

// StopContainer stops a container, which is stopped once this answers.
func (c *Client) StopContainer(ctx context.Context, vmUUID string, id string) error {
	return c.act(ctx, http.MethodPost, c.containerPath(vmUUID, id, "stop"), nil, nil)
}

// RestartContainer stops a container and starts it again.
func (c *Client) RestartContainer(ctx context.Context, vmUUID string, id string) error {
	return c.act(ctx, http.MethodPost, c.containerPath(vmUUID, id, "restart"), nil, nil)
}

// ContainerLogs are what a container wrote, as query narrows it.
func (c *Client) ContainerLogs(ctx context.Context, vmUUID string, id string, query LogsQuery) (Logs, error) {
	var logs Logs
	if err := c.get(ctx, c.containerPath(vmUUID, id, "logs"), query.values(), &logs); err != nil {
		return Logs{}, err
	}

	return logs, nil
}

// ContainerStats samples what a container uses now.
func (c *Client) ContainerStats(ctx context.Context, vmUUID string, id string) (ContainerStats, error) {
	var stats ContainerStats
	if err := c.get(ctx, c.containerPath(vmUUID, id, "stats"), nil, &stats); err != nil {
		return ContainerStats{}, err
	}

	return stats, nil
}

// ContainerUsage is what a container used over span.
func (c *Client) ContainerUsage(ctx context.Context, vmUUID string, id string, span Span) (Usage, error) {
	var usage Usage
	if err := c.get(ctx, c.containerPath(vmUUID, id, "usage"), span.values(), &usage); err != nil {
		return Usage{}, err
	}

	return usage, nil
}

// ConnectContainer puts a container on another network of its VM, named by
// its name or its Docker id, where its neighbours reach it by aliases too.
func (c *Client) ConnectContainer(ctx context.Context, vmUUID string, id string, network string, aliases []string) error {
	body := struct {
		Network string   `json:"network"`
		Aliases []string `json:"aliases,omitempty"`
	}{Network: network, Aliases: aliases}

	return c.act(ctx, http.MethodPost, c.containerPath(vmUUID, id, "networks"), nil, body)
}

// DisconnectContainer takes a container off a network of its VM, named by
// its name or its Docker id.
func (c *Client) DisconnectContainer(ctx context.Context, vmUUID string, id string, network string) error {
	return c.act(ctx, http.MethodDelete, c.containerPath(vmUUID, id, "networks", network), nil, nil)
}

func (c *Client) containerPath(vmUUID string, id string, rest ...string) string {
	return c.path(append([]string{"vms", vmUUID, "containers", id}, rest...)...)
}

// patient is the client with what it asks bounded by PatientTimeout.
func (c *Client) patient() *Client {
	return &Client{api: c.api.WithTimeout(PatientTimeout), audience: c.audience}
}
