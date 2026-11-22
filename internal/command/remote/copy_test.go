package remote

import (
	"archive/tar"
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSplitCopyArg(t *testing.T) {
	t.Parallel()

	cases := map[string][2]string{
		"web:/etc/hosts":    {"web", "/etc/hosts"},
		"web:relative":      {"web", "relative"},
		"web:":              {"web", ""},
		"relative/file":     {"", "relative/file"},
		"./file:with-colon": {"", "./file:with-colon"},
		"-":                 {"", "-"},
	}

	if runtime.GOOS != "windows" {
		cases["/abs/file:with-colon"] = [2]string{"", "/abs/file:with-colon"}
	}

	for arg, want := range cases {
		reference, path := splitCopyArg(arg)
		assert.Equal(t, want, [2]string{reference, path}, arg)
	}
}

func TestPlanCopy(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		src  string
		dst  string
		want copyPlan
		err  string
	}{
		{name: "out of a workload", src: "web:/etc/hosts", dst: "hosts", want: copyPlan{download: true, reference: "web", remotePath: "/etc/hosts", localPath: "hosts"}},
		{name: "out of a workload, to the output", src: "web:/etc", dst: "-", want: copyPlan{download: true, reference: "web", remotePath: "/etc", localPath: "-"}},
		{name: "into a workload", src: "hosts", dst: "web:/etc/", want: copyPlan{reference: "web", remotePath: "/etc/", localPath: "hosts"}},
		{name: "into a workload, from the input", src: "-", dst: "web:/tmp", want: copyPlan{reference: "web", remotePath: "/tmp", localPath: "-"}},
		{name: "between workloads", src: "web:/a", dst: "db:/b", err: "copying between VMs is not supported"},
		{name: "between local paths", src: "a", dst: "b", err: "must specify at least one VM source"},
		{name: "from nothing", src: "", dst: "web:/b", err: "source can not be empty"},
		{name: "to nothing", src: "web:/a", dst: "", err: "destination can not be empty"},
		{name: "a workload without a path", src: "web:", dst: "b", err: "the path in web: can not be empty"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			got, err := planCopy(c.src, c.dst, "VM")

			if len(c.err) > 0 {
				assert.EqualError(t, err, c.err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, c.want, got)
		})
	}
}

// needs skips a test when a tool it runs is not on this machine.
func needs(t *testing.T, tools ...string) {
	t.Helper()

	for _, tool := range tools {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is not on this machine", tool)
		}
	}
}

// files makes, under root, a directory, dir, holding a.txt, a file,
// file.txt, a symlink to it, and an empty directory, out.
func files(t *testing.T, root string) {
	t.Helper()

	require.NoError(t, os.MkdirAll(filepath.Join(root, "dir"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "dir", "a.txt"), []byte("a"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "file.txt"), []byte("hello"), 0o644))
	require.NoError(t, os.Symlink("file.txt", filepath.Join(root, "to-file")))
	require.NoError(t, os.Mkdir(filepath.Join(root, "out"), 0o755))
}

// read is what the file at path holds.
func read(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	require.NoError(t, err)

	return string(content)
}

// archived are the names and contents of the entries of a tar archive.
func archived(t *testing.T, data []byte) map[string]string {
	t.Helper()

	found := map[string]string{}
	archive := tar.NewReader(bytes.NewReader(data))

	for {
		header, err := archive.Next()
		if errors.Is(err, io.EOF) {
			return found
		}

		require.NoError(t, err)

		content, err := io.ReadAll(archive)
		require.NoError(t, err)

		found[header.Name] = string(content)
	}
}

// archiveOf is a tar archive of one file.
func archiveOf(t *testing.T, name, content string) []byte {
	t.Helper()

	var buffer bytes.Buffer

	archive := tar.NewWriter(&buffer)
	require.NoError(t, archive.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeReg, Mode: 0o644, Size: int64(len(content))}))

	_, err := io.WriteString(archive, content)
	require.NoError(t, err)
	require.NoError(t, archive.Close())

	return buffer.Bytes()
}
