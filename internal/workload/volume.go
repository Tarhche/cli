package workload

import (
	"cmp"
	"context"
	"net/http"
	"time"
)

// VolumeManifest is a docker volume in a Docker VM, in its kind's manifest
// shape.
type VolumeManifest struct {
	Kind     string       `json:"kind"`
	Metadata Metadata     `json:"metadata"`
	Spec     VolumeSpec   `json:"spec"`
	Status   VolumeStatus `json:"status"`

	kept
}

// UnmarshalJSON reads a volume, keeping it as it was answered.
func (m *VolumeManifest) UnmarshalJSON(data []byte) error {
	type plain VolumeManifest

	k, err := keep(data, (*plain)(m))
	m.kept = k

	return err
}

// VolumeSpec is what a volume was asked to be.
type VolumeSpec struct {
	Name   string            `json:"name"`
	Driver string            `json:"driver,omitempty"`
	Labels map[string]string `json:"labels,omitempty"`
}

// VolumeStatus is what a volume is doing: in its kind's words, and as its
// VM's dockerd last said, once it is made.
type VolumeStatus struct {
	Status
	VolumeDocker

	Failure *Failure `json:"failure,omitempty"`
}

// VolumeDocker is a volume as its VM's dockerd last had it.
type VolumeDocker struct {
	Name       string            `json:"name,omitempty"`
	Driver     string            `json:"driver,omitempty"`
	Mountpoint string            `json:"mountpoint,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
	InUse      bool              `json:"in_use,omitempty"`
	CreatedAt  time.Time         `json:"created_at,omitzero"`
}

// DockerName is what the volume is called, which is the only id it has.
func (m VolumeManifest) DockerName() string {
	return cmp.Or(m.Status.Name, m.Spec.Name, m.Metadata.Name)
}

// CreateVolumeRequest is a docker volume to create in a Docker VM, on its
// disk.
type CreateVolumeRequest struct {
	Name string `json:"name"`

	// Driver is local, which is also what none is.
	Driver string            `json:"driver,omitempty"`
	Labels map[string]string `json:"labels,omitempty"`
}

// Volumes are the volumes of the Docker VM of vmUUID.
func (c *Client) Volumes(ctx context.Context, vmUUID string) ([]VolumeManifest, error) {
	return items[VolumeManifest](ctx, c, c.path("vms", vmUUID, "volumes"), nil)
}

// CreateVolume creates a volume in the Docker VM of vmUUID.
func (c *Client) CreateVolume(ctx context.Context, vmUUID string, request CreateVolumeRequest) (VolumeManifest, error) {
	var volume VolumeManifest
	if err := c.post(ctx, c.path("vms", vmUUID, "volumes"), request, &volume); err != nil {
		return VolumeManifest{}, err
	}

	return volume, nil
}

// DeleteVolume removes the volume of name from the Docker VM of vmUUID: one
// a container uses only by force, and, when cascade says so, after what
// needs it. What needs it is what it is answered with.
func (c *Client) DeleteVolume(ctx context.Context, vmUUID string, name string, force bool, cascade bool) (Removal, error) {
	var removal Removal
	if err := c.api.Do(ctx, http.MethodDelete, c.path("vms", vmUUID, "volumes", name), flags(map[string]bool{"force": force, "cascade": cascade}), nil, &removal); err != nil {
		return Removal{}, err
	}

	return removal, nil
}

// VolumeDependents are what needs a volume, as its delete would be answered
// with them now.
func (c *Client) VolumeDependents(ctx context.Context, vmUUID string, name string) (Removal, error) {
	var removal Removal
	if err := c.get(ctx, c.path("vms", vmUUID, "volumes", name, "dependents"), nil, &removal); err != nil {
		return Removal{}, err
	}

	return removal, nil
}
