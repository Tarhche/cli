// Package workload holds the commands that manage workloads as docker's
// manage containers: VMs, containers, images, networks, volumes, stacks and
// snapshots, and the shortcuts docker has for them (ps, run, rm, ...).
package workload

import (
	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/command/remote"
)

// Register registers the workload groups and their shortcuts on c. The
// commands that reach into a workload are package remote's: its container
// commands go in the container group and at the top level, as docker's exec
// and cp are, and its VM commands in the vm group.
func Register(c *console.Console, cli *command.Cli) {
	c.RegisterGroup(
		containerGroup(cli),
		imageGroup(cli),
		networkGroup(cli),
		volumeGroup(cli),
		stackGroup(cli),
		vmGroup(cli),
		snapshotGroup(cli),
	)

	c.Register(
		command.Alias(newListContainersCommand(cli), "ps", command.Name+" ps [OPTIONS]"),
		command.Alias(newRunContainerCommand(cli), "run", command.Name+" run [OPTIONS] IMAGE [COMMAND] [ARG...]"),
		command.Alias(newStartContainersCommand(cli), "start", command.Name+" start [OPTIONS] CONTAINER [CONTAINER...]"),
		command.Alias(newStopContainersCommand(cli), "stop", command.Name+" stop [OPTIONS] CONTAINER [CONTAINER...]"),
		command.Alias(newRestartContainersCommand(cli), "restart", command.Name+" restart [OPTIONS] CONTAINER [CONTAINER...]"),
		command.Alias(newRemoveContainersCommand(cli), "rm", command.Name+" rm [OPTIONS] CONTAINER [CONTAINER...]"),
		command.Alias(newShowContainerLogsCommand(cli), "logs", command.Name+" logs [OPTIONS] CONTAINER"),
		command.Alias(newShowContainerStatsCommand(cli), "stats", command.Name+" stats [OPTIONS] [CONTAINER...]"),
		command.Alias(newShowContainerPortsCommand(cli), "port", command.Name+" port [OPTIONS] CONTAINER [PRIVATE_PORT[/PROTO]]"),
		command.Alias(newListImagesCommand(cli), "images", command.Name+" images [OPTIONS]"),
		command.Alias(newPullImageCommand(cli), "pull", command.Name+" pull [OPTIONS] NAME[:TAG|@DIGEST]"),
		command.Alias(newRemoveImagesCommand(cli), "rmi", command.Name+" rmi [OPTIONS] IMAGE [IMAGE...]"),
		newInspectCommand(cli),
	)

	c.Register(remote.ContainerCommands(cli)...)
}

func containerGroup(cli *command.Cli) *console.Group {
	return console.NewGroup("container", "manages containers, which live in Docker VMs.").
		Register(
			newListContainersCommand(cli),
			command.Alias(newListContainersCommand(cli), "ps", command.Name+" container ps [OPTIONS]"),
			command.Alias(newListContainersCommand(cli), "list", command.Name+" container list [OPTIONS]"),
			newRunContainerCommand(cli),
			newStartContainersCommand(cli),
			newStopContainersCommand(cli),
			newRestartContainersCommand(cli),
			newRemoveContainersCommand(cli),
			newShowContainerLogsCommand(cli),
			newShowContainerStatsCommand(cli),
			newInspectContainersCommand(cli),
			newShowContainerPortsCommand(cli),
			newConnectContainerCommand(cli),
			newDisconnectContainerCommand(cli),
		).
		Register(remote.ContainerCommands(cli)...)
}

func imageGroup(cli *command.Cli) *console.Group {
	return console.NewGroup("image", "manages images, which Docker VMs hold.").
		Register(
			newListImagesCommand(cli),
			command.Alias(newListImagesCommand(cli), "list", command.Name+" image list [OPTIONS]"),
			newPullImageCommand(cli),
			newRemoveImagesCommand(cli),
			newInspectImagesCommand(cli),
		)
}

func networkGroup(cli *command.Cli) *console.Group {
	return console.NewGroup("network", "manages networks, which live in Docker VMs.").
		Register(
			newListNetworksCommand(cli),
			command.Alias(newListNetworksCommand(cli), "list", command.Name+" network list [OPTIONS]"),
			newCreateNetworkCommand(cli),
			newRemoveNetworksCommand(cli),
			newInspectNetworksCommand(cli),
			newConnectNetworkCommand(cli),
			newDisconnectNetworkCommand(cli),
		)
}

func volumeGroup(cli *command.Cli) *console.Group {
	return console.NewGroup("volume", "manages volumes, which live on the disks of Docker VMs.").
		Register(
			newListVolumesCommand(cli),
			command.Alias(newListVolumesCommand(cli), "list", command.Name+" volume list [OPTIONS]"),
			newCreateVolumeCommand(cli),
			newRemoveVolumesCommand(cli),
			newInspectVolumesCommand(cli),
		)
}

func stackGroup(cli *command.Cli) *console.Group {
	return console.NewGroup("stack", "manages stacks, services of a compose file deployed into a Docker VM.").
		Register(
			newDeployStackCommand(cli),
			newListStacksCommand(cli),
			command.Alias(newListStacksCommand(cli), "list", command.Name+" stack list [OPTIONS]"),
			newListStackContainersCommand(cli),
			newRemoveStacksCommand(cli),
			newStartStacksCommand(cli),
			newStopStacksCommand(cli),
			newRestartStacksCommand(cli),
			newInspectStacksCommand(cli),
		)
}

func vmGroup(cli *command.Cli) *console.Group {
	return console.NewGroup("vm", "manages VMs: machines, and Docker VMs, which containers and stacks run in.").
		Register(
			newListVMsCommand(cli),
			command.Alias(newListVMsCommand(cli), "list", command.Name+" vm list [OPTIONS]"),
			newCreateVMCommand(cli),
			newRemoveVMsCommand(cli),
			newStartVMsCommand(cli),
			newStopVMsCommand(cli),
			newRestartVMsCommand(cli),
			newInspectVMsCommand(cli),
			newShowVMLogsCommand(cli),
			newShowVMStatsCommand(cli),
			newUpdateVMCommand(cli),
			newRestoreVMCommand(cli),
			newShowVMDependentsCommand(cli),
		).
		Register(remote.VMCommands(cli)...)
}

func snapshotGroup(cli *command.Cli) *console.Group {
	return console.NewGroup("snapshot", "manages snapshots, VMs' disks kept to restore.").
		Register(
			newListSnapshotsCommand(cli),
			command.Alias(newListSnapshotsCommand(cli), "list", command.Name+" snapshot list [OPTIONS]"),
			newCreateSnapshotCommand(cli),
			newRemoveSnapshotsCommand(cli),
			newInspectSnapshotsCommand(cli),
			newRenameSnapshotCommand(cli),
		)
}
