package workload

import (
	"context"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/workload"
)

// removeImagesCommand removes images from the Docker VMs that hold them.
type removeImagesCommand struct {
	cli   *command.Cli
	flags *console.FlagSet

	force    bool
	vm       string
	allUsers bool
}

func newRemoveImagesCommand(cli *command.Cli) *removeImagesCommand {
	return &removeImagesCommand{cli: cli}
}

func (c *removeImagesCommand) Name() string {
	return "rm"
}

func (c *removeImagesCommand) Description() string {
	return "Remove one or more images"
}

func (c *removeImagesCommand) Usage() string {
	return command.Name + " image rm [OPTIONS] IMAGE [IMAGE...]"
}

func (c *removeImagesCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.force, console.Long("force"), "removes an image a container uses too.", console.Short("f"))
	console.Var(flags, &c.vm, console.Long("vm"), "looks for the images in this Docker VM alone.")
	defineAllUsers(flags, &c.allUsers)
}

func (c *removeImagesCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *removeImagesCommand) run(ctx context.Context) error {
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

	return eachReference(c.cli, c.flags.Args(), func(reference string) error {
		image, err := workload.MatchImage(images, reference)
		if err != nil {
			return err
		}

		return client.DeleteImage(ctx, image.VM.Metadata.UUID, image.Item.Metadata.UUID, c.force)
	})
}
