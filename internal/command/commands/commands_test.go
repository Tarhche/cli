package commands

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/danceable/console"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/command"
)

func TestNew(t *testing.T) {
	t.Parallel()

	help := func(t *testing.T, args ...string) string {
		t.Helper()

		var out bytes.Buffer
		cli := command.NewCli(strings.NewReader(""), &out, &bytes.Buffer{})

		c, err := New(cli)
		require.NoError(t, err)

		require.Equal(t, console.ExitSuccess, c.Run(context.Background(), append(append([]string{command.Name}, args...), "--help")))

		return out.String()
	}

	t.Run("the global flags", func(t *testing.T) {
		t.Parallel()

		out := help(t)

		for _, flag := range []string{"--config", "--context", "--host", "--ingress", "--language", "--debug", "--desktop", "--version"} {
			assert.Contains(t, out, flag)
		}
	})

	t.Run("every group and docker's shortcuts", func(t *testing.T) {
		t.Parallel()

		out := help(t)

		for _, name := range []string{
			"article", "comment", "user", "role", "subscription", "file", "profile", "audit-log",
			"container", "vm", "image", "network", "volume", "stack", "snapshot", "context",
			"login", "logout", "version", "info", "ps", "run", "exec", "cp", "attach", "port-forward", "logs", "images", "rmi",
		} {
			assert.Contains(t, out, "\n  "+name+" ", name)
		}
	})

	t.Run("a vm reaches into itself as a container does", func(t *testing.T) {
		t.Parallel()

		out := help(t, "vm")

		for _, name := range []string{"attach", "exec", "cp", "port-forward", "create", "ls"} {
			assert.Contains(t, out, "\n  "+name+" ", name)
		}
	})
}
