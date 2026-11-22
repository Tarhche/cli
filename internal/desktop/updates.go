package desktop

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/Tarhche/cli/internal/upgrade"
	"github.com/Tarhche/cli/internal/version"
)

var (
	// ErrUpgrading is what upgrading fails with while another upgrade is
	// under way.
	ErrUpgrading = errors.New("tarhche is being upgraded already")

	// ErrNothingInstalled is what restarting fails with before an upgrade
	// installed a version to restart into.
	ErrNothingInstalled = errors.New("no newer version of tarhche was installed to restart into")
)

// Upgrader is what the app looks for newer releases of tarhche with, and
// installs one with in place of itself: an *upgrade.Upgrader.
type Upgrader interface {
	// Check reads the latest release and whether it is newer than the
	// version running.
	Check(ctx context.Context) (upgrade.Check, error)

	// Release reads the release of version.
	Release(ctx context.Context, version string) (upgrade.Release, error)

	// Target is what installing a release replaces: the running binary, or
	// the macOS app bundle it runs from; upgrade.ErrUnsupported in a build
	// that cannot upgrade itself.
	Target() (string, error)

	// Install puts what release holds for this platform in place of Target,
	// telling progress how much of its download arrived.
	Install(ctx context.Context, release upgrade.Release, progress upgrade.Progress) (upgrade.Installation, error)
}

// Release is a release of tarhche, as the frontend shows it.
type Release struct {
	// Version is the version it released, without the tag's v: 0.2.0.
	Version string `json:"version"`

	// Name is what it is titled.
	Name string `json:"name"`

	// Notes are what it says of itself, in Markdown.
	Notes string `json:"notes"`

	// URL is its page on GitHub.
	URL string `json:"url"`

	PublishedAt time.Time `json:"published_at"`
}

// Update is what looking for a newer release of tarhche found.
type Update struct {
	// Current is the version running: a release's, or dev for a build of
	// one's own.
	Current string `json:"current"`

	// Available reports whether Latest is newer than Current.
	Available bool `json:"available"`

	// Latest is the newest release.
	Latest Release `json:"latest"`

	// Target is what upgrading replaces: the binary running, or the macOS
	// app bundle it runs from.
	Target string `json:"target,omitempty"`

	// Unsupported says why this build cannot upgrade itself, when it
	// cannot: then there is no Target.
	Unsupported string `json:"unsupported,omitempty"`
}

// Upgraded is what an upgrade installed.
type Upgraded struct {
	Version string `json:"version"`

	// Path is what it replaced: the binary, or the macOS app bundle.
	Path string `json:"path"`
}

// Starter starts the program name with args apart from the app, which may
// quit while it runs on, and does not wait for it.
type Starter func(name string, args ...string) error

// Updates is how the frontend looks for a newer release of tarhche and
// upgrades the app to it, hearing how much of its download arrived in
// EventUpdateProgress, and then restarts into the version it installed.
type Updates struct {
	upgrader Upgrader
	emitter  Emitter
	quit     func()

	start Starter
	goos  string

	lock      sync.Mutex
	upgrading bool
	latest    *upgrade.Release
	installed *upgrade.Installation
}

// NewUpdates is the update service over upgrader, which tells emitter how
// much of an upgrade's download arrived, and restarts the app into what it
// installed by starting that and quitting with quit.
func NewUpdates(upgrader Upgrader, emitter Emitter, quit func()) *Updates {
	return &Updates{upgrader: upgrader, emitter: emitter, quit: quit, start: startDetached, goos: runtime.GOOS}
}

// Version is the version of tarhche running.
func (u *Updates) Version() string {
	return version.Version
}

// Check looks for a release newer than the version running, and says what
// upgrading to it replaces, or why this build cannot upgrade itself.
func (u *Updates) Check(ctx context.Context) (Update, error) {
	checked, err := u.upgrader.Check(ctx)
	if err != nil {
		return Update{}, err
	}

	update := Update{Current: checked.Current, Available: checked.Available, Latest: releaseOf(checked.Latest)}

	target, err := u.upgrader.Target()
	switch {
	case errors.Is(err, upgrade.ErrUnsupported):
		update.Unsupported = err.Error()
	case err != nil:
		return Update{}, err
	default:
		update.Target = target
	}

	u.lock.Lock()
	u.latest = &checked.Latest
	u.lock.Unlock()

	return update, nil
}

// Upgrade installs the release of version in place of the app, one upgrade
// at a time, telling the frontend how much of its download arrived in
// EventUpdateProgress: the release the last Check found, or the one
// published as version.
func (u *Updates) Upgrade(ctx context.Context, version string) (Upgraded, error) {
	if !u.begin() {
		return Upgraded{}, ErrUpgrading
	}
	defer u.end()

	release, err := u.release(ctx, version)
	if err != nil {
		return Upgraded{}, err
	}

	installed, err := u.upgrader.Install(ctx, release, u.progressOf(release.Version))
	if err != nil {
		return Upgraded{}, u.explained(err)
	}

	u.lock.Lock()
	u.installed = &installed
	u.lock.Unlock()

	return Upgraded{Version: installed.Version, Path: installed.Path}, nil
}

// Restart starts the version an upgrade installed as a desktop app, and
// quits this one once it started.
func (u *Updates) Restart() error {
	u.lock.Lock()
	installed := u.installed
	u.lock.Unlock()

	if installed == nil {
		return ErrNothingInstalled
	}

	name, args := restartCommand(u.goos, *installed)
	if err := u.start(name, args...); err != nil {
		return fmt.Errorf("starting tarhche %s: %w", installed.Version, err)
	}

	go u.quit()

	return nil
}

// begin takes the upgrade in hand, unless another one is under way.
func (u *Updates) begin() bool {
	u.lock.Lock()
	defer u.lock.Unlock()

	if u.upgrading {
		return false
	}

	u.upgrading = true

	return true
}

// end lets another upgrade begin.
func (u *Updates) end() {
	u.lock.Lock()
	u.upgrading = false
	u.lock.Unlock()
}

// release is the release of version: the one the last Check found when it
// is that one, and else the one published as version.
func (u *Updates) release(ctx context.Context, version string) (upgrade.Release, error) {
	u.lock.Lock()
	latest := u.latest
	u.lock.Unlock()

	if latest != nil && sameVersion(latest.Version, version) {
		return *latest, nil
	}

	return u.upgrader.Release(ctx, version)
}

// progressOf tells the frontend how much of the download of version
// arrived: each time a whole percent more of it did, or, while how large it
// is is not known, each mebibyte more, and once all of it did.
func (u *Updates) progressOf(version string) upgrade.Progress {
	told := int64(-1)

	return func(done, total int64) {
		step := done >> 20
		if total > 0 {
			step = done * 100 / total
		}

		if step == told && done != total {
			return
		}

		told = step
		u.emitter.Emit(EventUpdateProgress, UpdateProgress{Version: version, Done: done, Total: total})
	}
}

// explained is err, which names what could not be replaced, saying how to
// upgrade where the user may not write when that is why it failed: as an
// administrator, from a terminal.
func (u *Updates) explained(err error) error {
	if !errors.Is(err, upgrade.ErrPermission) {
		return err
	}

	if u.goos == "windows" {
		return fmt.Errorf("%w: only an administrator may, so run tarhche upgrade in a terminal opened as administrator", err)
	}

	return fmt.Errorf("%w: only an administrator may, so run sudo tarhche upgrade in a terminal", err)
}

// releaseOf is release as the frontend shows it.
func releaseOf(release upgrade.Release) Release {
	return Release{
		Version:     release.Version,
		Name:        release.Name,
		Notes:       release.Notes,
		URL:         release.URL,
		PublishedAt: release.PublishedAt,
	}
}

// sameVersion reports whether a and b name one version, a tag's v or not.
func sameVersion(a, b string) bool {
	return strings.TrimPrefix(a, "v") == strings.TrimPrefix(b, "v")
}

// restartCommand is what starts installed as a desktop app on goos: on
// macOS its app bundle, through open as any app is opened, and its binary
// with --desktop otherwise.
func restartCommand(goos string, installed upgrade.Installation) (string, []string) {
	if goos == "darwin" && strings.HasSuffix(installed.Path, ".app") {
		return "open", []string{"-n", installed.Path, "--args", "--desktop"}
	}

	return installed.Executable, []string{"--desktop"}
}
