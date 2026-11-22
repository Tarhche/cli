package workload

import (
	"context"
	"errors"
	"fmt"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/workload"
)

// vmUUIDOf is the uuid of the VM reference names, or nothing when it names
// none.
func vmUUIDOf(ctx context.Context, client *workload.Client, reference string) (string, error) {
	if len(reference) == 0 {
		return "", nil
	}

	vm, err := client.FindVM(ctx, reference)
	if err != nil {
		return "", err
	}

	return vm.Metadata.UUID, nil
}

// containersOf are the containers references are looked for among: those
// of the VM vm names, or of every Docker VM.
func containersOf(ctx context.Context, client *workload.Client, vm string) ([]workload.ContainerManifest, error) {
	vmUUID, err := vmUUIDOf(ctx, client, vm)
	if err != nil {
		return nil, err
	}

	return client.Containers(ctx, vmUUID)
}

// actOnContainers acts on the container each of references names, among
// those of the VM vm names or of every Docker VM, as docker does: each in
// turn, saying what failed on its own.
func actOnContainers(ctx context.Context, cli *command.Cli, allUsers bool, vm string, references []string, act func(client *workload.Client, container workload.ContainerManifest) error) error {
	client, err := clientOf(ctx, cli, allUsers)
	if err != nil {
		return err
	}

	containers, err := containersOf(ctx, client, vm)
	if err != nil {
		return err
	}

	return eachReference(cli, references, func(reference string) error {
		container, err := workload.MatchContainer(containers, reference)
		if err != nil {
			return err
		}

		return act(client, container)
	})
}

// actOnVMs acts on the VM each of references names, as docker does.
func actOnVMs(ctx context.Context, cli *command.Cli, allUsers bool, references []string, act func(client *workload.Client, vm workload.VMManifest) error) error {
	client, err := clientOf(ctx, cli, allUsers)
	if err != nil {
		return err
	}

	vms, err := client.VMs(ctx)
	if err != nil {
		return err
	}

	return eachReference(cli, references, func(reference string) error {
		vm, err := workload.MatchVM(vms, reference)
		if err != nil {
			return err
		}

		return act(client, vm)
	})
}

// actOnStacks acts on the stack each of references names, as docker does.
func actOnStacks(ctx context.Context, cli *command.Cli, allUsers bool, references []string, act func(client *workload.Client, stack workload.StackManifest) error) error {
	client, err := clientOf(ctx, cli, allUsers)
	if err != nil {
		return err
	}

	stacks, err := client.Stacks(ctx, "")
	if err != nil {
		return err
	}

	return eachReference(cli, references, func(reference string) error {
		stack, err := workload.MatchStack(stacks, reference)
		if err != nil {
			return err
		}

		return act(client, stack)
	})
}

// dockerVMsOf are the Docker VMs what lives in them is looked for in: the
// one reference names, or every one.
func dockerVMsOf(ctx context.Context, client *workload.Client, reference string) ([]workload.VMManifest, error) {
	if len(reference) == 0 {
		return client.DockerVMs(ctx)
	}

	vm, err := client.FindVM(ctx, reference)
	if err != nil {
		return nil, err
	}

	return []workload.VMManifest{vm}, nil
}

// errVMRequired is a Docker VM that has to be named.
var errVMRequired = errors.New("name the Docker VM with --vm")

// dockerVMOf is the Docker VM something is made in: the one reference
// names, or, when it names none, one's only Docker VM.
func dockerVMOf(ctx context.Context, client *workload.Client, reference string) (workload.VMManifest, error) {
	if len(reference) > 0 {
		return client.FindVM(ctx, reference)
	}

	if client.Audience() == workload.Everyone {
		return workload.VMManifest{}, fmt.Errorf("%w: anybody's are not chosen from", errVMRequired)
	}

	vms, err := client.DockerVMs(ctx)

	switch {
	case err != nil:
		return workload.VMManifest{}, err
	case len(vms) == 0:
		return workload.VMManifest{}, fmt.Errorf("you have no Docker VM: make one with %q, or %w", command.Name+" vm create --docker", errVMRequired)
	case len(vms) > 1:
		return workload.VMManifest{}, fmt.Errorf("you have %d Docker VMs: %w", len(vms), errVMRequired)
	}

	return vms[0], nil
}
