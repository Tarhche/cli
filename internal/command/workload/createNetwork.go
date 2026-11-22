package workload

import (
	"context"
	"fmt"
	"strings"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/workload"
)

// createNetworkCommand creates a network in a Docker VM.
type createNetworkCommand struct {
	cli   *command.Cli
	flags *console.FlagSet

	driver   string
	internal bool
	labels   list
	vm       string
	allUsers bool
}

func newCreateNetworkCommand(cli *command.Cli) *createNetworkCommand {
	return &createNetworkCommand{cli: cli}
}

func (c *createNetworkCommand) Name() string {
	return "create"
}

func (c *createNetworkCommand) Description() string {
	return "creates a network in a Docker VM."
}

func (c *createNetworkCommand) Usage() string {
	return command.Name + " network create [OPTIONS] NETWORK"
}

func (c *createNetworkCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.driver, console.Long("driver"), "the driver that manages it: bridge.", console.Short("d"))
	console.Var(flags, &c.internal, console.Long("internal"), "keeps what is on it from reaching anything beyond it.")
	console.Var(flags, &c.labels, console.Long("label"), "sets a label on it: key=value.")
	console.Var(flags, &c.vm, console.Long("vm"), "the Docker VM to create it in, which has to be named when there is more than one.")
	defineAllUsers(flags, &c.allUsers)
}

func (c *createNetworkCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *createNetworkCommand) run(ctx context.Context) error {
	if err := command.ExactArgs(c.flags, 1); err != nil {
		return err
	}

	client, err := clientOf(ctx, c.cli, c.allUsers)
	if err != nil {
		return err
	}

	vm, err := dockerVMOf(ctx, client, c.vm)
	if err != nil {
		return err
	}

	network, err := client.CreateNetwork(ctx, vm.Metadata.UUID, workload.CreateNetworkRequest{
		Name:     c.flags.Arg(0),
		Driver:   c.driver,
		Internal: c.internal,
		Labels:   labelsOf(c.labels),
	})
	if err != nil {
		return err
	}

	_, err = fmt.Fprintln(c.cli.Out, orDefault(network.Status.ID, network.Metadata.UUID))

	return err
}

// labelsOf are the labels docker's --label gives, each key=value, or a key
// alone for one with no value.
func labelsOf(given []string) map[string]string {
	if len(given) == 0 {
		return nil
	}

	labels := make(map[string]string, len(given))

	for _, label := range given {
		key, value, _ := strings.Cut(label, "=")
		labels[key] = value
	}

	return labels
}
