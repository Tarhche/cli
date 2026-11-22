package upgrade

import (
	"context"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/upgrade/upgradetest"
	"github.com/Tarhche/cli/internal/version"
)

// linux is the platform most tests upgrade a build of.
var linux = Platform{OS: "linux", Arch: "amd64"}

// macOS is the platform app bundles are upgraded on.
var macOS = Platform{OS: "darwin", Arch: "arm64"}

// published is when the releases of the tests were published.
var published = time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)

// binaryRelease is a release of version that holds platform's build of
// tarhche, whose binary is body.
func binaryRelease(t *testing.T, version string, platform Platform, body []byte) upgradetest.Release {
	t.Helper()

	files := map[string]upgradetest.File{
		platform.binary(): {Body: body, Mode: 0o755},
		"README.md":       {Body: []byte("# tarhche")},
	}

	archive := upgradetest.TarGz(t, files)
	if platform.OS == "windows" {
		archive = upgradetest.Zip(t, files)
	}

	return upgradetest.Release{
		Version:     version,
		Name:        "v" + version,
		Notes:       "## Changelog\n* what changed",
		PublishedAt: published,
		Files:       upgradetest.Checksummed(version, map[string][]byte{archiveName(version, platform): archive}),
	}
}

// appRelease is a release of version that holds an app bundle of files.
func appRelease(t *testing.T, version string, files map[string]upgradetest.File) upgradetest.Release {
	t.Helper()

	return upgradetest.Release{
		Version:     version,
		PublishedAt: published,
		Files:       upgradetest.Checksummed(version, map[string][]byte{appArchiveName(version): upgradetest.Zip(t, files)}),
	}
}

// app is what the archive of a release's app bundle holds, its binary
// holding body.
func app(body string) map[string]upgradetest.File {
	return map[string]upgradetest.File{
		"Tarhche.app/":                              {Mode: fs.ModeDir | 0o755},
		"Tarhche.app/Contents/Info.plist":           {Body: []byte("<plist/>")},
		"Tarhche.app/Contents/MacOS/tarhche":        {Body: []byte(body), Mode: 0o755},
		"Tarhche.app/Contents/Resources/icons.icns": {Body: []byte("icons")},
		"Tarhche.app/Contents/Resources/current":    {Body: []byte("icons.icns"), Mode: fs.ModeSymlink | 0o777},
		"__MACOSX/._Tarhche.app":                    {Body: []byte("resource fork")},
	}
}

// installedApp is an app bundle of the test's own, alone in a directory of
// its own, whose binary holds old: what an upgrade replaces.
func installedApp(t *testing.T) (string, string) {
	t.Helper()

	bundle := filepath.Join(t.TempDir(), "Tarhche.app")
	executable := filepath.Join(bundle, "Contents", "MacOS", "tarhche")

	require.NoError(t, os.MkdirAll(filepath.Dir(executable), 0o755))
	require.NoError(t, os.WriteFile(executable, []byte("old"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(bundle, "Contents", "Resources"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bundle, "Contents", "Resources", "old.txt"), []byte("old"), 0o644))

	return bundle, executable
}

// beside are the names of what is in the directory the app bundle at
// bundle is in.
func beside(t *testing.T, bundle string) []string {
	t.Helper()

	entries, err := os.ReadDir(filepath.Dir(bundle))
	require.NoError(t, err)

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}

	return names
}

// installed is a binary of the test's own called name, holding body: what
// an upgrade replaces.
func installed(t *testing.T, name string, body []byte, mode fs.FileMode) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, body, mode))

	return path
}

// leftovers are what an upgrade left in dir beside what it replaced.
func leftovers(t *testing.T, dir string) []string {
	t.Helper()

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	var left []string

	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".tarhche-upgrade-") {
			left = append(left, entry.Name())
		}
	}

	return left
}

// read is what the file at path holds.
func read(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	require.NoError(t, err)

	return string(content)
}

// latest reads the latest release from github through upgrader.
func latest(t *testing.T, upgrader *Upgrader) Release {
	t.Helper()

	check, err := upgrader.Check(context.Background())
	require.NoError(t, err)

	return check.Latest
}

// permissionDenied is what an operation the running user may not do fails
// with.
func permissionDenied(op, path string) error {
	return &fs.PathError{Op: op, Path: path, Err: fs.ErrPermission}
}

func TestNew(t *testing.T) {
	t.Parallel()

	t.Run("reads tarhche's releases on GitHub for the running binary", func(t *testing.T) {
		t.Parallel()

		upgrader := New()

		assert.Equal(t, API, upgrader.api)
		assert.Equal(t, Repository, upgrader.repository)
		assert.Same(t, http.DefaultClient, upgrader.client)
		assert.Equal(t, version.Version, upgrader.Current())
		assert.Equal(t, runtime.GOOS, upgrader.platform.OS)
		assert.Equal(t, runtime.GOARCH, upgrader.platform.Arch)
		assert.Equal(t, int64(maxArchive), upgrader.limit)
	})

	t.Run("takes what its options say instead", func(t *testing.T) {
		t.Parallel()

		client := &http.Client{}
		upgrader := New(WithAPI("http://api"), WithHTTPClient(client), WithUserAgent("agent"), WithCurrent("0.1.0"), WithExecutable("/bin/tarhche"), WithPlatform(linux))

		assert.Equal(t, "http://api", upgrader.api)
		assert.Same(t, client, upgrader.client)
		assert.Equal(t, "agent", upgrader.userAgent)
		assert.Equal(t, "0.1.0", upgrader.Current())
		assert.Equal(t, "/bin/tarhche", upgrader.executable)
		assert.Equal(t, linux, upgrader.platform)
	})

	t.Run("asks through the default client when given none", func(t *testing.T) {
		t.Parallel()

		assert.Same(t, http.DefaultClient, New(WithHTTPClient(nil)).client)
	})
}

func TestUpgraderCheck(t *testing.T) {
	t.Parallel()

	t.Run("finds a newer release", func(t *testing.T) {
		t.Parallel()

		github := upgradetest.New(t, binaryRelease(t, "0.3.0", linux, []byte("new")), binaryRelease(t, "0.2.0", linux, []byte("old")))
		upgrader := New(WithAPI(github.URL), WithCurrent("0.2.0"), WithUserAgent("tarhche-cli/0.2.0"))

		check, err := upgrader.Check(context.Background())
		require.NoError(t, err)

		assert.True(t, check.Available)
		assert.Equal(t, "0.2.0", check.Current)
		assert.Equal(t, "0.3.0", check.Latest.Version)
		assert.Equal(t, "v0.3.0", check.Latest.Tag)
		assert.Equal(t, "v0.3.0", check.Latest.Name)
		assert.Equal(t, "## Changelog\n* what changed", check.Latest.Notes)
		assert.Equal(t, github.URL+"/Tarhche/cli/releases/tag/v0.3.0", check.Latest.URL)
		assert.True(t, published.Equal(check.Latest.PublishedAt))
		require.Len(t, check.Latest.Assets, 2)
		assert.Equal(t, "tarhche_0.3.0_checksums.txt", check.Latest.Assets[0].Name)
		assert.Equal(t, "tarhche_0.3.0_linux_amd64.tar.gz", check.Latest.Assets[1].Name)
		assert.Equal(t, github.URL+"/Tarhche/cli/releases/download/v0.3.0/tarhche_0.3.0_linux_amd64.tar.gz", check.Latest.Assets[1].URL)
		assert.Positive(t, check.Latest.Assets[1].Size)

		requests := github.Requests()
		require.Len(t, requests, 1)
		assert.Equal(t, "/repos/Tarhche/cli/releases/latest", requests[0].Path)
		assert.Equal(t, "application/vnd.github+json", requests[0].Header.Get("Accept"))
		assert.Equal(t, "2022-11-28", requests[0].Header.Get("X-GitHub-Api-Version"))
		assert.Equal(t, "tarhche-cli/0.2.0", requests[0].Header.Get("User-Agent"))
	})

	t.Run("finds none newer than the version running", func(t *testing.T) {
		t.Parallel()

		github := upgradetest.New(t, binaryRelease(t, "0.2.0", linux, []byte("same")))

		check, err := New(WithAPI(github.URL), WithCurrent("0.2.0")).Check(context.Background())
		require.NoError(t, err)

		assert.False(t, check.Available)
		assert.Equal(t, "0.2.0", check.Latest.Version)
	})

	t.Run("takes no prerelease for the latest", func(t *testing.T) {
		t.Parallel()

		prerelease := binaryRelease(t, "0.4.0-rc.1", linux, []byte("next"))
		prerelease.Prerelease = true

		github := upgradetest.New(t, prerelease, binaryRelease(t, "0.3.0", linux, []byte("new")))

		check, err := New(WithAPI(github.URL), WithCurrent("0.2.0")).Check(context.Background())
		require.NoError(t, err)

		assert.Equal(t, "0.3.0", check.Latest.Version)
	})

	t.Run("finds any release newer than a build of one's own", func(t *testing.T) {
		t.Parallel()

		github := upgradetest.New(t, binaryRelease(t, "0.2.0", linux, []byte("release")))

		check, err := New(WithAPI(github.URL), WithCurrent("dev")).Check(context.Background())
		require.NoError(t, err)

		assert.True(t, check.Available)
	})

	t.Run("says when no release is published", func(t *testing.T) {
		t.Parallel()

		github := upgradetest.New(t)

		_, err := New(WithAPI(github.URL)).Check(context.Background())

		require.ErrorIs(t, err, ErrNoRelease)
		assert.EqualError(t, err, "no such release of tarhche: none is published yet")
	})

	t.Run("says when GitHub's rate limit was reached", func(t *testing.T) {
		t.Parallel()

		resets := time.Now().Add(20 * time.Minute).Truncate(time.Second)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resets.Unix(), 10))
			w.WriteHeader(http.StatusForbidden)
		}))
		t.Cleanup(server.Close)

		_, err := New(WithAPI(server.URL)).Check(context.Background())

		assert.EqualError(t, err, "GitHub's rate limit was reached: try again after "+resets.Local().Format("15:04"))
	})

	t.Run("says what else GitHub answered", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
		}))
		t.Cleanup(server.Close)

		_, err := New(WithAPI(server.URL)).Check(context.Background())

		assert.EqualError(t, err, "asking GitHub for the releases of tarhche: it answered 502 Bad Gateway")
	})

	t.Run("fails for an answer that is no release", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("<html>"))
		}))
		t.Cleanup(server.Close)

		_, err := New(WithAPI(server.URL)).Check(context.Background())

		assert.ErrorContains(t, err, "reading what GitHub answered of the releases of tarhche")
	})

	t.Run("fails when GitHub cannot be reached", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.NotFoundHandler())
		server.Close()

		_, err := New(WithAPI(server.URL)).Check(context.Background())

		assert.ErrorContains(t, err, "asking GitHub for the releases of tarhche")
	})

	t.Run("fails for an API that is no URL", func(t *testing.T) {
		t.Parallel()

		_, err := New(WithAPI("http://\x7f")).Check(context.Background())

		assert.ErrorContains(t, err, "asking GitHub for the releases of tarhche")
	})
}

func TestUpgraderRelease(t *testing.T) {
	t.Parallel()

	github := upgradetest.New(t, binaryRelease(t, "0.3.0", linux, []byte("new")), binaryRelease(t, "0.2.0", linux, []byte("old")))

	for _, asked := range []string{"0.2.0", "v0.2.0", " 0.2.0 ", "0.2"} {
		t.Run("reads the release of "+asked, func(t *testing.T) {
			t.Parallel()

			release, err := New(WithAPI(github.URL)).Release(context.Background(), asked)
			require.NoError(t, err)

			assert.Equal(t, "0.2.0", release.Version)
			assert.Equal(t, "v0.2.0", release.Tag)
		})
	}

	t.Run("says when a version was not released", func(t *testing.T) {
		t.Parallel()

		_, err := New(WithAPI(github.URL)).Release(context.Background(), "9.9.9")

		require.ErrorIs(t, err, ErrNoRelease)
		assert.EqualError(t, err, "no such release of tarhche: 9.9.9")
	})

	t.Run("refuses what is no version, asking nothing", func(t *testing.T) {
		t.Parallel()

		before := len(github.Requests())

		_, err := New(WithAPI(github.URL)).Release(context.Background(), "../latest")

		assert.EqualError(t, err, `"../latest" is no version of tarhche: a version is like 0.2.0`)
		assert.Len(t, github.Requests(), before)
	})
}
