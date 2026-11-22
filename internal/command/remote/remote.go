// Package remote holds the commands that reach into a workload over the
// workload ingress: a terminal (attach), a command (exec), files copied in
// and out (cp), and ports forwarded to the host (port-forward).
//
// A VM is reached directly; a container through its Docker VM, where the
// commands run docker exec, docker attach, docker cp and nc for it. One's
// own are reached by default, and anybody's with -A, under the permissions
// to reach anybody's; a stream refused for the roles says which permission
// it needs.
package remote

import (
	"context"
	"os"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/access"
	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/portforward"
	"github.com/Tarhche/cli/internal/reach"
	"github.com/Tarhche/cli/internal/stream"
	"github.com/Tarhche/cli/internal/workload"
)

// remote is what the commands reaching into a workload share: the CLI, how
// they find what a reference names among whose workloads, the local
// terminal they show a remote one on, and the local environment -e reads.
type remote struct {
	cli        *command.Cli
	resolver   func(workload.Audience) workload.Resolver
	terminal   terminal
	lookupEnv  func(string) (string, bool)
	forwarding []portforward.Option
}

// Option configures the commands.
type Option func(*remote)

// WithResolver finds what references name through what resolver gives for
// whose workloads they are found among, instead of asking the API.
func WithResolver(resolver func(workload.Audience) workload.Resolver) Option {
	return func(r *remote) { r.resolver = resolver }
}

func newRemote(cli *command.Cli, options ...Option) *remote {
	r := &remote{cli: cli, terminal: localTerminal{}, lookupEnv: os.LookupEnv}

	for _, option := range options {
		option(r)
	}

	return r
}

// ContainerCommands are the commands that reach into a container. Each call
// is a fresh set, so that they can be registered in more than one place.
func ContainerCommands(cli *command.Cli, options ...Option) []console.Command {
	r := newRemote(cli, options...)

	return []console.Command{
		newAttachContainerCommand(r),
		newExecContainerCommand(r),
		newCopyContainerCommand(r),
		newForwardContainerCommand(r),
	}
}

// VMCommands are the commands that reach into a VM. Each call is a fresh
// set.
func VMCommands(cli *command.Cli, options ...Option) []console.Command {
	r := newRemote(cli, options...)

	return []console.Command{
		newAttachVMCommand(r),
		newExecVMCommand(r),
		newCopyVMCommand(r),
		newForwardVMCommand(r),
	}
}

// resolve is the resolver references are found by among the workloads of
// audience: one's own, or anybody's.
func (r *remote) resolve(ctx context.Context, audience workload.Audience) (workload.Resolver, error) {
	if r.resolver != nil {
		return r.resolver(audience), nil
	}

	client, err := r.cli.Client(ctx)
	if err != nil {
		return nil, err
	}

	return workload.NewClient(client, audience).Resolver(), nil
}

// vm is the VM reference names among the workloads of audience.
func (r *remote) vm(ctx context.Context, audience workload.Audience, reference string) (workload.VM, error) {
	resolver, err := r.resolve(ctx, audience)
	if err != nil {
		return workload.VM{}, err
	}

	return resolver.VM(ctx, reference)
}

// container is the container reference names among the workloads of
// audience.
func (r *remote) container(ctx context.Context, audience workload.Audience, reference string) (workload.Container, error) {
	resolver, err := r.resolve(ctx, audience)
	if err != nil {
		return workload.Container{}, err
	}

	return resolver.Container(ctx, reference)
}

// dialer opens streams on the current context's ingress, signed in, on the
// VMs of audience: a stream refused for the roles is the permission it needs
// on them, denied.
func (r *remote) dialer(audience workload.Audience) (*stream.Dialer, error) {
	return reach.Dialer(r.cli, stream.WithExplainer(access.NewStreamExplainer(audience)))
}
