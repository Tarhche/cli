package desktop_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	testify "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/desktop"
	"github.com/Tarhche/cli/internal/desktop/mock"
)

func TestFiles(t *testing.T) {
	t.Parallel()

	container := desktop.Target{VM: "v-1", Container: "web"}

	newFiles := func(t *testing.T) (*desktop.Files, *mock.Copier, *mock.Chooser, *desktop.Workspace) {
		t.Helper()

		w, _ := newWorkspace(t, nil)
		copier, chooser := &mock.Copier{}, &mock.Chooser{}
		t.Cleanup(func() {
			copier.AssertExpectations(t)
			chooser.AssertExpectations(t)
		})

		return desktop.NewFiles(w, copier, chooser), copier, chooser, w
	}

	t.Run("what is chosen on the host is copied into the workload", func(t *testing.T) {
		t.Parallel()

		files, copier, chooser, w := newFiles(t)
		chooser.On("Open", "Copy into /tmp/").Return("/home/me/notes.txt", nil).Once()
		copier.On("CopyIn", testify.Anything, w.Cli(), desktop.CopyRequest{Target: container, Path: "/tmp/", LocalPath: "/home/me/notes.txt"}).Return(nil).Once()

		copied, err := files.CopyIn(context.Background(), container, "/tmp/")

		require.NoError(t, err)
		assert.Equal(t, "/home/me/notes.txt", copied)
	})

	t.Run("what is in the workload is copied where it is chosen on the host", func(t *testing.T) {
		t.Parallel()

		files, copier, chooser, w := newFiles(t)
		chooser.On("Save", "Copy /etc/hostname to", "hostname").Return("/home/me/hostname", nil).Once()
		copier.On("CopyOut", testify.Anything, w.Cli(), desktop.CopyRequest{Target: container, Path: "/etc/hostname", LocalPath: "/home/me/hostname"}).Return(nil).Once()

		copied, err := files.CopyOut(context.Background(), container, "/etc/hostname")

		require.NoError(t, err)
		assert.Equal(t, "/home/me/hostname", copied)
	})

	t.Run("nothing chosen copies nothing", func(t *testing.T) {
		t.Parallel()

		files, _, chooser, _ := newFiles(t)
		chooser.On("Open", "Copy into /tmp/").Return("", nil).Once()
		chooser.On("Save", "Copy /etc/hostname to", "hostname").Return("", nil).Once()

		in, inErr := files.CopyIn(context.Background(), container, "/tmp/")
		out, outErr := files.CopyOut(context.Background(), container, "/etc/hostname")

		require.NoError(t, inErr)
		require.NoError(t, outErr)
		assert.Empty(t, in)
		assert.Empty(t, out)
	})

	t.Run("a dialog that fails copies nothing", func(t *testing.T) {
		t.Parallel()

		files, _, chooser, _ := newFiles(t)
		failure := errors.New("no dialog")
		chooser.On("Open", "Copy into /tmp/").Return("", failure).Once()
		chooser.On("Save", "Copy /etc/hostname to", "hostname").Return("", failure).Once()

		_, inErr := files.CopyIn(context.Background(), container, "/tmp/")
		_, outErr := files.CopyOut(context.Background(), container, "/etc/hostname")

		assert.ErrorIs(t, inErr, failure)
		assert.ErrorIs(t, outErr, failure)
	})

	t.Run("a copy that fails says why", func(t *testing.T) {
		t.Parallel()

		files, copier, chooser, _ := newFiles(t)
		failure := errors.New("no such file")
		chooser.On("Open", "Copy into /tmp/").Return("/home/me/notes.txt", nil).Once()
		chooser.On("Save", "Copy /etc/hostname to", "hostname").Return("/home/me/hostname", nil).Once()
		copier.On("CopyIn", testify.Anything, testify.Anything, testify.Anything).Return(failure).Once()
		copier.On("CopyOut", testify.Anything, testify.Anything, testify.Anything).Return(failure).Once()

		in, inErr := files.CopyIn(context.Background(), container, "/tmp/")
		out, outErr := files.CopyOut(context.Background(), container, "/etc/hostname")

		assert.ErrorIs(t, inErr, failure)
		assert.ErrorIs(t, outErr, failure)
		assert.Empty(t, in)
		assert.Empty(t, out)
	})
}
