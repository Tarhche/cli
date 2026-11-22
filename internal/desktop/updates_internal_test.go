package desktop

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	testify "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/Tarhche/cli/internal/upgrade"
	upgrademock "github.com/Tarhche/cli/internal/upgrade/mock"
)

func TestRestartCommand(t *testing.T) {
	t.Parallel()

	bundle := upgrade.Installation{Version: "0.2.0", Path: "/Applications/Tarhche.app", Executable: "/Applications/Tarhche.app/Contents/MacOS/tarhche"}
	binary := upgrade.Installation{Version: "0.2.0", Path: "/usr/local/bin/tarhche", Executable: "/usr/local/bin/tarhche"}
	exe := upgrade.Installation{Version: "0.2.0", Path: `C:\Tools\tarhche.exe`, Executable: `C:\Tools\tarhche.exe`}

	cases := []struct {
		name      string
		goos      string
		installed upgrade.Installation
		command   string
		args      []string
	}{
		{name: "an app bundle on macOS is opened as an app", goos: "darwin", installed: bundle, command: "open", args: []string{"-n", "/Applications/Tarhche.app", "--args", "--desktop"}},
		{name: "a binary on macOS is started with --desktop", goos: "darwin", installed: binary, command: "/usr/local/bin/tarhche", args: []string{"--desktop"}},
		{name: "a binary on Linux is started with --desktop", goos: "linux", installed: binary, command: "/usr/local/bin/tarhche", args: []string{"--desktop"}},
		{name: "a binary on Windows is started with --desktop", goos: "windows", installed: exe, command: `C:\Tools\tarhche.exe`, args: []string{"--desktop"}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			command, args := restartCommand(c.goos, c.installed)

			assert.Equal(t, c.command, command)
			assert.Equal(t, c.args, args)
		})
	}
}

func TestUpdatesRestart(t *testing.T) {
	t.Parallel()

	release := upgrade.Release{Version: "0.2.0", Tag: "v0.2.0"}
	bundle := upgrade.Installation{Version: "0.2.0", Path: "/Applications/Tarhche.app", Executable: "/Applications/Tarhche.app/Contents/MacOS/tarhche"}

	// started is what a Starter was asked to start.
	type started struct {
		name string
		args []string
	}

	newUpdates := func(t *testing.T, startErr error) (*Updates, chan started, chan struct{}) {
		t.Helper()

		upgrader := &upgrademock.Upgrader{}
		upgrader.On("Release", testify.Anything, "0.2.0").Return(release, nil).Once()
		upgrader.On("Install", testify.Anything, release, testify.Anything).Return(bundle, nil).Once()
		t.Cleanup(func() { upgrader.AssertExpectations(t) })

		starts := make(chan started, 1)
		quits := make(chan struct{}, 1)

		updates := NewUpdates(upgrader, nil, func() { quits <- struct{}{} })
		updates.goos = "darwin"
		updates.start = func(name string, args ...string) error {
			starts <- started{name: name, args: slices.Clone(args)}

			return startErr
		}

		_, err := updates.Upgrade(context.Background(), "0.2.0")
		require.NoError(t, err)

		return updates, starts, quits
	}

	t.Run("it starts what was installed, and quits", func(t *testing.T) {
		t.Parallel()

		updates, starts, quits := newUpdates(t, nil)

		require.NoError(t, updates.Restart())

		assert.Equal(t, started{name: "open", args: []string{"-n", "/Applications/Tarhche.app", "--args", "--desktop"}}, <-starts)
		<-quits
	})

	t.Run("it does not quit when what was installed could not be started", func(t *testing.T) {
		t.Parallel()

		refused := errors.New("no such file")
		updates, starts, quits := newUpdates(t, refused)

		err := updates.Restart()

		require.ErrorIs(t, err, refused)
		assert.Contains(t, err.Error(), "starting tarhche 0.2.0")
		<-starts
		assert.Empty(t, quits)
	})
}

func TestUpdatesExplained(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		goos   string
		failed error
		want   string
	}{
		{name: "anywhere but Windows, with sudo", goos: "linux", failed: fmt.Errorf("cannot replace /usr/local/bin/tarhche: %w", upgrade.ErrPermission), want: "cannot replace /usr/local/bin/tarhche: permission denied: only an administrator may, so run sudo tarhche upgrade in a terminal"},
		{name: "on Windows, in an administrator's terminal", goos: "windows", failed: fmt.Errorf(`cannot replace C:\Program Files\Tarhche\tarhche.exe: %w`, upgrade.ErrPermission), want: `cannot replace C:\Program Files\Tarhche\tarhche.exe: permission denied: only an administrator may, so run tarhche upgrade in a terminal opened as administrator`},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			updates := NewUpdates(&upgrademock.Upgrader{}, nil, func() {})
			updates.goos = c.goos

			err := updates.explained(c.failed)

			require.ErrorIs(t, err, upgrade.ErrPermission)
			assert.EqualError(t, err, c.want)
		})
	}

	t.Run("any other failure is said as it is", func(t *testing.T) {
		t.Parallel()

		broken := errors.New("the archive is broken")

		assert.Equal(t, broken, NewUpdates(&upgrademock.Upgrader{}, nil, func() {}).explained(broken))
	})
}

// window is a window a test brings forward, which notes what it was asked.
type window struct {
	asked []string
}

func (w *window) UnMinimise() { w.asked = append(w.asked, "unminimise") }

func (w *window) Show() application.Window {
	w.asked = append(w.asked, "show")

	return nil
}

func (w *window) Focus() { w.asked = append(w.asked, "focus") }

// emitter is an Emitter a test hears the events of.
type emitter struct {
	emitted []string
}

func (e *emitter) Emit(name string, _ ...any) bool {
	e.emitted = append(e.emitted, name)

	return false
}

func TestCheckForUpdates(t *testing.T) {
	t.Parallel()

	t.Run("the window comes forward and its frontend is asked to look for a newer release", func(t *testing.T) {
		t.Parallel()

		w := &window{}
		e := &emitter{}

		checkForUpdates(w, e)

		assert.Equal(t, []string{"unminimise", "show", "focus"}, w.asked)
		assert.Equal(t, []string{EventCheckForUpdates}, e.emitted)
	})
}
