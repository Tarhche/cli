package desktop_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	testify "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/desktop"
	"github.com/Tarhche/cli/internal/desktop/mock"
	"github.com/Tarhche/cli/internal/upgrade"
	upgrademock "github.com/Tarhche/cli/internal/upgrade/mock"
	"github.com/Tarhche/cli/internal/version"
)

func TestUpdates(t *testing.T) {
	t.Parallel()

	published := time.Date(2026, 11, 30, 9, 0, 0, 0, time.UTC)
	newer := upgrade.Release{
		Version:     "0.2.0",
		Tag:         "v0.2.0",
		Name:        "v0.2.0",
		Notes:       "## Changelog\n### Features\n* 1a2b3c4 Feat: upgrades itself",
		URL:         "https://github.com/Tarhche/cli/releases/tag/v0.2.0",
		PublishedAt: published,
		Assets:      []upgrade.Asset{{Name: "tarhche_0.2.0_linux_amd64.tar.gz", URL: "https://example.com/a", Size: 1000}},
	}
	shown := desktop.Release{
		Version:     "0.2.0",
		Name:        "v0.2.0",
		Notes:       newer.Notes,
		URL:         newer.URL,
		PublishedAt: published,
	}
	installed := upgrade.Installation{Version: "0.2.0", Path: "/home/sarah/bin/tarhche", Executable: "/home/sarah/bin/tarhche"}

	newUpdates := func(t *testing.T) (*desktop.Updates, *upgrademock.Upgrader, *mock.Emitter) {
		t.Helper()

		upgrader := &upgrademock.Upgrader{}
		emitter := mock.NewEmitter()
		t.Cleanup(func() { upgrader.AssertExpectations(t) })

		return desktop.NewUpdates(upgrader, emitter, func() {}), upgrader, emitter
	}

	checked := func(upgrader *upgrademock.Upgrader, available bool) {
		upgrader.On("Check", testify.Anything).Return(upgrade.Check{Current: "0.1.0", Latest: newer, Available: available}, nil).Once()
		upgrader.On("Target").Return("/home/sarah/bin/tarhche", nil).Once()
	}

	t.Run("Version is the version running", func(t *testing.T) {
		t.Parallel()

		updates, _, _ := newUpdates(t)

		assert.Equal(t, version.Version, updates.Version())
	})

	t.Run("Check says a newer release, and what upgrading to it replaces", func(t *testing.T) {
		t.Parallel()

		updates, upgrader, _ := newUpdates(t)
		checked(upgrader, true)

		update, err := updates.Check(context.Background())

		require.NoError(t, err)
		assert.Equal(t, desktop.Update{Current: "0.1.0", Available: true, Latest: shown, Target: "/home/sarah/bin/tarhche"}, update)
	})

	t.Run("Check says when the version running is the latest", func(t *testing.T) {
		t.Parallel()

		updates, upgrader, _ := newUpdates(t)
		checked(upgrader, false)

		update, err := updates.Check(context.Background())

		require.NoError(t, err)
		assert.False(t, update.Available)
		assert.Equal(t, shown, update.Latest)
	})

	t.Run("Check says why a build cannot upgrade itself", func(t *testing.T) {
		t.Parallel()

		updates, upgrader, _ := newUpdates(t)
		upgrader.On("Check", testify.Anything).Return(upgrade.Check{Current: "0.1.0", Latest: newer, Available: true}, nil).Once()
		upgrader.On("Target").Return("", upgrade.ErrUnsupported).Once()

		update, err := updates.Check(context.Background())

		require.NoError(t, err)
		assert.Equal(t, upgrade.ErrUnsupported.Error(), update.Unsupported)
		assert.Empty(t, update.Target)
	})

	t.Run("Check fails as reading the latest release does", func(t *testing.T) {
		t.Parallel()

		updates, upgrader, _ := newUpdates(t)
		refused := errors.New("GitHub is unreachable")
		upgrader.On("Check", testify.Anything).Return(upgrade.Check{}, refused).Once()

		_, err := updates.Check(context.Background())

		require.ErrorIs(t, err, refused)
	})

	t.Run("Check fails when what upgrading replaces cannot be told", func(t *testing.T) {
		t.Parallel()

		updates, upgrader, _ := newUpdates(t)
		lost := errors.New("no executable")
		upgrader.On("Check", testify.Anything).Return(upgrade.Check{Current: "0.1.0", Latest: newer, Available: true}, nil).Once()
		upgrader.On("Target").Return("", lost).Once()

		_, err := updates.Check(context.Background())

		require.ErrorIs(t, err, lost)
	})

	t.Run("Upgrade installs the release the last check found, telling how much of it arrived", func(t *testing.T) {
		t.Parallel()

		updates, upgrader, emitter := newUpdates(t)
		checked(upgrader, true)
		upgrader.On("Install", testify.Anything, newer, testify.Anything).Run(func(args testify.Arguments) {
			progress := args.Get(2).(upgrade.Progress)
			progress(0, 1000)
			progress(5, 1000)
			progress(500, 1000)
			progress(1000, 1000)
		}).Return(installed, nil).Once()

		_, err := updates.Check(context.Background())
		require.NoError(t, err)

		upgraded, err := updates.Upgrade(context.Background(), "v0.2.0")

		require.NoError(t, err)
		assert.Equal(t, desktop.Upgraded{Version: "0.2.0", Path: "/home/sarah/bin/tarhche"}, upgraded)

		for _, arrived := range []int64{0, 500, 1000} {
			assert.Equal(t, mock.Event{Name: desktop.EventUpdateProgress, Data: desktop.UpdateProgress{Version: "0.2.0", Done: arrived, Total: 1000}}, emitter.Next(t))
		}

		assert.True(t, emitter.Quiet())
	})

	t.Run("Upgrade tells how much arrived by the mebibyte while how much there is is not known", func(t *testing.T) {
		t.Parallel()

		updates, upgrader, emitter := newUpdates(t)
		upgrader.On("Release", testify.Anything, "0.2.0").Return(newer, nil).Once()
		upgrader.On("Install", testify.Anything, newer, testify.Anything).Run(func(args testify.Arguments) {
			progress := args.Get(2).(upgrade.Progress)
			progress(0, 0)
			progress(512<<10, 0)
			progress(1<<20, 0)
			progress(3<<19, 0)
		}).Return(installed, nil).Once()

		_, err := updates.Upgrade(context.Background(), "0.2.0")

		require.NoError(t, err)
		assert.Equal(t, desktop.UpdateProgress{Version: "0.2.0"}, emitter.Next(t).Data)
		assert.Equal(t, desktop.UpdateProgress{Version: "0.2.0", Done: 1 << 20}, emitter.Next(t).Data)
		assert.True(t, emitter.Quiet())
	})

	t.Run("Upgrade reads a release no check found", func(t *testing.T) {
		t.Parallel()

		updates, upgrader, _ := newUpdates(t)
		checked(upgrader, true)
		older := upgrade.Release{Version: "0.1.5", Tag: "v0.1.5"}
		upgrader.On("Release", testify.Anything, "0.1.5").Return(older, nil).Once()
		upgrader.On("Install", testify.Anything, older, testify.Anything).Return(upgrade.Installation{Version: "0.1.5", Path: "/home/sarah/bin/tarhche"}, nil).Once()

		_, err := updates.Check(context.Background())
		require.NoError(t, err)

		upgraded, err := updates.Upgrade(context.Background(), "0.1.5")

		require.NoError(t, err)
		assert.Equal(t, "0.1.5", upgraded.Version)
	})

	t.Run("Upgrade fails as reading the release does, installing nothing", func(t *testing.T) {
		t.Parallel()

		updates, upgrader, _ := newUpdates(t)
		upgrader.On("Release", testify.Anything, "9.9.9").Return(upgrade.Release{}, upgrade.ErrNoRelease).Once()

		_, err := updates.Upgrade(context.Background(), "9.9.9")

		require.ErrorIs(t, err, upgrade.ErrNoRelease)
	})

	t.Run("Upgrade fails as installing does, and may be tried again", func(t *testing.T) {
		t.Parallel()

		updates, upgrader, _ := newUpdates(t)
		upgrader.On("Release", testify.Anything, "0.2.0").Return(newer, nil).Twice()
		upgrader.On("Install", testify.Anything, newer, testify.Anything).Return(upgrade.Installation{}, upgrade.ErrChecksum).Once()
		upgrader.On("Install", testify.Anything, newer, testify.Anything).Return(installed, nil).Once()

		_, err := updates.Upgrade(context.Background(), "0.2.0")
		require.ErrorIs(t, err, upgrade.ErrChecksum)

		upgraded, err := updates.Upgrade(context.Background(), "0.2.0")

		require.NoError(t, err)
		assert.Equal(t, "0.2.0", upgraded.Version)
	})

	t.Run("Upgrade says how to upgrade where only an administrator may write", func(t *testing.T) {
		t.Parallel()

		updates, upgrader, _ := newUpdates(t)
		upgrader.On("Release", testify.Anything, "0.2.0").Return(newer, nil).Once()
		upgrader.On("Install", testify.Anything, newer, testify.Anything).Return(upgrade.Installation{}, fmt.Errorf("replacing /usr/local/bin/tarhche: %w", upgrade.ErrPermission)).Once()
		upgrader.On("Target").Return("/usr/local/bin/tarhche", nil).Once()

		_, err := updates.Upgrade(context.Background(), "0.2.0")

		require.ErrorIs(t, err, upgrade.ErrPermission)
		assert.Contains(t, err.Error(), "only an administrator may replace /usr/local/bin/tarhche, so upgrade it with ")
	})

	t.Run("a second upgrade is refused while one is under way", func(t *testing.T) {
		t.Parallel()

		updates, upgrader, _ := newUpdates(t)
		installing := make(chan struct{})
		finish := make(chan struct{})
		upgrader.On("Release", testify.Anything, "0.2.0").Return(newer, nil).Once()
		upgrader.On("Install", testify.Anything, newer, testify.Anything).Run(func(testify.Arguments) {
			close(installing)
			<-finish
		}).Return(installed, nil).Once()

		first := make(chan error, 1)
		go func() {
			_, err := updates.Upgrade(context.Background(), "0.2.0")
			first <- err
		}()
		<-installing

		_, err := updates.Upgrade(context.Background(), "0.2.0")
		require.ErrorIs(t, err, desktop.ErrUpgrading)

		close(finish)
		require.NoError(t, <-first)
	})

	t.Run("Restart is refused before an upgrade installed anything", func(t *testing.T) {
		t.Parallel()

		updates, _, _ := newUpdates(t)

		require.ErrorIs(t, updates.Restart(), desktop.ErrNothingInstalled)
	})
}
