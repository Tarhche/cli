package workload

import (
	"cmp"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/Tarhche/cli/internal/workload"
)

// networkTable is docker's network ls, and the Docker VM each network is
// in.
const networkTable = `{{.ID}}\t{{.Name}}\t{{.Driver}}\t{{.Scope}}\t{{.VM}}`

// networkHeaders name the columns docker names otherwise.
var networkHeaders = map[string]string{"ID": "NETWORK ID"}

// networkRow is a network as docker's network ls lists one.
type networkRow struct {
	ID        string
	UUID      string
	Name      string
	Driver    string
	Scope     string
	Internal  bool
	CreatedAt time.Time
	Labels    map[string]string
	VM        string
	VMUUID    string
}

// Label is the value of the label of name, as docker's templates read one.
func (r networkRow) Label(name string) string {
	return r.Labels[name]
}

func networkRowOf(placed workload.Placed[workload.NetworkManifest], untruncated bool) networkRow {
	network := placed.Item

	id := network.Metadata.UUID
	if len(network.Status.ID) > 0 {
		id = truncated(network.Status.ID, untruncated)
	}

	labels := maps.Clone(network.Spec.Labels)
	if labels == nil {
		labels = make(map[string]string)
	}

	maps.Copy(labels, network.Status.Labels)

	return networkRow{
		ID:        id,
		UUID:      network.Metadata.UUID,
		Name:      network.DockerName(),
		Driver:    orDefault(network.Status.Driver, network.Spec.Driver),
		Scope:     network.Status.Scope,
		Internal:  network.Status.Internal || network.Spec.Internal,
		CreatedAt: network.Status.CreatedAt,
		Labels:    labels,
		VM:        orDefault(placed.VM.Metadata.Name, placed.VM.Metadata.UUID),
		VMUUID:    placed.VM.Metadata.UUID,
	}
}

// networkRows are networks as docker's network ls lists them, by name.
func networkRows(networks []workload.Placed[workload.NetworkManifest], untruncated bool) []networkRow {
	rows := make([]networkRow, len(networks))
	for i, network := range networks {
		rows[i] = networkRowOf(network, untruncated)
	}

	slices.SortStableFunc(rows, func(a, b networkRow) int {
		return cmp.Or(strings.Compare(a.Name, b.Name), strings.Compare(a.VM, b.VM))
	})

	return rows
}
