// Package workload holds the commands that manage workloads as docker's
// manage containers: VMs, containers, images, networks, volumes, stacks and
// snapshots, and the shortcuts docker has for them (ps, run, rm, ...).
package workload

import (
	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
)

// Register registers the workload groups and their shortcuts on c. The
// commands that reach into a workload are package remote's: its container
// commands go in the container group and at the top level, as docker's exec
// and cp are, and its VM commands in the vm group.
func Register(c *console.Console, cli *command.Cli) {}
