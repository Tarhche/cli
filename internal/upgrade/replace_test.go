package upgrade

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModeOf(t *testing.T) {
	t.Parallel()

	t.Run("is the mode of the binary", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, fs.FileMode(0o750), modeOf(installed(t, "tarhche", []byte("binary"), 0o750)))
	})

	t.Run("is 0755 for a binary that is not there", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, fs.FileMode(0o755), modeOf(filepath.Join(t.TempDir(), "missing")))
	})

	t.Run("is 0755 for a binary nobody may do anything with", func(t *testing.T) {
		t.Parallel()

		binary := installed(t, "tarhche", []byte("binary"), 0o755)
		require.NoError(t, os.Chmod(binary, 0))

		assert.Equal(t, fs.FileMode(0o755), modeOf(binary))
	})
}
