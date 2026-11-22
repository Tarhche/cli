package workload

import (
	"time"

	"github.com/Tarhche/cli/internal/formatter"
	"github.com/Tarhche/cli/internal/workload"
)

// snapshotTable lists snapshots: the VM each was taken of, what it takes to
// keep and the disk it is restored onto at least.
const snapshotTable = `{{.UUID}}\t{{.Name}}\t{{.VM}}\t{{.State}}\t{{.Size}}\t{{.Disk}}\t{{.RunningFor}}`

// snapshotHeaders name the columns that are not named after their field.
var snapshotHeaders = map[string]string{"RunningFor": "CREATED"}

// snapshotRow is a snapshot as a listing shows one.
type snapshotRow struct {
	UUID       string
	Name       string
	State      string
	Docker     bool
	Image      string
	Size       string
	Disk       string
	CreatedAt  time.Time
	RunningFor string
	VM         string
	VMUUID     string
	Owner      string
}

func snapshotRowOf(s workload.SnapshotManifest) snapshotRow {
	size, disk := "", ""

	if s.Status.Size > 0 {
		size = formatter.Bytes(s.Status.Size)
	}

	if s.Status.Disk > 0 {
		disk = formatter.Bytes(s.Status.Disk)
	}

	return snapshotRow{
		UUID:       s.Metadata.UUID,
		Name:       s.Metadata.Name,
		State:      stateOf(s.Status.Status, nil),
		Docker:     s.Docker,
		Image:      s.Status.Image,
		Size:       size,
		Disk:       disk,
		CreatedAt:  s.Metadata.CreatedAt,
		RunningFor: formatter.Ago(s.Metadata.CreatedAt),
		VM:         orDefault(s.Spec.VM.Name, s.Spec.VM.UUID),
		VMUUID:     s.Spec.VM.UUID,
		Owner:      ownerOf(s.Owner, s.Metadata.OwnerUUID),
	}
}
