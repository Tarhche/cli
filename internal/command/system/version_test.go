package system

import (
	"encoding/json"
	"fmt"
	"runtime"
	"testing"

	"github.com/danceable/console"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/command/commandtest"
	"github.com/Tarhche/cli/internal/version"
)

func TestVersionCommand(t *testing.T) {
	t.Parallel()

	t.Run("shows the CLI and the context it talks to", func(t *testing.T) {
		t.Parallel()

		h := commandtest.New(t, nil)
		withWebsite(t, h, "https://www.example")

		require.Equal(t, console.ExitSuccess, h.Run(newVersionCommand(h.Cli)), h.Err.String())

		lines := []string{
			"Client:",
			fmt.Sprintf("%-20s%s", " Version:", version.Version),
			fmt.Sprintf("%-20s%s", " Go version:", runtime.Version()),
			fmt.Sprintf("%-20s%s/%s", " OS/Arch:", runtime.GOOS, runtime.GOARCH),
			"",
			"Context:",
			fmt.Sprintf("%-20s%s", " Name:", "test"),
			fmt.Sprintf("%-20s%s", " API:", h.Server.URL),
			fmt.Sprintf("%-20s%s", " Ingress:", h.Server.URL),
			fmt.Sprintf("%-20s%s", " Website:", "https://www.example"),
		}

		want := ""
		for _, line := range lines {
			want += line + "\n"
		}

		assert.Equal(t, want, h.Out.String())
	})

	t.Run("shows it as json", func(t *testing.T) {
		t.Parallel()

		h := commandtest.New(t, nil)
		withWebsite(t, h, "https://www.example")

		require.Equal(t, console.ExitSuccess, h.Run(newVersionCommand(h.Cli), "--format", "json"))

		var shown versionInfo
		require.NoError(t, json.Unmarshal(h.Out.Bytes(), &shown))
		assert.Equal(t, versionInfo{
			Client:  clientInfo{Version: version.Version, GoVersion: runtime.Version(), Os: runtime.GOOS, Arch: runtime.GOARCH},
			Context: contextInfo{Name: "test", API: h.Server.URL, Ingress: h.Server.URL, Web: "https://www.example"},
		}, shown)
	})

	t.Run("shows it through a template", func(t *testing.T) {
		t.Parallel()

		h := commandtest.New(t, nil)

		require.Equal(t, console.ExitSuccess, h.Run(newVersionCommand(h.Cli), "-f", "{{.Client.Os}} {{.Context.Name}}"))

		assert.Equal(t, runtime.GOOS+" test\n", h.Out.String())
	})

	t.Run("fails for a template that is none", func(t *testing.T) {
		t.Parallel()

		h := commandtest.New(t, nil)

		assert.Equal(t, console.ExitFailure, h.Run(newVersionCommand(h.Cli), "--format", "{{"))
	})

	t.Run("fails for a context that does not exist", func(t *testing.T) {
		t.Parallel()

		h := commandtest.New(t, nil)
		h.Cli.Globals.Context = "missing"

		assert.Equal(t, console.ExitFailure, h.Run(newVersionCommand(h.Cli)))
	})

	t.Run("refuses an argument", func(t *testing.T) {
		t.Parallel()

		h := commandtest.New(t, nil)

		assert.Equal(t, console.ExitUsageError, h.Run(newVersionCommand(h.Cli), "client"))
	})
}

func TestWriteAligned(t *testing.T) {
	t.Parallel()

	t.Run("a template that cannot be parsed is an error", func(t *testing.T) {
		t.Parallel()

		assert.Error(t, writeAligned(nil, nil, "{{"))
	})

	t.Run("a template that cannot be executed is an error", func(t *testing.T) {
		t.Parallel()

		assert.Error(t, writeAligned(&failingWriter{}, struct{}{}, "{{.Missing}}"))
	})
}

// failingWriter is a writer nothing can be written to.
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, fmt.Errorf("closed")
}
