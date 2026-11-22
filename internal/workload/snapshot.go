package workload

import (
	"context"
	"net/http"
	"time"

	"github.com/Tarhche/cli/internal/api"
)

// SnapshotManifest is a snapshot, a VM's disk kept as an archive, in its
// kind's manifest shape, and beside it whether it was taken of a Docker VM
// and whose it is.
type SnapshotManifest struct {
	Kind     string         `json:"kind"`
	Metadata Metadata       `json:"metadata"`
	Spec     SnapshotSpec   `json:"spec"`
	Status   SnapshotStatus `json:"status"`

	Docker bool   `json:"docker"`
	Owner  *Owner `json:"owner,omitempty"`

	kept
}

// UnmarshalJSON reads a snapshot, keeping it as it was answered.
func (m *SnapshotManifest) UnmarshalJSON(data []byte) error {
	type plain SnapshotManifest

	k, err := keep(data, (*plain)(m))
	m.kept = k

	return err
}

// SnapshotSpec is the VM a snapshot was taken of, which may be gone since.
type SnapshotSpec struct {
	VM SnapshotVM `json:"vm"`
}

// SnapshotVM is the VM a snapshot was taken of.
type SnapshotVM struct {
	UUID string `json:"uuid"`
	Name string `json:"name,omitempty"`
}

// SnapshotStatus is what a snapshot is doing, and what it took: the image,
// the disk a VM it is restored onto needs at least and what it takes to
// keep, both in bytes, and the engine that took it.
type SnapshotStatus struct {
	Status

	Image       string    `json:"image,omitempty"`
	Disk        uint64    `json:"disk,omitempty"`
	Engine      string    `json:"engine,omitempty"`
	Size        int64     `json:"size,omitempty"`
	CompletedAt time.Time `json:"completed_at,omitzero"`
}

// Snapshots are every snapshot, or those of the VM of vmUUID alone when it
// is not empty.
func (c *Client) Snapshots(ctx context.Context, vmUUID string) ([]SnapshotManifest, error) {
	return api.All[SnapshotManifest](ctx, c.api, c.path("snapshots"), narrowed(vmUUID))
}

// Snapshot is the snapshot of uuid.
func (c *Client) Snapshot(ctx context.Context, uuid string) (SnapshotManifest, error) {
	var snapshot SnapshotManifest
	if err := c.get(ctx, c.path("snapshots", uuid), nil, &snapshot); err != nil {
		return SnapshotManifest{}, err
	}

	return snapshot, nil
}

// CreateSnapshot takes a snapshot of one's own VM of vmUUID, which has to be
// running or stopped, under name.
func (c *Client) CreateSnapshot(ctx context.Context, vmUUID string, name string) (SnapshotManifest, error) {
	var snapshot SnapshotManifest
	if err := c.post(ctx, creation("vms", vmUUID, "snapshots"), map[string]string{"name": name}, &snapshot); err != nil {
		return SnapshotManifest{}, err
	}

	return snapshot, nil
}

// RenameSnapshot calls the snapshot of uuid name.
func (c *Client) RenameSnapshot(ctx context.Context, uuid string, name string) (SnapshotManifest, error) {
	var snapshot SnapshotManifest
	if err := c.api.Patch(ctx, c.path("snapshots", uuid), map[string]string{"name": name}, &snapshot); err != nil {
		return SnapshotManifest{}, err
	}

	return snapshot, nil
}

// DeleteSnapshot removes the snapshot of uuid and the archive it kept.
func (c *Client) DeleteSnapshot(ctx context.Context, uuid string) error {
	return c.act(ctx, http.MethodDelete, c.path("snapshots", uuid), nil, nil)
}
