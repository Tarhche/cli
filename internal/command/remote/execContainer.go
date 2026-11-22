package remote

import (
	"context"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/stream"
)

// execContainerCommand runs a command in a container, by docker exec in its
// Docker VM, and exits with its status.
type execContainerCommand struct {
	remote *remote
	flags  *console.FlagSet

	interactive bool
	tty         bool
	env         list
	workdir     string
	user        string
	allUsers    bool
}

var _ console.Command = (*execContainerCommand)(nil)

func newExecContainerCommand(r *remote) *execContainerCommand {
	return &execContainerCommand{remote: r}
}

func (c *execContainerCommand) Name() string {
	return "exec"
}

func (c *execContainerCommand) Description() string {
	return "Execute a command in a running container"
}

func (c *execContainerCommand) Usage() string {
	return "exec [OPTIONS] CONTAINER COMMAND [ARG...]"
}

func (c *execContainerCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.interactive, console.Long("interactive"), "Keep STDIN open even if not attached", console.Short("i"))
	console.Var(flags, &c.tty, console.Long("tty"), "Allocate a pseudo-TTY", console.Short("t"))
	flags.Var(&c.env, console.Long("env"), "Set environment variables", console.Short("e"))
	console.Var(flags, &c.workdir, console.Long("workdir"), "Working directory inside the container", console.Short("w"))
	console.Var(flags, &c.user, console.Long("user"), `Username or UID (format: "<name|uid>[:<group|gid>]")`, console.Short("u"))
	defineAllUsers(flags, &c.allUsers, "container")
}

func (c *execContainerCommand) Run(ctx context.Context) console.ExitStatus {
	return c.remote.cli.Exit(c.run(ctx))
}

func (c *execContainerCommand) run(ctx context.Context) error {
	if err := command.MinArgs(c.flags, 2); err != nil {
		return err
	}

	env, err := c.remote.environment(c.env)
	if err != nil {
		return err
	}

	if c.tty && c.interactive && !c.remote.terminal.IsTerminal(c.remote.cli.In) {
		return errNotTTY
	}

	audience := audienceOf(c.allUsers)

	container, err := c.remote.container(ctx, audience, c.flags.Arg(0))
	if err != nil {
		return err
	}

	docker := []string{"docker", "exec"}

	if c.interactive {
		docker = append(docker, "--interactive")
	}

	if c.tty {
		docker = append(docker, "--tty")
	}

	for _, variable := range env {
		docker = append(docker, "--env", variable)
	}

	if len(c.workdir) > 0 {
		docker = append(docker, "--workdir", c.workdir)
	}

	if len(c.user) > 0 {
		docker = append(docker, "--user", c.user)
	}

	docker = append(append(docker, container.DockerID), c.flags.Args()[1:]...)

	return c.remote.exec(ctx, audience, container.VM.UUID, stream.ExecOptions{Command: docker, TTY: c.tty, Stderr: c.remote.cli.Err}, c.interactive)
}
