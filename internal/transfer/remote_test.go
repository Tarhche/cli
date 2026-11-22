package transfer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// failure is a command that ran and exited with code, saying stderr.
type failure struct {
	code   int
	stderr string
}

func (f *failure) Error() string { return strings.TrimSpace(f.stderr) }
func (f *failure) ExitCode() int { return f.code }

// commands records the commands a runner was asked to run.
type commands struct {
	lock sync.Mutex
	run  [][]string
}

func (c *commands) record(command []string) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.run = append(c.run, append([]string(nil), command...))
}

func (c *commands) all() [][]string {
	c.lock.Lock()
	defer c.lock.Unlock()

	return append([][]string(nil), c.run...)
}

// localRunner runs commands on this machine, as a VM runs them: the VM's
// tar and sh are this machine's.
type localRunner struct {
	commands
}

func needs(t *testing.T, tools ...string) {
	t.Helper()

	for _, tool := range tools {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is not on this machine", tool)
		}
	}
}

func (r *localRunner) Run(ctx context.Context, command []string, stdin io.Reader, stdout io.Writer) error {
	r.record(command)

	var stderr bytes.Buffer

	process := exec.CommandContext(ctx, command[0], command[1:]...)
	process.Stdin, process.Stdout, process.Stderr = stdin, stdout, &stderr

	err := process.Run()

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return &failure{code: exitErr.ExitCode(), stderr: stderr.String()}
	}

	return err
}

// scriptedRunner answers every command as its answer says.
type scriptedRunner struct {
	commands
	answer func(command []string, stdin io.Reader, stdout io.Writer) error
}

func (r *scriptedRunner) Run(_ context.Context, command []string, stdin io.Reader, stdout io.Writer) error {
	r.record(command)

	return r.answer(command, stdin, stdout)
}

// dockerRunner runs docker cp as a Docker VM runs it, the container's
// filesystem being root, a directory on this machine.
type dockerRunner struct {
	commands
	root  string
	local localRunner
}

func (r *dockerRunner) Run(ctx context.Context, command []string, stdin io.Reader, stdout io.Writer) error {
	r.record(command)

	if len(command) < 4 || command[0] != "docker" || command[1] != "cp" {
		return fmt.Errorf("docker cannot run %q", command)
	}

	args := command[2:]
	follow, owners := false, false

	for strings.HasPrefix(args[0], "--") {
		follow = follow || args[0] == "--follow-link"
		owners = owners || args[0] == "--archive"
		args = args[1:]
	}

	if args[1] == Stdio {
		_, path, _ := strings.Cut(args[0], ":")
		host := r.root + fromRoot(path)

		if _, err := os.Lstat(host); err != nil {
			return &failure{code: 1, stderr: "Error response from daemon: Could not find the file " + path + " in container"}
		}

		if follow {
			resolved, err := filepath.EvalSymlinks(host)
			if err != nil {
				return &failure{code: 1, stderr: err.Error()}
			}

			host = resolved
		}

		dir, base := localPaths.split(host)

		return r.local.Run(ctx, []string{"tar", "-c", "-f", "-", "-C", dir, "--", base}, nil, stdout)
	}

	_, path, _ := strings.Cut(args[1], ":")

	extract := []string{"tar", "-x", "-f", "-", "-C", r.root + fromRoot(path)}
	if !owners {
		extract = append(extract, "--no-same-owner")
	}

	return r.local.Run(ctx, extract, stdin, io.Discard)
}

func TestVM(t *testing.T) {
	t.Parallel()

	needs(t, "sh", "tar", "readlink")

	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "dir", "sub"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "dir", "a.txt"), []byte("a"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "file"), []byte("f"), 0o644))
	require.NoError(t, os.Symlink("file", filepath.Join(root, "to-file")))
	require.NoError(t, os.Symlink(filepath.Join(root, "dir"), filepath.Join(root, "to-dir")))
	require.NoError(t, os.Symlink("loop", filepath.Join(root, "loop")))

	join := func(name string) string { return filepath.Join(root, name) }

	t.Run("what a path is, as the shell finds it", func(t *testing.T) {
		t.Parallel()

		remote := VM(&localRunner{})

		cases := map[string]Info{
			join("dir"):                           {Exists: true, IsDir: true},
			join("dir") + "/":                     {Exists: true, IsDir: true},
			join("file"):                          {Exists: true},
			join("missing"):                       {},
			join("to-file"):                       {Exists: true, Link: "file"},
			join("to-dir"):                        {Exists: true, Link: join("dir")},
			strings.TrimPrefix(join("file"), "/"): {Exists: true},
		}

		for path, want := range cases {
			got, err := remote.Stat(context.Background(), path)
			require.NoError(t, err, path)
			assert.Equal(t, want, got, path)
		}
	})

	t.Run("what the shell cannot say is an error", func(t *testing.T) {
		t.Parallel()

		broken := errors.New("the vm went away")

		_, err := VM(&scriptedRunner{answer: func([]string, io.Reader, io.Writer) error { return broken }}).Stat(context.Background(), "/x")
		assert.ErrorIs(t, err, broken)

		_, err = VM(&scriptedRunner{answer: func(_ []string, _ io.Reader, stdout io.Writer) error {
			_, _ = io.WriteString(stdout, "socket\n")

			return nil
		}}).Stat(context.Background(), "/x")
		assert.EqualError(t, err, `cannot tell what /x is from "socket\n"`)
	})

	t.Run("an archive is made by tar, of what is found there", func(t *testing.T) {
		t.Parallel()

		runner := &localRunner{}

		var archive bytes.Buffer
		require.NoError(t, VM(runner).Archive(context.Background(), join("dir"), false, &archive))

		assert.Equal(t, []string{"dir/", "dir/a.txt: a", "dir/sub/"}, sortedEntries(t, archive.Bytes()))
		assert.Equal(t, []string{"tar", "-c", "-f", "-", "-C", root, "--", "dir"}, runner.all()[1])
	})

	t.Run("a symlink is archived as it is, or what it points to when it is followed", func(t *testing.T) {
		t.Parallel()

		var link, followed bytes.Buffer
		require.NoError(t, VM(&localRunner{}).Archive(context.Background(), join("to-file"), false, &link))
		require.NoError(t, VM(&localRunner{}).Archive(context.Background(), join("to-file"), true, &followed))

		assert.Equal(t, []string{"to-file -> file"}, sortedEntries(t, link.Bytes()))
		assert.Equal(t, []string{"file: f"}, sortedEntries(t, followed.Bytes()))
	})

	t.Run("a directory's contents are archived under the dot", func(t *testing.T) {
		t.Parallel()

		var archive bytes.Buffer
		require.NoError(t, VM(&localRunner{}).Archive(context.Background(), join("dir")+"/.", false, &archive))

		assert.Equal(t, []string{"./", "./a.txt: a", "./sub/"}, sortedEntries(t, archive.Bytes()))
	})

	t.Run("the root is archived as its contents", func(t *testing.T) {
		t.Parallel()

		runner := &scriptedRunner{answer: func(command []string, _ io.Reader, stdout io.Writer) error {
			if command[0] == "sh" {
				_, _ = io.WriteString(stdout, "directory\n")
			}

			return nil
		}}

		require.NoError(t, VM(runner).Archive(context.Background(), "/", false, io.Discard))
		assert.Equal(t, []string{"tar", "-c", "-f", "-", "-C", "/", "--", "."}, runner.all()[1])
	})

	t.Run("what is not there is not found, and a symlink that never ends is an error", func(t *testing.T) {
		t.Parallel()

		err := VM(&localRunner{}).Archive(context.Background(), join("missing"), false, io.Discard)

		var notFound *NotFoundError
		require.ErrorAs(t, err, &notFound)
		assert.Equal(t, join("missing"), notFound.Path)
		assert.Equal(t, "could not find the file "+join("missing"), err.Error())

		err = VM(&localRunner{}).Archive(context.Background(), join("loop"), true, io.Discard)
		assert.ErrorContains(t, err, "too many symlinks")

		broken := errors.New("the vm went away")
		err = VM(&scriptedRunner{answer: func([]string, io.Reader, io.Writer) error { return broken }}).Archive(context.Background(), "/x", false, io.Discard)
		assert.ErrorIs(t, err, broken)
	})

	t.Run("an archive is unpacked by tar, as whoever runs it unless the owners are kept", func(t *testing.T) {
		t.Parallel()

		runner := &scriptedRunner{answer: func(_ []string, stdin io.Reader, _ io.Writer) error {
			_, err := io.Copy(io.Discard, stdin)

			return err
		}}

		require.NoError(t, VM(runner).Extract(context.Background(), "tmp/x", strings.NewReader("archive"), false))
		require.NoError(t, VM(runner).Extract(context.Background(), "/tmp/x", strings.NewReader("archive"), true))

		assert.Equal(t, [][]string{
			{"tar", "-x", "-o", "-f", "-", "-C", "/tmp/x"},
			{"tar", "-x", "-f", "-", "-C", "/tmp/x"},
		}, runner.all())
	})
}

func TestContainer(t *testing.T) {
	t.Parallel()

	needs(t, "tar")

	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "data", "sub"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "data", "a.txt"), []byte("a"), 0o644))
	require.NoError(t, os.Symlink("data/a.txt", filepath.Join(root, "link")))

	t.Run("what a path is, from the first entry of its archive", func(t *testing.T) {
		t.Parallel()

		runner := &dockerRunner{root: root}
		remote := Container(runner, "web")

		cases := map[string]Info{
			"/data":       {Exists: true, IsDir: true},
			"/data/a.txt": {Exists: true},
			"/link":       {Exists: true, Link: "data/a.txt"},
			"/missing":    {},
		}

		for path, want := range cases {
			got, err := remote.Stat(context.Background(), path)
			require.NoError(t, err, path)
			assert.Equal(t, want, got, path)
		}

		assert.Contains(t, runner.all(), []string{"docker", "cp", "web:/data", "-"})
	})

	t.Run("a stat that could not ask docker is an error, and an archive that says nothing too", func(t *testing.T) {
		t.Parallel()

		broken := errors.New("the vm went away")

		_, err := Container(&scriptedRunner{answer: func([]string, io.Reader, io.Writer) error { return broken }}, "web").Stat(context.Background(), "/x")
		assert.ErrorIs(t, err, broken)

		_, err = Container(&scriptedRunner{answer: func([]string, io.Reader, io.Writer) error { return nil }}, "web").Stat(context.Background(), "/x")
		assert.ErrorContains(t, err, "cannot tell what /x is")
	})

	t.Run("an archive and an unpacking are docker cp's", func(t *testing.T) {
		t.Parallel()

		runner := &scriptedRunner{answer: func(_ []string, stdin io.Reader, _ io.Writer) error {
			if stdin != nil {
				_, err := io.Copy(io.Discard, stdin)

				return err
			}

			return nil
		}}
		remote := Container(runner, "web")

		require.NoError(t, remote.Archive(context.Background(), "/data", false, io.Discard))
		require.NoError(t, remote.Archive(context.Background(), "/link", true, io.Discard))
		require.NoError(t, remote.Extract(context.Background(), "/data", strings.NewReader("archive"), false))
		require.NoError(t, remote.Extract(context.Background(), "/data", strings.NewReader("archive"), true))

		assert.Equal(t, [][]string{
			{"docker", "cp", "web:/data", "-"},
			{"docker", "cp", "--follow-link", "web:/link", "-"},
			{"docker", "cp", "-", "web:/data"},
			{"docker", "cp", "--archive", "-", "web:/data"},
		}, runner.all())
	})
}

// sortedEntries describes the entries of an archive in order of name.
func sortedEntries(t *testing.T, data []byte) []string {
	t.Helper()

	described := entries(t, data)
	sort.Strings(described)

	return described
}
