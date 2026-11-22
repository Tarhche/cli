package system

import (
	"context"
	"runtime"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/version"
)

// versionTemplate is how version shows what it shows by default.
const versionTemplate = `Client:
 Version:	{{.Client.Version}}
 Go version:	{{.Client.GoVersion}}
 OS/Arch:	{{.Client.Os}}/{{.Client.Arch}}

Context:
 Name:	{{.Context.Name}}
 API:	{{.Context.API}}
 Ingress:	{{.Context.Ingress}}
 Website:	{{.Context.Web}}
`

// versionInfo is what version shows: the CLI, and the context it talks to.
type versionInfo struct {
	Client  clientInfo  `json:"Client"`
	Context contextInfo `json:"Context"`
}

// clientInfo is the CLI as it was built.
type clientInfo struct {
	Version   string `json:"Version"`
	GoVersion string `json:"GoVersion"`
	Os        string `json:"Os"`
	Arch      string `json:"Arch"`
}

// contextInfo is the context the CLI talks to.
type contextInfo struct {
	Name    string `json:"Name"`
	API     string `json:"API"`
	Ingress string `json:"Ingress"`
	Web     string `json:"Web"`
}

// versionCommand is docker's version: what the CLI is, and what it talks to.
type versionCommand struct {
	cli   *command.Cli
	flags *console.FlagSet

	format string
}

func newVersionCommand(cli *command.Cli) *versionCommand {
	return &versionCommand{cli: cli}
}

func (c *versionCommand) Name() string {
	return "version"
}

func (c *versionCommand) Description() string {
	return "Show the Tarhche CLI version information"
}

func (c *versionCommand) Usage() string {
	return command.Name + " version [OPTIONS]"
}

func (c *versionCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.format, console.Long("format"), formatUsage, console.Short("f"))
}

func (c *versionCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *versionCommand) run(context.Context) error {
	if err := command.NoArgs(c.flags); err != nil {
		return err
	}

	current, err := c.cli.Context()
	if err != nil {
		return err
	}

	return show(c.cli.Out, versionInfo{
		Client:  clientInfo{Version: version.Version, GoVersion: runtime.Version(), Os: runtime.GOOS, Arch: runtime.GOARCH},
		Context: contextInfo{Name: current.Name, API: current.API, Ingress: current.Ingress, Web: current.Web},
	}, c.format, versionTemplate)
}
