package system

import (
	"bytes"
	"context"
	"testing"

	"github.com/danceable/console"
	"github.com/stretchr/testify/assert"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/command/commandtest"
)

func TestRegister(t *testing.T) {
	t.Parallel()

	commands := [][]string{
		{"login"},
		{"logout"},
		{"version"},
		{"upgrade"},
		{"info"},
		{"context"},
		{"context", "ls"},
		{"context", "list"},
		{"context", "create"},
		{"context", "update"},
		{"context", "use"},
		{"context", "rm"},
		{"context", "remove"},
		{"context", "inspect"},
		{"context", "show"},
	}

	for _, path := range commands {
		t.Run("registers "+path[len(path)-1]+" with a help of its own", func(t *testing.T) {
			t.Parallel()

			h := commandtest.New(t, nil)

			var out, errOut bytes.Buffer
			c := console.New(command.Name, "", &out, &errOut)
			Register(c, h.Cli)

			arguments := append(append([]string{command.Name}, path...), "--help")

			assert.Equal(t, console.ExitSuccess, c.Run(context.Background(), arguments), errOut.String())
			assert.Contains(t, out.String(), "Usage:")
		})
	}

	t.Run("runs what it registers", func(t *testing.T) {
		t.Parallel()

		h := commandtest.New(t, nil)

		c := console.New(command.Name, "", h.Out, h.Err)
		Register(c, h.Cli)

		assert.Equal(t, console.ExitSuccess, c.Run(context.Background(), []string{command.Name, "context", "show"}))
		assert.Equal(t, "test\n", h.Out.String())
	})
}
