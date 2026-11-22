// Package upgrade replaces the running tarhche with one of its releases on
// GitHub: it reads what the releases are and say of themselves, picks the
// archive this platform was released in, checks it against the release's
// checksums and puts what it holds where the running binary is, or, run from
// a macOS app bundle, where the bundle is.
package upgrade

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/Tarhche/cli/internal/version"
)

// Repository is the GitHub repository tarhche is released from.
const Repository = "Tarhche/cli"

// API is GitHub's REST API, which releases are read from.
const API = "https://api.github.com"

// maxArchive is the most an archive, and what is extracted of it, may be:
// far more than a build of tarhche is, far less than would fill a disk.
const maxArchive = 512 << 20

var (
	// ErrUnsupported is what upgrading a build that cannot upgrade itself
	// fails with: the image's, which is upgraded by pulling a newer image.
	ErrUnsupported = errors.New("this build of tarhche cannot upgrade itself: pull a newer image of ghcr.io/tarhche/cli instead")

	// ErrNoRelease is what reading a release that is not published fails
	// with.
	ErrNoRelease = errors.New("no such release of tarhche")

	// ErrNoBuild is what installing a release that holds no build for this
	// platform fails with.
	ErrNoBuild = errors.New("the release holds no build of tarhche for this platform")

	// ErrChecksum is what installing an archive that does not match its
	// release's checksums fails with; nothing is replaced.
	ErrChecksum = errors.New("the archive does not match the release's checksum")

	// ErrPermission is what installing where the running user may not write
	// fails with; nothing is replaced.
	ErrPermission = errors.New("permission denied")
)

// Release is a published release of tarhche.
type Release struct {
	// Version is the version released, without the tag's v: 0.2.0.
	Version string
	// Tag is the git tag it was released from: v0.2.0.
	Tag string
	// Name is what the release is titled.
	Name string
	// Notes are what the release says of itself, in Markdown.
	Notes string
	// URL is the release's page on GitHub.
	URL string
	// PublishedAt is when it was published.
	PublishedAt time.Time
	// Assets are the files it was released with.
	Assets []Asset
}

// Asset is a file a release holds.
type Asset struct {
	// Name is the file's name: tarhche_0.2.0_linux_amd64.tar.gz.
	Name string
	// URL is where it is downloaded from.
	URL string
	// Size is how large it is, in bytes.
	Size int64
}

// Check is what looking for a newer release found.
type Check struct {
	// Current is the version running: a release's, or dev for a build of
	// one's own.
	Current string
	// Latest is the newest release, prereleases and drafts aside.
	Latest Release
	// Available reports whether Latest is newer than Current, which any
	// release is when Current is no release's version.
	Available bool
}

// Installation is what installing a release replaced.
type Installation struct {
	// Version is the version installed.
	Version string
	// Path is what was replaced: the binary, or the macOS app bundle it is
	// run from.
	Path string
	// Executable is the binary that runs the version installed: Path, or the
	// one inside the bundle.
	Executable string
}

// Progress is told, as a download goes on, how many bytes of it have
// arrived and how many there are in all (0 when that is not known).
type Progress func(done, total int64)

// Platform is what a build of tarhche runs on, as Go names it.
type Platform struct {
	// OS is GOOS: darwin, linux or windows.
	OS string
	// Arch is GOARCH: 386, amd64, arm or arm64.
	Arch string
	// Arm is GOARM when Arch is arm: 7.
	Arm string
}

// Upgrader reads tarhche's releases and installs them in place of the
// running binary.
type Upgrader struct {
	api        string
	repository string
	client     *http.Client
	userAgent  string
	current    string
	executable string
	platform   Platform
	files      files
	limit      int64
}

// Option configures an Upgrader.
type Option func(*Upgrader)

// WithAPI reads releases from api instead of GitHub's API (API).
func WithAPI(api string) Option {
	return func(u *Upgrader) { u.api = api }
}

// WithHTTPClient asks GitHub through client instead of
// http.DefaultClient.
func WithHTTPClient(client *http.Client) Option {
	return func(u *Upgrader) { u.client = client }
}

// WithUserAgent names the client to GitHub as agent.
func WithUserAgent(agent string) Option {
	return func(u *Upgrader) { u.userAgent = agent }
}

// WithCurrent says version is the one running, instead of version.Version.
func WithCurrent(version string) Option {
	return func(u *Upgrader) { u.current = version }
}

// WithExecutable says the running binary is at path, instead of where
// os.Executable says it is.
func WithExecutable(path string) Option {
	return func(u *Upgrader) { u.executable = path }
}

// WithPlatform says the running binary was built for platform, instead of
// the one it runs on.
func WithPlatform(platform Platform) Option {
	return func(u *Upgrader) { u.platform = platform }
}

// New is an Upgrader of the running binary, from GitHub's releases of
// Repository, as options say otherwise.
func New(options ...Option) *Upgrader {
	upgrader := &Upgrader{
		api:        API,
		repository: Repository,
		client:     http.DefaultClient,
		current:    version.Version,
		platform:   runningPlatform(),
		files:      osFiles,
		limit:      maxArchive,
	}

	for _, option := range options {
		option(upgrader)
	}

	if upgrader.client == nil {
		upgrader.client = http.DefaultClient
	}

	return upgrader
}

// Current is the version running: a release's, or dev for a build of one's
// own.
func (u *Upgrader) Current() string {
	return u.current
}

// Check reads the latest release and whether it is newer than the version
// running.
func (u *Upgrader) Check(ctx context.Context) (Check, error) {
	latest, err := u.read(ctx, "releases/latest", fmt.Errorf("%w: none is published yet", ErrNoRelease))
	if err != nil {
		return Check{}, err
	}

	return Check{Current: u.current, Latest: latest, Available: newer(latest.Version, u.current)}, nil
}

// Release reads the release of version (0.2.0 or v0.2.0): ErrNoRelease
// when none was published.
func (u *Upgrader) Release(ctx context.Context, version string) (Release, error) {
	tag, err := tagOf(version)
	if err != nil {
		return Release{}, err
	}

	return u.read(ctx, "releases/tags/"+url.PathEscape(tag), fmt.Errorf("%w: %s", ErrNoRelease, strings.TrimPrefix(tag, "v")))
}

// Target is what installing a release replaces: the running binary, or the
// macOS app bundle it is run from; ErrUnsupported in a build that cannot
// upgrade itself.
func (u *Upgrader) Target() (string, error) {
	target, err := u.target()
	if err != nil {
		return "", err
	}

	return target.path, nil
}

// Install downloads release's archive for this platform, telling progress
// how much of it arrived, checks it against the release's checksums and puts
// what it holds in place of Target. Nothing is replaced unless all of it
// succeeded.
func (u *Upgrader) Install(ctx context.Context, release Release, progress Progress) (Installation, error) {
	target, err := u.target()
	if err != nil {
		return Installation{}, err
	}

	name := u.archiveFor(release.Version, target)

	archive, found := assetNamed(release.Assets, name)
	if !found {
		return Installation{}, fmt.Errorf("%w (%s): it holds no %s", ErrNoBuild, u.platform.name(), name)
	}

	sum, err := u.checksum(ctx, release, name)
	if err != nil {
		return Installation{}, err
	}

	downloaded, err := u.download(ctx, archive, sum, target, progress)
	if err != nil {
		return Installation{}, err
	}

	defer func() { _ = u.files.remove(downloaded) }()

	if target.bundle {
		return u.replaceBundle(release.Version, target, archive.Name, downloaded)
	}

	return u.replaceBinary(release.Version, target, archive.Name, downloaded)
}

// files are the filesystem's operations an upgrade can fail at, which tests
// make fail.
type files struct {
	running    func() (string, error)
	createTemp func(dir, pattern string) (*os.File, error)
	mkdirTemp  func(dir, pattern string) (string, error)
	rename     func(from, to string) error
	remove     func(name string) error
	removeAll  func(path string) error
}

// osFiles are the operating system's own.
var osFiles = files{
	running:    os.Executable,
	createTemp: os.CreateTemp,
	mkdirTemp:  os.MkdirTemp,
	rename:     os.Rename,
	remove:     os.Remove,
	removeAll:  os.RemoveAll,
}
