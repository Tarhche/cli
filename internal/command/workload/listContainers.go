package workload

import (
	"context"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/formatter"
	"github.com/Tarhche/cli/internal/workload"
)

// listContainersCommand lists containers across one's Docker VMs, as
// docker's ps lists a daemon's.
type listContainersCommand struct {
	cli   *command.Cli
	flags *console.FlagSet

	all         bool
	quiet       bool
	format      string
	untruncated bool
	vm          string
	allUsers    bool
}

func newListContainersCommand(cli *command.Cli) *listContainersCommand {
	return &listContainersCommand{cli: cli}
}

func (c *listContainersCommand) Name() string {
	return "ls"
}

func (c *listContainersCommand) Description() string {
	return "lists containers."
}

func (c *listContainersCommand) Usage() string {
	return command.Name + " container ls [OPTIONS]"
}

func (c *listContainersCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.all, console.Long("all"), "shows every container (default shows just running).", console.Short("a"))
	console.Var(flags, &c.quiet, console.Long("quiet"), "only shows container ids.", console.Short("q"))
	console.Var(flags, &c.format, console.Long("format"), "formats the output: table, json, or a Go template.")
	console.Var(flags, &c.untruncated, console.Long("no-trunc"), "does not truncate the output.")
	console.Var(flags, &c.vm, console.Long("vm"), "only shows the containers of this Docker VM.")
	defineAllUsers(flags, &c.allUsers)
}

func (c *listContainersCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *listContainersCommand) run(ctx context.Context) error {
	if err := command.NoArgs(c.flags); err != nil {
		return err
	}

	client, err := clientOf(ctx, c.cli, c.allUsers)
	if err != nil {
		return err
	}

	vmUUID := ""

	if len(c.vm) > 0 {
		vm, err := client.FindVM(ctx, c.vm)
		if err != nil {
			return err
		}

		vmUUID = vm.Metadata.UUID
	}

	containers, err := client.Containers(ctx, vmUUID)
	if err != nil {
		return err
	}

	if !c.all {
		containers = running(containers)
	}

	return writeRows(c.cli.Out, containerRows(containers, c.untruncated), formatter.Options{
		Format:  c.format,
		Table:   containerTable,
		Quiet:   c.quiet,
		ID:      "{{.ID}}",
		Headers: containerHeaders,
	})
}

// running are the containers that run.
func running(containers []workload.ContainerManifest) []workload.ContainerManifest {
	var up []workload.ContainerManifest

	for _, c := range containers {
		if c.Running() {
			up = append(up, c)
		}
	}

	return up
}
