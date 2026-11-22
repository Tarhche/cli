package workload

import (
	"encoding/json"
	"net/url"
	"slices"
	"strconv"
	"time"
)

// Metadata is what every resource of the workload has, whatever its kind.
type Metadata struct {
	UUID      string            `json:"uuid,omitempty"`
	Name      string            `json:"name,omitempty"`
	Slug      string            `json:"slug,omitempty"`
	OwnerUUID string            `json:"owner_uuid,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`

	// Owners own it, and it goes with the last of them; Refs it uses and
	// does not own, the resource it lives in among them.
	Owners []Reference `json:"owners,omitempty"`
	Refs   []Reference `json:"refs,omitempty"`

	// Node is the node holding it.
	Node string `json:"node,omitempty"`

	// Lifetime is how long it is kept; none keeps it until it is deleted.
	Lifetime            time.Duration `json:"lifetime,omitempty"`
	ExpiresAt           time.Time     `json:"expires_at,omitzero"`
	DeletionRequestedAt time.Time     `json:"deletion_requested_at,omitzero"`
	CreatedAt           time.Time     `json:"created_at,omitzero"`
	UpdatedAt           time.Time     `json:"updated_at,omitzero"`
}

// Reference names one resource of the workload, of a kind.
type Reference struct {
	Kind string `json:"kind"`
	UUID string `json:"uuid"`
}

// Status is what every resource of the workload says of itself: its state
// in its kind's words, the state it is expected to be in, why, and since
// when.
type Status struct {
	State      string    `json:"state"`
	Expected   string    `json:"expected,omitempty"`
	Reason     string    `json:"reason,omitempty"`
	Since      time.Time `json:"since,omitzero"`
	ObservedAt time.Time `json:"observed_at,omitzero"`
}

// Failure is what the last command on something failed with.
type Failure struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Owner is whom something belongs to, as anybody's listings say.
type Owner struct {
	UUID     string `json:"uuid"`
	Name     string `json:"name,omitempty"`
	Avatar   string `json:"avatar,omitempty"`
	Username string `json:"username,omitempty"`
}

// Resources are what a VM is given: whole vCPUs, and memory and disk in
// bytes.
type Resources struct {
	CPUs   uint   `json:"cpus"`
	Memory uint64 `json:"memory"`
	Disk   uint64 `json:"disk"`
}

// Network is how much of the network a VM has, allow or deny each way:
// ingress lets the ingress reach its ports, egress lets it reach the public
// internet.
type Network struct {
	Ingress string `json:"ingress,omitempty"`
	Egress  string `json:"egress,omitempty"`
}

// The ways a VM's network is open or shut.
const (
	Allow = "allow"
	Deny  = "deny"
)

// NewDockerVM is a Docker VM to make for a container or a stack. What it
// leaves out is the workload's default.
type NewDockerVM struct {
	Name      string     `json:"name,omitempty"`
	Resources *Resources `json:"resources,omitempty"`
	Ports     []uint     `json:"ports,omitempty"`
	Network   *Network   `json:"network,omitempty"`
}

// VMChoice is the Docker VM something was asked for in: one by its uuid, or
// one made for it, with what it was made with.
type VMChoice struct {
	UUID      string     `json:"uuid,omitempty"`
	Name      string     `json:"name,omitempty"`
	Resources *Resources `json:"resources,omitempty"`
	Ports     []uint     `json:"ports,omitempty"`
	Network   *Network   `json:"network,omitempty"`
}

// ChosenVM is the Docker VM something went into, and whether it was made
// for it.
type ChosenVM struct {
	UUID    string `json:"uuid"`
	Name    string `json:"name"`
	Created bool   `json:"created"`
}

// Removal is what a delete of what stacks and containers need is answered
// with, and what reading its dependents says: what needs it, and whether one
// of them makes it again once it is gone.
type Removal struct {
	Dependents []Dependent `json:"dependents"`
	Recreated  bool        `json:"recreated"`
}

// Dependent is a stack, or a container nothing owns, that needs something:
// one that makes it again whenever it is gone (Recreates), or one that only
// waits on it while it is missing.
type Dependent struct {
	Kind      string `json:"kind"`
	UUID      string `json:"uuid"`
	Name      string `json:"name,omitempty"`
	Recreates bool   `json:"recreates"`
}

// Logs are the last lines something wrote over a span, oldest first.
type Logs struct {
	Items []LogLine `json:"items"`

	// Truncated says more was written than one answer carries: what is here
	// is the last of it.
	Truncated bool `json:"truncated"`
}

// LogLine is one line something wrote: a VM's, from its kernel, its main
// process or an exec (Source), or a container's, on stdout or stderr
// (Stream).
type LogLine struct {
	At     time.Time `json:"at"`
	Source string    `json:"source,omitempty"`
	Stream string    `json:"stream,omitempty"`
	Line   string    `json:"line"`
}

// The streams a container's line is written on.
const (
	Stdout = "stdout"
	Stderr = "stderr"
)

// LogsQuery is what of a log is read: what was written from one moment to
// another, both among it, and the last Tail lines of it. A zero moment
// bounds nothing, and a zero tail is as many as one answer carries.
type LogsQuery struct {
	From time.Time
	To   time.Time
	Tail uint
}

func (q LogsQuery) values() url.Values {
	values := q.span().values()
	if q.Tail > 0 {
		values.Set("tail", strconv.FormatUint(uint64(q.Tail), 10))
	}

	return values
}

func (q LogsQuery) span() Span {
	return Span{From: q.From, To: q.To}
}

// Span is a span of time, both ends among it. A zero end is now, and a zero
// start the hour before the end.
type Span struct {
	From time.Time
	To   time.Time
}

func (s Span) values() url.Values {
	values := url.Values{}

	if !s.From.IsZero() {
		values.Set("from", s.From.Format(time.RFC3339Nano))
	}

	if !s.To.IsZero() {
		values.Set("to", s.To.Format(time.RFC3339Nano))
	}

	return values
}

// Usage is what something used over a span, a step at a time.
type Usage struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`

	// Step is how many seconds each point spans.
	Step int64 `json:"step"`

	// Points are oldest first; a step nothing was sampled in has none.
	Points []UsagePoint `json:"points"`
}

// UsagePoint is what something used over a step: how busy it kept its
// vCPUs, on average, 0 to 100 of all of them, and the most it held and had
// counted by the step's end. Sizes are bytes; a VM's says its disk and a
// container's its block devices.
type UsagePoint struct {
	At          time.Time `json:"at"`
	CPUPercent  float64   `json:"cpu_percent"`
	MemoryUsed  uint64    `json:"memory_used"`
	MemoryLimit uint64    `json:"memory_limit"`
	DiskUsed    uint64    `json:"disk_used,omitempty"`
	DiskTotal   uint64    `json:"disk_total,omitempty"`
	NetworkRx   uint64    `json:"network_rx"`
	NetworkTx   uint64    `json:"network_tx"`
	BlockRead   uint64    `json:"block_read,omitempty"`
	BlockWrite  uint64    `json:"block_write,omitempty"`
}

// Placed is something that lives in a Docker VM, and the VM it lives in.
type Placed[T any] struct {
	VM   VMManifest
	Item T
}

// kept is what a manifest keeps of what the API answered for it, so that it
// is written out as it was read, whatever this client knows of its shape.
type kept struct {
	raw json.RawMessage
}

// Raw is the manifest as the API answered it; nothing for one that was not
// read from it.
func (k kept) Raw() json.RawMessage {
	return k.raw
}

// keep reads data into into, as if it had no UnmarshalJSON of its own, and
// is what is kept of it.
func keep[T any](data []byte, into *T) (kept, error) {
	if err := json.Unmarshal(data, into); err != nil {
		return kept{}, err
	}

	return kept{raw: slices.Clone(data)}, nil
}
