package transfer

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// workload makes the tree a copy is made from or to: a directory, dir,
// holding a.txt and sub/b.txt, a file, file.txt, and symlinks to both.
func workload(t *testing.T, root string) {
	t.Helper()

	require.NoError(t, os.MkdirAll(filepath.Join(root, "dir", "sub"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "dir", "a.txt"), []byte("a"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "dir", "sub", "b.txt"), []byte("b"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "file.txt"), []byte("hello"), 0o644))
	require.NoError(t, os.Symlink("file.txt", filepath.Join(root, "to-file")))
	require.NoError(t, os.Symlink("dir", filepath.Join(root, "to-dir")))
	require.NoError(t, os.Mkdir(filepath.Join(root, "out"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "existing.txt"), []byte("old"), 0o644))
}

// without is described, the tree workload makes left out.
func without(described []string) []string {
	made := map[string]bool{"dir/": true, "dir/a.txt: a": true, "dir/sub/": true, "dir/sub/b.txt: b": true, "file.txt: hello": true, "to-file -> file.txt": true, "to-dir -> dir": true, "out/": true, "existing.txt: old": true}

	var left []string

	for _, entry := range described {
		if !made[entry] {
			left = append(left, entry)
		}
	}

	return left
}

func TestDownload(t *testing.T) {
	t.Parallel()

	needs(t, "sh", "tar", "readlink")

	cases := []struct {
		name    string
		src     string
		dst     string
		options Options
		want    []string
		err     string
	}{
		{name: "a file into a directory", src: "file.txt", dst: "out", want: []string{"out/file.txt: hello"}},
		{name: "a file into a directory asserted", src: "file.txt", dst: "out/", want: []string{"out/file.txt: hello"}},
		{name: "a file to a new name", src: "file.txt", dst: "new.txt", want: []string{"new.txt: hello"}},
		{name: "a file onto a file, overwriting it", src: "file.txt", dst: "existing.txt", want: []string{"existing.txt: hello"}},
		{name: "a file into a directory that is not there", src: "file.txt", dst: "missing/", err: "no such directory"},
		{name: "a directory into a directory", src: "dir", dst: "out", want: []string{"out/dir/", "out/dir/a.txt: a", "out/dir/sub/", "out/dir/sub/b.txt: b"}},
		{name: "a directory to a new name", src: "dir", dst: "copy", want: []string{"copy/", "copy/a.txt: a", "copy/sub/", "copy/sub/b.txt: b"}},
		{name: "a directory's contents into a directory", src: "dir/.", dst: "out", want: []string{"out/a.txt: a", "out/sub/", "out/sub/b.txt: b"}},
		{name: "a directory's contents to a new name", src: "dir/.", dst: "copy", want: []string{"copy/", "copy/a.txt: a", "copy/sub/", "copy/sub/b.txt: b"}},
		{name: "a directory onto a file", src: "dir", dst: "existing.txt", err: "cannot copy a directory onto a file"},
		{name: "a symlink, as it is", src: "to-file", dst: "out", want: []string{"out/to-file -> file.txt"}},
		{name: "a symlink followed, named as it is", src: "to-file", dst: "out", options: Options{FollowLink: true}, want: []string{"out/to-file: hello"}},
		{name: "a symlink to a directory followed, to a new name", src: "to-dir", dst: "copy", options: Options{FollowLink: true}, want: []string{"copy/", "copy/a.txt: a", "copy/sub/", "copy/sub/b.txt: b"}},
		{name: "what is not there", src: "missing", dst: "out", err: "could not find the file"},
		{name: "into a directory that is not there", src: "file.txt", dst: "nowhere/new.txt", err: "does not exist"},
	}

	for _, c := range cases {
		t.Run(c.name+" of a vm", func(t *testing.T) {
			t.Parallel()

			vm, local := t.TempDir(), t.TempDir()
			workload(t, vm)
			workload(t, local)

			size, err := Download(context.Background(), VM(&localRunner{}), filepath.Join(vm, c.src)+trailing(c.src), filepath.Join(local, c.dst)+trailing(c.dst), nil, c.options)

			if len(c.err) > 0 {
				assert.ErrorContains(t, err, c.err)

				return
			}

			require.NoError(t, err)
			assert.Positive(t, size)
			assert.Equal(t, c.want, without(tree(t, local)))
		})

		t.Run(c.name+" of a container", func(t *testing.T) {
			t.Parallel()

			container, local := t.TempDir(), t.TempDir()
			workload(t, container)
			workload(t, local)

			_, err := Download(context.Background(), Container(&dockerRunner{root: container}, "web"), "/"+c.src, filepath.Join(local, c.dst)+trailing(c.dst), nil, c.options)

			if len(c.err) > 0 {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, c.want, without(tree(t, local)))
		})
	}

	t.Run("an archive written as it is", func(t *testing.T) {
		t.Parallel()

		vm := t.TempDir()
		workload(t, vm)

		var archive bytes.Buffer

		size, err := Download(context.Background(), VM(&localRunner{}), filepath.Join(vm, "dir"), Stdio, &archive, Options{})

		require.NoError(t, err)
		assert.EqualValues(t, archive.Len(), size)
		assert.Equal(t, []string{"dir/", "dir/a.txt: a", "dir/sub/", "dir/sub/b.txt: b"}, sortedEntries(t, archive.Bytes()))
	})
}

// trailing is what path ends with that joining it would clean away.
func trailing(path string) string {
	switch {
	case strings.HasSuffix(path, "/."):
		return "/."
	case strings.HasSuffix(path, "/"):
		return "/"
	}

	return ""
}

func TestUpload(t *testing.T) {
	t.Parallel()

	needs(t, "sh", "tar", "readlink")

	cases := []struct {
		name    string
		src     string
		dst     string
		options Options
		want    []string
		err     string
	}{
		{name: "a file into a directory", src: "file.txt", dst: "out", want: []string{"out/file.txt: hello"}},
		{name: "a file to a new name", src: "file.txt", dst: "new.txt", want: []string{"new.txt: hello"}},
		{name: "a file onto a file, overwriting it", src: "file.txt", dst: "existing.txt", want: []string{"existing.txt: hello"}},
		{name: "a file into a directory that is not there", src: "file.txt", dst: "missing/", err: "no such directory"},
		{name: "a directory into a directory", src: "dir", dst: "out", want: []string{"out/dir/", "out/dir/a.txt: a", "out/dir/sub/", "out/dir/sub/b.txt: b"}},
		{name: "a directory to a new name", src: "dir", dst: "copy", want: []string{"copy/", "copy/a.txt: a", "copy/sub/", "copy/sub/b.txt: b"}},
		{name: "a directory's contents into a directory", src: "dir/.", dst: "out", want: []string{"out/a.txt: a", "out/sub/", "out/sub/b.txt: b"}},
		{name: "a directory onto a file", src: "dir", dst: "existing.txt", err: "cannot copy a directory onto a file"},
		{name: "a symlink, as it is", src: "to-file", dst: "out", want: []string{"out/to-file -> file.txt"}},
		{name: "a symlink followed, named as it is", src: "to-file", dst: "out", options: Options{FollowLink: true}, want: []string{"out/to-file: hello"}},
		{name: "into what a symlink to a directory points to", src: "file.txt", dst: "to-dir", want: []string{"dir/file.txt: hello"}},
		{name: "owners kept", src: "file.txt", dst: "out", options: Options{CopyUIDGID: true}, want: []string{"out/file.txt: hello"}},
		{name: "what is not there", src: "missing", dst: "out", err: "no such file or directory"},
		{name: "into a directory that is not there", src: "file.txt", dst: "nowhere/new.txt", err: "nowhere"},
	}

	for _, c := range cases {
		t.Run(c.name+" into a vm", func(t *testing.T) {
			t.Parallel()

			local, vm := t.TempDir(), t.TempDir()
			workload(t, local)
			workload(t, vm)

			size, err := Upload(context.Background(), VM(&localRunner{}), filepath.Join(local, c.src)+trailing(c.src), filepath.Join(vm, c.dst)+trailing(c.dst), nil, c.options)

			if len(c.err) > 0 {
				assert.ErrorContains(t, err, c.err)

				return
			}

			require.NoError(t, err)
			assert.Positive(t, size)
			assert.Equal(t, c.want, without(tree(t, vm)))
		})

		t.Run(c.name+" into a container", func(t *testing.T) {
			t.Parallel()

			local, container := t.TempDir(), t.TempDir()
			workload(t, local)
			workload(t, container)

			_, err := Upload(context.Background(), Container(&dockerRunner{root: container}, "web"), filepath.Join(local, c.src)+trailing(c.src), "/"+c.dst, nil, c.options)

			if len(c.err) > 0 {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, c.want, without(tree(t, container)))
		})
	}

	t.Run("an archive read as it is, into a directory", func(t *testing.T) {
		t.Parallel()

		vm := t.TempDir()
		workload(t, vm)

		archive := archiveOf(t, dirEntry("piped/"), fileEntry("piped/x", "x"))

		size, err := Upload(context.Background(), VM(&localRunner{}), Stdio, filepath.Join(vm, "out"), bytes.NewReader(archive), Options{})

		require.NoError(t, err)
		assert.EqualValues(t, len(archive), size)
		assert.Equal(t, []string{"out/piped/", "out/piped/x: x"}, without(tree(t, vm)))
	})

	t.Run("an archive read as it is, into what is not a directory", func(t *testing.T) {
		t.Parallel()

		vm := t.TempDir()
		workload(t, vm)

		for _, dst := range []string{"file.txt", "missing"} {
			_, err := Upload(context.Background(), VM(&localRunner{}), Stdio, filepath.Join(vm, dst), strings.NewReader(""), Options{})

			assert.ErrorIs(t, err, ErrNotDirectory, dst)
		}
	})

	t.Run("a destination that is a symlink to itself", func(t *testing.T) {
		t.Parallel()

		local, vm := t.TempDir(), t.TempDir()
		workload(t, local)
		require.NoError(t, os.Symlink("loop", filepath.Join(vm, "loop")))

		_, err := Upload(context.Background(), VM(&localRunner{}), filepath.Join(local, "file.txt"), filepath.Join(vm, "loop"), nil, Options{})

		assert.ErrorContains(t, err, "too many symlinks")
	})

	t.Run("a destination that cannot be found out is an error", func(t *testing.T) {
		t.Parallel()

		broken := errors.New("the vm went away")
		remote := &fakeRemote{statErr: broken}

		_, err := Upload(context.Background(), remote, "file.txt", "/out", nil, Options{})

		assert.ErrorIs(t, err, broken)
	})

	t.Run("what cannot be read is the error, the unpacking stopped", func(t *testing.T) {
		t.Parallel()

		if _, err := os.Stat("/proc/self/mem"); err != nil {
			t.Skip("there is no file here whose reading fails")
		}

		remote := &fakeRemote{info: Info{Exists: true, IsDir: true}, extract: func(r io.Reader) error {
			_, err := io.Copy(io.Discard, r)

			return err
		}}

		_, err := Upload(context.Background(), remote, "/proc/self/mem", "/out", nil, Options{})

		assert.ErrorContains(t, err, "input/output error")
	})

	t.Run("an unpacking that fails is the error, however much was left to send", func(t *testing.T) {
		t.Parallel()

		local := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(local, "big"), bytes.Repeat([]byte("x"), 1<<20), 0o644))

		refused := errors.New("tar: cannot write: no space left on device")
		remote := &fakeRemote{info: Info{Exists: true, IsDir: true}, extract: func(io.Reader) error { return refused }}

		_, err := Upload(context.Background(), remote, filepath.Join(local, "big"), "/out", nil, Options{})

		assert.ErrorIs(t, err, refused)
	})
}

// fakeRemote is a remote that answers as a test says.
type fakeRemote struct {
	info    Info
	statErr error
	archive func(w io.Writer) error
	extract func(r io.Reader) error
}

func (f *fakeRemote) Stat(context.Context, string) (Info, error) {
	return f.info, f.statErr
}

func (f *fakeRemote) Archive(_ context.Context, _ string, _ bool, w io.Writer) error {
	return f.archive(w)
}

func (f *fakeRemote) Extract(_ context.Context, _ string, r io.Reader, _ bool) error {
	return f.extract(r)
}

func TestDownloadFailures(t *testing.T) {
	t.Parallel()

	t.Run("an archive that would write outside the destination is refused, however its writer ends", func(t *testing.T) {
		t.Parallel()

		malicious := archiveOf(t, dirEntry("dir/"), fileEntry("dir/../../evil", "x"), fileEntry("dir/padding", strings.Repeat("p", 1<<20)))
		remote := &fakeRemote{archive: func(w io.Writer) error {
			_, err := w.Write(malicious)

			return err
		}}

		parent := t.TempDir()
		local := filepath.Join(parent, "local")
		require.NoError(t, os.Mkdir(local, 0o755))

		_, err := Download(context.Background(), remote, "/dir", filepath.Join(local, "out"), nil, Options{})

		assert.ErrorContains(t, err, "outside the archive")
		assert.Equal(t, []string{"local/", "local/out/"}, tree(t, parent))
	})

	t.Run("a remote that fails partway is the error", func(t *testing.T) {
		t.Parallel()

		partial := archiveOf(t, dirEntry("dir/"), fileEntry("dir/a.txt", strings.Repeat("a", 4096)))
		broken := errors.New("tar: dir/a.txt: file changed as we read it")
		remote := &fakeRemote{archive: func(w io.Writer) error {
			_, _ = w.Write(partial[:2048])

			return broken
		}}

		_, err := Download(context.Background(), remote, "/dir", filepath.Join(t.TempDir(), "out"), nil, Options{})

		assert.ErrorIs(t, err, broken)
	})

	t.Run("a remote that fails once all was sent is the error", func(t *testing.T) {
		t.Parallel()

		complete := archiveOf(t, fileEntry("a.txt", "a"))
		broken := errors.New("tar: exiting with failure status due to previous errors")
		remote := &fakeRemote{archive: func(w io.Writer) error {
			_, _ = w.Write(complete)

			return broken
		}}

		_, err := Download(context.Background(), remote, "/a.txt", filepath.Join(t.TempDir(), "a.txt"), nil, Options{})

		assert.ErrorIs(t, err, broken)
	})

	t.Run("a remote that sends nothing is the error", func(t *testing.T) {
		t.Parallel()

		remote := &fakeRemote{archive: func(io.Writer) error { return nil }}

		_, err := Download(context.Background(), remote, "/a.txt", filepath.Join(t.TempDir(), "a.txt"), nil, Options{})

		assert.EqualError(t, err, "the archive is empty")
	})
}

func TestCause(t *testing.T) {
	t.Parallel()

	local := errors.New("refusing")
	remote := errors.New("tar failed")

	assert.NoError(t, cause(nil, nil))
	assert.Equal(t, local, cause(local, nil))
	assert.Equal(t, remote, cause(nil, remote))
	assert.Equal(t, remote, cause(local, remote))
	assert.Equal(t, local, cause(local, errors.Join(errors.New("writing"), local)))
	assert.Equal(t, local, cause(local, context.Canceled))
}

func TestArchiveName(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "hosts", archiveName("/etc/hosts"))
	assert.Equal(t, "etc", archiveName("/etc/"))
	assert.Equal(t, ".", archiveName("/etc/."))
	assert.Equal(t, ".", archiveName("/"))
}
