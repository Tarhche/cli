package desktop

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// The roots of the workload's routes: one's own things are read and acted
// on under mine, and created under everyone's.
const (
	mineRoot     = "/api/dashboard/my/workload"
	everyoneRoot = "/api/dashboard/workload"
)

// route is one of the workload's routes: its method, and its path, which
// names what it is asked of in braces, as package access matches it.
type route struct {
	method string
	path   string
}

// at is the route asked of values, which take the place of what its path
// names in braces, in order, as they are: the client escapes them.
func (r route) at(values ...string) route {
	segments := strings.Split(r.path, "/")

	for i, segment := range segments {
		if strings.HasPrefix(segment, "{") && len(values) > 0 {
			segments[i], values = values[0], values[1:]
		}
	}

	return route{method: r.method, path: strings.Join(segments, "/")}
}

// The routes the service asks, which what the session may do is read off.
var (
	vmsRoute          = route{http.MethodGet, mine("vms")}
	vmRoute           = route{http.MethodGet, mine("vms", "{uuid}")}
	createVMRoute     = route{http.MethodPost, everyone("vms")}
	startVMRoute      = route{http.MethodPost, mine("vms", "{uuid}", "start")}
	stopVMRoute       = route{http.MethodPost, mine("vms", "{uuid}", "stop")}
	restartVMRoute    = route{http.MethodPost, mine("vms", "{uuid}", "restart")}
	restoreVMRoute    = route{http.MethodPost, mine("vms", "{uuid}", "restore")}
	deleteVMRoute     = route{http.MethodDelete, mine("vms", "{uuid}")}
	vmDependentsRoute = route{http.MethodGet, mine("vms", "{uuid}", "dependents")}
	vmLogsRoute       = route{http.MethodGet, mine("vms", "{uuid}", "logs")}
	vmUsageRoute      = route{http.MethodGet, mine("vms", "{uuid}", "usage")}

	snapshotsRoute      = route{http.MethodGet, mine("snapshots")}
	snapshotRoute       = route{http.MethodGet, mine("snapshots", "{uuid}")}
	createSnapshotRoute = route{http.MethodPost, everyone("vms", "{uuid}", "snapshots")}
	renameSnapshotRoute = route{http.MethodPatch, mine("snapshots", "{uuid}")}
	deleteSnapshotRoute = route{http.MethodDelete, mine("snapshots", "{uuid}")}

	containersRoute       = route{http.MethodGet, mine("containers")}
	containerRoute        = route{http.MethodGet, mine("vms", "{uuid}", "containers", "{id}")}
	runContainerRoute     = route{http.MethodPost, everyone("containers")}
	startContainerRoute   = route{http.MethodPost, mine("vms", "{uuid}", "containers", "{id}", "start")}
	stopContainerRoute    = route{http.MethodPost, mine("vms", "{uuid}", "containers", "{id}", "stop")}
	restartContainerRoute = route{http.MethodPost, mine("vms", "{uuid}", "containers", "{id}", "restart")}
	deleteContainerRoute  = route{http.MethodDelete, mine("vms", "{uuid}", "containers", "{id}")}
	containerLogsRoute    = route{http.MethodGet, mine("vms", "{uuid}", "containers", "{id}", "logs")}
	containerStatsRoute   = route{http.MethodGet, mine("vms", "{uuid}", "containers", "{id}", "stats")}
	containerUsageRoute   = route{http.MethodGet, mine("vms", "{uuid}", "containers", "{id}", "usage")}

	imagesRoute      = route{http.MethodGet, mine("vms", "{uuid}", "images")}
	pullImageRoute   = route{http.MethodPost, mine("vms", "{uuid}", "images")}
	deleteImageRoute = route{http.MethodDelete, mine("vms", "{uuid}", "images", "{id}")}

	networksRoute          = route{http.MethodGet, mine("vms", "{uuid}", "networks")}
	createNetworkRoute     = route{http.MethodPost, mine("vms", "{uuid}", "networks")}
	deleteNetworkRoute     = route{http.MethodDelete, mine("vms", "{uuid}", "networks", "{id}")}
	networkDependentsRoute = route{http.MethodGet, mine("vms", "{uuid}", "networks", "{id}", "dependents")}

	volumesRoute          = route{http.MethodGet, mine("vms", "{uuid}", "volumes")}
	createVolumeRoute     = route{http.MethodPost, mine("vms", "{uuid}", "volumes")}
	deleteVolumeRoute     = route{http.MethodDelete, mine("vms", "{uuid}", "volumes", "{name}")}
	volumeDependentsRoute = route{http.MethodGet, mine("vms", "{uuid}", "volumes", "{name}", "dependents")}

	stacksRoute       = route{http.MethodGet, mine("stacks")}
	stackRoute        = route{http.MethodGet, mine("stacks", "{uuid}")}
	deployStackRoute  = route{http.MethodPost, everyone("stacks")}
	startStackRoute   = route{http.MethodPost, mine("stacks", "{uuid}", "start")}
	stopStackRoute    = route{http.MethodPost, mine("stacks", "{uuid}", "stop")}
	restartStackRoute = route{http.MethodPost, mine("stacks", "{uuid}", "restart")}
	deleteStackRoute  = route{http.MethodDelete, mine("stacks", "{uuid}")}
)

// Workloads is how the frontend reaches one's own workload: VMs and their
// snapshots, and the containers, images, networks, volumes and stacks of
// Docker VMs. Everything is answered as the API answers it, a resource in
// its kind's manifest shape ({kind, metadata, spec, status}).
type Workloads struct {
	workspace *Workspace
}

// NewWorkloads is the workload service of workspace.
func NewWorkloads(workspace *Workspace) *Workloads {
	return &Workloads{workspace: workspace}
}

// VMResources are whole vCPUs, and memory and disk in bytes.
type VMResources struct {
	CPUs   uint   `json:"cpus"`
	Memory uint64 `json:"memory"`
	Disk   uint64 `json:"disk"`
}

// VMNetwork says whether anything reaches a VM's ports (ingress) and
// whether it reaches out (egress): allow or deny.
type VMNetwork struct {
	Ingress string `json:"ingress"`
	Egress  string `json:"egress"`
}

// VMRequest is a VM to create.
type VMRequest struct {
	Name string `json:"name"`

	// Kind is machine, an OS image somebody opens a terminal in, or docker,
	// which containers and stacks run in.
	Kind string `json:"kind"`

	// Image is what a machine boots; empty is its kind's default.
	Image     string      `json:"image,omitempty"`
	Resources VMResources `json:"resources"`

	// Ports are the guest ports the ingress serves.
	Ports   []uint    `json:"ports"`
	Network VMNetwork `json:"network"`

	// PersistentDisk keeps what is written to its disk across a stop and a
	// start.
	PersistentDisk bool `json:"persistent_disk"`

	// LifetimeSeconds is how long it is kept; zero keeps it until it is
	// deleted.
	LifetimeSeconds int64 `json:"lifetime_seconds"`

	// SnapshotUUID makes it from one of one's snapshots.
	SnapshotUUID string `json:"snapshot_uuid,omitempty"`
}

// NewDockerVM is a Docker VM to make for a container or a stack; what it
// leaves out is the workload's default.
type NewDockerVM struct {
	Name      string       `json:"name,omitempty"`
	Resources *VMResources `json:"resources,omitempty"`
	Ports     []uint       `json:"ports,omitempty"`
	Network   *VMNetwork   `json:"network,omitempty"`
}

// PortBinding publishes a container's port on its VM; a host port of zero
// lets docker pick one.
type PortBinding struct {
	ContainerPort uint   `json:"container_port"`
	HostPort      uint   `json:"host_port"`
	HostIP        string `json:"host_ip,omitempty"`

	// Protocol is tcp or udp; empty is tcp.
	Protocol string `json:"protocol,omitempty"`
}

// Mount is a volume of the VM, a path of the VM (bind), or a tmpfs,
// mounted into a container at an absolute path.
type Mount struct {
	Type     string `json:"type"`
	Source   string `json:"source,omitempty"`
	Target   string `json:"target"`
	ReadOnly bool   `json:"read_only,omitempty"`
}

// ContainerRequest is a container to run: in the Docker VM VMUUID names, in
// a new one VM describes, or, with neither, in a new one with the defaults.
type ContainerRequest struct {
	VMUUID string       `json:"vm_uuid,omitempty"`
	VM     *NewDockerVM `json:"vm,omitempty"`

	Name       string   `json:"name,omitempty"`
	Image      string   `json:"image"`
	Command    []string `json:"command,omitempty"`
	Entrypoint []string `json:"entrypoint,omitempty"`

	// Env is the environment, each as KEY=value.
	Env        []string      `json:"env,omitempty"`
	WorkingDir string        `json:"working_dir,omitempty"`
	Ports      []PortBinding `json:"ports,omitempty"`
	Mounts     []Mount       `json:"mounts,omitempty"`
	Networks   []string      `json:"networks,omitempty"`

	// RestartPolicy is no, always, unless-stopped or on-failure.
	RestartPolicy string `json:"restart_policy,omitempty"`

	// CPUs is in cores and Memory in bytes; zero is no limit beyond the VM's.
	CPUs       float64 `json:"cpus,omitempty"`
	Memory     uint64  `json:"memory,omitempty"`
	PidsLimit  int64   `json:"pids_limit,omitempty"`
	Privileged bool    `json:"privileged,omitempty"`
	User       string  `json:"user,omitempty"`
	Runtime    string  `json:"runtime,omitempty"`
}

// StackRequest is a stack to deploy from its compose file, in a Docker VM
// chosen as a container's is.
type StackRequest struct {
	Name    string       `json:"name"`
	Compose string       `json:"compose"`
	VMUUID  string       `json:"vm_uuid,omitempty"`
	VM      *NewDockerVM `json:"vm,omitempty"`
}

// NetworkRequest is a docker network to create in a Docker VM.
type NetworkRequest struct {
	Name     string            `json:"name"`
	Driver   string            `json:"driver,omitempty"`
	Internal bool              `json:"internal,omitempty"`
	Labels   map[string]string `json:"labels,omitempty"`
}

// VolumeRequest is a volume to create in a Docker VM.
type VolumeRequest struct {
	Name   string            `json:"name"`
	Driver string            `json:"driver,omitempty"`
	Labels map[string]string `json:"labels,omitempty"`
}

// LogsQuery narrows a log to what was written from one moment to another
// (RFC 3339, both among it), and to its last lines.
type LogsQuery struct {
	From string `json:"from,omitempty"`
	To   string `json:"to,omitempty"`
	Tail uint   `json:"tail,omitempty"`
}

// UsageQuery is the span what was used is read over (RFC 3339): no end is
// now, and no start the hour before the end.
type UsageQuery struct {
	From string `json:"from,omitempty"`
	To   string `json:"to,omitempty"`
}

// VMs are a page of one's VMs; the first page is 1.
func (w *Workloads) VMs(ctx context.Context, page uint) (json.RawMessage, error) {
	return w.send(ctx, vmsRoute, pageQuery(page, ""), nil)
}

// VM is one of one's VMs.
func (w *Workloads) VM(ctx context.Context, uuid string) (json.RawMessage, error) {
	return w.send(ctx, vmRoute.at(uuid), nil, nil)
}

// CreateVM creates a VM.
func (w *Workloads) CreateVM(ctx context.Context, request VMRequest) (json.RawMessage, error) {
	return w.send(ctx, createVMRoute, nil, request)
}

// StartVM starts one of one's VMs.
func (w *Workloads) StartVM(ctx context.Context, uuid string) error {
	return w.act(ctx, startVMRoute.at(uuid))
}

// StopVM stops one of one's VMs.
func (w *Workloads) StopVM(ctx context.Context, uuid string) error {
	return w.act(ctx, stopVMRoute.at(uuid))
}

// RestartVM restarts one of one's VMs.
func (w *Workloads) RestartVM(ctx context.Context, uuid string) error {
	return w.act(ctx, restartVMRoute.at(uuid))
}

// RestoreVM replaces the disk of one of one's VMs with a snapshot's.
func (w *Workloads) RestoreVM(ctx context.Context, uuid, snapshotUUID string) error {
	_, err := w.send(ctx, restoreVMRoute.at(uuid), nil, map[string]string{"snapshot_uuid": snapshotUUID})

	return err
}

// DeleteVM deletes one of one's VMs, and with cascade the stacks and the
// containers in it, which would otherwise make it again. It answers what
// needs it, when anything does.
func (w *Workloads) DeleteVM(ctx context.Context, uuid string, cascade bool) (json.RawMessage, error) {
	return w.send(ctx, deleteVMRoute.at(uuid), flags(map[string]bool{"cascade": cascade}), nil)
}

// VMDependents are what needs one of one's VMs, which deleting it either
// takes with it or leaves to make it again.
func (w *Workloads) VMDependents(ctx context.Context, uuid string) (json.RawMessage, error) {
	return w.send(ctx, vmDependentsRoute.at(uuid), nil, nil)
}

// VMLogs are what one of one's VMs wrote.
func (w *Workloads) VMLogs(ctx context.Context, uuid string, query LogsQuery) (json.RawMessage, error) {
	return w.send(ctx, vmLogsRoute.at(uuid), query.values(), nil)
}

// VMUsage is what one of one's VMs used over a span, a step at a time.
func (w *Workloads) VMUsage(ctx context.Context, uuid string, query UsageQuery) (json.RawMessage, error) {
	return w.send(ctx, vmUsageRoute.at(uuid), query.values(), nil)
}

// Snapshots are a page of one's snapshots, of one VM's when vm names one.
func (w *Workloads) Snapshots(ctx context.Context, page uint, vm string) (json.RawMessage, error) {
	return w.send(ctx, snapshotsRoute, pageQuery(page, vm), nil)
}

// Snapshot is one of one's snapshots.
func (w *Workloads) Snapshot(ctx context.Context, uuid string) (json.RawMessage, error) {
	return w.send(ctx, snapshotRoute.at(uuid), nil, nil)
}

// CreateSnapshot takes a snapshot of one of one's VMs, which has to be
// running or stopped.
func (w *Workloads) CreateSnapshot(ctx context.Context, vmUUID, name string) (json.RawMessage, error) {
	return w.send(ctx, createSnapshotRoute.at(vmUUID), nil, map[string]string{"name": name})
}

// RenameSnapshot renames one of one's snapshots.
func (w *Workloads) RenameSnapshot(ctx context.Context, uuid, name string) (json.RawMessage, error) {
	return w.send(ctx, renameSnapshotRoute.at(uuid), nil, map[string]string{"name": name})
}

// DeleteSnapshot deletes one of one's snapshots, and its archive.
func (w *Workloads) DeleteSnapshot(ctx context.Context, uuid string) error {
	_, err := w.send(ctx, deleteSnapshotRoute.at(uuid), nil, nil)

	return err
}

// Containers are one's containers, of one Docker VM's when vm names one.
func (w *Workloads) Containers(ctx context.Context, vm string) (json.RawMessage, error) {
	return w.send(ctx, containersRoute, pageQuery(0, vm), nil)
}

// Container is one of one's containers, in its Docker VM, by its uuid, its
// Docker id or its name.
func (w *Workloads) Container(ctx context.Context, vm, id string) (json.RawMessage, error) {
	return w.send(ctx, containerRoute.at(vm, id), nil, nil)
}

// RunContainer creates a container and starts it.
func (w *Workloads) RunContainer(ctx context.Context, request ContainerRequest) (json.RawMessage, error) {
	return w.send(ctx, runContainerRoute, nil, request)
}

// StartContainer starts one of one's containers.
func (w *Workloads) StartContainer(ctx context.Context, vm, id string) error {
	return w.act(ctx, startContainerRoute.at(vm, id))
}

// StopContainer stops one of one's containers.
func (w *Workloads) StopContainer(ctx context.Context, vm, id string) error {
	return w.act(ctx, stopContainerRoute.at(vm, id))
}

// RestartContainer restarts one of one's containers.
func (w *Workloads) RestartContainer(ctx context.Context, vm, id string) error {
	return w.act(ctx, restartContainerRoute.at(vm, id))
}

// DeleteContainer deletes one of one's containers, a running one only with
// force.
func (w *Workloads) DeleteContainer(ctx context.Context, vm, id string, force bool) error {
	_, err := w.send(ctx, deleteContainerRoute.at(vm, id), flags(map[string]bool{"force": force}), nil)

	return err
}

// ContainerLogs are what one of one's containers wrote.
func (w *Workloads) ContainerLogs(ctx context.Context, vm, id string, query LogsQuery) (json.RawMessage, error) {
	return w.send(ctx, containerLogsRoute.at(vm, id), query.values(), nil)
}

// ContainerStats is a sample of what one of one's containers uses now.
func (w *Workloads) ContainerStats(ctx context.Context, vm, id string) (json.RawMessage, error) {
	return w.send(ctx, containerStatsRoute.at(vm, id), nil, nil)
}

// ContainerUsage is what one of one's containers used over a span, a step
// at a time.
func (w *Workloads) ContainerUsage(ctx context.Context, vm, id string, query UsageQuery) (json.RawMessage, error) {
	return w.send(ctx, containerUsageRoute.at(vm, id), query.values(), nil)
}

// Images are the images of one of one's Docker VMs.
func (w *Workloads) Images(ctx context.Context, vm string) (json.RawMessage, error) {
	return w.send(ctx, imagesRoute.at(vm), nil, nil)
}

// PullImage pulls an image, as name[:tag] or name@digest, into one of one's
// Docker VMs.
func (w *Workloads) PullImage(ctx context.Context, vm, reference string) (json.RawMessage, error) {
	return w.send(ctx, pullImageRoute.at(vm), nil, map[string]string{"reference": reference})
}

// DeleteImage deletes an image of one of one's Docker VMs, one in use only
// with force.
func (w *Workloads) DeleteImage(ctx context.Context, vm, id string, force bool) error {
	_, err := w.send(ctx, deleteImageRoute.at(vm, id), flags(map[string]bool{"force": force}), nil)

	return err
}

// Networks are the docker networks of one of one's Docker VMs.
func (w *Workloads) Networks(ctx context.Context, vm string) (json.RawMessage, error) {
	return w.send(ctx, networksRoute.at(vm), nil, nil)
}

// CreateNetwork creates a docker network in one of one's Docker VMs.
func (w *Workloads) CreateNetwork(ctx context.Context, vm string, request NetworkRequest) (json.RawMessage, error) {
	return w.send(ctx, createNetworkRoute.at(vm), nil, request)
}

// DeleteNetwork deletes a docker network of one of one's Docker VMs, and
// with cascade what needs it.
func (w *Workloads) DeleteNetwork(ctx context.Context, vm, id string, cascade bool) (json.RawMessage, error) {
	return w.send(ctx, deleteNetworkRoute.at(vm, id), flags(map[string]bool{"cascade": cascade}), nil)
}

// NetworkDependents are what needs a docker network of one of one's Docker
// VMs.
func (w *Workloads) NetworkDependents(ctx context.Context, vm, id string) (json.RawMessage, error) {
	return w.send(ctx, networkDependentsRoute.at(vm, id), nil, nil)
}

// Volumes are the volumes of one of one's Docker VMs.
func (w *Workloads) Volumes(ctx context.Context, vm string) (json.RawMessage, error) {
	return w.send(ctx, volumesRoute.at(vm), nil, nil)
}

// CreateVolume creates a volume in one of one's Docker VMs.
func (w *Workloads) CreateVolume(ctx context.Context, vm string, request VolumeRequest) (json.RawMessage, error) {
	return w.send(ctx, createVolumeRoute.at(vm), nil, request)
}

// DeleteVolume deletes a volume of one of one's Docker VMs, with cascade
// what needs it, and one in use only with force.
func (w *Workloads) DeleteVolume(ctx context.Context, vm, name string, cascade, force bool) (json.RawMessage, error) {
	return w.send(ctx, deleteVolumeRoute.at(vm, name), flags(map[string]bool{"cascade": cascade, "force": force}), nil)
}

// VolumeDependents are what needs a volume of one of one's Docker VMs.
func (w *Workloads) VolumeDependents(ctx context.Context, vm, name string) (json.RawMessage, error) {
	return w.send(ctx, volumeDependentsRoute.at(vm, name), nil, nil)
}

// Stacks are a page of one's stacks.
func (w *Workloads) Stacks(ctx context.Context, page uint) (json.RawMessage, error) {
	return w.send(ctx, stacksRoute, pageQuery(page, ""), nil)
}

// Stack is one of one's stacks.
func (w *Workloads) Stack(ctx context.Context, uuid string) (json.RawMessage, error) {
	return w.send(ctx, stackRoute.at(uuid), nil, nil)
}

// DeployStack deploys a stack.
func (w *Workloads) DeployStack(ctx context.Context, request StackRequest) (json.RawMessage, error) {
	return w.send(ctx, deployStackRoute, nil, request)
}

// StartStack starts one of one's stacks.
func (w *Workloads) StartStack(ctx context.Context, uuid string) error {
	return w.act(ctx, startStackRoute.at(uuid))
}

// StopStack stops one of one's stacks.
func (w *Workloads) StopStack(ctx context.Context, uuid string) error {
	return w.act(ctx, stopStackRoute.at(uuid))
}

// RestartStack restarts one of one's stacks.
func (w *Workloads) RestartStack(ctx context.Context, uuid string) error {
	return w.act(ctx, restartStackRoute.at(uuid))
}

// DeleteStack deletes one of one's stacks: its containers and networks,
// keeping its volumes and its VM.
func (w *Workloads) DeleteStack(ctx context.Context, uuid string) error {
	_, err := w.send(ctx, deleteStackRoute.at(uuid), nil, nil)

	return err
}

// act asks r, which answers nothing.
func (w *Workloads) act(ctx context.Context, r route) error {
	_, err := w.send(ctx, r, nil, nil)

	return err
}

// send asks r with query and body, and is what it answered.
func (w *Workloads) send(ctx context.Context, r route, query url.Values, body any) (json.RawMessage, error) {
	client, err := w.workspace.Client(ctx)
	if err != nil {
		return nil, err
	}

	var answer json.RawMessage
	if err := client.Do(ctx, r.method, r.path, query, body, &answer); err != nil {
		return nil, err
	}

	return answered(answer), nil
}

func (q LogsQuery) values() url.Values {
	values := url.Values{}
	set(values, "from", q.From)
	set(values, "to", q.To)

	if q.Tail > 0 {
		values.Set("tail", strconv.FormatUint(uint64(q.Tail), 10))
	}

	return values
}

func (q UsageQuery) values() url.Values {
	values := url.Values{}
	set(values, "from", q.From)
	set(values, "to", q.To)

	return values
}

// mine is the route of one's own under the workload's: its segments as
// they are, which the client escapes.
func mine(segments ...string) string {
	return mineRoot + "/" + strings.Join(segments, "/")
}

// everyone is the route of anybody's under the workload's, which is where
// one's own things are created.
func everyone(segments ...string) string {
	return everyoneRoot + "/" + strings.Join(segments, "/")
}

func pageQuery(page uint, vm string) url.Values {
	values := url.Values{}
	if page > 0 {
		values.Set("page", strconv.FormatUint(uint64(page), 10))
	}

	set(values, "vm", vm)

	return values
}

func flags(asked map[string]bool) url.Values {
	values := url.Values{}
	for name, yes := range asked {
		if yes {
			values.Set(name, "true")
		}
	}

	return values
}

func set(values url.Values, name, value string) {
	if len(value) > 0 {
		values.Set(name, value)
	}
}
