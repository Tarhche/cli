package workload

import (
	"strconv"
	"strings"
	"time"

	"github.com/Tarhche/cli/internal/formatter"
	"github.com/Tarhche/cli/internal/workload"
)

// vmTable lists VMs: what each is, what it is given and what it publishes.
const vmTable = `{{.UUID}}\t{{.Name}}\t{{.Image}}\t{{.State}}\t{{.CPUs}}\t{{.Memory}}\t{{.Disk}}\t{{.Ports}}\t{{.RunningFor}}`

// vmHeaders name the columns that are not named after their field.
var vmHeaders = map[string]string{"RunningFor": "CREATED"}

// vmRow is a VM as a listing shows one.
type vmRow struct {
	UUID string
	Name string
	Slug string

	// Type is machine or docker, and Image what a machine boots, or docker
	// for a Docker VM, whose image is the workload's.
	Type  string
	Image string

	State      string
	CPUs       uint
	Memory     string
	Disk       string
	Ports      string
	URLs       string
	CreatedAt  time.Time
	RunningFor string
	Owner      string
}

func vmRowOf(vm workload.VMManifest) vmRow {
	kind, image := workload.KindMachine, vm.Spec.Image
	if vm.Docker {
		kind, image = workload.KindDocker, workload.KindDocker
	}

	urls := make([]string, len(vm.URLs))
	for i, u := range vm.URLs {
		urls[i] = u.URL
	}

	return vmRow{
		UUID:       vm.Metadata.UUID,
		Name:       vm.Metadata.Name,
		Slug:       vm.Metadata.Slug,
		Type:       kind,
		Image:      image,
		State:      stateOf(vm.Status.Status, nil),
		CPUs:       vm.Spec.Resources.CPUs,
		Memory:     formatter.Bytes(vm.Spec.Resources.Memory),
		Disk:       formatter.Bytes(vm.Spec.Resources.Disk),
		Ports:      joinPorts(vm.Spec.Ports),
		URLs:       strings.Join(urls, ", "),
		CreatedAt:  vm.Metadata.CreatedAt,
		RunningFor: formatter.Ago(vm.Metadata.CreatedAt),
		Owner:      ownerOf(vm.Owner, vm.Metadata.OwnerUUID),
	}
}

// joinPorts are ports, as a listing shows them.
func joinPorts(ports []uint) string {
	written := make([]string, len(ports))
	for i, port := range ports {
		written[i] = strconv.FormatUint(uint64(port), 10)
	}

	return strings.Join(written, ", ")
}

// dependentTable lists what needs something.
const dependentTable = `{{.Kind}}\t{{.UUID}}\t{{.Name}}\t{{.Recreates}}`

// dependentRow is something that needs something else, and whether it makes
// it again once it is gone.
type dependentRow struct {
	Kind      string
	UUID      string
	Name      string
	Recreates bool
}

func dependentRows(removal workload.Removal) []dependentRow {
	rows := make([]dependentRow, len(removal.Dependents))
	for i, d := range removal.Dependents {
		rows[i] = dependentRow{Kind: d.Kind, UUID: d.UUID, Name: d.Name, Recreates: d.Recreates}
	}

	return rows
}

// usageTable is what a VM used, a step at a time.
const usageTable = `{{.Time}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.MemPerc}}\t{{.DiskUsage}}\t{{.NetIO}}`

// usageHeaders name the columns docker's stats names otherwise.
var usageHeaders = map[string]string{
	"CPUPerc":   "CPU %",
	"MemUsage":  "MEM USAGE / LIMIT",
	"MemPerc":   "MEM %",
	"DiskUsage": "DISK USAGE / TOTAL",
	"NetIO":     "NET I/O",
}

// usageRow is what a VM used over a step, as docker's stats shows what a
// container uses.
type usageRow struct {
	At        time.Time
	Time      string
	CPUPerc   string
	MemUsage  string
	MemPerc   string
	DiskUsage string
	DiskPerc  string
	NetIO     string
}

func usageRowOf(point workload.UsagePoint) usageRow {
	return usageRow{
		At:        point.At,
		Time:      point.At.UTC().Format(time.RFC3339),
		CPUPerc:   percent(point.CPUPercent),
		MemUsage:  formatter.Bytes(point.MemoryUsed) + " / " + formatter.Bytes(point.MemoryLimit),
		MemPerc:   percent(share(point.MemoryUsed, point.MemoryLimit)),
		DiskUsage: formatter.Bytes(point.DiskUsed) + " / " + formatter.Bytes(point.DiskTotal),
		DiskPerc:  percent(share(point.DiskUsed, point.DiskTotal)),
		NetIO:     formatter.Bytes(point.NetworkRx) + " / " + formatter.Bytes(point.NetworkTx),
	}
}
