package transfer

import (
	"archive/tar"
	"bytes"
	"errors"
	"io"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// entry is an entry of an archive a test makes.
type entry struct {
	name    string
	kind    byte
	body    string
	link    string
	mode    int64
	modTime time.Time
}

func dirEntry(name string) entry { return entry{name: name, kind: tar.TypeDir, mode: 0o755} }
func fileEntry(name, body string) entry {
	return entry{name: name, kind: tar.TypeReg, body: body, mode: 0o644}
}
func symlinkEntry(name, link string) entry {
	return entry{name: name, kind: tar.TypeSymlink, link: link}
}
func linkEntry(name, link string) entry { return entry{name: name, kind: tar.TypeLink, link: link} }

// archiveOf is a tar archive of entries.
func archiveOf(t *testing.T, entries ...entry) []byte {
	t.Helper()

	var buffer bytes.Buffer

	archive := tar.NewWriter(&buffer)

	for _, e := range entries {
		header := &tar.Header{Name: e.name, Typeflag: e.kind, Linkname: e.link, Mode: e.mode, ModTime: e.modTime, Size: int64(len(e.body))}
		require.NoError(t, archive.WriteHeader(header))

		_, err := io.WriteString(archive, e.body)
		require.NoError(t, err)
	}

	require.NoError(t, archive.Close())

	return buffer.Bytes()
}

// tree describes what is under root: "name/" for a directory, "name: body"
// for a file and "name -> link" for a symlink, in order.
func tree(t *testing.T, root string) []string {
	t.Helper()

	var described []string

	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		require.NoError(t, err)

		if p == root {
			return nil
		}

		name := filepath.ToSlash(strings.TrimPrefix(p, root+string(filepath.Separator)))

		switch {
		case d.Type()&fs.ModeSymlink != 0:
			link, err := os.Readlink(p)
			require.NoError(t, err)

			described = append(described, name+" -> "+link)
		case d.IsDir():
			described = append(described, name+"/")
		default:
			content, err := os.ReadFile(p)
			require.NoError(t, err)

			described = append(described, name+": "+string(content))
		}

		return nil
	})
	require.NoError(t, err)

	sort.Strings(described)

	return described
}

// entries describes the entries of an archive as tree describes a tree.
func entries(t *testing.T, data []byte) []string {
	t.Helper()

	var described []string

	archive := tar.NewReader(bytes.NewReader(data))

	for {
		header, err := archive.Next()
		if errors.Is(err, io.EOF) {
			return described
		}

		require.NoError(t, err)

		switch header.Typeflag {
		case tar.TypeSymlink:
			described = append(described, header.Name+" -> "+header.Linkname)
		case tar.TypeDir:
			described = append(described, header.Name)
		default:
			content, err := io.ReadAll(archive)
			require.NoError(t, err)

			described = append(described, header.Name+": "+string(content))
		}
	}
}

func TestPack(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	src := filepath.Join(root, "src")
	require.NoError(t, os.MkdirAll(filepath.Join(src, "sub"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(src, "a.txt"), []byte("a"), 0o640))
	require.NoError(t, os.WriteFile(filepath.Join(src, "sub", "b.txt"), []byte("b"), 0o600))
	require.NoError(t, os.Symlink("a.txt", filepath.Join(src, "link")))

	listener, err := net.Listen("unix", filepath.Join(src, "socket"))
	require.NoError(t, err)

	t.Cleanup(func() { _ = listener.Close() })

	t.Run("a directory is archived under the root it is given, a symlink as it is and a socket not at all", func(t *testing.T) {
		t.Parallel()

		var archive bytes.Buffer
		require.NoError(t, pack(&archive, src, "copy", false))

		assert.Equal(t, []string{"copy/", "copy/a.txt: a", "copy/link -> a.txt", "copy/sub/", "copy/sub/b.txt: b"}, entries(t, archive.Bytes()))
	})

	t.Run("a directory's contents are archived under the dot", func(t *testing.T) {
		t.Parallel()

		var archive bytes.Buffer
		require.NoError(t, pack(&archive, src+"/.", ".", false))

		assert.Equal(t, []string{"./", "./a.txt: a", "./link -> a.txt", "./sub/", "./sub/b.txt: b"}, entries(t, archive.Bytes()))
	})

	t.Run("a file is archived as the root", func(t *testing.T) {
		t.Parallel()

		var archive bytes.Buffer
		require.NoError(t, pack(&archive, filepath.Join(src, "a.txt"), "renamed.txt", false))

		assert.Equal(t, []string{"renamed.txt: a"}, entries(t, archive.Bytes()))
	})

	t.Run("modes are kept, and owners only when asked to be", func(t *testing.T) {
		t.Parallel()

		headers := func(owners bool) map[string]*tar.Header {
			var archive bytes.Buffer
			require.NoError(t, pack(&archive, src, "src", owners))

			found := map[string]*tar.Header{}
			reader := tar.NewReader(&archive)

			for {
				header, err := reader.Next()
				if errors.Is(err, io.EOF) {
					return found
				}

				require.NoError(t, err)
				found[header.Name] = header
			}
		}

		anonymous := headers(false)
		assert.EqualValues(t, 0o750, anonymous["src/"].Mode&0o777)
		assert.EqualValues(t, 0o640, anonymous["src/a.txt"].Mode&0o777)
		assert.Zero(t, anonymous["src/a.txt"].Uid)
		assert.Zero(t, anonymous["src/a.txt"].Gid)
		assert.Empty(t, anonymous["src/a.txt"].Uname)

		owned := headers(true)
		assert.Equal(t, os.Getuid(), owned["src/a.txt"].Uid)
		assert.Equal(t, os.Getgid(), owned["src/a.txt"].Gid)
	})

	t.Run("an archive that cannot be written is an error", func(t *testing.T) {
		t.Parallel()

		assert.Error(t, pack(failingWriter{}, src, "src", false))
	})

	t.Run("what cannot be read is an error", func(t *testing.T) {
		t.Parallel()

		assert.Error(t, pack(io.Discard, filepath.Join(root, "missing"), "missing", false))
		assert.Error(t, packEntry(tar.NewWriter(io.Discard), filepath.Join(root, "missing"), "missing", false))
	})
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("cannot write") }

// unpackInto unpacks data, an archive of what was asked for as name, where a
// copy to dst lands.
func unpackInto(t *testing.T, data []byte, name, dst string) error {
	t.Helper()

	destination, err := localDestination(dst)
	require.NoError(t, err)

	return unpack(bytes.NewReader(data), name, destination)
}

func TestUnpack(t *testing.T) {
	t.Parallel()

	ssl := archiveOf(t, dirEntry("ssl/"), fileEntry("ssl/a.pem", "a"), dirEntry("ssl/certs/"), fileEntry("ssl/certs/b.pem", "b"), symlinkEntry("ssl/link", "a.pem"), linkEntry("ssl/hard", "ssl/a.pem"))
	contents := archiveOf(t, dirEntry("./"), fileEntry("./a.pem", "a"), dirEntry("./certs/"), fileEntry("./certs/b.pem", "b"))
	hosts := archiveOf(t, fileEntry("hosts", "127.0.0.1 localhost"))

	cases := []struct {
		name    string
		archive []byte
		as      string
		dst     string
		before  func(t *testing.T, root string)
		want    []string
		err     string
	}{
		{
			name: "a directory into a directory that is there", archive: ssl, as: "ssl", dst: "out",
			want: []string{"out/", "out/ssl/", "out/ssl/a.pem: a", "out/ssl/certs/", "out/ssl/certs/b.pem: b", "out/ssl/hard: a", "out/ssl/link -> a.pem"},
		},
		{
			name: "a directory to a new name", archive: ssl, as: "ssl", dst: "copy",
			want: []string{"copy/", "copy/a.pem: a", "copy/certs/", "copy/certs/b.pem: b", "copy/hard: a", "copy/link -> a.pem", "out/"},
		},
		{
			name: "a directory's contents into a directory that is there", archive: contents, as: ".", dst: "out",
			want: []string{"out/", "out/a.pem: a", "out/certs/", "out/certs/b.pem: b"},
		},
		{
			name: "a directory's contents to a new name", archive: contents, as: ".", dst: "copy/",
			want: []string{"copy/", "copy/a.pem: a", "copy/certs/", "copy/certs/b.pem: b", "out/"},
		},
		{
			name: "a directory that was followed is named as it was asked for", archive: ssl, as: "link-to-ssl", dst: "out",
			want: []string{"out/", "out/link-to-ssl/", "out/link-to-ssl/a.pem: a", "out/link-to-ssl/certs/", "out/link-to-ssl/certs/b.pem: b", "out/link-to-ssl/hard: a", "out/link-to-ssl/link -> a.pem"},
		},
		{
			name: "a file into a directory that is there", archive: hosts, as: "hosts", dst: "out",
			want: []string{"out/", "out/hosts: 127.0.0.1 localhost"},
		},
		{
			name: "a file to a new name", archive: hosts, as: "hosts", dst: "renamed",
			want: []string{"out/", "renamed: 127.0.0.1 localhost"},
		},
		{
			name: "a file onto a file, which it overwrites", archive: hosts, as: "hosts", dst: "existing",
			before: func(t *testing.T, root string) {
				require.NoError(t, os.WriteFile(filepath.Join(root, "existing"), []byte("old"), 0o644))
			},
			want: []string{"existing: 127.0.0.1 localhost", "out/"},
		},
		{
			name: "a file onto a symlink to a file, which it overwrites", archive: hosts, as: "hosts", dst: "to-existing",
			before: func(t *testing.T, root string) {
				require.NoError(t, os.WriteFile(filepath.Join(root, "existing"), []byte("old"), 0o644))
				require.NoError(t, os.Symlink("existing", filepath.Join(root, "to-existing")))
			},
			want: []string{"existing: 127.0.0.1 localhost", "out/", "to-existing -> existing"},
		},
		{
			name: "a directory onto a file", archive: ssl, as: "ssl", dst: "existing",
			before: func(t *testing.T, root string) {
				require.NoError(t, os.WriteFile(filepath.Join(root, "existing"), []byte("old"), 0o644))
			},
			err: "cannot copy a directory onto a file",
		},
		{
			name: "a file into a directory that is not there", archive: hosts, as: "hosts", dst: "missing/",
			err: "no such directory",
		},
		{
			name: "a directory merged into one that is there, whose files it overwrites", archive: ssl, as: "ssl", dst: "out",
			before: func(t *testing.T, root string) {
				require.NoError(t, os.MkdirAll(filepath.Join(root, "out", "ssl", "certs"), 0o700))
				require.NoError(t, os.WriteFile(filepath.Join(root, "out", "ssl", "a.pem"), []byte("old"), 0o644))
				require.NoError(t, os.WriteFile(filepath.Join(root, "out", "ssl", "kept.pem"), []byte("kept"), 0o644))
				require.NoError(t, os.Symlink("elsewhere", filepath.Join(root, "out", "ssl", "link")))
			},
			want: []string{"out/", "out/ssl/", "out/ssl/a.pem: a", "out/ssl/certs/", "out/ssl/certs/b.pem: b", "out/ssl/hard: a", "out/ssl/kept.pem: kept", "out/ssl/link -> a.pem"},
		},
		{
			name: "a file onto a directory", archive: hosts, as: "hosts", dst: "out",
			before: func(t *testing.T, root string) {
				require.NoError(t, os.Mkdir(filepath.Join(root, "out", "hosts"), 0o755))
			},
			err: "cannot overwrite the directory",
		},
		{
			name: "a directory onto a file in a directory", archive: ssl, as: "ssl", dst: "out",
			before: func(t *testing.T, root string) {
				require.NoError(t, os.WriteFile(filepath.Join(root, "out", "ssl"), []byte("old"), 0o644))
			},
			err: "with a directory",
		},
		{
			name: "a directory's files into one that is a file", archive: archiveOf(t, fileEntry("ssl/certs/b.pem", "b")), as: "ssl", dst: "out",
			before: func(t *testing.T, root string) {
				require.NoError(t, os.MkdirAll(filepath.Join(root, "out", "ssl"), 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(root, "out", "ssl", "certs"), []byte("old"), 0o644))
			},
			err: "not a directory",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			require.NoError(t, os.Mkdir(filepath.Join(root, "out"), 0o755))

			if c.before != nil {
				c.before(t, root)
			}

			err := unpackInto(t, c.archive, c.as, filepath.Join(root, c.dst)+trailing(c.dst))

			if len(c.err) > 0 {
				require.Error(t, err)
				assert.Contains(t, err.Error(), c.err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, c.want, tree(t, root))
		})
	}
}

func TestUnpackRefuses(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		archive []entry
		as      string
		err     string
	}{
		{name: "an entry above the archive", archive: []entry{dirEntry("ssl/"), fileEntry("../evil", "x")}, as: "ssl", err: "outside the archive"},
		{name: "an entry that climbs out", archive: []entry{dirEntry("ssl/"), fileEntry("ssl/../../evil", "x")}, as: "ssl", err: "outside the archive"},
		{name: "a contents entry that climbs out", archive: []entry{dirEntry("./"), fileEntry("./../evil", "x")}, as: ".", err: "outside the archive"},
		{name: "an entry beside what was copied", archive: []entry{dirEntry("ssl/"), fileEntry("other/evil", "x")}, as: "ssl", err: "is not in \"ssl\""},
		{name: "a symlink that points out", archive: []entry{dirEntry("ssl/"), symlinkEntry("ssl/link", "../../etc")}, as: "ssl", err: "points outside"},
		{name: "a contents symlink that points out", archive: []entry{dirEntry("./"), symlinkEntry("./link", "../x")}, as: ".", err: "points outside"},
		{name: "a write through a symlink pointing out", archive: []entry{dirEntry("ssl/"), symlinkEntry("ssl/link", "/etc"), fileEntry("ssl/link/passwd", "x")}, as: "ssl", err: "through the symlink"},
		{name: "a write through a symlink pointing in", archive: []entry{dirEntry("ssl/"), symlinkEntry("ssl/link", "."), fileEntry("ssl/link/x", "x")}, as: "ssl", err: "through the symlink"},
		{name: "a directory through a symlink", archive: []entry{dirEntry("ssl/"), symlinkEntry("ssl/link", "/tmp"), dirEntry("ssl/link/sub/")}, as: "ssl", err: "through the symlink"},
		{name: "a hard link out", archive: []entry{dirEntry("ssl/"), linkEntry("ssl/hard", "../etc/passwd")}, as: "ssl", err: "outside the archive"},
		{name: "a hard link beside what was copied", archive: []entry{dirEntry("ssl/"), linkEntry("ssl/hard", "etc/passwd")}, as: "ssl", err: "is not in \"ssl\""},
		{name: "a hard link through a symlink", archive: []entry{dirEntry("ssl/"), symlinkEntry("ssl/link", "/etc"), linkEntry("ssl/hard", "ssl/link/passwd")}, as: "ssl", err: "through the symlink"},
		{name: "a hard link to a symlink", archive: []entry{dirEntry("ssl/"), symlinkEntry("ssl/link", "/etc/passwd"), linkEntry("ssl/hard", "ssl/link")}, as: "ssl", err: "which is not a file"},
		{name: "a hard link to nothing", archive: []entry{dirEntry("ssl/"), linkEntry("ssl/hard", "ssl/missing")}, as: "ssl", err: "no such file"},
		{name: "a file in place of the directory copied into", archive: []entry{fileEntry(".", "x")}, as: ".", err: "would replace the directory"},
		{name: "a write through a symlink further up", archive: []entry{dirEntry("ssl/"), symlinkEntry("ssl/link", "/tmp"), fileEntry("ssl/link/deeper/x", "x")}, as: "ssl", err: "through the symlink"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			out := filepath.Join(root, "out")
			require.NoError(t, os.Mkdir(out, 0o755))

			err := unpackInto(t, archiveOf(t, c.archive...), c.as, out)

			require.Error(t, err)
			assert.Contains(t, err.Error(), c.err)

			_, statErr := os.Lstat(filepath.Join(root, "evil"))
			assert.ErrorIs(t, statErr, fs.ErrNotExist)
		})
	}
}

func TestUnpackEntries(t *testing.T) {
	t.Parallel()

	t.Run("absolute names are taken from the root, a symlink's absolute target is kept, and what is not a file, a directory or a link is left out", func(t *testing.T) {
		t.Parallel()

		out := t.TempDir()
		data := archiveOf(t,
			dirEntry("/ssl/"),
			fileEntry("/ssl/a.pem", "a"),
			symlinkEntry("/ssl/abs", "/etc/ssl"),
			entry{name: "ssl/null", kind: tar.TypeChar},
			entry{name: "ssl/fifo", kind: tar.TypeFifo},
		)

		require.NoError(t, unpackInto(t, data, "ssl", out))
		assert.Equal(t, []string{"ssl/", "ssl/a.pem: a", "ssl/abs -> /etc/ssl"}, tree(t, out))
	})

	t.Run("modes and times are as the archive says, but for a directory that was there", func(t *testing.T) {
		t.Parallel()

		out := t.TempDir()
		then := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
		data := archiveOf(t,
			entry{name: "ssl/", kind: tar.TypeDir, mode: 0o750, modTime: then},
			entry{name: "ssl/key.pem", kind: tar.TypeReg, body: "k", mode: 0o600, modTime: then},
			entry{name: "ssl/locked/", kind: tar.TypeDir, mode: 0o500, modTime: then},
			entry{name: "ssl/locked/x", kind: tar.TypeReg, body: "x", mode: 0o444, modTime: then},
		)

		require.NoError(t, os.Chmod(out, 0o711))
		require.NoError(t, unpackInto(t, archiveOf(t, entry{name: "./", kind: tar.TypeDir, mode: 0o700}), ".", out))
		require.NoError(t, unpackInto(t, data, "ssl", out))

		for name, want := range map[string]fs.FileMode{".": 0o711, "ssl": 0o750, "ssl/key.pem": 0o600, "ssl/locked": 0o500, "ssl/locked/x": 0o444} {
			info, err := os.Stat(filepath.Join(out, name))
			require.NoError(t, err, name)
			assert.Equal(t, want, info.Mode().Perm(), name)

			if name != "." {
				assert.True(t, then.Equal(info.ModTime()), name)
			}
		}
	})

	t.Run("an archive cut short is an error", func(t *testing.T) {
		t.Parallel()

		data := archiveOf(t, dirEntry("ssl/"), fileEntry("ssl/a.pem", strings.Repeat("a", 2048)))

		err := unpackInto(t, data[:1024+512+100], "ssl", t.TempDir())

		assert.ErrorIs(t, err, io.ErrUnexpectedEOF)
	})

	t.Run("an archive whose second header is garbage is an error", func(t *testing.T) {
		t.Parallel()

		data := archiveOf(t, dirEntry("ssl/"))
		data = append(data[:512], bytes.Repeat([]byte{'x'}, 512)...)

		err := unpackInto(t, data, "ssl", t.TempDir())

		assert.ErrorContains(t, err, "reading the archive")
	})

	t.Run("an empty archive is an error", func(t *testing.T) {
		t.Parallel()

		err := unpackInto(t, archiveOf(t), "ssl", t.TempDir())

		assert.EqualError(t, err, "the archive is empty")
	})

	t.Run("an archive that is not one is an error", func(t *testing.T) {
		t.Parallel()

		err := unpackInto(t, bytes.Repeat([]byte{'x'}, 1024), "ssl", t.TempDir())

		assert.ErrorContains(t, err, "reading the archive")
	})

	t.Run("an archive of the root is one of its contents", func(t *testing.T) {
		t.Parallel()

		out := t.TempDir()

		require.NoError(t, unpackInto(t, archiveOf(t, dirEntry("/"), fileEntry("/hosts", "h")), ".", out))
		assert.Equal(t, []string{"hosts: h"}, tree(t, out))
	})

	t.Run("an archive of a directory's contents whose first entry is in it", func(t *testing.T) {
		t.Parallel()

		out := t.TempDir()

		require.NoError(t, unpackInto(t, archiveOf(t, fileEntry("./a", "a"), fileEntry("./b/c", "c")), ".", filepath.Join(out, "copy")))
		assert.Equal(t, []string{"copy/", "copy/a: a", "copy/b/", "copy/b/c: c"}, tree(t, out))
	})

	t.Run("names too long to be made are errors", func(t *testing.T) {
		t.Parallel()

		long := strings.Repeat("n", 300)

		for _, data := range [][]byte{
			archiveOf(t, dirEntry("ssl/"), fileEntry("ssl/"+long, "x")),
			archiveOf(t, dirEntry("ssl/"), dirEntry("ssl/"+long+"/")),
			archiveOf(t, dirEntry("ssl/"), fileEntry("ssl/"+long+"/x", "x")),
		} {
			err := unpackInto(t, data, "ssl", t.TempDir())

			assert.ErrorContains(t, err, "file name too long")
		}
	})

	t.Run("a directory to unpack into that is not there is an error", func(t *testing.T) {
		t.Parallel()

		data := archiveOf(t, fileEntry("hosts", "x"))
		archive := tar.NewReader(bytes.NewReader(data))

		first, err := archive.Next()
		require.NoError(t, err)

		assert.Error(t, extract(archive, first, filepath.Join(t.TempDir(), "missing"), "hosts", "hosts"))
	})
}
