package desktop

import (
	"context"

	"github.com/Tarhche/cli/internal/access"
	"github.com/Tarhche/cli/internal/stream"
)

// The streams into a VM, by the action the ingress opens each on.
const (
	streamAttach  = "attach"
	streamExec    = "exec"
	streamForward = "forward"
)

// Abilities say which of the workload's pages and actions the session's
// permissions allow, each kind's read off the routes the service asks it
// of, and its streams off the permissions a node opens them under: a hint of
// what to offer, as the API and the nodes decide every request.
type Abilities struct {
	VMs        KindAbilities `json:"vms"`
	Containers KindAbilities `json:"containers"`
	Images     KindAbilities `json:"images"`
	Networks   KindAbilities `json:"networks"`
	Volumes    KindAbilities `json:"volumes"`
	Stacks     KindAbilities `json:"stacks"`
	Snapshots  KindAbilities `json:"snapshots"`
}

// KindAbilities say what may be done with one kind of the workload's; what
// it does not have is never allowed.
type KindAbilities struct {
	// List is its page: listing them, and what the page needs beside, as an
	// image's, a network's and a volume's needs one's VMs listed to choose
	// the Docker VM they are in.
	List bool `json:"list"`

	// Show is reading one, which its own page is.
	Show bool `json:"show"`

	// Create is making one: a VM created, a container run, a stack
	// deployed, a snapshot taken of a VM, an image pulled, a network or a
	// volume created.
	Create bool `json:"create"`

	Start   bool `json:"start"`
	Stop    bool `json:"stop"`
	Restart bool `json:"restart"`

	// Update is changing one: a snapshot renamed.
	Update bool `json:"update"`

	// Restore is replacing a VM's disk with a snapshot's, chosen among its
	// snapshots.
	Restore bool `json:"restore"`

	// Delete is deleting one, what needs it read first where something can.
	Delete bool `json:"delete"`

	// Logs is reading what it wrote, and Usage what it used and uses.
	Logs  bool `json:"logs"`
	Usage bool `json:"usage"`

	// Attach, Exec and Forward are the streams into it: a VM's own shell, a
	// command run in it, and a connection to a port it publishes. A
	// container is reached through its Docker VM's Exec alone.
	Attach  bool `json:"attach"`
	Exec    bool `json:"exec"`
	Forward bool `json:"forward"`
}

// Abilities are which of the workload's pages and actions the session's
// permissions allow.
func (w *Workloads) Abilities(ctx context.Context) (Abilities, error) {
	granted, err := w.workspace.Granted(ctx)
	if err != nil {
		return Abilities{}, err
	}

	return abilitiesOf(granted), nil
}

// abilitiesOf are what granted allows of the workload.
func abilitiesOf(granted access.Granted) Abilities {
	may := func(routes ...route) bool {
		for _, r := range routes {
			if !granted.Allows(r.method, r.path) {
				return false
			}
		}

		return true
	}

	streams := func(action string) bool {
		return granted.Has(ownStreamPermission(stream.KindVMs, action)) || granted.Has(anybodysStreamPermission(stream.KindVMs, action))
	}

	return Abilities{
		VMs: KindAbilities{
			List:    may(vmsRoute),
			Show:    may(vmRoute),
			Create:  may(createVMRoute),
			Start:   may(startVMRoute),
			Stop:    may(stopVMRoute),
			Restart: may(restartVMRoute),
			Restore: may(restoreVMRoute, snapshotsRoute),
			Delete:  may(deleteVMRoute, vmDependentsRoute),
			Logs:    may(vmLogsRoute),
			Usage:   may(vmUsageRoute),
			Attach:  streams(streamAttach),
			Exec:    streams(streamExec),
			Forward: streams(streamForward),
		},
		Containers: KindAbilities{
			List:    may(containersRoute),
			Show:    may(containerRoute),
			Create:  may(runContainerRoute),
			Start:   may(startContainerRoute),
			Stop:    may(stopContainerRoute),
			Restart: may(restartContainerRoute),
			Delete:  may(deleteContainerRoute),
			Logs:    may(containerLogsRoute),
			Usage:   may(containerStatsRoute, containerUsageRoute),
			Exec:    streams(streamExec),
		},
		Images: KindAbilities{
			List:   may(vmsRoute, imagesRoute),
			Create: may(pullImageRoute),
			Delete: may(deleteImageRoute),
		},
		Networks: KindAbilities{
			List:   may(vmsRoute, networksRoute),
			Create: may(createNetworkRoute),
			Delete: may(deleteNetworkRoute, networkDependentsRoute),
		},
		Volumes: KindAbilities{
			List:   may(vmsRoute, volumesRoute),
			Create: may(createVolumeRoute),
			Delete: may(deleteVolumeRoute, volumeDependentsRoute),
		},
		Stacks: KindAbilities{
			List:    may(stacksRoute),
			Show:    may(stackRoute),
			Create:  may(deployStackRoute),
			Start:   may(startStackRoute),
			Stop:    may(stopStackRoute),
			Restart: may(restartStackRoute),
			Delete:  may(deleteStackRoute),
		},
		Snapshots: KindAbilities{
			List:   may(snapshotsRoute),
			Show:   may(snapshotRoute),
			Create: may(createSnapshotRoute),
			Update: may(renameSnapshotRoute),
			Delete: may(deleteSnapshotRoute),
		},
	}
}

// ownStreamPermission is the permission a node opens a stream of action
// into one's own workload of kind under: self.workload.vms.exec, say.
func ownStreamPermission(kind, action string) string {
	return "self.workload." + kind + "." + action
}

// anybodysStreamPermission is the permission a node opens a stream of action
// into anybody's workload of kind under, one's own among them.
func anybodysStreamPermission(kind, action string) string {
	return "workload." + kind + "." + action
}
