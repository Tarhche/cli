package remote

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/stream/streamtest"
)

func TestCopyVMCommand(t *testing.T) {
	t.Parallel()

	needs(t, "sh", "tar", "readlink")

	// setup is a vm whose filesystem is this machine's, files made in a
	// directory of it, beside a local directory of the same files.
	setup := func(t *testing.T, input string) (*harnessed, string, string) {
		t.Helper()

		vm, local := t.TempDir(), t.TempDir()
		files(t, vm)
		files(t, local)

		ingress := &streamtest.Ingress{Exec: streamtest.Run}
		base, r, terminal := harness(t, ingress, input)

		return &harnessed{Harness: base, remote: r, terminal: terminal, ingress: ingress}, vm, local
	}

	t.Run("a file out of the vm, to a new name", func(t *testing.T) {
		t.Parallel()

		h, vm, local := setup(t, "")

		assert.Zero(t, h.Run(newCopyVMCommand(h.remote), "machine:"+filepath.Join(vm, "file.txt"), filepath.Join(local, "copy.txt")), h.Err.String())
		assert.Equal(t, "hello", read(t, filepath.Join(local, "copy.txt")))
		assert.Empty(t, h.Err.String())

		for _, request := range h.ingress.Requests() {
			assert.Equal(t, machine.UUID, request.UUID)
		}
	})

	t.Run("a directory into the vm, into a directory there", func(t *testing.T) {
		t.Parallel()

		h, vm, local := setup(t, "")

		assert.Zero(t, h.Run(newCopyVMCommand(h.remote), filepath.Join(local, "dir"), "machine:"+filepath.Join(vm, "out")), h.Err.String())
		assert.Equal(t, "a", read(t, filepath.Join(vm, "out", "dir", "a.txt")))
		assert.True(t, slices.ContainsFunc(h.ingress.Requests(), func(request streamtest.Request) bool {
			return slices.Contains(payloadOf(t, request).Command, "-o")
		}), "unpacked as whoever unpacks it")
	})

	t.Run("owners kept, and a symlink followed, when asked", func(t *testing.T) {
		t.Parallel()

		h, vm, local := setup(t, "")

		assert.Zero(t, h.Run(newCopyVMCommand(h.remote), "-a", filepath.Join(local, "file.txt"), "machine:"+filepath.Join(vm, "out")), h.Err.String())
		assert.False(t, slices.ContainsFunc(h.ingress.Requests(), func(request streamtest.Request) bool {
			return slices.Contains(payloadOf(t, request).Command, "-o")
		}), "unpacked keeping its owners")

		h, vm, local = setup(t, "")

		assert.Zero(t, h.Run(newCopyVMCommand(h.remote), "-L", "machine:"+filepath.Join(vm, "to-file"), filepath.Join(local, "out")), h.Err.String())

		info, err := os.Lstat(filepath.Join(local, "out", "to-file"))
		require.NoError(t, err)
		assert.True(t, info.Mode().IsRegular())
	})

	t.Run("an archive out to the output, and one in from the input", func(t *testing.T) {
		t.Parallel()

		h, vm, _ := setup(t, "")

		assert.Zero(t, h.Run(newCopyVMCommand(h.remote), "machine:"+filepath.Join(vm, "dir"), "-"), h.Err.String())
		assert.Equal(t, map[string]string{"dir/": "", "dir/a.txt": "a"}, archived(t, h.Out.Bytes()))

		h, vm, _ = setup(t, string(archiveOf(t, "piped.txt", "piped")))

		assert.Zero(t, h.Run(newCopyVMCommand(h.remote), "-", "machine:"+filepath.Join(vm, "out")), h.Err.String())
		assert.Equal(t, "piped", read(t, filepath.Join(vm, "out", "piped.txt")))
	})

	t.Run("how much was copied is said on a terminal, unless quiet", func(t *testing.T) {
		t.Parallel()

		h, vm, local := setup(t, "")
		h.terminal.terminals[h.Cli.Err] = true

		destination := "machine:" + filepath.Join(vm, "out")

		assert.Zero(t, h.Run(newCopyVMCommand(h.remote), filepath.Join(local, "file.txt"), destination), h.Err.String())
		assert.Equal(t, "Successfully copied 2KiB to "+destination+"\n", h.Err.String())

		h, vm, local = setup(t, "")
		h.terminal.terminals[h.Cli.Err] = true

		assert.Zero(t, h.Run(newCopyVMCommand(h.remote), "-q", filepath.Join(local, "file.txt"), "machine:"+filepath.Join(vm, "out")), h.Err.String())
		assert.Empty(t, h.Err.String())

		h, vm, _ = setup(t, "")
		h.terminal.terminals[h.Cli.Err] = true

		assert.Zero(t, h.Run(newCopyVMCommand(h.remote), "machine:"+filepath.Join(vm, "file.txt"), "-"), h.Err.String())
		assert.Empty(t, h.Err.String())
	})

	t.Run("a file that is not there is said as docker says it", func(t *testing.T) {
		t.Parallel()

		h, vm, local := setup(t, "")
		missing := filepath.Join(vm, "missing")

		assert.Equal(t, 1, h.Run(newCopyVMCommand(h.remote), "machine:"+missing, local))
		assert.Equal(t, "Error: could not find the file "+missing+" in vm machine\n", h.Err.String())
	})

	t.Run("what cannot be copied is the error", func(t *testing.T) {
		t.Parallel()

		h, vm, local := setup(t, "")
		assert.Equal(t, 1, h.Run(newCopyVMCommand(h.remote), "machine:"+filepath.Join(vm, "dir"), filepath.Join(local, "file.txt")))
		assert.Equal(t, "Error: cannot copy a directory onto a file: "+filepath.Join(local, "file.txt")+"\n", h.Err.String())

		h, _, _ = setup(t, "")
		assert.Equal(t, 1, h.Run(newCopyVMCommand(h.remote), "machine:/a", "web:/b"))
		assert.Equal(t, "Error: copying between VMs is not supported\n", h.Err.String())

		h, _, _ = setup(t, "")
		assert.Equal(t, 1, h.Run(newCopyVMCommand(h.remote), "nope:/a", "b"))
		assert.Equal(t, "Error: no such vm: nope\n", h.Err.String())

		h, _, _ = setup(t, "")
		withoutIngress(t, h.Harness)
		assert.Equal(t, 1, h.Run(newCopyVMCommand(h.remote), "machine:/a", "b"))
		assert.True(t, strings.HasPrefix(h.Err.String(), "Error: the context \"test\" names no workload ingress"))
	})

	t.Run("a source and a destination are required", func(t *testing.T) {
		t.Parallel()

		h, _, _ := setup(t, "")

		assert.Equal(t, 2, h.Run(newCopyVMCommand(h.remote), "machine:/a"))
	})
}
