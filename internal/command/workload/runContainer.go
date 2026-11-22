package workload

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/units"
	"github.com/Tarhche/cli/internal/workload"
)

// runContainerCommand creates a container from an image and starts it, in a
// Docker VM of one's own or in one made for it, and follows what it writes
// unless it is detached.
type runContainerCommand struct {
	cli   *command.Cli
	flags *console.FlagSet

	name       string
	env        list
	envFiles   list
	publish    list
	volumes    list
	mounts     list
	tmpfs      list
	networks   list
	restart    string
	entrypoint string
	user       string
	workdir    string
	privileged bool
	cpus       float64
	memory     units.Size
	pidsLimit  int64
	runtime    string
	detach     bool

	vm    string
	newVM newVMFlags

	// lookupEnv is the environment --env KEY reads a value from.
	lookupEnv func(string) (string, bool)

	// every is how often what it writes is read again while it is followed.
	every time.Duration
}

func newRunContainerCommand(cli *command.Cli) *runContainerCommand {
	return &runContainerCommand{cli: cli, lookupEnv: os.LookupEnv, every: followEvery}
}

func (c *runContainerCommand) Name() string {
	return "run"
}

func (c *runContainerCommand) Description() string {
	return "creates and runs a new container from an image."
}

func (c *runContainerCommand) Usage() string {
	return command.Name + " container run [OPTIONS] IMAGE [COMMAND] [ARG...]"
}

func (c *runContainerCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.name, console.Long("name"), "the container's name.")
	console.Var(flags, &c.env, console.Long("env"), "sets an environment variable, KEY=value, or KEY for its value here.", console.Short("e"))
	console.Var(flags, &c.envFiles, console.Long("env-file"), "reads environment variables from a file.")
	console.Var(flags, &c.publish, console.Long("publish"), "publishes a port on the VM: [ip:][host port:]container port[/protocol].", console.Short("p"))
	console.Var(flags, &c.volumes, console.Long("volume"), "mounts a volume or a path of the VM: [source:]target[:ro|rw].", console.Short("v"))
	console.Var(flags, &c.mounts, console.Long("mount"), "mounts a volume, a path of the VM or a tmpfs: type=...,source=...,target=...[,readonly].")
	console.Var(flags, &c.tmpfs, console.Long("tmpfs"), "mounts a tmpfs at a path.")
	console.Var(flags, &c.networks, console.Long("network"), "puts it on a network of the VM besides the default one.")
	console.Var(flags, &c.restart, console.Long("restart"), "when docker restarts it: no, always, unless-stopped or on-failure.")
	console.Var(flags, &c.entrypoint, console.Long("entrypoint"), "runs this instead of the image's entrypoint.")
	console.Var(flags, &c.user, console.Long("user"), "whom it runs as: name|uid[:group|gid].", console.Short("u"))
	console.Var(flags, &c.workdir, console.Long("workdir"), "the directory it runs in.", console.Short("w"))
	console.Var(flags, &c.privileged, console.Long("privileged"), "gives it every device and capability of the VM.")
	console.Var(flags, &c.cpus, console.Long("cpus"), "the most cores it uses.")
	console.Var(flags, &c.memory, console.Long("memory"), "the most memory it uses, as 512m.", console.Short("m"))
	console.Var(flags, &c.pidsLimit, console.Long("pids-limit"), "the most processes it runs at once.")
	console.Var(flags, &c.runtime, console.Long("runtime"), "the runtime the VM's dockerd runs it with.")
	console.Var(flags, &c.detach, console.Long("detach"), "runs it in the background and prints its id.", console.Short("d"))
	console.Var(flags, &c.vm, console.Long("vm"), "the Docker VM it runs in; without it, a Docker VM is made for it.")
	c.newVM.define(flags)
}

func (c *runContainerCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *runContainerCommand) run(ctx context.Context) error {
	if err := command.MinArgs(c.flags, 1); err != nil {
		return err
	}

	request, err := c.request()
	if err != nil {
		return err
	}

	client, err := clientOf(ctx, c.cli, false)
	if err != nil {
		return err
	}

	if request.VMUUID, err = vmUUIDOf(ctx, client, c.vm); err != nil {
		return err
	}

	created, err := client.CreateContainer(ctx, request)
	if err != nil {
		return err
	}

	if err := sayMadeVM(c.cli.Err, created.VM); err != nil {
		return err
	}

	if _, err := fmt.Fprintln(c.cli.Out, fullContainerID(created.Container)); err != nil || c.detach {
		return err
	}

	return c.follow(ctx, client, created.VM.UUID, created.Container.Metadata.UUID)
}

// request is the container the flags and the arguments ask for.
func (c *runContainerCommand) request() (workload.CreateContainerRequest, error) {
	arguments := c.flags.Args()

	request := workload.CreateContainerRequest{
		VM:            c.newVM.described(),
		Name:          c.name,
		Image:         arguments[0],
		Command:       arguments[1:],
		Networks:      c.networks,
		RestartPolicy: c.restart,
		User:          c.user,
		WorkingDir:    c.workdir,
		Privileged:    c.privileged,
		CPUs:          c.cpus,
		Memory:        uint64(c.memory),
		PidsLimit:     c.pidsLimit,
		Runtime:       c.runtime,
	}

	if len(c.entrypoint) > 0 {
		request.Entrypoint = []string{c.entrypoint}
	}

	files := make([][]byte, len(c.envFiles))
	for i, path := range c.envFiles {
		content, err := os.ReadFile(path)
		if err != nil {
			return workload.CreateContainerRequest{}, err
		}

		files[i] = content
	}

	request.Env = environment(files, c.env, c.lookupEnv)

	for _, spec := range c.publish {
		binding, err := publishedOf(spec)
		if err != nil {
			return workload.CreateContainerRequest{}, usageError(c.flags, "%s", err)
		}

		request.Ports = append(request.Ports, binding)
	}

	for _, spec := range c.volumes {
		mount, err := volumeOf(spec)
		if err != nil {
			return workload.CreateContainerRequest{}, usageError(c.flags, "%s", err)
		}

		request.Mounts = append(request.Mounts, mount)
	}

	for _, spec := range c.mounts {
		mount, err := mountOf(spec)
		if err != nil {
			return workload.CreateContainerRequest{}, usageError(c.flags, "%s", err)
		}

		request.Mounts = append(request.Mounts, mount)
	}

	for _, spec := range c.tmpfs {
		request.Mounts = append(request.Mounts, tmpfsOf(spec))
	}

	return request, nil
}

// follow writes what the container writes until it ends, and exits as it
// exited, or until it is interrupted.
func (c *runContainerCommand) follow(ctx context.Context, client *workload.Client, vmUUID, uuid string) error {
	end := &containerEnd{client: client, vmUUID: vmUUID, uuid: uuid}

	follower := &logFollower{
		read: func(ctx context.Context, query workload.LogsQuery) (workload.Logs, error) {
			return client.ContainerLogs(ctx, vmUUID, uuid, query)
		},
		ended: end.ended,
		write: lineWriter(c.cli.Out, c.cli.Err, false),
		every: c.every,
	}

	if err := follower.run(ctx, workload.LogsQuery{}, false, true); err != nil {
		return err
	}

	status, err := end.exitStatus()
	if err != nil || status == 0 {
		return err
	}

	return command.StatusError{Code: status}
}
