package content

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/danceable/console"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/command/commandtest"
)

// uploaded is a file the uploads API was sent.
type uploaded struct {
	name    string
	content string
	length  int64
}

// uploads is an API that takes files as the API's uploads do, answering
// each with a uuid of its own, after refusing as many tokens first as it
// is told to.
type uploads struct {
	lock    sync.Mutex
	refuse  int
	files   []uploaded
	answers []string
}

func (u *uploads) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	u.lock.Lock()
	defer u.lock.Unlock()

	if r.Method != http.MethodPost || r.URL.Path != uploadRoute || r.Header.Get("Authorization") != "Bearer "+commandtest.Token {
		rw.WriteHeader(http.StatusNotFound)

		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)

		return
	}

	content, _ := io.ReadAll(file)
	u.files = append(u.files, uploaded{name: header.Filename, content: string(content), length: r.ContentLength})

	if u.refuse > 0 {
		u.refuse--
		rw.WriteHeader(http.StatusUnauthorized)

		return
	}

	answer := fmt.Sprintf(`{"uuid":"f%d","name":%q}`, len(u.files), header.Filename)
	if len(u.answers) > 0 {
		answer, u.answers = u.answers[0], u.answers[1:]
	}

	rw.WriteHeader(http.StatusCreated)
	_, _ = io.WriteString(rw, answer)
}

// filesOf are files holding contents, by name, in a directory of the test's
// own, and the paths to them in the order of names.
func filesOf(t *testing.T, contents map[string]string, names ...string) []string {
	t.Helper()

	dir := t.TempDir()
	paths := make([]string, len(names))

	for i, name := range names {
		paths[i] = filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(paths[i], []byte(contents[name]), 0o600))
	}

	return paths
}

func TestUploadCommand(t *testing.T) {
	t.Parallel()

	t.Run("each file as a form of its own, writing the uuid each was given", func(t *testing.T) {
		t.Parallel()

		api := &uploads{}
		h := harness(t, api)

		paths := filesOf(t, map[string]string{"cat.png": "meow", "notes.txt": "a note\n"}, "cat.png", "notes.txt")

		status := h.Run(newUploadCommand(h.Cli, "tarhche file"), paths...)

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		require.Len(t, api.files, 2)
		assert.Equal(t, "cat.png", api.files[0].name)
		assert.Equal(t, "meow", api.files[0].content)
		assert.Positive(t, api.files[0].length, "the form is sent with its length")
		assert.Equal(t, "notes.txt", api.files[1].name)
		assert.Equal(t, "a note\n", api.files[1].content)
		assert.Equal(t, "f1\nf2\n", h.Out.String())
	})

	t.Run("sent again, as it was, once a token is renewed", func(t *testing.T) {
		t.Parallel()

		api := &uploads{refuse: 1}
		h := harness(t, api)

		status := h.Run(newUploadCommand(h.Cli, "tarhche file"), filesOf(t, map[string]string{"a.txt": "again"}, "a.txt")...)

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		require.Len(t, api.files, 2)
		assert.Equal(t, api.files[0], api.files[1])
		assert.Equal(t, "f2\n", h.Out.String())
	})

	t.Run("what was uploaded, and an error for what was not", func(t *testing.T) {
		t.Parallel()

		api := &uploads{}
		h := harness(t, api)

		missing := filepath.Join(t.TempDir(), "missing.txt")
		paths := append([]string{missing, t.TempDir()}, filesOf(t, map[string]string{"a.txt": "a"}, "a.txt")...)

		status := h.Run(newUploadCommand(h.Cli, "tarhche file"), paths...)

		assert.Equal(t, console.ExitFailure, status)
		assert.Equal(t, "f1\n", h.Out.String())
		assert.Contains(t, h.Err.String(), missing+": ")
		assert.Contains(t, h.Err.String(), "not a regular file")
	})

	t.Run("what the api refuses", func(t *testing.T) {
		t.Parallel()

		h := harness(t, newFakeAPI(map[string][]reply{"POST " + uploadRoute: answered(http.StatusBadRequest, `{"errors":{"size":"exceeds_limit"}}`)}))

		status := h.Run(newUploadCommand(h.Cli, "tarhche file"), filesOf(t, map[string]string{"big.bin": "big"}, "big.bin")...)

		assert.Equal(t, console.ExitFailure, status)
		assert.Contains(t, h.Err.String(), "big.bin: size: exceeds_limit")
	})

	t.Run("an answer that cannot be read", func(t *testing.T) {
		t.Parallel()

		api := &uploads{answers: []string{"{"}}
		h := harness(t, api)

		status := h.Run(newUploadCommand(h.Cli, "tarhche file"), filesOf(t, map[string]string{"a.txt": "a"}, "a.txt")...)

		assert.Equal(t, console.ExitFailure, status)
		assert.Contains(t, h.Err.String(), "reading the answer")
	})

	t.Run("a file that is gone before it is sent", func(t *testing.T) {
		t.Parallel()

		path := filesOf(t, map[string]string{"a.txt": "a"}, "a.txt")[0]

		form, err := newUploadForm(path)
		require.NoError(t, err)
		require.NoError(t, os.Remove(path))

		_, err = form.open()
		assert.ErrorIs(t, err, os.ErrNotExist)
	})

	t.Run("no file is a usage error", func(t *testing.T) {
		t.Parallel()

		h := harness(t, nil)

		assert.Equal(t, console.ExitUsageError, h.Run(newUploadCommand(h.Cli, "tarhche file")))
	})

	t.Run("its name and what it does", func(t *testing.T) {
		t.Parallel()

		upload := newUploadCommand(nil, "tarhche file")

		assert.Equal(t, "upload", upload.Name())
		assert.Equal(t, "Upload one or more files", upload.Description())
		assert.Equal(t, "tarhche file upload PATH [PATH...]", upload.Usage())
	})

	t.Run("an output that cannot be written", func(t *testing.T) {
		t.Parallel()

		unwritable(t, &uploads{}, func(cli *command.Cli) console.Command {
			return newUploadCommand(cli, "tarhche file")
		}, filesOf(t, map[string]string{"a.txt": "a"}, "a.txt")...)
	})

	t.Run("without a client", func(t *testing.T) {
		t.Parallel()

		noClient(t, func(cli *command.Cli) console.Command { return newUploadCommand(cli, "tarhche file") }, "a.txt")
	})
}
