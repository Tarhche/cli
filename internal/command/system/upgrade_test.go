package system

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/danceable/console"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/command/commandtest"
	"github.com/Tarhche/cli/internal/upgrade"
	"github.com/Tarhche/cli/internal/upgrade/upgradetest"
)

// linux is the platform the upgrades of these tests replace a build of.
var linux = upgrade.Platform{OS: "linux", Arch: "amd64"}

// threeDaysAgo is when the releases of these tests were published.
var threeDaysAgo = time.Now().Add(-72 * time.Hour)

// release is a release of version that holds a build of tarhche for linux
// whose binary holds body.
func release(t *testing.T, version, body string) upgradetest.Release {
	t.Helper()

	archive := upgradetest.TarGz(t, map[string]upgradetest.File{"tarhche": {Body: []byte(body), Mode: 0o755}})

	return upgradetest.Release{
		Version:     version,
		Name:        "v" + version,
		Notes:       "## Changelog\r\n* what changed in " + version + "\r\n",
		PublishedAt: threeDaysAgo,
		Files:       upgradetest.Checksummed(version, map[string][]byte{"tarhche_" + version + "_linux_amd64.tar.gz": archive}),
	}
}

// upgradeHarness is an upgrade command whose binary is the test's own,
// holding old, and whose releases github publishes.
type upgradeHarness struct {
	*commandtest.Harness

	command *upgradeCommand
	binary  string
	github  *upgradetest.GitHub
}

// newUpgradeHarness is an upgrade command running current, whose releases
// are releases.
func newUpgradeHarness(t *testing.T, current string, releases ...upgradetest.Release) *upgradeHarness {
	t.Helper()

	binary := filepath.Join(t.TempDir(), "tarhche")
	require.NoError(t, os.WriteFile(binary, []byte("old"), 0o755))

	github := upgradetest.New(t, releases...)
	h := commandtest.New(t, nil)

	upgrader := upgrade.New(upgrade.WithAPI(github.URL), upgrade.WithExecutable(binary), upgrade.WithCurrent(current), upgrade.WithPlatform(linux))

	cmd := newUpgradeCommand(h.Cli, upgrader)
	cmd.interactive = func() bool { return false }
	cmd.terminal = func() bool { return false }
	cmd.goos = "linux"

	return &upgradeHarness{Harness: h, command: cmd, binary: binary, github: github}
}

// installed is what the binary holds.
func (h *upgradeHarness) installed(t *testing.T) string {
	t.Helper()

	content, err := os.ReadFile(h.binary)
	require.NoError(t, err)

	return string(content)
}

func TestUpgradeCommand(t *testing.T) {
	t.Parallel()

	t.Run("says when the version running is the latest", func(t *testing.T) {
		t.Parallel()

		h := newUpgradeHarness(t, "0.2.0", release(t, "0.2.0", "same"))

		require.Equal(t, console.ExitSuccess, h.Run(h.command), h.Err.String())

		assert.Equal(t, "tarhche 0.2.0 is the latest version.\n", h.Out.String())
		assert.Equal(t, "old", h.installed(t))
	})

	t.Run("says when the version running is newer than the latest release", func(t *testing.T) {
		t.Parallel()

		h := newUpgradeHarness(t, "0.3.0-rc.1", release(t, "0.2.0", "older"))

		require.Equal(t, console.ExitSuccess, h.Run(h.command), h.Err.String())

		assert.Equal(t, "tarhche 0.3.0-rc.1 is newer than its latest release, 0.2.0.\n", h.Out.String())
	})

	t.Run("tells what a newer release says, and stops when told to only check", func(t *testing.T) {
		t.Parallel()

		h := newUpgradeHarness(t, "0.2.0", release(t, "0.3.0", "new"))
		h.command.interactive = func() bool { return true }

		require.Equal(t, console.ExitSuccess, h.Run(h.command, "--check"), h.Err.String())

		assert.Equal(t, "tarhche 0.3.0 is available (you have 0.2.0), released 3 days ago:\n"+
			"\n## Changelog\n* what changed in 0.3.0\n"+
			"\n"+h.github.URL+"/Tarhche/cli/releases/tag/v0.3.0\n", h.Out.String())
		assert.Empty(t, h.Err.String())
		assert.Equal(t, "old", h.installed(t))
	})

	t.Run("upgrades once it is told yes", func(t *testing.T) {
		t.Parallel()

		h := newUpgradeHarness(t, "0.2.0", release(t, "0.3.0", "new"))
		h.command.interactive = func() bool { return true }
		h.In.Reset("y\n")

		require.Equal(t, console.ExitSuccess, h.Run(h.command), h.Err.String())

		assert.Equal(t, "\nUpgrade "+h.binary+" to 0.3.0? [y/N] ", h.Err.String())
		assert.True(t, strings.HasSuffix(h.Out.String(), "\nUpgraded "+h.binary+" to 0.3.0.\n"), h.Out.String())
		assert.Equal(t, "new", h.installed(t))
	})

	t.Run("takes an answer that ends no line", func(t *testing.T) {
		t.Parallel()

		h := newUpgradeHarness(t, "0.2.0", release(t, "0.3.0", "new"))
		h.command.interactive = func() bool { return true }
		h.In.Reset("YES")

		require.Equal(t, console.ExitSuccess, h.Run(h.command), h.Err.String())

		assert.Equal(t, "new", h.installed(t))
	})

	for _, answer := range []string{"n\n", "\n", ""} {
		t.Run(fmt.Sprintf("does nothing once it is answered %q", answer), func(t *testing.T) {
			t.Parallel()

			h := newUpgradeHarness(t, "0.2.0", release(t, "0.3.0", "new"))
			h.command.interactive = func() bool { return true }
			h.In.Reset(answer)

			require.Equal(t, console.ExitSuccess, h.Run(h.command), h.Err.String())

			assert.NotContains(t, h.Out.String(), "Upgraded")
			assert.Equal(t, "old", h.installed(t))
		})
	}

	t.Run("upgrades without asking when told to", func(t *testing.T) {
		t.Parallel()

		h := newUpgradeHarness(t, "0.2.0", release(t, "0.3.0", "new"))

		require.Equal(t, console.ExitSuccess, h.Run(h.command, "-y"), h.Err.String())

		assert.Empty(t, h.Err.String())
		assert.Contains(t, h.Out.String(), "Upgraded "+h.binary+" to 0.3.0.\n")
		assert.Equal(t, "new", h.installed(t))
	})

	t.Run("does not upgrade unasked when nobody is at a terminal", func(t *testing.T) {
		t.Parallel()

		h := newUpgradeHarness(t, "0.2.0", release(t, "0.3.0", "new"))

		require.Equal(t, console.ExitFailure, h.Run(h.command))

		assert.Equal(t, "Error: nobody is at a terminal to agree to it: run 'tarhche upgrade --yes' to upgrade tarhche without being asked\n", h.Err.String())
		assert.Equal(t, "old", h.installed(t))
	})

	t.Run("shows how much of the download arrived on a terminal", func(t *testing.T) {
		t.Parallel()

		h := newUpgradeHarness(t, "0.2.0", release(t, "0.3.0", "new"))
		h.command.terminal = func() bool { return true }

		require.Equal(t, console.ExitSuccess, h.Run(h.command, "--yes"), h.Err.String())

		assert.Contains(t, h.Err.String(), "\rDownloading: 100% (")
		assert.True(t, strings.HasSuffix(h.Err.String(), "\n"))
	})

	t.Run("offers a development build the latest release", func(t *testing.T) {
		t.Parallel()

		h := newUpgradeHarness(t, "dev", release(t, "0.3.0", "new"))

		require.Equal(t, console.ExitSuccess, h.Run(h.command, "--yes"), h.Err.String())

		assert.True(t, strings.HasPrefix(h.Out.String(), "This is a development build of tarhche (dev): its latest release is 0.3.0, released 3 days ago:\n"), h.Out.String())
		assert.Contains(t, h.Out.String(), "Upgraded "+h.binary+" to 0.3.0.\n")
	})

	t.Run("downgrades to the version it is told", func(t *testing.T) {
		t.Parallel()

		h := newUpgradeHarness(t, "0.2.0", release(t, "0.2.0", "same"), release(t, "0.1.0", "older"))
		h.command.interactive = func() bool { return true }
		h.In.Reset("y\n")

		require.Equal(t, console.ExitSuccess, h.Run(h.command, "v0.1.0"), h.Err.String())

		assert.True(t, strings.HasPrefix(h.Out.String(), "tarhche 0.1.0, released 3 days ago (you have 0.2.0):\n\n## Changelog\n* what changed in 0.1.0\n"), h.Out.String())
		assert.Equal(t, "\nDowngrade "+h.binary+" to 0.1.0? [y/N] ", h.Err.String())
		assert.Contains(t, h.Out.String(), "Downgraded "+h.binary+" to 0.1.0.\n")
		assert.Equal(t, "older", h.installed(t))
	})

	t.Run("reinstalls the version running when it is told to", func(t *testing.T) {
		t.Parallel()

		h := newUpgradeHarness(t, "0.2.0", release(t, "0.2.0", "same"))

		require.Equal(t, console.ExitSuccess, h.Run(h.command, "--yes", "0.2.0"), h.Err.String())

		assert.Contains(t, h.Out.String(), "Reinstalled "+h.binary+" to 0.2.0.\n")
		assert.Equal(t, "same", h.installed(t))
	})

	t.Run("tells what the version it is told says, and stops when told to only check", func(t *testing.T) {
		t.Parallel()

		h := newUpgradeHarness(t, "0.2.0", release(t, "0.3.0", "new"))

		require.Equal(t, console.ExitSuccess, h.Run(h.command, "--check", "0.3.0"), h.Err.String())

		assert.Contains(t, h.Out.String(), "* what changed in 0.3.0")
		assert.NotContains(t, h.Out.String(), "Upgraded")
		assert.Equal(t, "old", h.installed(t))
	})

	t.Run("says to agree to an upgrade to the version it is told without being asked", func(t *testing.T) {
		t.Parallel()

		h := newUpgradeHarness(t, "0.2.0", release(t, "0.2.0", "same"), release(t, "0.1.0", "older"))

		require.Equal(t, console.ExitFailure, h.Run(h.command, "0.1.0"))

		assert.Equal(t, "Error: nobody is at a terminal to agree to it: run 'tarhche upgrade --yes 0.1.0' to downgrade tarhche without being asked\n", h.Err.String())
	})

	t.Run("says when the version it is told was not released", func(t *testing.T) {
		t.Parallel()

		h := newUpgradeHarness(t, "0.2.0", release(t, "0.2.0", "same"))

		require.Equal(t, console.ExitFailure, h.Run(h.command, "9.9.9"))

		assert.Equal(t, "Error: no such release of tarhche: 9.9.9\n", h.Err.String())
	})

	t.Run("fails when the releases cannot be read", func(t *testing.T) {
		t.Parallel()

		h := newUpgradeHarness(t, "0.2.0")

		require.Equal(t, console.ExitFailure, h.Run(h.command))

		assert.Equal(t, "Error: no such release of tarhche: none is published yet\n", h.Err.String())
	})

	t.Run("fails when the running binary cannot be found", func(t *testing.T) {
		t.Parallel()

		h := newUpgradeHarness(t, "0.2.0", release(t, "0.3.0", "new"))
		require.NoError(t, os.Remove(h.binary))

		require.Equal(t, console.ExitFailure, h.Run(h.command, "--yes"))

		assert.Contains(t, h.Err.String(), "Error: finding the running binary")
	})

	t.Run("fails when the release cannot be installed, ending the line it showed", func(t *testing.T) {
		t.Parallel()

		broken := release(t, "0.3.0", "new")
		broken.Files["tarhche_0.3.0_checksums.txt"] = []byte(strings.Repeat("0", 64) + "  tarhche_0.3.0_linux_amd64.tar.gz\n")

		h := newUpgradeHarness(t, "0.2.0", broken)
		h.command.terminal = func() bool { return true }

		require.Equal(t, console.ExitFailure, h.Run(h.command, "--yes"))

		assert.Contains(t, h.Err.String(), "\nError: the archive does not match the release's checksum: ")
		assert.Equal(t, "old", h.installed(t))
	})

	t.Run("fails when the answer cannot be read", func(t *testing.T) {
		t.Parallel()

		h := newUpgradeHarness(t, "0.2.0", release(t, "0.3.0", "new"))
		h.command.interactive = func() bool { return true }
		h.Cli.In = iotest.ErrReader(errors.New("the terminal is gone"))

		require.Equal(t, console.ExitFailure, h.Run(h.command))

		assert.Contains(t, h.Err.String(), "Error: the terminal is gone\n")
		assert.Equal(t, "old", h.installed(t))
	})

	t.Run("refuses more than one version", func(t *testing.T) {
		t.Parallel()

		h := newUpgradeHarness(t, "0.2.0")

		assert.Equal(t, console.ExitUsageError, h.Run(h.command, "0.1.0", "0.2.0"))
	})
}

func TestUpgradeCommandExplain(t *testing.T) {
	t.Parallel()

	denied := fmt.Errorf("cannot replace /usr/local/bin/tarhche: %w", upgrade.ErrPermission)

	// given is the harness's command, given args.
	given := func(t *testing.T, args ...string) *upgradeCommand {
		t.Helper()

		cmd := newUpgradeHarness(t, "0.2.0").command

		flags := console.NewFlagSet("upgrade", io.Discard)
		cmd.Configure(flags)
		require.NoError(t, flags.Parse(args))

		return cmd
	}

	t.Run("says to upgrade as root what only root may write", func(t *testing.T) {
		t.Parallel()

		cmd := given(t, "0.3.0")
		cmd.goos = "darwin"

		err := cmd.explain(denied)

		require.ErrorIs(t, err, upgrade.ErrPermission)
		assert.EqualError(t, err, "cannot replace /usr/local/bin/tarhche: permission denied: run 'sudo tarhche upgrade 0.3.0' to upgrade it as root")
	})

	t.Run("says to upgrade as an administrator what only one may write on Windows", func(t *testing.T) {
		t.Parallel()

		cmd := given(t)
		cmd.goos = "windows"

		assert.EqualError(t, cmd.explain(denied), "cannot replace /usr/local/bin/tarhche: permission denied: run 'tarhche upgrade' again in a terminal run as administrator")
	})

	t.Run("leaves any other error as it is", func(t *testing.T) {
		t.Parallel()

		other := errors.New("other")

		assert.Same(t, other, given(t).explain(other))
	})
}

func TestNewUpgradeCommand(t *testing.T) {
	t.Parallel()

	t.Run("asks and shows progress only at a terminal", func(t *testing.T) {
		t.Parallel()

		h := commandtest.New(t, nil)
		cmd := newUpgradeCommand(h.Cli, upgrade.New())

		assert.False(t, cmd.interactive())
		assert.False(t, cmd.terminal())
	})
}

func TestChangeOf(t *testing.T) {
	t.Parallel()

	tests := []struct {
		version, current string
		change           change
	}{
		{version: "0.3.0", current: "0.2.0", change: upgrading},
		{version: "0.1.0", current: "0.2.0", change: downgrading},
		{version: "0.2.0", current: "v0.2.0", change: reinstalling},
		{version: "0.2.0", current: "dev", change: upgrading},
	}

	for _, test := range tests {
		t.Run(test.version+" over "+test.current+" is "+test.change.verb, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.change, changeOf(test.version, test.current))
		})
	}
}

func TestReleased(t *testing.T) {
	t.Parallel()

	t.Run("says when a release was published", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, ", released about an hour ago", released(upgrade.Release{PublishedAt: time.Now().Add(-time.Hour)}))
	})

	t.Run("says nothing when that is not known", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, released(upgrade.Release{}))
	})
}

func TestProgressLine(t *testing.T) {
	t.Parallel()

	t.Run("shows a share of what there is, once a step", func(t *testing.T) {
		t.Parallel()

		var out bytes.Buffer

		line := &progressLine{out: &out}
		line.show(0, 2048)
		line.show(1, 2048)
		line.show(1024, 2048)
		line.show(2048, 2048)
		line.end()

		assert.Equal(t, fmt.Sprintf("\r%-50s\r%-50s\r%-50s\n",
			"Downloading: 0% (0B of 2KiB)",
			"Downloading: 50% (1KiB of 2KiB)",
			"Downloading: 100% (2KiB of 2KiB)"), out.String())
	})

	t.Run("shows what arrived a mebibyte at a time when what there is is not known", func(t *testing.T) {
		t.Parallel()

		var out bytes.Buffer

		line := &progressLine{out: &out}
		line.show(512, 0)
		line.show(1024, 0)
		line.show(1<<20, 0)
		line.end()

		assert.Equal(t, fmt.Sprintf("\r%-50s\r%-50s\n", "Downloading: 512B", "Downloading: 1MiB"), out.String())
	})

	t.Run("ends nothing when it showed nothing", func(t *testing.T) {
		t.Parallel()

		var out bytes.Buffer

		(&progressLine{out: &out}).end()

		assert.Empty(t, out.String())
	})
}
