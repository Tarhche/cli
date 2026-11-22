package workload

import (
	"cmp"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/Tarhche/cli/internal/workload"
)

// volumeTable is docker's volume ls, and the Docker VM each volume is in.
const volumeTable = `{{.Driver}}\t{{.Name}}\t{{.VM}}`

// volumeHeaders name the columns docker names otherwise.
var volumeHeaders = map[string]string{"Name": "VOLUME NAME"}

// volumeRow is a volume as docker's volume ls lists one.
type volumeRow struct {
	Driver     string
	Name       string
	UUID       string
	Mountpoint string
	InUse      bool
	CreatedAt  time.Time
	Labels     map[string]string
	VM         string
	VMUUID     string
}

// Label is the value of the label of name, as docker's templates read one.
func (r volumeRow) Label(name string) string {
	return r.Labels[name]
}

func volumeRowOf(placed workload.Placed[workload.VolumeManifest]) volumeRow {
	volume := placed.Item

	labels := maps.Clone(volume.Spec.Labels)
	if labels == nil {
		labels = make(map[string]string)
	}

	maps.Copy(labels, volume.Status.Labels)

	return volumeRow{
		Driver:     orDefault(volume.Status.Driver, orDefault(volume.Spec.Driver, "local")),
		Name:       volume.DockerName(),
		UUID:       volume.Metadata.UUID,
		Mountpoint: volume.Status.Mountpoint,
		InUse:      volume.Status.InUse,
		CreatedAt:  volume.Status.CreatedAt,
		Labels:     labels,
		VM:         orDefault(placed.VM.Metadata.Name, placed.VM.Metadata.UUID),
		VMUUID:     placed.VM.Metadata.UUID,
	}
}

// volumeRows are volumes as docker's volume ls lists them, by name.
func volumeRows(volumes []workload.Placed[workload.VolumeManifest]) []volumeRow {
	rows := make([]volumeRow, len(volumes))
	for i, volume := range volumes {
		rows[i] = volumeRowOf(volume)
	}

	slices.SortStableFunc(rows, func(a, b volumeRow) int {
		return cmp.Or(strings.Compare(a.Name, b.Name), strings.Compare(a.VM, b.VM))
	})

	return rows
}
