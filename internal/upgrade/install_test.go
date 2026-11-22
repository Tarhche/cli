//go:build !nodesktop

package upgrade

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/upgrade/upgradetest"
)

func TestUpgraderTarget(t *testing.T) {
	t.Parallel()

	t.Run("is the running binary", func(t *testing.T) {
		t.Parallel()

		running, err := os.Executable()
		require.NoError(t, err)

		resolved, err := filepath.EvalSymlinks(running)
		require.NoError(t, err)

		target, err := New().Target()
		require.NoError(t, err)

		assert.Equal(t, resolved, target)
	})

	t.Run("is what a link to the binary points at", func(t *testing.T) {
		t.Parallel()

		binary := installed(t, "tarhche", []byte("binary"), 0o755)
		link := filepath.Join(t.TempDir(), "tarhche")
		require.NoError(t, os.Symlink(binary, link))

		target, err := New(WithExecutable(link), WithPlatform(linux)).Target()
		require.NoError(t, err)

		assert.Equal(t, binary, target)
	})

	t.Run("is the app bundle a macOS binary is run from", func(t *testing.T) {
		t.Parallel()

		bundle := filepath.Join(t.TempDir(), "Tarhche.app")
		executable := filepath.Join(bundle, "Contents", "MacOS", "tarhche")
		require.NoError(t, os.MkdirAll(filepath.Dir(executable), 0o755))
		require.NoError(t, os.WriteFile(executable, []byte("binary"), 0o755))

		target, err := New(WithExecutable(executable), WithPlatform(Platform{OS: "darwin", Arch: "arm64"})).Target()
		require.NoError(t, err)

		assert.Equal(t, bundle, target)
	})

	t.Run("is the binary in what is shaped as a bundle elsewhere", func(t *testing.T) {
		t.Parallel()

		executable := filepath.Join(t.TempDir(), "Tarhche.app", "Contents", "MacOS", "tarhche")
		require.NoError(t, os.MkdirAll(filepath.Dir(executable), 0o755))
		require.NoError(t, os.WriteFile(executable, []byte("binary"), 0o755))

		target, err := New(WithExecutable(executable), WithPlatform(linux)).Target()
		require.NoError(t, err)

		assert.Equal(t, executable, target)
	})

	t.Run("is an absolute path", func(t *testing.T) {
		t.Parallel()

		binary := installed(t, "tarhche", []byte("binary"), 0o755)
		working, err := os.Getwd()
		require.NoError(t, err)

		relative, err := filepath.Rel(working, binary)
		require.NoError(t, err)

		target, err := New(WithExecutable(relative), WithPlatform(linux)).Target()
		require.NoError(t, err)

		assert.Equal(t, binary, target)
	})

	t.Run("fails for a binary that is not there", func(t *testing.T) {
		t.Parallel()

		_, err := New(WithExecutable(filepath.Join(t.TempDir(), "missing"))).Target()

		assert.ErrorContains(t, err, "finding the running binary")
	})

	t.Run("fails when the running binary cannot be found", func(t *testing.T) {
		t.Parallel()

		upgrader := New()
		upgrader.files.running = func() (string, error) { return "", errors.New("no such process") }

		_, err := upgrader.Target()

		assert.EqualError(t, err, "finding the running binary: no such process")
	})
}

func TestUpgraderInstall(t *testing.T) {
	t.Parallel()

	t.Run("replaces the binary with the release's, keeping its mode", func(t *testing.T) {
		t.Parallel()

		github := upgradetest.New(t, binaryRelease(t, "0.3.0", linux, []byte("new")))
		binary := installed(t, "tarhche", []byte("old"), 0o700)
		upgrader := New(WithAPI(github.URL), WithExecutable(binary), WithPlatform(linux), WithCurrent("0.2.0"))

		installation, err := upgrader.Install(context.Background(), latest(t, upgrader), nil)
		require.NoError(t, err)

		assert.Equal(t, Installation{Version: "0.3.0", Path: binary, Executable: binary}, installation)
		assert.Equal(t, "new", read(t, binary))

		info, err := os.Stat(binary)
		require.NoError(t, err)
		assert.Equal(t, fs.FileMode(0o700), info.Mode().Perm())
		assert.Empty(t, leftovers(t, filepath.Dir(binary)))
	})

	t.Run("tells how much of the download arrived", func(t *testing.T) {
		t.Parallel()

		body := []byte(strings.Repeat("new build ", 20000))
		github := upgradetest.New(t, binaryRelease(t, "0.3.0", linux, body))
		binary := installed(t, "tarhche", []byte("old"), 0o755)
		upgrader := New(WithAPI(github.URL), WithExecutable(binary), WithPlatform(linux))

		var told [][2]int64

		_, err := upgrader.Install(context.Background(), latest(t, upgrader), func(done, total int64) {
			told = append(told, [2]int64{done, total})
		})
		require.NoError(t, err)

		require.NotEmpty(t, told)

		last := told[len(told)-1]
		assert.Equal(t, last[1], last[0])
		assert.Positive(t, last[1])

		for i := 1; i < len(told); i++ {
			assert.Greater(t, told[i][0], told[i-1][0])
		}
	})

	t.Run("replaces a running Windows binary by putting it aside", func(t *testing.T) {
		t.Parallel()

		windows := Platform{OS: "windows", Arch: "amd64"}
		github := upgradetest.New(t, binaryRelease(t, "0.3.0", windows, []byte("new")))
		binary := installed(t, "tarhche.exe", []byte("old"), 0o755)
		require.NoError(t, os.WriteFile(binary+".old", []byte("older"), 0o755))

		upgrader := New(WithAPI(github.URL), WithExecutable(binary), WithPlatform(windows))

		var renames [][2]string

		upgrader.files.rename = func(from, to string) error {
			renames = append(renames, [2]string{filepath.Base(from), filepath.Base(to)})

			return os.Rename(from, to)
		}

		installation, err := upgrader.Install(context.Background(), latest(t, upgrader), nil)
		require.NoError(t, err)

		assert.Equal(t, binary, installation.Executable)
		assert.Equal(t, "new", read(t, binary))
		assert.NoFileExists(t, binary+".old")
		require.Len(t, renames, 2)
		assert.Equal(t, [2]string{"tarhche.exe", "tarhche.exe.old"}, renames[0])
		assert.Equal(t, "tarhche.exe", renames[1][1])
		assert.Empty(t, leftovers(t, filepath.Dir(binary)))
	})

	t.Run("puts a Windows binary aside under a name of its own while an earlier one still runs", func(t *testing.T) {
		t.Parallel()

		windows := Platform{OS: "windows", Arch: "arm64"}
		github := upgradetest.New(t, binaryRelease(t, "0.3.0", windows, []byte("new")))
		binary := installed(t, "tarhche.exe", []byte("old"), 0o755)
		require.NoError(t, os.WriteFile(binary+".old", []byte("still running"), 0o755))

		upgrader := New(WithAPI(github.URL), WithExecutable(binary), WithPlatform(windows))
		upgrader.files.remove = func(name string) error {
			if strings.HasSuffix(name, ".old") {
				return permissionDenied("remove", name)
			}

			return os.Remove(name)
		}

		_, err := upgrader.Install(context.Background(), latest(t, upgrader), nil)
		require.NoError(t, err)

		assert.Equal(t, "new", read(t, binary))
		assert.Equal(t, "still running", read(t, binary+".old"))

		entries, err := os.ReadDir(filepath.Dir(binary))
		require.NoError(t, err)

		var aside []string

		for _, entry := range entries {
			if regexp.MustCompile(`^tarhche\.exe\.\d+\.old$`).MatchString(entry.Name()) {
				aside = append(aside, entry.Name())
			}
		}

		require.Len(t, aside, 1)
		assert.Equal(t, "old", read(t, filepath.Join(filepath.Dir(binary), aside[0])))
	})

	t.Run("puts the Windows binary back when the new one cannot take its place", func(t *testing.T) {
		t.Parallel()

		windows := Platform{OS: "windows", Arch: "386"}
		github := upgradetest.New(t, binaryRelease(t, "0.3.0", windows, []byte("new")))
		binary := installed(t, "tarhche.exe", []byte("old"), 0o755)

		upgrader := New(WithAPI(github.URL), WithExecutable(binary), WithPlatform(windows))
		upgrader.files.rename = func(from, to string) error {
			if strings.Contains(filepath.Base(from), ".tarhche-upgrade-") {
				return errors.New("in use")
			}

			return os.Rename(from, to)
		}

		_, err := upgrader.Install(context.Background(), latest(t, upgrader), nil)

		assert.EqualError(t, err, "replacing "+binary+": in use")
		assert.Equal(t, "old", read(t, binary))
		assert.NoFileExists(t, binary+".old")
		assert.Empty(t, leftovers(t, filepath.Dir(binary)))
	})

	t.Run("says when the Windows binary cannot be put back", func(t *testing.T) {
		t.Parallel()

		windows := Platform{OS: "windows", Arch: "amd64"}
		github := upgradetest.New(t, binaryRelease(t, "0.3.0", windows, []byte("new")))
		binary := installed(t, "tarhche.exe", []byte("old"), 0o755)

		upgrader := New(WithAPI(github.URL), WithExecutable(binary), WithPlatform(windows))
		upgrader.files.rename = func(from, to string) error {
			if filepath.Base(to) == "tarhche.exe.old" {
				return os.Rename(from, to)
			}

			return errors.New("in use")
		}

		_, err := upgrader.Install(context.Background(), latest(t, upgrader), nil)

		assert.EqualError(t, err, "replacing "+binary+": in use\nputting "+binary+" back from "+binary+".old: in use")
	})

	t.Run("refuses to put aside a Windows binary the user may not", func(t *testing.T) {
		t.Parallel()

		windows := Platform{OS: "windows", Arch: "amd64"}
		github := upgradetest.New(t, binaryRelease(t, "0.3.0", windows, []byte("new")))
		binary := installed(t, "tarhche.exe", []byte("old"), 0o755)

		upgrader := New(WithAPI(github.URL), WithExecutable(binary), WithPlatform(windows))
		upgrader.files.rename = func(from, to string) error { return permissionDenied("rename", from) }

		_, err := upgrader.Install(context.Background(), latest(t, upgrader), nil)

		require.ErrorIs(t, err, ErrPermission)
		assert.EqualError(t, err, "cannot replace "+binary+": permission denied")
		assert.Equal(t, "old", read(t, binary))
		assert.Empty(t, leftovers(t, filepath.Dir(binary)))
	})

	t.Run("refuses to write beside a binary the user may not", func(t *testing.T) {
		t.Parallel()

		github := upgradetest.New(t, binaryRelease(t, "0.3.0", linux, []byte("new")))
		binary := installed(t, "tarhche", []byte("old"), 0o755)

		upgrader := New(WithAPI(github.URL), WithExecutable(binary), WithPlatform(linux))
		upgrader.files.createTemp = func(dir, _ string) (*os.File, error) { return nil, permissionDenied("open", dir) }

		_, err := upgrader.Install(context.Background(), latest(t, upgrader), nil)

		require.ErrorIs(t, err, ErrPermission)
		assert.EqualError(t, err, "cannot replace "+binary+": permission denied")
		assert.Equal(t, "old", read(t, binary))
	})

	t.Run("refuses to rename over a binary the user may not", func(t *testing.T) {
		t.Parallel()

		github := upgradetest.New(t, binaryRelease(t, "0.3.0", linux, []byte("new")))
		binary := installed(t, "tarhche", []byte("old"), 0o755)

		upgrader := New(WithAPI(github.URL), WithExecutable(binary), WithPlatform(linux))
		upgrader.files.rename = func(from, to string) error { return permissionDenied("rename", to) }

		_, err := upgrader.Install(context.Background(), latest(t, upgrader), nil)

		require.ErrorIs(t, err, ErrPermission)
		assert.Equal(t, "old", read(t, binary))
		assert.Empty(t, leftovers(t, filepath.Dir(binary)))
	})

	t.Run("fails when the binary cannot be staged", func(t *testing.T) {
		t.Parallel()

		github := upgradetest.New(t, binaryRelease(t, "0.3.0", linux, []byte("new")))
		binary := installed(t, "tarhche", []byte("old"), 0o755)

		upgrader := New(WithAPI(github.URL), WithExecutable(binary), WithPlatform(linux))

		created := 0
		upgrader.files.createTemp = func(dir, pattern string) (*os.File, error) {
			if created++; created > 1 {
				return nil, permissionDenied("open", dir)
			}

			return os.CreateTemp(dir, pattern)
		}

		_, err := upgrader.Install(context.Background(), latest(t, upgrader), nil)

		require.ErrorIs(t, err, ErrPermission)
		assert.Equal(t, "old", read(t, binary))
		assert.Empty(t, leftovers(t, filepath.Dir(binary)))
	})

	t.Run("refuses a release that holds no build for the platform", func(t *testing.T) {
		t.Parallel()

		github := upgradetest.New(t, binaryRelease(t, "0.3.0", linux, []byte("new")))
		arm := Platform{OS: "linux", Arch: "arm", Arm: "7"}
		binary := installed(t, "tarhche", []byte("old"), 0o755)
		upgrader := New(WithAPI(github.URL), WithExecutable(binary), WithPlatform(arm))

		_, err := upgrader.Install(context.Background(), latest(t, upgrader), nil)

		require.ErrorIs(t, err, ErrNoBuild)
		assert.EqualError(t, err, "the release holds no build of tarhche for this platform (linux/armv7): it holds no tarhche_0.3.0_linux_armv7.tar.gz")
	})

	t.Run("refuses a release that lists no checksums", func(t *testing.T) {
		t.Parallel()

		release := binaryRelease(t, "0.3.0", linux, []byte("new"))
		delete(release.Files, "tarhche_0.3.0_checksums.txt")

		github := upgradetest.New(t, release)
		binary := installed(t, "tarhche", []byte("old"), 0o755)
		upgrader := New(WithAPI(github.URL), WithExecutable(binary), WithPlatform(linux))

		_, err := upgrader.Install(context.Background(), latest(t, upgrader), nil)

		require.ErrorIs(t, err, ErrChecksum)
		assert.EqualError(t, err, "the archive does not match the release's checksum: the release lists no checksums")
	})

	t.Run("refuses an archive the checksums do not list", func(t *testing.T) {
		t.Parallel()

		release := binaryRelease(t, "0.3.0", linux, []byte("new"))
		release.Files["tarhche_0.3.0_checksums.txt"] = []byte("abc  tarhche_0.3.0_darwin_arm64.tar.gz\n")

		github := upgradetest.New(t, release)
		binary := installed(t, "tarhche", []byte("old"), 0o755)
		upgrader := New(WithAPI(github.URL), WithExecutable(binary), WithPlatform(linux))

		_, err := upgrader.Install(context.Background(), latest(t, upgrader), nil)

		require.ErrorIs(t, err, ErrChecksum)
		assert.EqualError(t, err, "the archive does not match the release's checksum: its checksums list no tarhche_0.3.0_linux_amd64.tar.gz")
	})

	t.Run("refuses an archive that does not match its checksum", func(t *testing.T) {
		t.Parallel()

		release := binaryRelease(t, "0.3.0", linux, []byte("new"))
		release.Files["tarhche_0.3.0_checksums.txt"] = []byte(strings.Repeat("0", 64) + "  tarhche_0.3.0_linux_amd64.tar.gz\n")

		github := upgradetest.New(t, release)
		binary := installed(t, "tarhche", []byte("old"), 0o755)
		upgrader := New(WithAPI(github.URL), WithExecutable(binary), WithPlatform(linux))

		_, err := upgrader.Install(context.Background(), latest(t, upgrader), nil)

		require.ErrorIs(t, err, ErrChecksum)
		assert.Contains(t, err.Error(), "tarhche_0.3.0_linux_amd64.tar.gz's SHA-256 is ")
		assert.Equal(t, "old", read(t, binary))
		assert.Empty(t, leftovers(t, filepath.Dir(binary)))
	})

	t.Run("refuses an archive that holds no binary", func(t *testing.T) {
		t.Parallel()

		archive := upgradetest.TarGz(t, map[string]upgradetest.File{"bin/tarhche": {Body: []byte("nested"), Mode: 0o755}})
		release := upgradetest.Release{Version: "0.3.0", Files: upgradetest.Checksummed("0.3.0", map[string][]byte{"tarhche_0.3.0_linux_amd64.tar.gz": archive})}

		github := upgradetest.New(t, release)
		binary := installed(t, "tarhche", []byte("old"), 0o755)
		upgrader := New(WithAPI(github.URL), WithExecutable(binary), WithPlatform(linux))

		_, err := upgrader.Install(context.Background(), latest(t, upgrader), nil)

		require.ErrorIs(t, err, ErrNoBuild)
		assert.EqualError(t, err, "the release holds no build of tarhche for this platform: tarhche_0.3.0_linux_amd64.tar.gz holds no tarhche")
		assert.Equal(t, "old", read(t, binary))
		assert.Empty(t, leftovers(t, filepath.Dir(binary)))
	})

	t.Run("refuses an archive larger than a build may be", func(t *testing.T) {
		t.Parallel()

		github := upgradetest.New(t, binaryRelease(t, "0.3.0", linux, []byte("new")))
		binary := installed(t, "tarhche", []byte("old"), 0o755)
		upgrader := New(WithAPI(github.URL), WithExecutable(binary), WithPlatform(linux))
		upgrader.limit = 16

		_, err := upgrader.Install(context.Background(), latest(t, upgrader), nil)

		assert.EqualError(t, err, "tarhche_0.3.0_linux_amd64.tar.gz is larger than a build of tarhche may be")
		assert.Empty(t, leftovers(t, filepath.Dir(binary)))
	})

	t.Run("refuses a binary larger than a build may be", func(t *testing.T) {
		t.Parallel()

		github := upgradetest.New(t, binaryRelease(t, "0.3.0", linux, make([]byte, 64<<10)))
		binary := installed(t, "tarhche", []byte("old"), 0o755)
		upgrader := New(WithAPI(github.URL), WithExecutable(binary), WithPlatform(linux))
		upgrader.limit = 32 << 10

		_, err := upgrader.Install(context.Background(), latest(t, upgrader), nil)

		assert.EqualError(t, err, "extracting tarhche from tarhche_0.3.0_linux_amd64.tar.gz: it is larger than a build of tarhche may be")
		assert.Equal(t, "old", read(t, binary))
		assert.Empty(t, leftovers(t, filepath.Dir(binary)))
	})

	t.Run("refuses a download that goes on past what a build may be", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		server := httptest.NewServer(mux)
		t.Cleanup(server.Close)

		mux.HandleFunc("/archive", func(w http.ResponseWriter, _ *http.Request) {
			for range 64 {
				_, _ = w.Write(make([]byte, 1024))
				w.(http.Flusher).Flush()
			}
		})

		binary := installed(t, "tarhche", []byte("old"), 0o755)
		upgrader := New(WithExecutable(binary), WithPlatform(linux))
		upgrader.limit = 16 << 10

		archive := Asset{Name: "tarhche_0.3.0_linux_amd64.tar.gz", URL: server.URL + "/archive"}

		var totals []int64

		_, err := upgrader.download(context.Background(), archive, "sum", target{path: binary}, func(_, total int64) {
			totals = append(totals, total)
		})

		assert.EqualError(t, err, "tarhche_0.3.0_linux_amd64.tar.gz is larger than a build of tarhche may be")
		assert.Contains(t, totals, int64(0))
		assert.Empty(t, leftovers(t, filepath.Dir(binary)))
	})

	t.Run("fails when the download breaks off", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Length", "4096")
			_, _ = w.Write([]byte("part of it"))
		}))
		t.Cleanup(server.Close)

		binary := installed(t, "tarhche", []byte("old"), 0o755)
		archive := Asset{Name: "tarhche_0.3.0_linux_amd64.tar.gz", URL: server.URL}

		_, err := New(WithExecutable(binary), WithPlatform(linux)).download(context.Background(), archive, "sum", target{path: binary}, nil)

		assert.ErrorContains(t, err, "downloading tarhche_0.3.0_linux_amd64.tar.gz: unexpected EOF")
		assert.Empty(t, leftovers(t, filepath.Dir(binary)))
	})

	t.Run("fails when the archive cannot be downloaded", func(t *testing.T) {
		t.Parallel()

		github := upgradetest.New(t, binaryRelease(t, "0.3.0", linux, []byte("new")))
		binary := installed(t, "tarhche", []byte("old"), 0o755)
		upgrader := New(WithAPI(github.URL), WithExecutable(binary), WithPlatform(linux))

		release := latest(t, upgrader)
		release.Assets[1].URL = github.URL + "/files/v0.3.0/gone.tar.gz"

		_, err := upgrader.Install(context.Background(), release, nil)

		assert.EqualError(t, err, "downloading tarhche_0.3.0_linux_amd64.tar.gz: GitHub answered 404 Not Found")
		assert.Empty(t, leftovers(t, filepath.Dir(binary)))
	})

	t.Run("fails when the checksums cannot be downloaded", func(t *testing.T) {
		t.Parallel()

		github := upgradetest.New(t, binaryRelease(t, "0.3.0", linux, []byte("new")))
		binary := installed(t, "tarhche", []byte("old"), 0o755)
		upgrader := New(WithAPI(github.URL), WithExecutable(binary), WithPlatform(linux))

		release := latest(t, upgrader)
		release.Assets[0].URL = "http://\x7f"

		_, err := upgrader.Install(context.Background(), release, nil)

		assert.ErrorContains(t, err, "downloading tarhche_0.3.0_checksums.txt")
	})

	t.Run("stops when it is canceled", func(t *testing.T) {
		t.Parallel()

		github := upgradetest.New(t, binaryRelease(t, "0.3.0", linux, []byte("new")))
		binary := installed(t, "tarhche", []byte("old"), 0o755)
		upgrader := New(WithAPI(github.URL), WithExecutable(binary), WithPlatform(linux))
		release := latest(t, upgrader)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := upgrader.Install(ctx, release, nil)

		require.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, "old", read(t, binary))
	})

	t.Run("fails for a binary that is not there", func(t *testing.T) {
		t.Parallel()

		_, err := New(WithExecutable(filepath.Join(t.TempDir(), "missing"))).Install(context.Background(), Release{Version: "0.3.0"}, nil)

		assert.ErrorContains(t, err, "finding the running binary")
	})

	t.Run("replaces the app bundle a macOS binary is run from", func(t *testing.T) {
		t.Parallel()

		github := upgradetest.New(t, appRelease(t, "0.3.0", app("new")))
		bundle, executable := installedApp(t)
		upgrader := New(WithAPI(github.URL), WithExecutable(executable), WithPlatform(macOS))

		installation, err := upgrader.Install(context.Background(), latest(t, upgrader), nil)
		require.NoError(t, err)

		assert.Equal(t, Installation{Version: "0.3.0", Path: bundle, Executable: executable}, installation)
		assert.Equal(t, "new", read(t, executable))
		assert.Equal(t, "<plist/>", read(t, filepath.Join(bundle, "Contents", "Info.plist")))
		assert.NoFileExists(t, filepath.Join(bundle, "Contents", "Resources", "old.txt"))

		link, err := os.Readlink(filepath.Join(bundle, "Contents", "Resources", "current"))
		require.NoError(t, err)
		assert.Equal(t, "icons.icns", link)

		assert.Equal(t, []string{"Tarhche.app"}, beside(t, bundle))
	})

	t.Run("makes the bundle's binary runnable", func(t *testing.T) {
		t.Parallel()

		files := app("new")
		files["Tarhche.app/Contents/MacOS/tarhche"] = upgradetest.File{Body: []byte("new"), Mode: 0o644}

		github := upgradetest.New(t, appRelease(t, "0.3.0", files))
		_, executable := installedApp(t)
		upgrader := New(WithAPI(github.URL), WithExecutable(executable), WithPlatform(macOS))

		_, err := upgrader.Install(context.Background(), latest(t, upgrader), nil)
		require.NoError(t, err)

		info, err := os.Stat(executable)
		require.NoError(t, err)
		assert.Equal(t, fs.FileMode(0o755), info.Mode().Perm())
	})

	t.Run("puts the app bundle back when the new one cannot take its place", func(t *testing.T) {
		t.Parallel()

		github := upgradetest.New(t, appRelease(t, "0.3.0", app("new")))
		bundle, executable := installedApp(t)

		upgrader := New(WithAPI(github.URL), WithExecutable(executable), WithPlatform(macOS))
		upgrader.files.rename = func(from, to string) error {
			if filepath.Base(from) == appBundle && to == bundle {
				return errors.New("busy")
			}

			return os.Rename(from, to)
		}

		_, err := upgrader.Install(context.Background(), latest(t, upgrader), nil)

		assert.EqualError(t, err, "replacing "+bundle+": busy")
		assert.Equal(t, "old", read(t, executable))
		assert.FileExists(t, filepath.Join(bundle, "Contents", "Resources", "old.txt"))
		assert.Equal(t, []string{"Tarhche.app"}, beside(t, bundle))
	})

	t.Run("keeps the app bundle where it was put aside when it cannot be put back", func(t *testing.T) {
		t.Parallel()

		github := upgradetest.New(t, appRelease(t, "0.3.0", app("new")))
		bundle, executable := installedApp(t)

		upgrader := New(WithAPI(github.URL), WithExecutable(executable), WithPlatform(macOS))
		upgrader.files.rename = func(from, to string) error {
			if to == bundle {
				return errors.New("busy")
			}

			return os.Rename(from, to)
		}

		_, err := upgrader.Install(context.Background(), latest(t, upgrader), nil)

		left := leftovers(t, filepath.Dir(bundle))
		require.Len(t, left, 1)

		aside := filepath.Join(filepath.Dir(bundle), left[0], "replaced.app")
		assert.EqualError(t, err, "replacing "+bundle+": busy\nputting "+bundle+" back from "+aside+": busy")
		assert.Equal(t, "old", read(t, filepath.Join(aside, "Contents", "MacOS", "tarhche")))
	})

	t.Run("refuses to put aside an app bundle the user may not", func(t *testing.T) {
		t.Parallel()

		github := upgradetest.New(t, appRelease(t, "0.3.0", app("new")))
		bundle, executable := installedApp(t)

		upgrader := New(WithAPI(github.URL), WithExecutable(executable), WithPlatform(macOS))
		upgrader.files.rename = func(from, to string) error { return permissionDenied("rename", from) }

		_, err := upgrader.Install(context.Background(), latest(t, upgrader), nil)

		require.ErrorIs(t, err, ErrPermission)
		assert.EqualError(t, err, "cannot replace "+bundle+": permission denied")
		assert.Equal(t, "old", read(t, executable))
		assert.Equal(t, []string{"Tarhche.app"}, beside(t, bundle))
	})

	t.Run("refuses to extract beside an app bundle the user may not", func(t *testing.T) {
		t.Parallel()

		github := upgradetest.New(t, appRelease(t, "0.3.0", app("new")))
		bundle, executable := installedApp(t)

		upgrader := New(WithAPI(github.URL), WithExecutable(executable), WithPlatform(macOS))
		upgrader.files.mkdirTemp = func(dir, _ string) (string, error) { return "", permissionDenied("mkdir", dir) }

		_, err := upgrader.Install(context.Background(), latest(t, upgrader), nil)

		require.ErrorIs(t, err, ErrPermission)
		assert.Equal(t, []string{"Tarhche.app"}, beside(t, bundle))
	})

	t.Run("refuses an app bundle that holds no binary", func(t *testing.T) {
		t.Parallel()

		files := app("new")
		delete(files, "Tarhche.app/Contents/MacOS/tarhche")

		github := upgradetest.New(t, appRelease(t, "0.3.0", files))
		bundle, executable := installedApp(t)
		upgrader := New(WithAPI(github.URL), WithExecutable(executable), WithPlatform(macOS))

		_, err := upgrader.Install(context.Background(), latest(t, upgrader), nil)

		require.ErrorIs(t, err, ErrNoBuild)
		assert.EqualError(t, err, "the release holds no build of tarhche for this platform: tarhche_0.3.0_darwin_app.zip holds no Tarhche.app/Contents/MacOS/tarhche")
		assert.Equal(t, "old", read(t, executable))
		assert.Equal(t, []string{"Tarhche.app"}, beside(t, bundle))
	})

	t.Run("refuses an app bundle whose binary is no file", func(t *testing.T) {
		t.Parallel()

		files := app("new")
		files["Tarhche.app/Contents/MacOS/tarhche"] = upgradetest.File{Body: []byte("../Info.plist"), Mode: fs.ModeSymlink | 0o777}

		github := upgradetest.New(t, appRelease(t, "0.3.0", files))
		bundle, executable := installedApp(t)
		upgrader := New(WithAPI(github.URL), WithExecutable(executable), WithPlatform(macOS))

		_, err := upgrader.Install(context.Background(), latest(t, upgrader), nil)

		require.Error(t, err)
		assert.Equal(t, "old", read(t, executable))
		assert.Equal(t, []string{"Tarhche.app"}, beside(t, bundle))
	})

	t.Run("refuses an app bundle whose archive escapes", func(t *testing.T) {
		t.Parallel()

		files := app("new")
		files["Tarhche.app/../evil"] = upgradetest.File{Body: []byte("evil")}

		github := upgradetest.New(t, appRelease(t, "0.3.0", files))
		bundle, executable := installedApp(t)
		upgrader := New(WithAPI(github.URL), WithExecutable(executable), WithPlatform(macOS))

		_, err := upgrader.Install(context.Background(), latest(t, upgrader), nil)

		assert.EqualError(t, err, "extracting Tarhche.app from tarhche_0.3.0_darwin_app.zip: Tarhche.app/../evil would land out of where the bundle is extracted")
		assert.Equal(t, "old", read(t, executable))
		assert.Equal(t, []string{"Tarhche.app"}, beside(t, bundle))
	})
}
