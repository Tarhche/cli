package workload

import (
	"cmp"
	"context"
	"net/http"
	"time"
)

// NetworkManifest is a docker network in a Docker VM, in its kind's
// manifest shape.
type NetworkManifest struct {
	Kind     string        `json:"kind"`
	Metadata Metadata      `json:"metadata"`
	Spec     NetworkSpec   `json:"spec"`
	Status   NetworkStatus `json:"status"`

	kept
}

// UnmarshalJSON reads a network, keeping it as it was answered.
func (m *NetworkManifest) UnmarshalJSON(data []byte) error {
	type plain NetworkManifest

	k, err := keep(data, (*plain)(m))
	m.kept = k

	return err
}

// NetworkSpec is what a network was asked to be.
type NetworkSpec struct {
	Name     string            `json:"name"`
	Driver   string            `json:"driver,omitempty"`
	Internal bool              `json:"internal,omitempty"`
	Labels   map[string]string `json:"labels,omitempty"`
}

// NetworkStatus is what a network is doing: in its kind's words, and as its
// VM's dockerd last said, once it is made.
type NetworkStatus struct {
	Status
	NetworkDocker

	Failure *Failure `json:"failure,omitempty"`
}

// NetworkDocker is a network as its VM's dockerd last had it.
type NetworkDocker struct {
	ID         string            `json:"id,omitempty"`
	Name       string            `json:"name,omitempty"`
	Driver     string            `json:"driver,omitempty"`
	Scope      string            `json:"scope,omitempty"`
	Internal   bool              `json:"internal,omitempty"`
	Containers []string          `json:"containers,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
	CreatedAt  time.Time         `json:"created_at,omitzero"`
}

// DockerName is what the network is called: by its VM's dockerd, else as it
// was asked for.
func (m NetworkManifest) DockerName() string {
	return cmp.Or(m.Status.Name, m.Spec.Name, m.Metadata.Name)
}

// CreateNetworkRequest is a docker network to create in a Docker VM.
type CreateNetworkRequest struct {
	Name string `json:"name"`

	// Driver is bridge, which is also what none is.
	Driver string `json:"driver,omitempty"`

	// Internal keeps what is on it from reaching anything beyond it.
	Internal bool              `json:"internal,omitempty"`
	Labels   map[string]string `json:"labels,omitempty"`
}

// Networks are the networks of the Docker VM of vmUUID.
func (c *Client) Networks(ctx context.Context, vmUUID string) ([]NetworkManifest, error) {
	return items[NetworkManifest](ctx, c, c.path("vms", vmUUID, "networks"), nil)
}

// CreateNetwork creates a network in the Docker VM of vmUUID.
func (c *Client) CreateNetwork(ctx context.Context, vmUUID string, request CreateNetworkRequest) (NetworkManifest, error) {
	var network NetworkManifest
	if err := c.post(ctx, c.path("vms", vmUUID, "networks"), request, &network); err != nil {
		return NetworkManifest{}, err
	}

	return network, nil
}

// DeleteNetwork removes the network id names, its uuid, its id or its name,
// from the Docker VM of vmUUID, and first, when cascade says so, what needs
// it. What needs it is what it is answered with.
func (c *Client) DeleteNetwork(ctx context.Context, vmUUID string, id string, cascade bool) (Removal, error) {
	var removal Removal
	if err := c.api.Do(ctx, http.MethodDelete, c.path("vms", vmUUID, "networks", id), flags(map[string]bool{"cascade": cascade}), nil, &removal); err != nil {
		return Removal{}, err
	}

	return removal, nil
}

// NetworkDependents are what needs a network, as its delete would be
// answered with them now.
func (c *Client) NetworkDependents(ctx context.Context, vmUUID string, id string) (Removal, error) {
	var removal Removal
	if err := c.get(ctx, c.path("vms", vmUUID, "networks", id, "dependents"), nil, &removal); err != nil {
		return Removal{}, err
	}

	return removal, nil
}
