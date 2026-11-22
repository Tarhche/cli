package workload

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/workload"
)

// deployStackCommand deploys a stack from a compose file into a Docker VM
// of one's own, or into one made for it.
type deployStackCommand struct {
	cli   *command.Cli
	flags *console.FlagSet

	file  string
	vm    string
	newVM newVMFlags
}

func newDeployStackCommand(cli *command.Cli) *deployStackCommand {
	return &deployStackCommand{cli: cli}
}

func (c *deployStackCommand) Name() string {
	return "deploy"
}

func (c *deployStackCommand) Description() string {
	return "Deploy a new stack from a compose file"
}

func (c *deployStackCommand) Usage() string {
	return command.Name + " stack deploy [OPTIONS] -c FILE STACK"
}

func (c *deployStackCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.file, console.Long("compose-file"), "the compose file of the stack, or - for the standard input.", console.Short("c"))
	console.Var(flags, &c.vm, console.Long("vm"), "the Docker VM it is deployed into; without it, a Docker VM is made for it.")
	c.newVM.define(flags)
}

func (c *deployStackCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *deployStackCommand) run(ctx context.Context) error {
	if err := command.ExactArgs(c.flags, 1); err != nil {
		return err
	}

	if len(c.file) == 0 {
		return usageError(c.flags, "a compose file is required: give it with -c")
	}

	compose, err := c.read()
	if err != nil {
		return err
	}

	client, err := clientOf(ctx, c.cli, false)
	if err != nil {
		return err
	}

	vmUUID, err := vmUUIDOf(ctx, client, c.vm)
	if err != nil {
		return err
	}

	created, err := client.CreateStack(ctx, workload.CreateStackRequest{
		Name:    c.flags.Arg(0),
		Compose: string(compose),
		VMUUID:  vmUUID,
		VM:      c.newVM.described(),
	})
	if err != nil {
		return err
	}

	if err := sayMadeVM(c.cli.Err, created.VM); err != nil {
		return err
	}

	_, err = fmt.Fprintln(c.cli.Out, created.Stack.Metadata.UUID)

	return err
}

// read is the compose file, from the standard input for -.
func (c *deployStackCommand) read() ([]byte, error) {
	if c.file == "-" {
		return io.ReadAll(c.cli.In)
	}

	return os.ReadFile(c.file)
}
