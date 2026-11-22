package workload

import (
	"context"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Tarhche/cli/internal/api"
	"github.com/Tarhche/cli/internal/formatter"
	"github.com/Tarhche/cli/internal/workload"
)

// containerTable is docker's ps, and the Docker VM each container is in.
const containerTable = `{{.ID}}\t{{.Image}}\t{{.Command}}\t{{.RunningFor}}\t{{.Status}}\t{{.Ports}}\t{{.Names}}\t{{.VM}}`

// containerHeaders name the columns docker names otherwise.
var containerHeaders = map[string]string{"ID": "CONTAINER ID", "RunningFor": "CREATED"}

// commandLength is how much of a command docker shows, unless it is asked
// not to truncate it.
const commandLength = 20

// The states a container ends in, which a follow of its log ends at.
const (
	stateCompleted = "completed"
	stateFailed    = "failed"
)

// dockerStates are the states of a container whose VM's dockerd says what
// it is doing now, in docker's words.
var dockerStates = map[string]bool{
	"running":    true,
	"starting":   true,
	"stopping":   true,
	"restarting": true,
	"stopped":    true,
	"completed":  true,
}

// exited is docker's word for a container that ran to its end, and the
// status it exited with.
var exited = regexp.MustCompile(`^Exited \((-?\d+)\)`)

// containerRow is a container as docker's ps lists one.
type containerRow struct {
	ID         string
	UUID       string
	Image      string
	Command    string
	CreatedAt  time.Time
	RunningFor string
	Ports      string
	State      string
	Status     string
	Names      string
	Networks   string
	Mounts     string
	Labels     map[string]string

	// Service is the service of the stack it is of, when it is.
	Service string

	VM     string
	VMUUID string
}

// Label is the value of the label of name, as docker's templates read one.
func (r containerRow) Label(name string) string {
	return r.Labels[name]
}

func containerRowOf(c workload.ContainerManifest, untruncated bool) containerRow {
	created := c.Status.CreatedAt
	if created.IsZero() {
		created = c.Metadata.CreatedAt
	}

	labels := maps.Clone(c.Status.Labels)
	if labels == nil {
		labels = make(map[string]string)
	}

	maps.Copy(labels, c.Metadata.Labels)

	networks, mounts := c.Spec.Networks, c.Spec.Mounts
	if made(c) {
		networks, mounts = c.Status.Networks, c.Status.Mounts
	}

	return containerRow{
		ID:         containerIDOf(c, untruncated),
		UUID:       c.Metadata.UUID,
		Image:      orDefault(c.Status.Image, c.Spec.Image),
		Command:    commandOf(c, untruncated),
		CreatedAt:  created,
		RunningFor: formatter.Ago(created),
		Ports:      portsOf(c),
		State:      c.Status.State,
		Status:     containerStatusOf(c),
		Names:      c.DockerName(),
		Networks:   strings.Join(networks, ","),
		Mounts:     mountsOf(mounts),
		Labels:     labels,
		Service:    labels[workload.LabelStackService],
		VM:         orDefault(c.VMName, c.VMUUID),
		VMUUID:     c.VMUUID,
	}
}

// made reports whether a container's VM's dockerd has made it, and so says
// what it publishes and mounts and the networks it is on.
func made(c workload.ContainerManifest) bool {
	return len(c.Status.ID) > 0
}

// bindingsOf are the ports a container publishes: as its VM's dockerd says
// once it made it, and as it was asked until then.
func bindingsOf(c workload.ContainerManifest) []workload.PortBinding {
	if made(c) {
		return c.Status.Ports
	}

	return c.Spec.Ports
}

// containerRows are containers as docker's ps lists them, the newest
// first.
func containerRows(containers []workload.ContainerManifest, untruncated bool) []containerRow {
	rows := make([]containerRow, len(containers))
	for i, c := range containers {
		rows[i] = containerRowOf(c, untruncated)
	}

	slices.SortStableFunc(rows, func(a, b containerRow) int { return b.CreatedAt.Compare(a.CreatedAt) })

	return rows
}

// containerIDOf is the Docker id of a container as docker shows it, or its
// uuid until docker has made it.
func containerIDOf(c workload.ContainerManifest, untruncated bool) string {
	if len(c.Status.ID) == 0 {
		return c.Metadata.UUID
	}

	return truncated(c.Status.ID, untruncated)
}

// fullContainerID is the Docker id of a container, or its uuid until docker
// has made it.
func fullContainerID(c workload.ContainerManifest) string {
	return orDefault(c.Status.ID, c.Metadata.UUID)
}

// commandOf is what a container runs, quoted, as docker's ps shows it.
func commandOf(c workload.ContainerManifest, untruncated bool) string {
	command := c.Status.Command
	if len(command) == 0 {
		command = strings.Join(append(slices.Clone(c.Spec.Entrypoint), c.Spec.Command...), " ")
	}

	if !untruncated {
		command = ellipsis(command, commandLength)
	}

	return strconv.Quote(command)
}

// containerStatusOf is what a container is doing: in docker's words while
// its VM's dockerd says so, else in its kind's, with why it failed.
func containerStatusOf(c workload.ContainerManifest) string {
	state := c.Status.State

	switch {
	case len(c.Status.DockerStatus) > 0 && dockerStates[state]:
		return c.Status.DockerStatus
	case state == stateFailed && c.Status.Failure != nil:
		return state + ": " + c.Status.Failure.Message
	case len(c.Status.Reason) > 0 && state != "running":
		return state + ": " + c.Status.Reason
	default:
		return state
	}
}

// portsOf are the ports a container publishes, as docker's ps shows them.
func portsOf(c workload.ContainerManifest) string {
	bindings := bindingsOf(c)

	ports := make([]string, len(bindings))
	for i, binding := range bindings {
		ports[i] = portOf(binding)
	}

	return strings.Join(ports, ", ")
}

// portOf is one published port, as docker shows it: 0.0.0.0:8080->80/tcp,
// or 80/tcp when it is published on no port of the VM.
func portOf(binding workload.PortBinding) string {
	private := privatePort(binding)
	if binding.HostPort == 0 {
		return private
	}

	return publicPort(binding) + "->" + private
}

// privatePort is the container's side of a published port: 80/tcp.
func privatePort(binding workload.PortBinding) string {
	return strconv.FormatUint(uint64(binding.ContainerPort), 10) + "/" + orDefault(binding.Protocol, "tcp")
}

// publicPort is the VM's side of a published port: 0.0.0.0:8080, or 8080
// for every address.
func publicPort(binding workload.PortBinding) string {
	port := strconv.FormatUint(uint64(binding.HostPort), 10)
	if len(binding.HostIP) == 0 {
		return port
	}

	if strings.Contains(binding.HostIP, ":") {
		return "[" + binding.HostIP + "]:" + port
	}

	return binding.HostIP + ":" + port
}

// mountsOf are what is mounted into a container, as docker's ps names them.
func mountsOf(mounts []workload.Mount) string {
	names := make([]string, len(mounts))
	for i, m := range mounts {
		names[i] = orDefault(m.Source, m.Target)
	}

	return strings.Join(names, ",")
}

// containerEnd watches a container for its end, as a follow of its log
// does: it has ended once it ran to its end, could not be made, or is gone.
type containerEnd struct {
	client *workload.Client
	vmUUID string
	uuid   string

	// last is the container as it was last read, once it ended.
	last workload.ContainerManifest
	gone bool
}

func (e *containerEnd) ended(ctx context.Context) (bool, error) {
	container, err := e.client.Container(ctx, e.vmUUID, e.uuid)

	switch {
	case api.IsNotFound(err):
		e.gone = true

		return true, nil
	case err != nil:
		return false, err
	}

	e.last = container

	return container.Status.State == stateCompleted || container.Status.State == stateFailed, nil
}

// exitStatus is the status the container ended with, as docker's run exits
// with it: a failure for one that could not be made, and the status docker
// says it exited with otherwise.
func (e *containerEnd) exitStatus() (int, error) {
	if e.gone {
		return 0, nil
	}

	if e.last.Status.State == stateFailed {
		reason := e.last.Status.Reason
		if failure := e.last.Status.Failure; failure != nil {
			reason = failure.Message
		}

		return 0, fmt.Errorf("container %s failed: %s", e.last.DockerName(), reason)
	}

	matched := exited.FindStringSubmatch(e.last.Status.DockerStatus)
	if matched == nil {
		return 0, nil
	}

	status, err := strconv.Atoi(matched[1])
	if err != nil {
		return 0, nil
	}

	return status, nil
}
