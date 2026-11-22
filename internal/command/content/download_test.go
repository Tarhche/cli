package content

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/danceable/console"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/command"
)

func TestDownloadCommand(t *testing.T) {
	t.Parallel()

	files := func() *fakeAPI {
		return newFakeAPI(map[string][]reply{"GET /files/f1": answer("\x89PNG bytes")})
	}

	t.Run("to the standard output, as it is", func(t *testing.T) {
		t.Parallel()

		api := files()
		h := harness(t, api)

		status := h.Run(newDownloadCommand(h.Cli, "tarhche file"), "f1")

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.Equal(t, []string{"GET /files/f1"}, api.routes())
		assert.Equal(t, "*/*", api.requests()[0].Header.Get("Accept"))
		assert.Equal(t, "\x89PNG bytes", h.Out.String())
	})

	t.Run("to the standard output when asked for -", func(t *testing.T) {
		t.Parallel()

		h := harness(t, files())

		status := h.Run(newDownloadCommand(h.Cli, "tarhche file"), "-o", "-", "f1")

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.Equal(t, "\x89PNG bytes", h.Out.String())
	})

	t.Run("to a file", func(t *testing.T) {
		t.Parallel()

		h := harness(t, files())
		path := filepath.Join(t.TempDir(), "cat.png")

		status := h.Run(newDownloadCommand(h.Cli, "tarhche file"), "--output", path, "f1")

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.Empty(t, h.Out.String())

		content, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, "\x89PNG bytes", string(content))
	})

	t.Run("a file that cannot be created", func(t *testing.T) {
		t.Parallel()

		h := harness(t, files())

		status := h.Run(newDownloadCommand(h.Cli, "tarhche file"), "-o", filepath.Join(t.TempDir(), "missing", "cat.png"), "f1")

		assert.Equal(t, console.ExitFailure, status)
		assert.Contains(t, h.Err.String(), "no such file or directory")
	})

	t.Run("no file is left of what could not be read whole", func(t *testing.T) {
		t.Parallel()

		h := harness(t, http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			rw.Header().Set("Content-Length", "100")
			_, _ = rw.Write([]byte("part"))
		}))
		path := filepath.Join(t.TempDir(), "cat.png")

		status := h.Run(newDownloadCommand(h.Cli, "tarhche file"), "-o", path, "f1")

		assert.Equal(t, console.ExitFailure, status)
		assert.Contains(t, h.Err.String(), "writing "+path)
		assert.NoFileExists(t, path)
	})

	t.Run("a file that is not there", func(t *testing.T) {
		t.Parallel()

		h := harness(t, files())

		status := h.Run(newDownloadCommand(h.Cli, "tarhche file"), "f9")

		assert.Equal(t, console.ExitFailure, status)
		assert.Equal(t, "Error: no such file: f9\n", h.Err.String())
	})

	for name, args := range map[string][]string{
		"no uuid":          nil,
		"two uuids":        {"f1", "f2"},
		"a uuid it is not": {"a/b"},
	} {
		t.Run(name+" is a usage error", func(t *testing.T) {
			t.Parallel()

			api := files()
			h := harness(t, api)

			assert.Equal(t, console.ExitUsageError, h.Run(newDownloadCommand(h.Cli, "tarhche file"), args...))
			assert.Empty(t, api.routes())
		})
	}

	t.Run("its name and what it does", func(t *testing.T) {
		t.Parallel()

		download := newDownloadCommand(nil, "tarhche file")

		assert.Equal(t, "download", download.Name())
		assert.Equal(t, "Download a file to the standard output, or to a file", download.Description())
		assert.Equal(t, "tarhche file download [OPTIONS] UUID", download.Usage())
	})

	t.Run("without a client", func(t *testing.T) {
		t.Parallel()

		noClient(t, func(cli *command.Cli) console.Command { return newDownloadCommand(cli, "tarhche file") }, "f1")
	})
}
