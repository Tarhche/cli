package remote

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/stream/streamtest"
)

// dockerCopy serves docker cp as a Docker VM runs it, the container's
// filesystem being root, a directory on this machine, and tar doing the
// copying.
func dockerCopy(root string) func(*streamtest.Exec) {
	return func(e *streamtest.Exec) {
		command := e.Payload.Command
		if len(command) < 4 || command[0] != "docker" || command[1] != "cp" {
			_, _ = fmt.Fprintf(e.Stderr, "docker cannot run %q\n", command)
			_ = e.Exit(1)

			return
		}

		args := command[2:]
		follow := false

		for strings.HasPrefix(args[0], "--") {
			follow = follow || args[0] == "--follow-link"
			args = args[1:]
		}

		if args[1] != "-" {
			_, path, _ := strings.Cut(args[1], ":")
			e.Payload.Command = []string{"tar", "-x", "-f", "-", "-C", root + path}
			streamtest.Run(e)

			return
		}

		_, path, _ := strings.Cut(args[0], ":")
		host := root + path

		if _, err := os.Lstat(host); err != nil {
			_, _ = fmt.Fprintf(e.Stderr, "Error response from daemon: Could not find the file %s in container %s\n", path, api.DockerID)
			_ = e.Exit(1)

			return
		}

		if follow {
			host, _ = filepath.EvalSymlinks(host)
		}

		dir, base := filepath.Dir(filepath.Clean(host)), filepath.Base(filepath.Clean(host))
		if trimmed, found := strings.CutSuffix(host, "/."); found {
			dir, base = trimmed, "."
		}

		e.Payload.Command = []string{"tar", "-c", "-f", "-", "-C", dir, "--", base}
		streamtest.Run(e)
	}
}

func TestCopyContainerCommand(t *testing.T) {
	t.Parallel()

	needs(t, "tar")

	// setup is a container whose filesystem is a directory of this
	// machine, files made in it, beside a local directory of the same files.
	setup := func(t *testing.T) (*harnessed, string, string) {
		t.Helper()

		container, local := t.TempDir(), t.TempDir()
		files(t, container)
		files(t, local)

		ingress := &streamtest.Ingress{Exec: dockerCopy(container)}
		base, r, terminal := harness(t, ingress, "")

		return &harnessed{Harness: base, remote: r, terminal: terminal, ingress: ingress}, container, local
	}

	commands := func(t *testing.T, ingress *streamtest.Ingress) [][]string {
		t.Helper()

		var run [][]string

		for _, request := range ingress.Requests() {
			assert.Equal(t, dockerVM.UUID, request.UUID)

			run = append(run, payloadOf(t, request).Command)
		}

		return run
	}

	t.Run("a file out of the container, by docker cp in its vm", func(t *testing.T) {
		t.Parallel()

		h, _, local := setup(t)

		assert.Zero(t, h.Run(newCopyContainerCommand(h.remote), "api:/file.txt", filepath.Join(local, "copy.txt")), h.Err.String())
		assert.Equal(t, "hello", read(t, filepath.Join(local, "copy.txt")))
		assert.Equal(t, [][]string{{"docker", "cp", "abc123:/file.txt", "-"}}, commands(t, h.ingress))
	})

	t.Run("a symlink followed out of the container, named as it is", func(t *testing.T) {
		t.Parallel()

		h, _, local := setup(t)

		assert.Zero(t, h.Run(newCopyContainerCommand(h.remote), "-L", "api:/to-file", filepath.Join(local, "out")), h.Err.String())
		assert.Equal(t, "hello", read(t, filepath.Join(local, "out", "to-file")))
		assert.Equal(t, [][]string{{"docker", "cp", "--follow-link", "abc123:/to-file", "-"}}, commands(t, h.ingress))
	})

	t.Run("a directory into the container, which is looked at first by docker cp", func(t *testing.T) {
		t.Parallel()

		h, container, local := setup(t)

		assert.Zero(t, h.Run(newCopyContainerCommand(h.remote), "-a", filepath.Join(local, "dir"), "api:/out"), h.Err.String())
		assert.Equal(t, "a", read(t, filepath.Join(container, "out", "dir", "a.txt")))
		assert.Equal(t, [][]string{{"docker", "cp", "abc123:/out", "-"}, {"docker", "cp", "--archive", "-", "abc123:/out"}}, commands(t, h.ingress))
	})

	t.Run("a file into the container, to a name that is not there", func(t *testing.T) {
		t.Parallel()

		h, container, local := setup(t)

		assert.Zero(t, h.Run(newCopyContainerCommand(h.remote), filepath.Join(local, "file.txt"), "api:/out/renamed.txt"), h.Err.String())
		assert.Equal(t, "hello", read(t, filepath.Join(container, "out", "renamed.txt")))
		assert.Equal(t, [][]string{{"docker", "cp", "abc123:/out/renamed.txt", "-"}, {"docker", "cp", "-", "abc123:/out"}}, commands(t, h.ingress))
	})

	t.Run("a file that is not there is said as docker says it", func(t *testing.T) {
		t.Parallel()

		h, _, local := setup(t)

		assert.Equal(t, 1, h.Run(newCopyContainerCommand(h.remote), "api:/missing", local))
		assert.Equal(t, "Error: Error response from daemon: Could not find the file /missing in container abc123\n", h.Err.String())
	})

	t.Run("what cannot be copied is the error", func(t *testing.T) {
		t.Parallel()

		h, _, _ := setup(t)
		assert.Equal(t, 1, h.Run(newCopyContainerCommand(h.remote), "api:/a", "db:/b"))
		assert.Equal(t, "Error: copying between containers is not supported\n", h.Err.String())

		h, _, _ = setup(t)
		assert.Equal(t, 1, h.Run(newCopyContainerCommand(h.remote), "a", "b"))
		assert.Equal(t, "Error: must specify at least one container source\n", h.Err.String())

		h, _, _ = setup(t)
		assert.Equal(t, 1, h.Run(newCopyContainerCommand(h.remote), "nope:/a", "b"))
		assert.Equal(t, "Error: no such container: nope\n", h.Err.String())

		h, _, _ = setup(t)
		withoutIngress(t, h.Harness)
		assert.Equal(t, 1, h.Run(newCopyContainerCommand(h.remote), "api:/a", "b"))
		require.True(t, strings.HasPrefix(h.Err.String(), "Error: the context \"test\" names no workload ingress"))
	})

	t.Run("a source and a destination are required", func(t *testing.T) {
		t.Parallel()

		h, _, _ := setup(t)

		assert.Equal(t, 2, h.Run(newCopyContainerCommand(h.remote), "api:/a"))
	})
}
