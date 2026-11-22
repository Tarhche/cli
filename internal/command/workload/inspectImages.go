package workload

import (
	"context"
	"encoding/json"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/workload"
)

// inspectImagesCommand writes images as the API answers them.
type inspectImagesCommand struct {
	cli   *command.Cli
	flags *console.FlagSet

	format   string
	vm       string
	allUsers bool
}

func newInspectImagesCommand(cli *command.Cli) *inspectImagesCommand {
	return &inspectImagesCommand{cli: cli}
}

func (c *inspectImagesCommand) Name() string {
	return "inspect"
}

func (c *inspectImagesCommand) Description() string {
	return "displays detailed information on one or more images."
}

func (c *inspectImagesCommand) Usage() string {
	return command.Name + " image inspect [OPTIONS] IMAGE [IMAGE...]"
}

func (c *inspectImagesCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.format, console.Long("format"), "formats the output with a Go template.", console.Short("f"))
	console.Var(flags, &c.vm, console.Long("vm"), "looks for the images in this Docker VM alone.")
	defineAllUsers(flags, &c.allUsers)
}

func (c *inspectImagesCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *inspectImagesCommand) run(ctx context.Context) error {
	if err := command.MinArgs(c.flags, 1); err != nil {
		return err
	}

	client, err := clientOf(ctx, c.cli, c.allUsers)
	if err != nil {
		return err
	}

	vms, err := dockerVMsOf(ctx, client, c.vm)
	if err != nil {
		return err
	}

	images, err := client.ImagesIn(ctx, vms)
	if err != nil {
		return err
	}

	return inspectEach(c.cli, c.flags.Args(), c.format, func(reference string) (json.RawMessage, error) {
		image, err := workload.MatchImage(images, reference)

		return image.Item.Raw(), err
	})
}
