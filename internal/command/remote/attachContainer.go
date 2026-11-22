package remote

import (
	"context"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/stream"
)

// attachContainerCommand attaches the local terminal to a running
// container, by docker attach in its Docker VM.
type attachContainerCommand struct {
	remote *remote
	flags  *console.FlagSet

	detachKeys string
	noStdin    bool
	sigProxy   bool
}

var _ console.Command = (*attachContainerCommand)(nil)

func newAttachContainerCommand(r *remote) *attachContainerCommand {
	return &attachContainerCommand{remote: r}
}

func (c *attachContainerCommand) Name() string {
	return "attach"
}

func (c *attachContainerCommand) Description() string {
	return "Attach local standard input, output, and error streams to a running container"
}

func (c *attachContainerCommand) Usage() string {
	return "attach [OPTIONS] CONTAINER"
}

func (c *attachContainerCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.detachKeys, console.Long("detach-keys"), "Override the key sequence for detaching a container")
	console.Var(flags, &c.noStdin, console.Long("no-stdin"), "Do not attach STDIN")
	console.Var(flags, &c.sigProxy, console.Long("sig-proxy"), "Proxy all received signals to the process", console.Default(true))
}

func (c *attachContainerCommand) Run(ctx context.Context) console.ExitStatus {
	return c.remote.cli.Exit(c.run(ctx))
}

func (c *attachContainerCommand) run(ctx context.Context) error {
	if err := command.ExactArgs(c.flags, 1); err != nil {
		return err
	}

	container, err := c.remote.container(ctx, c.flags.Arg(0))
	if err != nil {
		return err
	}

	docker := []string{"docker", "attach"}

	if len(c.detachKeys) > 0 {
		docker = append(docker, "--detach-keys", c.detachKeys)
	}

	if c.noStdin {
		docker = append(docker, "--no-stdin")
	}

	if !c.sigProxy {
		docker = append(docker, "--sig-proxy=false")
	}

	docker = append(docker, container.DockerID)
	tty := !c.noStdin && c.remote.terminal.IsTerminal(c.remote.cli.In)

	return c.remote.exec(ctx, container.VM.UUID, stream.ExecOptions{Command: docker, TTY: tty, Stderr: c.remote.cli.Err}, !c.noStdin)
}
