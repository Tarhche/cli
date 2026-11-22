package transfer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFlavor(t *testing.T) {
	t.Parallel()

	t.Run("a path splits into its directory and its last element, a trailing dot kept", func(t *testing.T) {
		t.Parallel()

		cases := map[string][2]string{
			"/etc/hosts":    {"/etc", "hosts"},
			"/etc/ssl/":     {"/etc", "ssl"},
			"/etc/ssl/.":    {"/etc/ssl", "."},
			"/etc/ssl/./":   {"/etc/ssl", "."},
			"relative/file": {"relative", "file"},
			"file":          {".", "file"},
			".":             {".", "."},
			"/":             {"/", "/"},
		}

		for p, want := range cases {
			dir, base := remotePaths.split(p)
			assert.Equal(t, want, [2]string{dir, base}, p)
		}
	})

	t.Run("a path ending with a separator or a dot says it is a directory", func(t *testing.T) {
		t.Parallel()

		assert.True(t, remotePaths.assertsDirectory("/etc/"))
		assert.True(t, remotePaths.assertsDirectory("/etc/."))
		assert.False(t, remotePaths.assertsDirectory("/etc"))
	})

	t.Run("a symlink points at its target, from the directory it is in", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "/usr/lib/real", remotePaths.linkTarget("/usr/lib/link", "real"))
		assert.Equal(t, "/usr/real", remotePaths.linkTarget("/usr/lib/link", "../real"))
		assert.Equal(t, "/opt/real", remotePaths.linkTarget("/usr/lib/link", "/opt/real"))
	})
}

func TestPlace(t *testing.T) {
	t.Parallel()

	file := source{name: "hosts"}
	dir := source{isDir: true, name: "ssl"}
	contents := source{isDir: true, name: "."}

	cases := []struct {
		name string
		src  source
		dst  destination
		want placement
		err  error
	}{
		{name: "into a directory that is there", src: file, dst: destination{path: "/tmp", exists: true, isDir: true}, want: placement{dir: "/tmp", root: "hosts"}},
		{name: "a directory into a directory that is there", src: dir, dst: destination{path: "/tmp/", exists: true, isDir: true}, want: placement{dir: "/tmp/", root: "ssl"}},
		{name: "contents into a directory that is there", src: contents, dst: destination{path: "/tmp", exists: true, isDir: true}, want: placement{dir: "/tmp", root: "."}},
		{name: "onto a file that is there", src: file, dst: destination{path: "/tmp/old", exists: true}, want: placement{dir: "/tmp", root: "old"}},
		{name: "a directory onto a file", src: dir, dst: destination{path: "/tmp/old", exists: true}, err: ErrCannotCopyDir},
		{name: "a directory to a new name", src: dir, dst: destination{path: "/tmp/new"}, want: placement{dir: "/tmp", root: "new"}},
		{name: "contents to a new name", src: contents, dst: destination{path: "/tmp/new/"}, want: placement{dir: "/tmp", root: "new"}},
		{name: "a file to a new name", src: file, dst: destination{path: "/tmp/new"}, want: placement{dir: "/tmp", root: "new"}},
		{name: "a file into a directory that is not there", src: file, dst: destination{path: "/tmp/new/"}, err: ErrNoSuchDirectory},
		{name: "a file into a directory's contents that are not there", src: file, dst: destination{path: "/tmp/new/."}, err: ErrNoSuchDirectory},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			got, err := place(c.src, c.dst, remotePaths)

			if c.err != nil {
				assert.ErrorIs(t, err, c.err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, c.want, got)
		})
	}
}

func TestLocalDestination(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, "dir"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "file"), []byte("x"), 0o644))
	require.NoError(t, os.Symlink("dir", filepath.Join(root, "to-dir")))
	require.NoError(t, os.Symlink(filepath.Join(root, "file"), filepath.Join(root, "to-file")))
	require.NoError(t, os.Symlink("missing", filepath.Join(root, "broken")))
	require.NoError(t, os.Symlink("loop", filepath.Join(root, "loop")))

	join := func(name string) string { return filepath.Join(root, name) }

	cases := []struct {
		name string
		path string
		want destination
		err  string
	}{
		{name: "a directory", path: join("dir"), want: destination{path: join("dir"), exists: true, isDir: true}},
		{name: "a file", path: join("file"), want: destination{path: join("file"), exists: true}},
		{name: "a path that is not there, in a directory that is", path: join("new"), want: destination{path: join("new")}},
		{name: "a symlink to a directory is the directory", path: join("to-dir"), want: destination{path: join("dir"), exists: true, isDir: true}},
		{name: "a symlink to a file is the file", path: join("to-file"), want: destination{path: join("file"), exists: true}},
		{name: "a broken symlink is what it points to, to be made", path: join("broken"), want: destination{path: join("missing")}},
		{name: "a path in a directory that is not there", path: join("nowhere/new"), err: "the directory " + join("nowhere") + " does not exist"},
		{name: "a path in a file", path: join("file") + "/new/x", err: "not a directory"},
		{name: "a symlink to itself", path: join("loop"), err: "too many symlinks"},
		{name: "a name too long to be one", path: join(strings.Repeat("n", 300)), err: "file name too long"},
		{name: "a name too long in a directory that is not there", path: join(strings.Repeat("n", 300) + "/x"), err: "file name too long"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			got, err := localDestination(c.path)

			if len(c.err) > 0 {
				require.Error(t, err)
				assert.Contains(t, err.Error(), c.err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, c.want, got)
		})
	}
}

func TestLocalSource(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "real", "dir"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "real", "file"), []byte("x"), 0o644))
	require.NoError(t, os.Symlink("real", filepath.Join(root, "linked")))
	require.NoError(t, os.Symlink("file", filepath.Join(root, "real", "to-file")))
	require.NoError(t, os.Symlink("dir", filepath.Join(root, "real", "to-dir")))

	join := func(names ...string) string { return filepath.Join(append([]string{root}, names...)...) }

	cases := []struct {
		name   string
		path   string
		follow bool
		want   string
		source source
		err    bool
	}{
		{name: "a file", path: join("real", "file"), want: join("real", "file"), source: source{name: "file"}},
		{name: "the symlinks of its directory are followed", path: join("linked", "file"), want: join("real", "file"), source: source{name: "file"}},
		{name: "a symlink is copied as it is", path: join("real", "to-file"), want: join("real", "to-file"), source: source{name: "to-file"}},
		{name: "a symlink is followed when asked to, keeping its name", path: join("real", "to-file"), follow: true, want: join("real", "file"), source: source{name: "to-file"}},
		{name: "a directory's contents", path: join("real", "dir") + "/.", want: join("real", "dir") + "/.", source: source{isDir: true, name: "."}},
		{name: "a followed symlink's contents", path: join("real", "to-dir") + "/.", follow: true, want: join("real", "dir") + "/.", source: source{isDir: true, name: "."}},
		{name: "a symlink with a trailing separator is its directory, keeping its name", path: join("real", "to-dir") + "/", want: join("real", "dir") + "/", source: source{isDir: true, name: "to-dir"}},
		{name: "what is not there", path: join("real", "missing"), err: true},
		{name: "what is not there, followed", path: join("real", "missing"), follow: true, err: true},
		{name: "what is in a directory that is not there", path: join("nowhere", "file"), err: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			got, src, err := localSource(c.path, c.follow)

			if c.err {
				assert.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, c.want, got)
			assert.Equal(t, c.source, src)
		})
	}

	t.Run("a relative path is taken from the working directory", func(t *testing.T) {
		t.Parallel()

		got, src, err := localSource("paths_test.go", false)

		require.NoError(t, err)
		assert.Equal(t, "./paths_test.go", filepath.ToSlash(got))
		assert.Equal(t, source{name: "paths_test.go"}, src)
	})
}
